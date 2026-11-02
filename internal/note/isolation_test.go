package note

import (
	"context"
	"errors"
	"testing"

	"casal/internal/testutil"
)

func newNote(owner, hh, title, visibility string) Note {
	return Note{HouseholdID: hh, OwnerID: owner, Title: title, Kind: KindText, Body: "texto", Visibility: visibility}
}

func titles(ns []Note) []string {
	out := []string{}
	for _, n := range ns {
		out = append(out, n.Title)
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

	// nota do casal: a mais exposta, pois qualquer membro do casal A a lê e edita
	created, err := repo.Create(ctx, newNote(userA, hhA, "segredo do casal A", VisibilityHousehold))
	if err != nil {
		t.Fatal(err)
	}

	list, err := repo.List(ctx, hhB, userB, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Errorf("o casal B não deveria ver notas do A, viu %v", titles(list))
	}
	if _, err := repo.Get(ctx, hhB, created.ID, userB); !errors.Is(err, ErrNotFound) {
		t.Errorf("B leu uma nota de A: %v", err)
	}
	if _, err := repo.Update(ctx, hhB, userB, created, created.Version); !errors.Is(err, ErrNotFound) {
		t.Errorf("B editou uma nota do casal A: %v", err)
	}
	if err := repo.Delete(ctx, hhB, created.ID, userB); !errors.Is(err, ErrNotFound) {
		t.Errorf("B apagou uma nota de A: %v", err)
	}
}

// Dentro do mesmo casal, a visibilidade decide o que cada pessoa lê e altera.
// Esta é a trava que só existe no código (o RLS isola casais, não parceiros).
func TestVisibilityInsideHousehold(t *testing.T) {
	db := testutil.OpenDB(t)
	ctx := context.Background()

	ana := testutil.NewUser(t, db, "ana")
	beto := testutil.NewUser(t, db, "beto")
	hh := testutil.NewHousehold(t, db, ana, beto)
	repo := NewPostgresRepo(db)

	private, err := repo.Create(ctx, newNote(ana, hh, "privada", VisibilityPrivate))
	if err != nil {
		t.Fatal(err)
	}
	view, err := repo.Create(ctx, newNote(ana, hh, "só leitura", VisibilityView))
	if err != nil {
		t.Fatal(err)
	}
	shared, err := repo.Create(ctx, newNote(ana, hh, "do casal", VisibilityHousehold))
	if err != nil {
		t.Fatal(err)
	}

	// leitura: o Beto não vê a privada
	if _, err := repo.Get(ctx, hh, private.ID, beto); !errors.Is(err, ErrNotFound) {
		t.Errorf("Beto leu a nota privada da Ana: %v", err)
	}
	list, err := repo.List(ctx, hh, beto, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Errorf("Beto deveria ver 2 notas (view e household), viu %v", titles(list))
	}
	for _, n := range list {
		if n.ID == private.ID {
			t.Error("a nota privada apareceu na lista do Beto")
		}
	}
	all, err := repo.List(ctx, hh, ana, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Errorf("Ana deveria ver as 3, viu %v", titles(all))
	}

	// a busca também respeita a visibilidade
	found, err := repo.List(ctx, hh, beto, "privada", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 0 {
		t.Errorf("a busca do Beto vazou a nota privada: %v", titles(found))
	}

	// escrita: só o dono edita a privada e a de leitura; a do casal, os dois
	for _, n := range []Note{private, view} {
		if _, err := repo.Update(ctx, hh, beto, n, n.Version); !errors.Is(err, ErrNotFound) {
			t.Errorf("Beto editou %q: %v", n.Title, err)
		}
	}
	edited := shared
	edited.Body = "o Beto escreveu"
	got, err := repo.Update(ctx, hh, beto, edited, shared.Version)
	if err != nil {
		t.Fatalf("Beto deveria editar a nota do casal: %v", err)
	}
	if got.Body != "o Beto escreveu" || got.Version != shared.Version+1 {
		t.Errorf("edição mal gravada: %+v", got)
	}

	// o Beto não consegue esconder a nota do casal, mesmo mandando outra visibilidade
	edited = got
	edited.Visibility = VisibilityPrivate
	got, err = repo.Update(ctx, hh, beto, edited, got.Version)
	if err != nil {
		t.Fatal(err)
	}
	if got.Visibility != VisibilityHousehold {
		t.Errorf("o parceiro mudou a visibilidade para %q", got.Visibility)
	}

	// só o dono apaga
	for _, n := range []Note{view, shared} {
		if err := repo.Delete(ctx, hh, n.ID, beto); !errors.Is(err, ErrNotFound) {
			t.Errorf("Beto apagou %q: %v", n.Title, err)
		}
	}
	if err := repo.Delete(ctx, hh, private.ID, ana); err != nil {
		t.Errorf("a Ana deveria apagar a própria nota: %v", err)
	}
}

func TestUpdateWithStaleVersionWritesNothing(t *testing.T) {
	db := testutil.OpenDB(t)
	ctx := context.Background()

	ana := testutil.NewUser(t, db, "ana")
	hh := testutil.NewHousehold(t, db, ana)
	repo := NewPostgresRepo(db)

	n, err := repo.Create(ctx, newNote(ana, hh, "original", VisibilityPrivate))
	if err != nil {
		t.Fatal(err)
	}
	n.Title = "primeira edição"
	if _, err := repo.Update(ctx, hh, ana, n, n.Version); err != nil {
		t.Fatal(err)
	}

	n.Title = "edição com versão velha"
	if _, err := repo.Update(ctx, hh, ana, n, n.Version); !errors.Is(err, ErrNotFound) {
		t.Errorf("versão velha deveria falhar: %v", err)
	}
	cur, err := repo.Get(ctx, hh, n.ID, ana)
	if err != nil {
		t.Fatal(err)
	}
	if cur.Title != "primeira edição" || cur.Version != 2 {
		t.Errorf("a edição velha alterou a nota: %+v", cur)
	}
}

func TestSearchAndTags(t *testing.T) {
	db := testutil.OpenDB(t)
	ctx := context.Background()

	ana := testutil.NewUser(t, db, "ana")
	hh := testutil.NewHousehold(t, db, ana)
	repo := NewPostgresRepo(db)
	ins := func(n Note) {
		t.Helper()
		if _, err := repo.Create(ctx, n); err != nil {
			t.Fatal(err)
		}
	}

	a := newNote(ana, hh, "Ideias de viagem", VisibilityPrivate)
	a.Tags = []string{"viagem"}
	ins(a)
	b := newNote(ana, hh, "Mercado", VisibilityPrivate)
	b.Kind, b.Body = KindChecklist, ""
	b.Items = []Item{{Text: "leite integral"}, {Text: "100% suco"}}
	b.Tags = []string{"casa"}
	b.Pinned = true
	ins(b)

	cases := []struct {
		name, q, tag string
		want         []string
	}{
		{"sem filtro: fixada primeiro", "", "", []string{"Mercado", "Ideias de viagem"}},
		{"busca no título, sem diferenciar maiúsculas", "VIAGEM", "", []string{"Ideias de viagem"}},
		{"busca dentro dos itens da checklist", "leite", "", []string{"Mercado"}},
		{"% na busca é texto comum", "100%", "", []string{"Mercado"}},
		{"% sozinho não casa tudo", "%", "", []string{"Mercado"}},
		{"filtra por etiqueta", "", "casa", []string{"Mercado"}},
		{"etiqueta inexistente", "", "outra", []string{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := repo.List(ctx, hh, ana, c.q, c.tag)
			if err != nil {
				t.Fatal(err)
			}
			g := titles(got)
			if len(g) != len(c.want) {
				t.Fatalf("veio %v, queria %v", g, c.want)
			}
			for i := range g {
				if g[i] != c.want[i] {
					t.Errorf("veio %v, queria %v", g, c.want)
				}
			}
		})
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
	if _, err := repo.Create(ctx, newNote(userA, hhA, "do A", VisibilityHousehold)); err != nil {
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
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM notes WHERE owner_id = $1`, userA).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("o RLS deveria esconder a nota do casal A, mas %d linha(s) apareceram", n)
	}
}
