package task

import (
	"context"
	"testing"
	"time"

	"casal/internal/db"
	"casal/internal/testutil"
)

func TestHandoffOnLeavePassesOpenTasksToWhoStays(t *testing.T) {
	conn := testutil.OpenDB(t)
	ctx := context.Background()
	ana := testutil.NewUser(t, conn, "ana")
	beto := testutil.NewUser(t, conn, "beto")
	hh := testutil.NewHousehold(t, conn, ana, beto)
	repo := NewPostgresRepo(conn)

	mk := func(creator string, assignee *string, title string) Task {
		t.Helper()
		got, err := repo.Create(ctx, Task{HouseholdID: hh, CreatedBy: creator, AssigneeID: assignee, Title: title})
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	byAna := mk(ana, &beto, "a Ana pediu ao Beto")
	forAna := mk(beto, &ana, "o Beto pediu à Ana")
	ownAna := mk(ana, &ana, "da Ana para ela mesma")
	shared := mk(ana, nil, "do casal, criada pela Ana")
	betos := mk(beto, &beto, "do Beto para ele mesmo")

	// a Ana tinha feito um pedido ao Beto, que recusou; a Ana assume (vira dela) e depois sai
	declined := mk(ana, &beto, "recusada pelo Beto")
	if _, err := repo.Decline(ctx, hh, declined.ID, beto, "não dá"); err != nil {
		t.Fatal(err)
	}
	// uma já concluída fica como histórico
	finished := mk(ana, &ana, "feita pela Ana")
	if _, _, err := repo.Complete(ctx, hh, finished.ID, ana, time.Now(), nil); err != nil {
		t.Fatal(err)
	}

	testutil.RunHook(t, conn, repo.HandleLeave, db.Handoff{UserID: ana, From: hh, RemainingUserID: beto})

	type row struct {
		creator, status string
		assignee        *string
	}
	get := func(id string) row {
		t.Helper()
		var r row
		var a *string
		testutil.SystemScan(t, conn, `SELECT created_by, status, assignee_id FROM tasks WHERE id = $1`, []any{id}, &r.creator, &r.status, &a)
		r.assignee = a
		return r
	}
	if got := get(byAna.ID); got.creator != beto || *got.assignee != beto || got.status != StatusOpen {
		t.Errorf("o pedido da Ana ao Beto agora é do Beto para ele mesmo: %+v", got)
	}
	if got := get(forAna.ID); got.creator != beto || *got.assignee != beto {
		t.Errorf("o que era para a Ana passa para o Beto: %+v", got)
	}
	if got := get(ownAna.ID); got.creator != beto || *got.assignee != beto {
		t.Errorf("a tarefa da Ana para ela mesma passa para o Beto: %+v", got)
	}
	if got := get(shared.ID); got.creator != beto || got.assignee != nil {
		t.Errorf("a do casal continua do casal, agora criada pelo Beto: %+v", got)
	}
	if got := get(betos.ID); got.creator != beto || *got.assignee != beto {
		t.Errorf("a do Beto não muda: %+v", got)
	}
	if got := get(declined.ID); got.status != StatusOpen || got.creator != beto || *got.assignee != beto {
		t.Errorf("a recusada fica do Beto para ele mesmo e reabre (ninguém recusa a própria): %+v", got)
	}
	var reason *string
	testutil.SystemScan(t, conn, `SELECT decline_reason FROM tasks WHERE id = $1`, []any{declined.ID}, &reason)
	if reason != nil {
		t.Errorf("a justificativa da recusa deveria sair: %v", *reason)
	}
	if got := get(finished.ID); got.creator != ana || got.status != StatusDone {
		t.Errorf("a concluída fica como histórico: %+v", got)
	}
}

func TestHandoffLeavingAloneChangesNothing(t *testing.T) {
	conn := testutil.OpenDB(t)
	ctx := context.Background()
	ana := testutil.NewUser(t, conn, "ana")
	hh := testutil.NewHousehold(t, conn, ana)
	repo := NewPostgresRepo(conn)
	task, err := repo.Create(ctx, Task{HouseholdID: hh, CreatedBy: ana, AssigneeID: &ana, Title: "minha"})
	if err != nil {
		t.Fatal(err)
	}
	testutil.RunHook(t, conn, repo.HandleLeave, db.Handoff{UserID: ana, From: hh})
	var creator string
	testutil.SystemScan(t, conn, `SELECT created_by FROM tasks WHERE id = $1`, []any{task.ID}, &creator)
	if creator != ana {
		t.Errorf("sem ninguém que fique, nada muda: %v", creator)
	}
}
