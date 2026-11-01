package reminder

import (
	"context"
	"log/slog"
	"time"
)

// Notifier envia o aviso a todos os aparelhos de uma pessoa.
// A feature push implementa esta interface.
type Notifier interface {
	Notify(ctx context.Context, userID, title, body string) error
}

type Worker struct {
	repo     Repository
	notifier Notifier
	log      *slog.Logger
	interval time.Duration
	batch    int
	now      func() time.Time
}

func NewWorker(repo Repository, notifier Notifier, log *slog.Logger) *Worker {
	return &Worker{repo: repo, notifier: notifier, log: log, interval: 15 * time.Second, batch: 50, now: time.Now}
}

// Run bloqueia até o contexto ser cancelado. Rode com: go worker.Run(ctx)
func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		w.tick(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (w *Worker) tick(ctx context.Context) {
	for ctx.Err() == nil {
		n, err := w.repo.ProcessDue(ctx, w.now(), w.batch, w.send)
		if err != nil {
			w.log.Error("falha ao processar lembretes", "err", err)
			return
		}
		if n < w.batch {
			return // não há mais vencidos agora
		}
	}
}

// send nunca devolve erro de envio: uma falha de push não deve travar o lembrete
// (ele continua visível no app). Pendência: registrar tentativas e reenviar com espera.
func (w *Worker) send(ctx context.Context, r Reminder) error {
	title := "Lembrete de " + r.CreatedByName
	if r.CreatedBy == r.RecipientID {
		title = "Seu lembrete"
	}
	if err := w.notifier.Notify(ctx, r.RecipientID, title, r.Body); err != nil {
		w.log.Warn("aviso não enviado", "reminder", r.ID, "err", err)
	}
	return nil
}
