package ops

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"casal/internal/db"
	"casal/internal/identity"
)

// retention é por quanto tempo os eventos e os dias de atividade ficam: o bastante para comparar com o mesmo
// período do ano seguinte.
const retention = 13 * 30 * 24 * time.Hour

// Events grava os eventos de produto (product_events). Só nomes: a assinatura nem aceita texto livre.
type Events struct {
	db  *sql.DB
	log *slog.Logger
}

func NewEvents(conn *sql.DB, log *slog.Logger) *Events { return &Events{db: conn, log: log} }

// Record grava o evento fora da transação do negócio e nunca falha a operação: o erro só vai para o log.
// Sem userID, usa a pessoa da requisição, se houver.
func (e *Events) Record(ctx context.Context, name, userID, householdID string) {
	if userID == "" {
		if p, ok := identity.From(ctx); ok {
			userID = p.UserID
		}
	}
	// o evento não pode morrer junto com a requisição que o originou (o cliente pode ter desconectado)
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	err := db.InTx(ctx, e.db, db.Scope{System: true}, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO product_events (name, user_id, household_id) VALUES ($1, $2, $3)`,
			name, nullable(userID), nullable(householdID))
		return err
	})
	if err != nil {
		e.log.Warn("não consegui gravar o evento de produto", "event", name, "err", err)
	}
}

// Purge apaga eventos e dias de atividade com mais de 13 meses.
func (e *Events) Purge(ctx context.Context) (int64, error) {
	var total int64
	cutoff := time.Now().Add(-retention)
	err := db.InTx(ctx, e.db, db.Scope{System: true}, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `DELETE FROM product_events WHERE occurred_at < $1`, cutoff)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		total += n
		res, err = tx.ExecContext(ctx, `DELETE FROM user_activity WHERE day < $1::date`, cutoff)
		if err != nil {
			return err
		}
		n, _ = res.RowsAffected()
		total += n
		return nil
	})
	return total, err
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}
