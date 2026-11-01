package reminder

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"casal/internal/db"
)

var (
	ErrInvalidInput = errors.New("dados inválidos")
	ErrNotFound     = errors.New("lembrete não encontrado")
)

type Reminder struct {
	ID            string     `json:"id"`
	HouseholdID   string     `json:"-"`
	CreatedBy     string     `json:"created_by"`
	CreatedByName string     `json:"created_by_name"`
	RecipientID   string     `json:"recipient_id"`
	Body          string     `json:"body"`
	RemindAt      time.Time  `json:"remind_at"`
	Status        string     `json:"status"`
	SentAt        *time.Time `json:"sent_at,omitempty"`
	ResolvedAt    *time.Time `json:"resolved_at,omitempty"`
}

type Repository interface {
	Create(ctx context.Context, r Reminder) (Reminder, error)
	List(ctx context.Context, householdID, userID string) ([]Reminder, error)
	Resolve(ctx context.Context, householdID, id, userID string, now time.Time) (Reminder, error)
	Snooze(ctx context.Context, householdID, id, userID string, remindAt time.Time) (Reminder, error)
	Cancel(ctx context.Context, householdID, id, userID string) error
	// ProcessDue pega até limit lembretes vencidos, chama fn para cada um e os marca como enviados.
	ProcessDue(ctx context.Context, now time.Time, limit int, fn func(context.Context, Reminder) error) (int, error)
}

type PostgresRepo struct{ db *sql.DB }

func NewPostgresRepo(db *sql.DB) *PostgresRepo { return &PostgresRepo{db: db} }

// Colunas lidas em todas as consultas. O JOIN em users é só leitura, para mostrar o nome.
const cols = `r.id, r.household_id, r.created_by, u.name, r.recipient_id, r.body,
	r.remind_at, r.status, r.sent_at, r.resolved_at`

type scanner interface{ Scan(dest ...any) error }

func scan(s scanner) (Reminder, error) {
	var r Reminder
	var sent, resolved sql.NullTime
	err := s.Scan(&r.ID, &r.HouseholdID, &r.CreatedBy, &r.CreatedByName, &r.RecipientID,
		&r.Body, &r.RemindAt, &r.Status, &sent, &resolved)
	if err != nil {
		return Reminder{}, err
	}
	if sent.Valid {
		r.SentAt = &sent.Time
	}
	if resolved.Valid {
		r.ResolvedAt = &resolved.Time
	}
	return r, nil
}

func (p *PostgresRepo) Create(ctx context.Context, in Reminder) (Reminder, error) {
	var out Reminder
	err := db.InTx(ctx, p.db, db.Scope{HouseholdID: in.HouseholdID}, func(tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx, `
			WITH ins AS (
				INSERT INTO reminders (household_id, created_by, recipient_id, body, remind_at)
				VALUES ($1, $2, $3, $4, $5)
				RETURNING *
			)
			SELECT `+cols+` FROM ins r JOIN users u ON u.id = r.created_by`,
			in.HouseholdID, in.CreatedBy, in.RecipientID, in.Body, in.RemindAt)
		var err error
		out, err = scan(row)
		return err
	})
	return out, err
}

// List devolve os lembretes que a pessoa recebeu ou criou, os mais recentes primeiro.
func (p *PostgresRepo) List(ctx context.Context, householdID, userID string) ([]Reminder, error) {
	out := []Reminder{}
	err := db.InTx(ctx, p.db, db.Scope{HouseholdID: householdID}, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
			SELECT `+cols+`
			FROM reminders r JOIN users u ON u.id = r.created_by
			WHERE r.household_id = $1
			  AND (r.recipient_id = $2 OR r.created_by = $2)
			  AND r.status <> 'canceled'
			ORDER BY r.remind_at DESC
			LIMIT 100`, householdID, userID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			r, err := scan(rows)
			if err != nil {
				return err
			}
			out = append(out, r)
		}
		return rows.Err()
	})
	return out, err
}

// Só o destinatário resolve. O household_id no WHERE e o RLS isolam um casal do outro.
func (p *PostgresRepo) Resolve(ctx context.Context, householdID, id, userID string, now time.Time) (Reminder, error) {
	return p.update(ctx, householdID, `
		WITH upd AS (
			UPDATE reminders SET status = 'resolved', resolved_at = $4
			WHERE id = $1 AND household_id = $2 AND recipient_id = $3
			  AND status IN ('scheduled', 'sent')
			RETURNING *
		)
		SELECT `+cols+` FROM upd r JOIN users u ON u.id = r.created_by`,
		id, householdID, userID, now)
}

func (p *PostgresRepo) Snooze(ctx context.Context, householdID, id, userID string, remindAt time.Time) (Reminder, error) {
	return p.update(ctx, householdID, `
		WITH upd AS (
			UPDATE reminders SET status = 'scheduled', remind_at = $4, sent_at = NULL
			WHERE id = $1 AND household_id = $2 AND recipient_id = $3
			  AND status IN ('scheduled', 'sent')
			RETURNING *
		)
		SELECT `+cols+` FROM upd r JOIN users u ON u.id = r.created_by`,
		id, householdID, userID, remindAt)
}

// update roda um UPDATE ... RETURNING dentro do escopo do casal; sem linha, é ErrNotFound.
func (p *PostgresRepo) update(ctx context.Context, householdID, query string, args ...any) (Reminder, error) {
	var out Reminder
	err := db.InTx(ctx, p.db, db.Scope{HouseholdID: householdID}, func(tx *sql.Tx) error {
		var err error
		out, err = scan(tx.QueryRowContext(ctx, query, args...))
		return err
	})
	if errors.Is(err, sql.ErrNoRows) {
		return Reminder{}, ErrNotFound
	}
	return out, err
}

// Só quem criou cancela.
func (p *PostgresRepo) Cancel(ctx context.Context, householdID, id, userID string) error {
	return db.InTx(ctx, p.db, db.Scope{HouseholdID: householdID}, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `
			UPDATE reminders SET status = 'canceled'
			WHERE id = $1 AND household_id = $2 AND created_by = $3
			  AND status IN ('scheduled', 'sent')`, id, householdID, userID)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrNotFound
		}
		return nil
	})
}

// ProcessDue atravessa todos os casais, por isso roda no escopo de sistema.
func (p *PostgresRepo) ProcessDue(ctx context.Context, now time.Time, limit int, fn func(context.Context, Reminder) error) (int, error) {
	count := 0
	err := db.InTx(ctx, p.db, db.Scope{System: true}, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
			SELECT `+cols+`
			FROM reminders r JOIN users u ON u.id = r.created_by
			WHERE r.status = 'scheduled' AND r.remind_at <= $1
			ORDER BY r.remind_at
			LIMIT $2
			FOR UPDATE OF r SKIP LOCKED`, now, limit)
		if err != nil {
			return err
		}
		var due []Reminder
		for rows.Next() {
			r, err := scan(rows)
			if err != nil {
				rows.Close()
				return err
			}
			due = append(due, r)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close() // precisa fechar antes de usar a transação de novo

		for _, r := range due {
			if err := fn(ctx, r); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx,
				`UPDATE reminders SET status = 'sent', sent_at = $2 WHERE id = $1`, r.ID, now); err != nil {
				return err
			}
		}
		count = len(due)
		return nil
	})
	if err != nil {
		return 0, err
	}
	return count, nil
}
