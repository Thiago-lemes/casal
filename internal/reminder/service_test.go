package reminder

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"
)

type fakeRepo struct {
	Repository
	created []Reminder
}

func (f *fakeRepo) Create(_ context.Context, r Reminder) (Reminder, error) {
	r.ID = "r1"
	r.CreatedByName = "Ana"
	r.Status = "scheduled"
	f.created = append(f.created, r)
	return r, nil
}

type fakeMembers struct{ ok bool }

func (f fakeMembers) AreMembers(context.Context, string, ...string) (bool, error) { return f.ok, nil }

var fixedNow = time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)

type notified struct{ userID, title, body string }

type fakeNotifier struct {
	sent []notified
	err  error
}

func (f *fakeNotifier) Notify(_ context.Context, userID, title, body string) error {
	f.sent = append(f.sent, notified{userID, title, body})
	return f.err
}

func newTestService(membersOK bool) (*Service, *fakeRepo) {
	svc, repo, _ := newTestServiceWithNotifier(membersOK)
	return svc, repo
}

func newTestServiceWithNotifier(membersOK bool) (*Service, *fakeRepo, *fakeNotifier) {
	repo := &fakeRepo{}
	n := &fakeNotifier{}
	svc := NewService(repo, fakeMembers{ok: membersOK}, n, slog.New(slog.NewTextHandler(io.Discard, nil)))
	svc.now = func() time.Time { return fixedNow }
	return svc, repo, n
}

func TestCreateDefaultsRecipientToCreator(t *testing.T) {
	svc, repo := newTestService(true)
	r, err := svc.Create(context.Background(), "h1", "u1", "", "  comprar pão ", fixedNow.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if r.RecipientID != "u1" || r.Body != "comprar pão" || len(repo.created) != 1 {
		t.Errorf("resultado inesperado: %+v", r)
	}
}

func TestCreateValidation(t *testing.T) {
	cases := []struct {
		name      string
		membersOK bool
		body      string
		at        time.Time
	}{
		{"texto vazio", true, "   ", fixedNow.Add(time.Hour)},
		{"texto longo demais", true, strings.Repeat("a", 501), fixedNow.Add(time.Hour)},
		{"sem horário", true, "ok", time.Time{}},
		{"horário no passado", true, "ok", fixedNow.Add(-time.Hour)},
		{"destinatário de outro casal", false, "ok", fixedNow.Add(time.Hour)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, repo := newTestService(c.membersOK)
			_, err := svc.Create(context.Background(), "h1", "u1", "u2", c.body, c.at)
			if !errors.Is(err, ErrInvalidInput) {
				t.Errorf("esperava ErrInvalidInput, veio %v", err)
			}
			if len(repo.created) != 0 {
				t.Error("nada deveria ter sido gravado")
			}
		})
	}
}

func TestCreateAllowsAFewSecondsInThePast(t *testing.T) {
	svc, _ := newTestService(true)
	if _, err := svc.Create(context.Background(), "h1", "u1", "u1", "agora", fixedNow.Add(-30*time.Second)); err != nil {
		t.Errorf("um lembrete para \"agora\" deveria ser aceito: %v", err)
	}
}

func TestSnoozeLimits(t *testing.T) {
	svc, _ := newTestService(true)
	for _, minutes := range []int{0, -5, 1441} {
		if _, err := svc.Snooze(context.Background(), "h1", "r1", "u1", minutes); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%d minutos deveria ser inválido, veio %v", minutes, err)
		}
	}
}

func TestCreateNotifiesPartner(t *testing.T) {
	svc, _, n := newTestServiceWithNotifier(true)
	if _, err := svc.Create(context.Background(), "h", "ana", "bia", "buscar o pacote", fixedNow.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	want := notified{"bia", "Ana criou um lembrete para você", "buscar o pacote"}
	if len(n.sent) != 1 || n.sent[0] != want {
		t.Fatalf("avisos = %+v, quero só %+v", n.sent, want)
	}
}

func TestCreateForSelfDoesNotNotify(t *testing.T) {
	svc, _, n := newTestServiceWithNotifier(true)
	for _, recipient := range []string{"", "ana"} {
		if _, err := svc.Create(context.Background(), "h", "ana", recipient, "beber água", fixedNow.Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	if len(n.sent) != 0 {
		t.Fatalf("lembrete para si mesmo não deve avisar, mas avisou: %+v", n.sent)
	}
}

func TestCreateSucceedsWhenNotifyFails(t *testing.T) {
	svc, repo, n := newTestServiceWithNotifier(true)
	n.err = errors.New("push fora do ar")
	if _, err := svc.Create(context.Background(), "h", "ana", "bia", "x", fixedNow.Add(time.Hour)); err != nil {
		t.Fatalf("falha no aviso não pode falhar a criação: %v", err)
	}
	if len(repo.created) != 1 {
		t.Fatal("o lembrete deveria ter sido criado")
	}
}

func TestCreateRejectedDoesNotNotify(t *testing.T) {
	svc, _, n := newTestServiceWithNotifier(false)
	if _, err := svc.Create(context.Background(), "h", "ana", "bia", "x", fixedNow.Add(time.Hour)); err == nil {
		t.Fatal("esperava erro: a pessoa não é do casal")
	}
	if len(n.sent) != 0 {
		t.Fatal("não deve avisar quando a criação é recusada")
	}
}
