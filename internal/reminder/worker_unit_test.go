package reminder

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"
)

type fakeQueue struct {
	due  []Reminder
	sent []string
}

func (f *fakeQueue) Claim(context.Context, time.Time, int, time.Duration) ([]Reminder, error) {
	out := f.due
	f.due = nil
	return out, nil
}

func (f *fakeQueue) MarkSent(_ context.Context, id string, _ time.Time) error {
	f.sent = append(f.sent, id)
	return nil
}

type funcNotifier func(ctx context.Context, userID, title, body string) error

func (f funcNotifier) Notify(ctx context.Context, userID, title, body string) error {
	return f(ctx, userID, title, body)
}

func runOnce(q Queue, n Notifier) {
	NewWorker(q, n, slog.New(slog.DiscardHandler)).tick(context.Background())
}

func TestDeliveryFailureKeepsReminderForRetry(t *testing.T) {
	q := &fakeQueue{due: []Reminder{{ID: "r1", Attempts: 1}}}
	runOnce(q, funcNotifier(func(context.Context, string, string, string) error { return errors.New("fora do ar") }))
	if len(q.sent) != 0 {
		t.Error("com falha e tentativas sobrando, o lembrete deve voltar à fila, não ser concluído")
	}
}

func TestDeliveryGivesUpAfterMaxAttempts(t *testing.T) {
	q := &fakeQueue{due: []Reminder{{ID: "r1", Attempts: maxAttempts}}}
	runOnce(q, funcNotifier(func(context.Context, string, string, string) error { return errors.New("fora do ar") }))
	if len(q.sent) != 1 {
		t.Error("esgotadas as tentativas, o lembrete deve ser concluído para não repetir para sempre")
	}
}

func TestDeliverySuccessMarksSent(t *testing.T) {
	q := &fakeQueue{due: []Reminder{{ID: "r1", Attempts: 1, CreatedBy: "a", RecipientID: "b", CreatedByName: "Ana"}}}
	var title string
	runOnce(q, funcNotifier(func(_ context.Context, _, tt, _ string) error { title = tt; return nil }))
	if len(q.sent) != 1 || title != "Lembrete de Ana" {
		t.Errorf("sent=%v title=%q", q.sent, title)
	}
}

func TestSelfReminderTitle(t *testing.T) {
	q := &fakeQueue{due: []Reminder{{ID: "r1", Attempts: 1, CreatedBy: "a", RecipientID: "a"}}}
	var title string
	runOnce(q, funcNotifier(func(_ context.Context, _, tt, _ string) error { title = tt; return nil }))
	if title != "Seu lembrete" {
		t.Errorf("título inesperado: %q", title)
	}
}
