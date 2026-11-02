package note

import (
	"context"
	"testing"

	"casal/internal/db"
	"casal/internal/testutil"
)

type row struct {
	owner, household, visibility string
	version                      int
}

func TestHandoffOnLeaveAndJoin(t *testing.T) {
	conn := testutil.OpenDB(t)
	ctx := context.Background()
	ana := testutil.NewUser(t, conn, "ana")
	beto := testutil.NewUser(t, conn, "beto")
	carlos := testutil.NewUser(t, conn, "carlos")
	hh := testutil.NewHousehold(t, conn, ana, beto)
	hh2 := testutil.NewHousehold(t, conn, carlos)
	repo := NewPostgresRepo(conn)

	mk := func(owner, home, title, visibility string) Note {
		t.Helper()
		n, err := repo.Create(ctx, newNote(owner, home, title, visibility))
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	shared := mk(ana, hh, "do casal da Ana", VisibilityHousehold)
	view := mk(ana, hh, "só leitura da Ana", VisibilityView)
	private := mk(ana, hh, "privada da Ana", VisibilityPrivate)
	betos := mk(beto, hh, "do Beto", VisibilityHousehold)
	carlos1 := mk(carlos, hh2, "do Carlos", VisibilityHousehold)

	get := func(id string) row {
		t.Helper()
		var r row
		testutil.SystemScan(t, conn, `SELECT owner_id, household_id, visibility, version FROM notes WHERE id = $1`,
			[]any{id}, &r.owner, &r.household, &r.visibility, &r.version)
		return r
	}

	// --- a Ana sai: o que era do casal fica com o Beto; o que o Beto lia deixa de ser compartilhado
	testutil.RunHook(t, conn, repo.HandleLeave, db.Handoff{UserID: ana, From: hh, RemainingUserID: beto})

	if got := get(shared.ID); got.owner != beto || got.visibility != VisibilityHousehold || got.household != hh {
		t.Errorf("a nota do casal deveria passar para o Beto: %+v", got)
	}
	if got := get(view.ID); got.owner != ana || got.visibility != VisibilityPrivate {
		t.Errorf("a nota que o Beto lia deveria virar privada e continuar da Ana: %+v", got)
	}
	if got := get(private.ID); got.owner != ana || got.visibility != VisibilityPrivate || got.version != private.Version {
		t.Errorf("a nota privada não muda: %+v", got)
	}
	if got := get(betos.ID); got.owner != beto || got.version != betos.Version {
		t.Errorf("a nota do Beto não muda: %+v", got)
	}
	// depois da saída, o Beto não vê o que ficou com a Ana, e vê o que passou para ele
	list, err := repo.List(ctx, hh, beto, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Errorf("o Beto deveria ver 2 notas (a dele e a que herdou): %v", titles(list))
	}

	// --- a Ana entra em outro casal: as notas dela vêm junto, sempre privadas
	testutil.RunHook(t, conn, repo.HandleJoin, db.Handoff{UserID: ana, To: hh2})

	for _, n := range []Note{view, private} {
		if got := get(n.ID); got.owner != ana || got.household != hh2 || got.visibility != VisibilityPrivate {
			t.Errorf("a nota %q deveria vir para o novo casal, privada: %+v", n.Title, got)
		}
	}
	if got := get(shared.ID); got.household != hh || got.owner != beto {
		t.Errorf("a nota que passou para o Beto não pode acompanhar a Ana: %+v", got)
	}
	if got := get(carlos1.ID); got.household != hh2 || got.owner != carlos || got.version != carlos1.Version {
		t.Errorf("o que já era do novo casal não muda: %+v", got)
	}
	// o novo parceiro não vê as notas que chegaram (privadas)
	list, err = repo.List(ctx, hh2, carlos, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != carlos1.ID {
		t.Errorf("o Carlos só deveria ver a própria nota: %v", titles(list))
	}
	// e a Ana vê as dela
	list, err = repo.List(ctx, hh2, ana, "", "")
	if err != nil || len(list) != 3 {
		t.Errorf("a Ana deveria ver as 2 dela mais a do Carlos (do casal): %v %v", titles(list), err)
	}
}

// Quem sai sozinho não tem a quem passar: as notas do casal não mudam de dono, e a adoção as torna privadas.
func TestHandoffLeavingAloneKeepsOwnershipAndJoinMakesThemPrivate(t *testing.T) {
	conn := testutil.OpenDB(t)
	ctx := context.Background()
	ana := testutil.NewUser(t, conn, "ana")
	carlos := testutil.NewUser(t, conn, "carlos")
	hh := testutil.NewHousehold(t, conn, ana)
	hh2 := testutil.NewHousehold(t, conn, carlos)
	repo := NewPostgresRepo(conn)

	n, err := repo.Create(ctx, newNote(ana, hh, "do casal", VisibilityHousehold))
	if err != nil {
		t.Fatal(err)
	}
	testutil.RunHook(t, conn, repo.HandleLeave, db.Handoff{UserID: ana, From: hh})
	var owner, visibility string
	testutil.SystemScan(t, conn, `SELECT owner_id, visibility FROM notes WHERE id = $1`, []any{n.ID}, &owner, &visibility)
	if owner != ana || visibility != VisibilityHousehold {
		t.Errorf("sem ninguém para herdar, a nota fica como está: %s %s", owner, visibility)
	}

	testutil.RunHook(t, conn, repo.HandleJoin, db.Handoff{UserID: ana, To: hh2})
	list, err := repo.List(ctx, hh2, carlos, "", "")
	if err != nil || len(list) != 0 {
		t.Errorf("a nota que era do casal não pode aparecer ao novo parceiro: %v %v", titles(list), err)
	}
}
