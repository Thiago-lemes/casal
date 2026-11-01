package reminder

import (
	"context"
	"errors"
	"testing"
	"time"

	"casal/internal/testutil"
)

func TestHouseholdsAreIsolated(t *testing.T) {
	db := testutil.OpenDB(t)
	ctx := context.Background()

	userA := testutil.NewUser(t, db, "a")
	userB := testutil.NewUser(t, db, "b")
	hhA := testutil.NewHousehold(t, db, userA)
	hhB := testutil.NewHousehold(t, db, userB)
	repo := NewPostgresRepo(db)

	created, err := repo.Create(ctx, Reminder{
		HouseholdID: hhA, CreatedBy: userA, RecipientID: userA,
		Body: "segredo do casal A", RemindAt: time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}

	list, err := repo.List(ctx, hhB, userB)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Errorf("o casal B não deveria ver lembretes do A, viu %d", len(list))
	}

	if _, err := repo.Resolve(ctx, hhB, created.ID, userB, time.Now()); !errors.Is(err, ErrNotFound) {
		t.Errorf("B resolveu um lembrete de A: %v", err)
	}
	if _, err := repo.Resolve(ctx, hhA, created.ID, userB, time.Now()); !errors.Is(err, ErrNotFound) {
		t.Errorf("quem não é o destinatário resolveu: %v", err)
	}
	if err := repo.Cancel(ctx, hhB, created.ID, userB); !errors.Is(err, ErrNotFound) {
		t.Errorf("B cancelou um lembrete de A: %v", err)
	}

	if _, err := repo.Resolve(ctx, hhA, created.ID, userA, time.Now()); err != nil {
		t.Errorf("A deveria conseguir resolver o próprio lembrete: %v", err)
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
	repo := NewPostgresRepo(db)
	if _, err := repo.Create(ctx, Reminder{
		HouseholdID: hhA, CreatedBy: userA, RecipientID: userA,
		Body: "do A", RemindAt: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}

	// escopo do casal B, consulta SEM filtro de casal: não pode enxergar a linha do A
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.household_id', $1, true)`, hhB); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM reminders WHERE recipient_id = $1`, userA).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("o RLS deveria esconder o lembrete do casal A, mas %d linha(s) apareceram", n)
	}
}
