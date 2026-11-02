package routine

import (
	"context"
	"testing"

	"casal/internal/db"
	"casal/internal/testutil"
)

func TestHandoffOnLeaveAndJoin(t *testing.T) {
	conn := testutil.OpenDB(t)
	ctx := context.Background()
	ana := testutil.NewUser(t, conn, "ana")
	beto := testutil.NewUser(t, conn, "beto")
	carlos := testutil.NewUser(t, conn, "carlos")
	hh := testutil.NewHousehold(t, conn, ana, beto)
	hh2 := testutil.NewHousehold(t, conn, carlos)
	repo := NewPostgresRepo(conn)

	mk := func(owner, home, title, visibility string) Block {
		t.Helper()
		b, err := repo.Create(ctx, newBlock(owner, home, title, visibility))
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	shared := mk(ana, hh, "jantar do casal", VisibilityHousehold)
	busy := mk(ana, hh, "ocupado da Ana", VisibilityBusy)
	view := mk(ana, hh, "detalhes da Ana", VisibilityView)
	private := mk(ana, hh, "privado da Ana", VisibilityPrivate)
	betos := mk(beto, hh, "do Beto", VisibilityView)
	carlos1 := mk(carlos, hh2, "do Carlos", VisibilityHousehold)

	// uma exceção num bloco da Ana: precisa acompanhá-la
	if err := repo.SetException(ctx, hh, ana, Exception{BlockID: private.ID, Date: day("2026-11-03"), Skipped: true}); err != nil {
		t.Fatal(err)
	}

	type row struct {
		owner, household, visibility string
		version                      int
	}
	get := func(id string) row {
		t.Helper()
		var r row
		testutil.SystemScan(t, conn, `SELECT owner_id, household_id, visibility, version FROM routine_blocks WHERE id = $1`,
			[]any{id}, &r.owner, &r.household, &r.visibility, &r.version)
		return r
	}

	// --- a Ana sai: o bloco do casal fica com o Beto; o que ele via deixa de ser compartilhado
	testutil.RunHook(t, conn, repo.HandleLeave, db.Handoff{UserID: ana, From: hh, RemainingUserID: beto})

	if got := get(shared.ID); got.owner != beto || got.visibility != VisibilityHousehold || got.household != hh {
		t.Errorf("o bloco do casal deveria passar para o Beto: %+v", got)
	}
	for _, b := range []Block{busy, view, private} {
		if got := get(b.ID); got.owner != ana || got.visibility != VisibilityPrivate {
			t.Errorf("%q deveria virar privado e continuar da Ana: %+v", b.Title, got)
		}
	}
	if got := get(betos.ID); got.owner != beto || got.visibility != VisibilityView || got.version != betos.Version {
		t.Errorf("o bloco do Beto não muda: %+v", got)
	}
	list, err := repo.ListVisible(ctx, hh, beto)
	if err != nil || len(list) != 2 {
		t.Errorf("o Beto deveria ver só o dele e o que herdou: %v %v", titles(list), err)
	}

	// --- a Ana entra em outro casal: os blocos e as exceções vêm junto, sempre privados
	testutil.RunHook(t, conn, repo.HandleJoin, db.Handoff{UserID: ana, To: hh2})

	for _, b := range []Block{busy, view, private} {
		if got := get(b.ID); got.owner != ana || got.household != hh2 || got.visibility != VisibilityPrivate {
			t.Errorf("%q deveria vir para o novo casal, privado: %+v", b.Title, got)
		}
	}
	if got := get(shared.ID); got.household != hh || got.owner != beto {
		t.Errorf("o bloco que passou para o Beto não acompanha a Ana: %+v", got)
	}
	if got := get(carlos1.ID); got.household != hh2 || got.version != carlos1.Version {
		t.Errorf("o que já era do novo casal não muda: %+v", got)
	}
	exc, err := repo.ListExceptions(ctx, hh2, ana, day("2026-11-01"), day("2026-11-30"))
	if err != nil || len(exc) != 1 || exc[0].BlockID != private.ID {
		t.Errorf("a exceção deveria acompanhar o bloco: %+v %v", exc, err)
	}
	var excHousehold string
	testutil.SystemScan(t, conn, `SELECT household_id FROM routine_exceptions WHERE block_id = $1`, []any{private.ID}, &excHousehold)
	if excHousehold != hh2 {
		t.Errorf("household_id da exceção deveria seguir o do bloco: %v", excHousehold)
	}
	// o novo parceiro não vê o que chegou
	list, err = repo.ListVisible(ctx, hh2, carlos)
	if err != nil || len(list) != 1 || list[0].ID != carlos1.ID {
		t.Errorf("o Carlos só deveria ver o próprio bloco: %v %v", titles(list), err)
	}
}
