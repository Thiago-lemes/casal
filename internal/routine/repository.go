package routine

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
	ErrNotFound     = apperr.NewCoded(apperr.NotFound, "routine_not_found", "bloco não encontrado")
	ErrForbidden    = apperr.NewCoded(apperr.Forbidden, "routine_forbidden", "você não pode alterar este bloco")
	ErrConflict     = apperr.NewCoded(apperr.Conflict, "routine_conflict", "o bloco foi alterado por outra pessoa; recarregue")
)

type Repository interface {
	Create(ctx context.Context, b Block) (Block, error)
	// Get e ListVisible devolvem só o que a pessoa pode ver (os próprios blocos e os do parceiro que não são privados).
	Get(ctx context.Context, householdID, id, viewerID string) (Block, error)
	ListVisible(ctx context.Context, householdID, viewerID string) ([]Block, error)
	// Update só grava se a versão ainda for a informada e se a pessoa puder editar.
	Update(ctx context.Context, householdID, userID string, b Block, version int) (Block, error)
	Delete(ctx context.Context, householdID, id, ownerID string) error
	// ListExceptions devolve as exceções entre os dias from e to dos blocos que a pessoa pode ver.
	ListExceptions(ctx context.Context, householdID, viewerID string, from, to time.Time) ([]Exception, error)
	SetException(ctx context.Context, householdID, userID string, e Exception) error
	DeleteException(ctx context.Context, householdID, userID, blockID string, date time.Time) error
}

type PostgresRepo struct{ db *sql.DB }

func NewPostgresRepo(db *sql.DB) *PostgresRepo { return &PostgresRepo{db: db} }

// O JOIN em users é só leitura, para mostrar o nome e saber o fuso do dono.
const cols = `b.id, b.household_id, b.owner_id, u.name, u.timezone, b.title, b.category, b.location,
	b.visibility, b.recurrence, b.weekdays, b.interval_weeks, b.start_date, b.end_date,
	b.start_min, b.end_min, b.version, b.created_at, b.updated_at`

// Quem pode ver: o dono, ou o parceiro quando o bloco não é privado.
func visibleTo(param string) string {
	return `(b.owner_id = ` + param + ` OR b.visibility <> 'private')`
}

type scanner interface{ Scan(dest ...any) error }

func scan(s scanner) (Block, error) {
	var b Block
	var weekdays int16
	var start time.Time
	var end sql.NullTime
	err := s.Scan(&b.ID, &b.HouseholdID, &b.OwnerID, &b.OwnerName, &b.OwnerTZ, &b.Title, &b.Category,
		&b.Location, &b.Visibility, &b.Recurrence, &weekdays, &b.IntervalWeeks, &start, &end,
		&b.StartMin, &b.EndMin, &b.Version, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		return Block{}, err
	}
	b.Weekdays = uint8(weekdays)
	b.StartDate = civil(start)
	if end.Valid {
		d := civil(end.Time)
		b.EndDate = &d
	}
	return b, nil
}

// civil normaliza um date do banco para meia-noite UTC, o formato que o resto do pacote usa.
func civil(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func endDateArg(b Block) any {
	if b.EndDate == nil {
		return nil
	}
	return DateKey(*b.EndDate)
}

func (p *PostgresRepo) Create(ctx context.Context, in Block) (Block, error) {
	var out Block
	err := db.InTx(ctx, p.db, db.Scope{HouseholdID: in.HouseholdID}, func(tx *sql.Tx) error {
		var err error
		out, err = scan(tx.QueryRowContext(ctx, `
			WITH ins AS (
				INSERT INTO routine_blocks (household_id, owner_id, title, category, location, visibility,
					recurrence, weekdays, interval_weeks, start_date, end_date, start_min, end_min)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10::date, $11::date, $12, $13)
				RETURNING *
			)
			SELECT `+cols+` FROM ins b JOIN users u ON u.id = b.owner_id`,
			in.HouseholdID, in.OwnerID, in.Title, in.Category, in.Location, in.Visibility,
			in.Recurrence, int16(in.Weekdays), in.IntervalWeeks, DateKey(in.StartDate), endDateArg(in),
			in.StartMin, in.EndMin))
		return err
	})
	return out, err
}

func (p *PostgresRepo) Get(ctx context.Context, householdID, id, viewerID string) (Block, error) {
	var out Block
	err := db.InTx(ctx, p.db, db.Scope{HouseholdID: householdID}, func(tx *sql.Tx) error {
		var err error
		out, err = scan(tx.QueryRowContext(ctx, `
			SELECT `+cols+`
			FROM routine_blocks b JOIN users u ON u.id = b.owner_id
			WHERE b.id = $1 AND b.household_id = $2 AND `+visibleTo("$3"),
			id, householdID, viewerID))
		return err
	})
	if errors.Is(err, sql.ErrNoRows) {
		return Block{}, ErrNotFound
	}
	return out, err
}

func (p *PostgresRepo) ListVisible(ctx context.Context, householdID, viewerID string) ([]Block, error) {
	out := []Block{}
	err := db.InTx(ctx, p.db, db.Scope{HouseholdID: householdID}, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
			SELECT `+cols+`
			FROM routine_blocks b JOIN users u ON u.id = b.owner_id
			WHERE b.household_id = $1 AND `+visibleTo("$2")+`
			ORDER BY b.start_min, b.created_at
			LIMIT 500`, householdID, viewerID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			b, err := scan(rows)
			if err != nil {
				return err
			}
			out = append(out, b)
		}
		return rows.Err()
	})
	return out, err
}

// Quem pode editar: o dono, ou os dois quando o bloco é do casal. Só o dono muda a visibilidade
// (o CASE protege isso mesmo que o chamador se esqueça).
func (p *PostgresRepo) Update(ctx context.Context, householdID, userID string, in Block, version int) (Block, error) {
	var out Block
	err := db.InTx(ctx, p.db, db.Scope{HouseholdID: householdID}, func(tx *sql.Tx) error {
		var err error
		out, err = scan(tx.QueryRowContext(ctx, `
			WITH upd AS (
				UPDATE routine_blocks b SET title = $4, category = $5, location = $6,
					visibility = CASE WHEN b.owner_id = $3 THEN $7 ELSE b.visibility END,
					recurrence = $8, weekdays = $9, interval_weeks = $10, start_date = $11::date,
					end_date = $12::date, start_min = $13, end_min = $14,
					version = b.version + 1, updated_at = now()
				WHERE b.id = $1 AND b.household_id = $2 AND b.version = $15
				  AND (b.owner_id = $3 OR b.visibility = 'household')
				RETURNING *
			)
			SELECT `+cols+` FROM upd b JOIN users u ON u.id = b.owner_id`,
			in.ID, householdID, userID, in.Title, in.Category, in.Location, in.Visibility,
			in.Recurrence, int16(in.Weekdays), in.IntervalWeeks, DateKey(in.StartDate), endDateArg(in),
			in.StartMin, in.EndMin, version))
		return err
	})
	if errors.Is(err, sql.ErrNoRows) {
		return Block{}, ErrNotFound
	}
	return out, err
}

// Só o dono apaga, mesmo um bloco do casal. As exceções vão junto (ON DELETE CASCADE).
func (p *PostgresRepo) Delete(ctx context.Context, householdID, id, ownerID string) error {
	return db.InTx(ctx, p.db, db.Scope{HouseholdID: householdID}, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `
			DELETE FROM routine_blocks WHERE id = $1 AND household_id = $2 AND owner_id = $3`,
			id, householdID, ownerID)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrNotFound
		}
		return nil
	})
}

func (p *PostgresRepo) ListExceptions(ctx context.Context, householdID, viewerID string, from, to time.Time) ([]Exception, error) {
	out := []Exception{}
	err := db.InTx(ctx, p.db, db.Scope{HouseholdID: householdID}, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
			SELECT e.block_id, e.on_date, e.skipped, coalesce(e.start_min, 0), coalesce(e.end_min, 0)
			FROM routine_exceptions e JOIN routine_blocks b ON b.id = e.block_id
			WHERE e.household_id = $1 AND b.household_id = $1 AND `+visibleTo("$2")+`
			  AND e.on_date BETWEEN $3::date AND $4::date`,
			householdID, viewerID, DateKey(from), DateKey(to))
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var e Exception
			var d time.Time
			if err := rows.Scan(&e.BlockID, &d, &e.Skipped, &e.StartMin, &e.EndMin); err != nil {
				return err
			}
			e.Date = civil(d)
			out = append(out, e)
		}
		return rows.Err()
	})
	return out, err
}

// SetException cria ou troca a exceção do dia. Só vale se a pessoa puder editar o bloco.
func (p *PostgresRepo) SetException(ctx context.Context, householdID, userID string, e Exception) error {
	return db.InTx(ctx, p.db, db.Scope{HouseholdID: householdID}, func(tx *sql.Tx) error {
		var start, end any
		if !e.Skipped {
			start, end = e.StartMin, e.EndMin
		}
		res, err := tx.ExecContext(ctx, `
			INSERT INTO routine_exceptions (block_id, household_id, on_date, skipped, start_min, end_min)
			SELECT b.id, b.household_id, $4::date, $5, $6, $7
			FROM routine_blocks b
			WHERE b.id = $1 AND b.household_id = $2 AND (b.owner_id = $3 OR b.visibility = 'household')
			ON CONFLICT (block_id, on_date)
			DO UPDATE SET skipped = EXCLUDED.skipped, start_min = EXCLUDED.start_min, end_min = EXCLUDED.end_min`,
			e.BlockID, householdID, userID, DateKey(e.Date), e.Skipped, start, end)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrNotFound
		}
		return nil
	})
}

func (p *PostgresRepo) DeleteException(ctx context.Context, householdID, userID, blockID string, date time.Time) error {
	return db.InTx(ctx, p.db, db.Scope{HouseholdID: householdID}, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `
			DELETE FROM routine_exceptions e USING routine_blocks b
			WHERE e.block_id = b.id AND e.block_id = $1 AND e.household_id = $2 AND b.household_id = $2
			  AND e.on_date = $4::date AND (b.owner_id = $3 OR b.visibility = 'household')`,
			blockID, householdID, userID, DateKey(date))
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrNotFound
		}
		return nil
	})
}

// HandleLeave roda quando alguém sai do casal. Os blocos do casal passam para quem ficou; os que o parceiro
// via (detalhes ou só "ocupado") deixam de ser compartilhados e acompanham quem saiu. Roda na transação da saída.
// Os horários são relógio de parede no fuso do dono: um bloco do casal que muda de dono passa a valer no fuso dele.
func (p *PostgresRepo) HandleLeave(ctx context.Context, tx *sql.Tx, h db.Handoff) error {
	if err := db.SetScope(ctx, tx, db.Scope{System: true}); err != nil {
		return err
	}
	if h.RemainingUserID != "" {
		if _, err := tx.ExecContext(ctx, `
			UPDATE routine_blocks SET owner_id = $3, version = version + 1
			WHERE household_id = $1 AND owner_id = $2 AND visibility = 'household'`,
			h.From, h.UserID, h.RemainingUserID); err != nil {
			return err
		}
	}
	_, err := tx.ExecContext(ctx, `
		UPDATE routine_blocks SET visibility = 'private', version = version + 1
		WHERE household_id = $1 AND owner_id = $2 AND visibility IN ('busy', 'view')`, h.From, h.UserID)
	return err
}

// HandleJoin roda quando a pessoa passa a ter um casal: os blocos dela (e as exceções) vêm junto,
// sempre privados (nada vira visível ao novo parceiro sem a pessoa escolher).
func (p *PostgresRepo) HandleJoin(ctx context.Context, tx *sql.Tx, h db.Handoff) error {
	if err := db.SetScope(ctx, tx, db.Scope{System: true}); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE routine_blocks SET household_id = $2, visibility = 'private', version = version + 1
		WHERE owner_id = $1 AND household_id <> $2`, h.UserID, h.To); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `
		UPDATE routine_exceptions e SET household_id = b.household_id
		FROM routine_blocks b
		WHERE e.block_id = b.id AND e.household_id <> b.household_id`)
	return err
}
