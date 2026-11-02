package module

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"casal/internal/testutil"
)

func TestHouseholdsHaveIndependentModules(t *testing.T) {
	db := testutil.OpenDB(t)
	ctx := context.Background()
	userA := testutil.NewUser(t, db, "a")
	userB := testutil.NewUser(t, db, "b")
	hhA := testutil.NewHousehold(t, db, userA)
	hhB := testutil.NewHousehold(t, db, userB)
	repo := NewPostgresRepo(db)

	if changed, err := repo.Enable(ctx, hhA, Finance); err != nil || !changed {
		t.Fatalf("ligar no casal A: %v %v", changed, err)
	}
	if on, err := repo.IsEnabled(ctx, hhB, Finance); err != nil || on {
		t.Errorf("ligar no A não liga no B: %v %v", on, err)
	}
	if got, err := repo.Enabled(ctx, hhB); err != nil || len(got) != 0 {
		t.Errorf("o casal B não deveria ver módulos do A: %v %v", got, err)
	}
	// desligar no B não mexe no A
	if err := repo.Disable(ctx, hhB, Finance); err != nil {
		t.Fatal(err)
	}
	if on, _ := repo.IsEnabled(ctx, hhA, Finance); !on {
		t.Error("desligar no casal B desligou o do A")
	}
}

func TestConcurrentEnableRunsHookOnce(t *testing.T) {
	db := testutil.OpenDB(t)
	ctx := context.Background()
	ana := testutil.NewUser(t, db, "ana")
	hh := testutil.NewHousehold(t, db, ana)
	repo := NewPostgresRepo(db)

	var ran atomic.Int32
	hook := func(context.Context, *sql.Tx, string) error { ran.Add(1); return nil }

	const n = 6
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := repo.Enable(ctx, hh, Finance, hook)
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if ran.Load() != 1 {
		t.Errorf("o gancho (categorias padrão) deveria rodar uma vez, rodou %d", ran.Load())
	}
}

func TestFailingHookRollsBackEnable(t *testing.T) {
	db := testutil.OpenDB(t)
	ctx := context.Background()
	ana := testutil.NewUser(t, db, "ana")
	hh := testutil.NewHousehold(t, db, ana)
	repo := NewPostgresRepo(db)

	boom := errors.New("falhou")
	_, err := repo.Enable(ctx, hh, Finance, func(context.Context, *sql.Tx, string) error { return boom })
	if !errors.Is(err, boom) {
		t.Fatalf("esperava o erro do gancho: %v", err)
	}
	if on, _ := repo.IsEnabled(ctx, hh, Finance); on {
		t.Error("o módulo ficou ligado apesar do gancho ter falhado")
	}
}

// Com o papel casal_app (RLS ligado), o banco barra o casal errado mesmo sem o WHERE household_id.
func TestRLSBlocksOtherHouseholdWithoutWhere(t *testing.T) {
	db := testutil.OpenDB(t)
	ctx := context.Background()

	var rls bool
	if err := db.QueryRow(`SELECT NOT rolbypassrls AND NOT rolsuper FROM pg_roles WHERE rolname = current_user`).Scan(&rls); err != nil || !rls {
		t.Skip("TEST_DATABASE_URL precisa usar o papel casal_app para testar o RLS")
	}

	userA := testutil.NewUser(t, db, "rls-a")
	hhA := testutil.NewHousehold(t, db, userA)
	hhB := testutil.NewHousehold(t, db)
	if _, err := NewPostgresRepo(db).Enable(ctx, hhA, Finance); err != nil {
		t.Fatal(err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.household_id', $1, true)`, hhB); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM household_modules WHERE household_id = $1`, hhA).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("o RLS deveria esconder o módulo do casal A, mas %d linha(s) apareceram", n)
	}
}
