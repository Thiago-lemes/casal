package reminder

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"casal/internal/testutil"
)

func (f *fakeRepo) CreateWithRef(_ context.Context, r Reminder, ref string) (Reminder, error) {
	r.ID = "r-" + ref
	r.Status = "scheduled"
	f.created = append(f.created, r)
	return r, nil
}

func TestCreateWithRefValidation(t *testing.T) {
	cases := []struct {
		name      string
		membersOK bool
		body, ref string
		at        time.Time
	}{
		{"texto vazio", true, " ", "bill:1", fixedNow},
		{"sem horário", true, "ok", "bill:1", time.Time{}},
		{"sem ref", true, "ok", "", fixedNow},
		{"ref longo demais", true, "ok", strings.Repeat("r", 101), fixedNow},
		{"fora do casal", false, "ok", "bill:1", fixedNow},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, repo := newTestService(c.membersOK)
			_, err := svc.CreateWithRef(context.Background(), "h1", "u1", "u2", c.body, c.at, c.ref)
			if !errors.Is(err, ErrInvalidInput) || len(repo.created) != 0 {
				t.Errorf("esperava ErrInvalidInput sem gravar: %v (gravou %d)", err, len(repo.created))
			}
		})
	}
}

// Quem chama decide se o horário já passou: o lembrete de uma conta que vence amanhã, "3 dias antes", é aceito.
func TestCreateWithRefAcceptsPastTimeAndDefaultsRecipient(t *testing.T) {
	svc, repo := newTestService(true)
	r, err := svc.CreateWithRef(context.Background(), "h1", "u1", "", "luz vence", fixedNow.Add(-48*time.Hour), "bill:1:d3")
	if err != nil || r.RecipientID != "u1" || len(repo.created) != 1 {
		t.Errorf("resultado inesperado: %+v %v", r, err)
	}
}

func refReminder(hh, user, body string, at time.Time) Reminder {
	return Reminder{HouseholdID: hh, CreatedBy: user, RecipientID: user, Body: body, RemindAt: at}
}

func TestRefLifecycle(t *testing.T) {
	db := testutil.OpenDB(t)
	ctx := context.Background()
	ana := testutil.NewUser(t, db, "ana")
	hh := testutil.NewHousehold(t, db, ana)
	repo := NewPostgresRepo(db)
	at := time.Now().Add(time.Hour)

	first, err := repo.CreateWithRef(ctx, refReminder(hh, ana, "luz vence em 3 dias", at), "bill:1:d3")
	if err != nil {
		t.Fatal(err)
	}
	// criar de novo com o ref ativo devolve o mesmo lembrete, sem duplicar
	again, err := repo.CreateWithRef(ctx, refReminder(hh, ana, "outro texto", at), "bill:1:d3")
	if err != nil || again.ID != first.ID || again.Body != first.Body {
		t.Errorf("com o ref ativo, deveria devolver o existente: %+v %v", again, err)
	}
	if _, err := repo.CreateWithRef(ctx, refReminder(hh, ana, "luz vence hoje", at), "bill:1:d0"); err != nil {
		t.Fatal(err)
	}

	if n, err := repo.CancelByRef(ctx, hh, "bill:1:d3"); err != nil || n != 1 {
		t.Fatalf("cancelar por ref: %d %v", n, err)
	}
	if n, _ := repo.CancelByRef(ctx, hh, "bill:1:d3"); n != 0 {
		t.Errorf("cancelar de novo não encontra nada, cancelou %d", n)
	}
	list, _ := repo.List(ctx, hh, ana)
	if len(list) != 1 || list[0].Body != "luz vence hoje" {
		t.Errorf("sobra só o outro ref pendente: %+v", list)
	}

	// o ref cancelado fica livre: refazer cria um lembrete novo
	redo, err := repo.CreateWithRef(ctx, refReminder(hh, ana, "luz vence em 3 dias (nova data)", at), "bill:1:d3")
	if err != nil || redo.ID == first.ID {
		t.Errorf("depois de cancelar, o ref deveria criar um novo: %+v %v", redo, err)
	}
}

func TestRefIsPerHousehold(t *testing.T) {
	db := testutil.OpenDB(t)
	ctx := context.Background()
	userA := testutil.NewUser(t, db, "a")
	userB := testutil.NewUser(t, db, "b")
	hhA := testutil.NewHousehold(t, db, userA)
	hhB := testutil.NewHousehold(t, db, userB)
	repo := NewPostgresRepo(db)
	at := time.Now().Add(time.Hour)

	a, err := repo.CreateWithRef(ctx, refReminder(hhA, userA, "do A", at), "bill:1")
	if err != nil {
		t.Fatal(err)
	}
	b, err := repo.CreateWithRef(ctx, refReminder(hhB, userB, "do B", at), "bill:1")
	if err != nil || b.ID == a.ID {
		t.Errorf("o mesmo ref em casais diferentes são lembretes diferentes: %+v %v", b, err)
	}
	if n, err := repo.CancelByRef(ctx, hhB, "bill:1"); err != nil || n != 1 {
		t.Fatalf("cancelar no B: %d %v", n, err)
	}
	list, _ := repo.List(ctx, hhA, userA)
	if len(list) != 1 || list[0].ID != a.ID {
		t.Errorf("cancelar por ref no casal B não pode tocar o do A: %+v", list)
	}
}
