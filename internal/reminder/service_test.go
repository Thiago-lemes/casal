package reminder

import (
	"context"
	"errors"
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
	r.Status = "scheduled"
	f.created = append(f.created, r)
	return r, nil
}

type fakeMembers struct{ ok bool }

func (f fakeMembers) AreMembers(context.Context, string, ...string) (bool, error) { return f.ok, nil }

var fixedNow = time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)

func newTestService(membersOK bool) (*Service, *fakeRepo) {
	repo := &fakeRepo{}
	svc := NewService(repo, fakeMembers{ok: membersOK})
	svc.now = func() time.Time { return fixedNow }
	return svc, repo
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
