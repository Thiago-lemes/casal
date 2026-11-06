package reminder

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"casal/internal/apperr"
	"casal/internal/db"
)

var (
	ErrInvalidInput = apperr.New(apperr.Invalid, "dados inválidos")
	ErrNotFound     = apperr.NewCoded(apperr.NotFound, "reminder_not_found", "lembrete não encontrado")
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
	Attempts      int        `json:"-"` // quantas vezes o agendador já tentou avisar
}

type Repository interface {
	Create(ctx context.Context, r Reminder) (Reminder, error)
	List(ctx context.Context, householdID, userID string) ([]Reminder, error)
	Resolve(ctx context.Context, householdID, id, userID string, now time.Time) (Reminder, error)
	Snooze(ctx context.Context, householdID, id, userID string, remindAt time.Time) (Reminder, error)
	Cancel(ctx context.Context, householdID, id, userID string) error
	// CreateWithRef grava um lembrete de outro módulo. Se já há um ativo com o mesmo ref no casal, devolve esse
	// (criar de novo é idempotente).
	CreateWithRef(ctx context.Context, r Reminder, ref string) (Reminder, error)
	// CancelByRef cancela os lembretes pendentes com esse ref e devolve quantos eram.
	CancelByRef(ctx context.Context, householdID, ref string) (int64, error)
}

type PostgresRepo struct{ db *sql.DB }

func NewPostgresRepo(db *sql.DB) *PostgresRepo { return &PostgresRepo{db: db} }

// Colunas lidas em todas as consultas. O JOIN em users é só leitura, para mostrar o nome.
const cols = `r.id, r.household_id, r.created_by, u.name, r.recipient_id, r.body,
	r.remind_at, r.status, r.sent_at, r.resolved_at, r.attempts`

type scanner interface{ Scan(dest ...any) error }

func scan(s scanner) (Reminder, error) {
	var r Reminder
	var sent, resolved sql.NullTime
	err := s.Scan(&r.ID, &r.HouseholdID, &r.CreatedBy, &r.CreatedByName, &r.RecipientID,
		&r.Body, &r.RemindAt, &r.Status, &sent, &resolved, &r.Attempts)
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

func (p *PostgresRepo) CreateWithRef(ctx context.Context, in Reminder, ref string) (Reminder, error) {
	var out Reminder
	err := db.InTx(ctx, p.db, db.Scope{HouseholdID: in.HouseholdID}, func(tx *sql.Tx) error {
		// O índice único dos ativos por ref faz a idempotência. DO NOTHING não devolve a linha existente,
		// então ela é lida em seguida (a transação é a mesma e o escopo é o do casal).
		var id string
		err := tx.QueryRowContext(ctx, `
			INSERT INTO reminders (household_id, created_by, recipient_id, body, remind_at, ref)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (household_id, ref) WHERE ref IS NOT NULL AND status <> 'canceled' DO NOTHING
			RETURNING id`,
			in.HouseholdID, in.CreatedBy, in.RecipientID, in.Body, in.RemindAt, ref).Scan(&id)
		if errors.Is(err, sql.ErrNoRows) {
			err = tx.QueryRowContext(ctx, `
				SELECT id FROM reminders
				WHERE household_id = $1 AND ref = $2 AND status <> 'canceled'`, in.HouseholdID, ref).Scan(&id)
		}
		if err != nil {
			return err
		}
		out, err = scan(tx.QueryRowContext(ctx, `
			SELECT `+cols+` FROM reminders r JOIN users u ON u.id = r.created_by
			WHERE r.id = $1 AND r.household_id = $2`, id, in.HouseholdID))
		return err
	})
	return out, err
}

func (p *PostgresRepo) CancelByRef(ctx context.Context, householdID, ref string) (int64, error) {
	var n int64
	err := db.InTx(ctx, p.db, db.Scope{HouseholdID: householdID}, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `
			UPDATE reminders SET status = 'canceled'
			WHERE household_id = $1 AND ref = $2 AND status IN ('scheduled', 'sent')`, householdID, ref)
		if err != nil {
			return err
		}
		n, _ = res.RowsAffected()
		return nil
	})
	return n, err
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
			UPDATE reminders SET status = 'scheduled', remind_at = $4, sent_at = NULL, claimed_until = NULL, attempts = 0
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

// Claim reserva até limit lembretes vencidos por lease e devolve os reservados.
// A transação é curta: só marca a reserva. O envio acontece depois, fora dela.
// SKIP LOCKED e a reserva impedem que dois agendadores peguem o mesmo lembrete.
// Atravessa todos os casais, por isso roda no escopo de sistema.
func (p *PostgresRepo) Claim(ctx context.Context, now time.Time, limit int, lease time.Duration) ([]Reminder, error) {
	var out []Reminder
	err := db.InTx(ctx, p.db, db.Scope{System: true}, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
			WITH picked AS (
				SELECT id FROM reminders
				WHERE status = 'scheduled' AND remind_at <= $1
				  AND (claimed_until IS NULL OR claimed_until <= $1)
				ORDER BY remind_at
				LIMIT $2
				FOR UPDATE SKIP LOCKED
			), upd AS (
				UPDATE reminders r SET claimed_until = $3, attempts = r.attempts + 1
				FROM picked WHERE r.id = picked.id
				RETURNING r.*
			)
			SELECT `+cols+` FROM upd r JOIN users u ON u.id = r.created_by
			ORDER BY r.remind_at`, now, limit, now.Add(lease))
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

// MarkSent conclui o lembrete. Só vale se ele ainda estiver agendado: se a pessoa
// resolveu, adiou ou cancelou durante o envio, essa ação prevalece.
func (p *PostgresRepo) MarkSent(ctx context.Context, id string, now time.Time) error {
	return db.InTx(ctx, p.db, db.Scope{System: true}, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			UPDATE reminders SET status = 'sent', sent_at = $2, claimed_until = NULL
			WHERE id = $1 AND status = 'scheduled'`, id, now)
		return err
	})
}

// HandleLeave roda quando alguém sai do casal: os lembretes ainda pendentes entre as duas pessoas são cancelados
// (ninguém mais está lá para recebê-los ou resolvê-los). Os que a pessoa deixou para si mesma ficam com ela, menos os
// que outro módulo criou com ref (contas a pagar): esses são do casal, não da pessoa, e quem fica os refaz.
// Roda na transação da saída.
func (p *PostgresRepo) HandleLeave(ctx context.Context, tx *sql.Tx, h db.Handoff) error {
	if err := db.SetScope(ctx, tx, db.Scope{System: true}); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `
		UPDATE reminders SET status = 'canceled'
		WHERE household_id = $1 AND status IN ('scheduled', 'sent')
		  AND (created_by = $2 OR recipient_id = $2)
		  AND (created_by <> recipient_id OR ref IS NOT NULL)`, h.From, h.UserID)
	return err
}

// HandleJoin roda quando a pessoa passa a ter um casal: os lembretes que ela deixou para si mesma vêm junto.
func (p *PostgresRepo) HandleJoin(ctx context.Context, tx *sql.Tx, h db.Handoff) error {
	if err := db.SetScope(ctx, tx, db.Scope{System: true}); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `
		UPDATE reminders SET household_id = $2
		WHERE created_by = $1 AND recipient_id = $1 AND household_id <> $2 AND ref IS NULL`, h.UserID, h.To)
	return err
}

// CountOverdue conta os lembretes agendados que já deviam ter saído há mais de grace. Com o agendador sadio o
// resultado é zero; diferente disso por minutos é fila travada. Não lê conteúdo, só conta (escopo de sistema).
func (p *PostgresRepo) CountOverdue(ctx context.Context, now time.Time, grace time.Duration) (int, error) {
	var n int
	err := db.InTx(ctx, p.db, db.Scope{System: true}, func(tx *sql.Tx) error {
		return tx.QueryRowContext(ctx,
			`SELECT count(*) FROM reminders WHERE status = 'scheduled' AND remind_at <= $1`,
			now.Add(-grace)).Scan(&n)
	})
	return n, err
}
