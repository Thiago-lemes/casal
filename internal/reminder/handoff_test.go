package reminder

import (
	"context"
	"testing"
	"time"

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
	at := time.Now().Add(time.Hour)

	mk := func(creator, recipient, body string) Reminder {
		t.Helper()
		r, err := repo.Create(ctx, Reminder{HouseholdID: hh, CreatedBy: creator, RecipientID: recipient, Body: body, RemindAt: at})
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	toBeto := mk(ana, beto, "da Ana para o Beto")
	toAna := mk(beto, ana, "do Beto para a Ana")
	self := mk(ana, ana, "da Ana para ela mesma")
	betoSelf := mk(beto, beto, "do Beto para ele mesmo")

	status := func(id string) (st, household string) {
		t.Helper()
		testutil.SystemScan(t, conn, `SELECT status, household_id FROM reminders WHERE id = $1`, []any{id}, &st, &household)
		return
	}

	// --- a Ana sai: o que ia de um para o outro é cancelado; o de cada um para si fica
	testutil.RunHook(t, conn, repo.HandleLeave, db.Handoff{UserID: ana, From: hh, RemainingUserID: beto})
	for _, r := range []Reminder{toBeto, toAna} {
		if st, _ := status(r.ID); st != "canceled" {
			t.Errorf("%q deveria ser cancelado, está %s", r.Body, st)
		}
	}
	for _, r := range []Reminder{self, betoSelf} {
		if st, _ := status(r.ID); st != "scheduled" {
			t.Errorf("%q deveria continuar agendado, está %s", r.Body, st)
		}
	}

	// --- a Ana entra em outro casal: o lembrete dela para si vem junto; o do Beto não
	testutil.RunHook(t, conn, repo.HandleJoin, db.Handoff{UserID: ana, To: hh2})
	if _, h := status(self.ID); h != hh2 {
		t.Errorf("o lembrete da Ana para si deveria vir para o novo casal: %s", h)
	}
	if _, h := status(betoSelf.ID); h != hh {
		t.Errorf("o do Beto não pode mudar de casal: %s", h)
	}
	if _, h := status(toBeto.ID); h != hh {
		t.Errorf("o lembrete entre os dois (cancelado) fica no casal antigo: %s", h)
	}
	// o novo casal enxerga o lembrete que chegou, e o Carlos não vê lembretes dela
	list, err := repo.List(ctx, hh2, ana)
	if err != nil || len(list) != 1 || list[0].ID != self.ID {
		t.Errorf("a Ana deveria ver o lembrete dela no novo casal: %+v %v", list, err)
	}
	if list, err := repo.List(ctx, hh2, carlos); err != nil || len(list) != 0 {
		t.Errorf("o Carlos não participa desse lembrete: %+v %v", list, err)
	}
}
