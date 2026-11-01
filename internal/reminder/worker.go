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

// Queue é o que o agendador precisa do banco: reservar os vencidos e concluir cada um.
type Queue interface {
	Claim(ctx context.Context, now time.Time, limit int, lease time.Duration) ([]Reminder, error)
	MarkSent(ctx context.Context, id string, now time.Time) error
}

const (
	claimLease  = time.Minute      // se o processo cair, o lembrete volta à fila depois deste prazo
	sendTimeout = 10 * time.Second // limite para o aviso de cada lembrete
	maxAttempts = 3                // depois disso, o lembrete é dado como enviado (continua no app)
)

type Worker struct {
	queue    Queue
	notifier Notifier
	log      *slog.Logger
	interval time.Duration
	batch    int
	now      func() time.Time
}

func NewWorker(queue Queue, notifier Notifier, log *slog.Logger) *Worker {
	return &Worker{queue: queue, notifier: notifier, log: log, interval: 15 * time.Second, batch: 50, now: time.Now}
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
		due, err := w.queue.Claim(ctx, w.now(), w.batch, claimLease)
		if err != nil {
			w.log.Error("falha ao reservar lembretes", "err", err)
			return
		}
		for _, r := range due {
			w.deliver(ctx, r)
		}
		if len(due) < w.batch {
			return // não há mais vencidos agora
		}
	}
}

// deliver avisa a pessoa fora de qualquer transação. Se o aviso falhar, o lembrete
// fica reservado e volta à fila quando o prazo acabar; após maxAttempts tentativas
// ele é dado como enviado, para não repetir para sempre (segue visível no app).
func (w *Worker) deliver(ctx context.Context, r Reminder) {
	title := "Lembrete de " + r.CreatedByName
	if r.CreatedBy == r.RecipientID {
		title = "Seu lembrete"
	}

	sendCtx, cancel := context.WithTimeout(ctx, sendTimeout)
	err := w.notifier.Notify(sendCtx, r.RecipientID, title, r.Body)
	cancel()

	if err != nil {
		w.log.Warn("aviso não enviado", "reminder", r.ID, "attempt", r.Attempts, "err", err)
		if r.Attempts < maxAttempts {
			return
		}
	}
	if err := w.queue.MarkSent(ctx, r.ID, w.now()); err != nil {
		w.log.Error("falha ao concluir lembrete", "reminder", r.ID, "err", err)
	}
}
