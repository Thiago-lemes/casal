package shopping

import (
	"context"
	"errors"
	"testing"
	"time"

	"casal/internal/testutil"
)

func item(hh, user, name string, q *float64, unit, category string) Item {
	return Item{HouseholdID: hh, AddedBy: user, Name: name, Quantity: q, Unit: unit, Category: category, Source: SourceManual}
}

func names(its []Item) []string {
	out := []string{}
	for _, it := range its {
		out = append(out, it.Name)
	}
	return out
}

func TestHouseholdsAreIsolated(t *testing.T) {
	db := testutil.OpenDB(t)
	ctx := context.Background()

	userA := testutil.NewUser(t, db, "a")
	userB := testutil.NewUser(t, db, "b")
	hhA := testutil.NewHousehold(t, db, userA)
	hhB := testutil.NewHousehold(t, db, userB)
	repo := NewPostgresRepo(db)

	created, _, err := repo.Create(ctx, item(hhA, userA, "leite", nil, "", "grocery"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()

	// item recorrente já comprado no casal A: é o que um "repor" do casal B poderia reabrir por engano
	staple := item(hhA, userA, "café", nil, "", "grocery")
	staple.Recurring = true
	stapleA, _, err := repo.Create(ctx, staple)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.SetBought(ctx, hhA, stapleA.ID, userA, true, now.Add(-48*time.Hour)); err != nil {
		t.Fatal(err)
	}

	list, err := repo.List(ctx, hhB, now.Add(-time.Hour))
	if err != nil || len(list) != 0 {
		t.Errorf("o casal B não deveria ver itens do A: %v %v", names(list), err)
	}
	if _, err := repo.Update(ctx, hhB, Item{ID: created.ID, Name: "invadido", Category: "other"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("B editou um item de A: %v", err)
	}
	if _, err := repo.SetBought(ctx, hhB, created.ID, userB, true, now); !errors.Is(err, ErrNotFound) {
		t.Errorf("B comprou um item de A: %v", err)
	}
	if err := repo.Delete(ctx, hhB, created.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("B apagou um item de A: %v", err)
	}
	if n, err := repo.Restock(ctx, hhB); err != nil || n != 0 {
		t.Errorf("repor no casal B não pode mexer no A: %d %v", n, err)
	}
	if listA, err := repo.List(ctx, hhA, now.Add(-72*time.Hour)); err != nil {
		t.Fatal(err)
	} else {
		for _, it := range listA {
			if it.ID == stapleA.ID && it.BoughtAt == nil {
				t.Error("o casal B reabriu o item recorrente do casal A")
			}
		}
	}

	// o mesmo item nos dois casais não se soma: cada casal tem a sua lista
	other, merged, err := repo.Create(ctx, item(hhB, userB, "leite", nil, "", "grocery"))
	if err != nil || merged || other.ID == created.ID {
		t.Errorf("itens iguais de casais diferentes são independentes: %+v %v %v", other, merged, err)
	}
}

func TestEqualOpenItemsMergeAndSumQuantity(t *testing.T) {
	db := testutil.OpenDB(t)
	ctx := context.Background()
	ana := testutil.NewUser(t, db, "ana")
	hh := testutil.NewHousehold(t, db, ana)
	repo := NewPostgresRepo(db)

	first, merged, err := repo.Create(ctx, item(hh, ana, "Tomate", qty(2), "kg", "produce"))
	if err != nil || merged {
		t.Fatalf("o primeiro não soma: %v %v", merged, err)
	}
	// mesmo nome (sem diferenciar maiúsculas e espaços), unidade e categoria: soma
	second, merged, err := repo.Create(ctx, item(hh, ana, "  tomate ", qty(1.5), "kg", "produce"))
	if err != nil || !merged || second.ID != first.ID || second.Quantity == nil || *second.Quantity != 3.5 {
		t.Errorf("deveria somar no mesmo item: %+v merged=%v %v", second, merged, err)
	}
	// unidade diferente: outro item
	other, merged, err := repo.Create(ctx, item(hh, ana, "tomate", qty(3), "un", "produce"))
	if err != nil || merged || other.ID == first.ID {
		t.Errorf("unidade diferente não soma: %+v %v %v", other, merged, err)
	}
	// sem quantidade nos dois: continua sem quantidade
	a, _, _ := repo.Create(ctx, item(hh, ana, "pão", nil, "", "grocery"))
	b, merged, err := repo.Create(ctx, item(hh, ana, "pão", nil, "", "grocery"))
	if err != nil || !merged || b.ID != a.ID || b.Quantity != nil {
		t.Errorf("nulo com nulo continua nulo: %+v %v %v", b, merged, err)
	}
	// com e sem quantidade: soma o que há
	c, merged, err := repo.Create(ctx, item(hh, ana, "pão", qty(2), "", "grocery"))
	if err != nil || !merged || c.Quantity == nil || *c.Quantity != 2 {
		t.Errorf("nulo com 2 dá 2: %+v %v %v", c, merged, err)
	}
	// o que já foi comprado não recebe soma: o novo vira outro item aberto
	if _, err := repo.SetBought(ctx, hh, first.ID, ana, true, time.Now()); err != nil {
		t.Fatal(err)
	}
	again, merged, err := repo.Create(ctx, item(hh, ana, "tomate", qty(1), "kg", "produce"))
	if err != nil || merged || again.ID == first.ID {
		t.Errorf("comprado não soma, abre outro: %+v %v %v", again, merged, err)
	}
}

func TestConcurrentCreatesNeverDuplicate(t *testing.T) {
	db := testutil.OpenDB(t)
	ctx := context.Background()
	ana := testutil.NewUser(t, db, "ana")
	hh := testutil.NewHousehold(t, db, ana)
	repo := NewPostgresRepo(db)

	const n = 8
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		go func() {
			_, _, err := repo.Create(ctx, item(hh, ana, "ovos", qty(1), "un", "grocery"))
			errs <- err
		}()
	}
	for i := 0; i < n; i++ {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	list, err := repo.List(ctx, hh, time.Now().Add(-time.Hour))
	if err != nil || len(list) != 1 || *list[0].Quantity != n {
		t.Errorf("8 inclusões simultâneas deveriam virar 1 item com quantidade 8: %v %v", list, err)
	}
}

func TestBoughtWindowUndoAndDuplicates(t *testing.T) {
	db := testutil.OpenDB(t)
	ctx := context.Background()
	ana := testutil.NewUser(t, db, "ana")
	beto := testutil.NewUser(t, db, "beto")
	hh := testutil.NewHousehold(t, db, ana, beto)
	repo := NewPostgresRepo(db)

	now := time.Now()
	open, _, _ := repo.Create(ctx, item(hh, ana, "arroz", nil, "", "grocery"))
	fresh, _, _ := repo.Create(ctx, item(hh, ana, "feijão", nil, "", "grocery"))
	old, _, _ := repo.Create(ctx, item(hh, ana, "sal", nil, "", "grocery"))

	// qualquer um dos dois compra
	got, err := repo.SetBought(ctx, hh, fresh.ID, beto, true, now.Add(-time.Hour))
	if err != nil || got.BoughtAt == nil || got.BoughtBy == nil || *got.BoughtBy != beto {
		t.Fatalf("o Beto deveria poder comprar: %+v %v", got, err)
	}
	if _, err := repo.SetBought(ctx, hh, old.ID, ana, true, now.Add(-48*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.SetBought(ctx, hh, fresh.ID, ana, true, now); !errors.Is(err, ErrNotFound) {
		t.Errorf("comprar de novo um item já comprado: %v", err)
	}

	// abertos e o comprado na última hora aparecem; o de 48h atrás não
	list, err := repo.List(ctx, hh, now.Add(-24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].Name != "arroz" || list[1].Name != "feijão" {
		t.Errorf("abertos primeiro, depois o comprado recente; sem o antigo: veio %v", names(list))
	}
	if list[0].ID != open.ID || list[1].BoughtAt == nil {
		t.Errorf("ordem ou estado errados: %+v", list)
	}

	// desfazer a compra
	back, err := repo.SetBought(ctx, hh, fresh.ID, ana, false, now)
	if err != nil || back.BoughtAt != nil || back.BoughtBy != nil {
		t.Errorf("desfazer deveria reabrir o item: %+v %v", back, err)
	}
	if _, err := repo.SetBought(ctx, hh, fresh.ID, ana, false, now); !errors.Is(err, ErrNotFound) {
		t.Errorf("desfazer um item aberto: %v", err)
	}

	// com um igual já aberto, desfazer a compra não pode duplicar
	if _, err := repo.SetBought(ctx, hh, open.ID, ana, true, now); err != nil {
		t.Fatal(err)
	}
	if _, _, err := repo.Create(ctx, item(hh, ana, "arroz", nil, "", "grocery")); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.SetBought(ctx, hh, open.ID, ana, false, now); !errors.Is(err, ErrDuplicate) {
		t.Errorf("desfazer com um igual aberto deveria dar ErrDuplicate: %v", err)
	}
}

func TestUpdateAndDelete(t *testing.T) {
	db := testutil.OpenDB(t)
	ctx := context.Background()
	ana := testutil.NewUser(t, db, "ana")
	hh := testutil.NewHousehold(t, db, ana)
	repo := NewPostgresRepo(db)

	a, _, _ := repo.Create(ctx, item(hh, ana, "leite", qty(1), "l", "grocery"))
	b, _, _ := repo.Create(ctx, item(hh, ana, "suco", nil, "", "grocery"))
	got, err := repo.Update(ctx, hh, Item{ID: a.ID, Name: "leite integral", Quantity: qty(6), Unit: "l", Category: "grocery", Recurring: true})
	if err != nil || got.Name != "leite integral" || *got.Quantity != 6 || !got.Recurring {
		t.Errorf("edição mal gravada: %+v %v", got, err)
	}
	// renomear para um nome que já está aberto não duplica
	if _, err := repo.Update(ctx, hh, Item{ID: b.ID, Name: "Leite Integral", Quantity: qty(6), Unit: "l", Category: "grocery"}); !errors.Is(err, ErrDuplicate) {
		t.Errorf("renomear para um item aberto igual deveria dar ErrDuplicate: %v", err)
	}
	if err := repo.Delete(ctx, hh, a.ID); err != nil {
		t.Fatal(err)
	}
	if err := repo.Delete(ctx, hh, a.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("apagar de novo: %v", err)
	}
}

func TestRestockBringsBackOnlyRecurringBought(t *testing.T) {
	db := testutil.OpenDB(t)
	ctx := context.Background()
	ana := testutil.NewUser(t, db, "ana")
	hh := testutil.NewHousehold(t, db, ana)
	repo := NewPostgresRepo(db)
	now := time.Now()

	mk := func(name string, recurring, bought bool) Item {
		t.Helper()
		in := item(hh, ana, name, nil, "", "grocery")
		in.Recurring = recurring
		it, _, err := repo.Create(ctx, in)
		if err != nil {
			t.Fatal(err)
		}
		if bought {
			if it, err = repo.SetBought(ctx, hh, it.ID, ana, true, now.Add(-48*time.Hour)); err != nil {
				t.Fatal(err)
			}
		}
		return it
	}
	mk("leite", true, true)  // recorrente e comprado: volta
	mk("café", true, false)  // recorrente, mas já está na lista: nada a repor
	mk("sabão", false, true) // comprado mas não recorrente: não volta
	mk("ovos", true, true)   // recorrente e comprado: volta
	mk("ovos", true, true)   // o mesmo "ovos" comprado outra vez: só um volta, para não duplicar

	n, err := repo.Restock(ctx, hh)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("deveriam voltar leite e um ovos, voltaram %d", n)
	}
	list, _ := repo.List(ctx, hh, now.Add(-time.Hour))
	got := map[string]int{}
	for _, it := range list {
		if it.BoughtAt == nil {
			got[it.Name]++
			if it.Name != "café" && it.Source != SourceRecurring {
				t.Errorf("%s voltou com source %q, esperava recurring", it.Name, it.Source)
			}
		}
	}
	if got["leite"] != 1 || got["ovos"] != 1 || got["café"] != 1 || got["sabão"] != 0 || len(got) != 3 {
		t.Errorf("abertos depois de repor: %v", got)
	}

	// repor de novo não faz nada: já está tudo na lista
	if n, err := repo.Restock(ctx, hh); err != nil || n != 0 {
		t.Errorf("segunda reposição deveria ser vazia: %d %v", n, err)
	}
}

func TestPurgeBoughtKeepsRecurring(t *testing.T) {
	db := testutil.OpenDB(t)
	ctx := context.Background()
	ana := testutil.NewUser(t, db, "ana")
	hh := testutil.NewHousehold(t, db, ana)
	repo := NewPostgresRepo(db)
	now := time.Now()

	mk := func(name string, recurring bool, boughtAgo time.Duration) {
		t.Helper()
		in := item(hh, ana, name, nil, "", "other")
		in.Recurring = recurring
		it, _, err := repo.Create(ctx, in)
		if err != nil {
			t.Fatal(err)
		}
		if boughtAgo > 0 {
			if _, err = repo.SetBought(ctx, hh, it.ID, ana, true, now.Add(-boughtAgo)); err != nil {
				t.Fatal(err)
			}
		}
	}
	mk("velho comum", false, 40*24*time.Hour)
	mk("velho recorrente", true, 40*24*time.Hour)
	mk("recente", false, 2*24*time.Hour)
	mk("aberto", false, 0)

	if _, err := repo.PurgeBought(ctx, now.Add(-30*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	left := testutil.SystemStrings(t, db, `SELECT name FROM shopping_items WHERE household_id = $1 ORDER BY name`, hh)
	want := []string{"aberto", "recente", "velho recorrente"}
	if len(left) != len(want) {
		t.Fatalf("sobraram %v, queria %v", left, want)
	}
	for i := range want {
		if left[i] != want[i] {
			t.Errorf("sobraram %v, queria %v", left, want)
		}
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
	if _, _, err := repo.Create(ctx, item(hhA, userA, "do A", nil, "", "other")); err != nil {
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
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM shopping_items WHERE added_by = $1`, userA).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("o RLS deveria esconder o item do casal A, mas %d linha(s) apareceram", n)
	}
}
