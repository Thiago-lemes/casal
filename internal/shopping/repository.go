package shopping

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
	ErrNotFound     = apperr.NewCoded(apperr.NotFound, "shopping_not_found", "item não encontrado")
	ErrDuplicate    = apperr.NewCoded(apperr.Conflict, "shopping_duplicate", "esse item já está na lista")
)

const (
	SourceManual    = "manual"
	SourceMenu      = "menu"
	SourceRecurring = "recurring"
)

type Item struct {
	ID          string     `json:"id"`
	HouseholdID string     `json:"-"`
	Name        string     `json:"name"`
	Quantity    *float64   `json:"quantity,omitempty"`
	Unit        string     `json:"unit"`
	Category    string     `json:"category"`
	Recurring   bool       `json:"recurring"`
	Source      string     `json:"source"`
	AddedBy     string     `json:"added_by"`
	BoughtAt    *time.Time `json:"bought_at,omitempty"`
	BoughtBy    *string    `json:"bought_by,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

type Repository interface {
	// Create grava o item; se já houver um aberto igual (nome, unidade e categoria), soma a quantidade
	// nele e devolve merged = true.
	Create(ctx context.Context, it Item) (item Item, merged bool, err error)
	// List devolve os itens abertos e os comprados depois de boughtSince.
	List(ctx context.Context, householdID string, boughtSince time.Time) ([]Item, error)
	Update(ctx context.Context, householdID string, it Item) (Item, error)
	SetBought(ctx context.Context, householdID, id, userID string, bought bool, now time.Time) (Item, error)
	Delete(ctx context.Context, householdID, id string) error
	// Restock volta à lista os itens recorrentes já comprados e devolve quantos voltaram.
	Restock(ctx context.Context, householdID string) (int64, error)
	// PurgeBought apaga os comprados antes de cutoff, menos os recorrentes. Roda no escopo de sistema.
	PurgeBought(ctx context.Context, cutoff time.Time) (int64, error)
}

type PostgresRepo struct{ db *sql.DB }

func NewPostgresRepo(db *sql.DB) *PostgresRepo { return &PostgresRepo{db: db} }

const cols = `s.id, s.household_id, s.name, s.quantity, s.unit, s.category, s.recurring, s.source,
	s.added_by, s.bought_at, s.bought_by, s.created_at`

type scanner interface{ Scan(dest ...any) error }

func scan(s scanner, extra ...any) (Item, error) {
	var it Item
	var qty sql.NullFloat64
	var boughtAt sql.NullTime
	var boughtBy sql.NullString
	dest := append([]any{&it.ID, &it.HouseholdID, &it.Name, &qty, &it.Unit, &it.Category, &it.Recurring,
		&it.Source, &it.AddedBy, &boughtAt, &boughtBy, &it.CreatedAt}, extra...)
	if err := s.Scan(dest...); err != nil {
		return Item{}, err
	}
	if qty.Valid {
		it.Quantity = &qty.Float64
	}
	if boughtAt.Valid {
		it.BoughtAt = &boughtAt.Time
	}
	if boughtBy.Valid {
		it.BoughtBy = &boughtBy.String
	}
	return it, nil
}

func (p *PostgresRepo) Create(ctx context.Context, in Item) (Item, bool, error) {
	var out Item
	var merged bool
	err := db.InTx(ctx, p.db, db.Scope{HouseholdID: in.HouseholdID}, func(tx *sql.Tx) error {
		var err error
		// O índice único dos itens abertos faz o trabalho: item igual soma a quantidade (nula + nula continua nula).
		// xmax diferente de zero indica que a linha já existia e foi atualizada.
		out, err = scan(tx.QueryRowContext(ctx, `
			WITH ins AS (
				INSERT INTO shopping_items (household_id, name, quantity, unit, category, recurring, source, added_by)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
				ON CONFLICT (household_id, name_key, unit, category) WHERE bought_at IS NULL
				DO UPDATE SET
					quantity = CASE WHEN shopping_items.quantity IS NULL AND EXCLUDED.quantity IS NULL THEN NULL
					                ELSE least(100000, coalesce(shopping_items.quantity, 0) + coalesce(EXCLUDED.quantity, 0)) END,
					recurring = shopping_items.recurring OR EXCLUDED.recurring
				RETURNING *, (xmax <> 0) AS merged
			)
			SELECT `+cols+`, s.merged FROM ins s`,
			in.HouseholdID, in.Name, in.Quantity, in.Unit, in.Category, in.Recurring, in.Source, in.AddedBy), &merged)
		return err
	})
	return out, merged, err
}

func (p *PostgresRepo) List(ctx context.Context, householdID string, boughtSince time.Time) ([]Item, error) {
	out := []Item{}
	err := db.InTx(ctx, p.db, db.Scope{HouseholdID: householdID}, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
			SELECT `+cols+`
			FROM shopping_items s
			WHERE s.household_id = $1 AND (s.bought_at IS NULL OR s.bought_at > $2)
			ORDER BY s.bought_at IS NOT NULL, s.category, s.name_key
			LIMIT 500`, householdID, boughtSince)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			it, err := scan(rows)
			if err != nil {
				return err
			}
			out = append(out, it)
		}
		return rows.Err()
	})
	return out, err
}

func (p *PostgresRepo) Update(ctx context.Context, householdID string, in Item) (Item, error) {
	return p.one(ctx, householdID, `
		WITH upd AS (
			UPDATE shopping_items s SET name = $3, quantity = $4, unit = $5, category = $6, recurring = $7
			WHERE s.id = $1 AND s.household_id = $2
			RETURNING *
		)
		SELECT `+cols+` FROM upd s`,
		in.ID, householdID, in.Name, in.Quantity, in.Unit, in.Category, in.Recurring)
}

func (p *PostgresRepo) SetBought(ctx context.Context, householdID, id, userID string, bought bool, now time.Time) (Item, error) {
	if bought {
		return p.one(ctx, householdID, `
			WITH upd AS (
				UPDATE shopping_items s SET bought_at = $3, bought_by = $4
				WHERE s.id = $1 AND s.household_id = $2 AND s.bought_at IS NULL
				RETURNING *
			)
			SELECT `+cols+` FROM upd s`, id, householdID, now, userID)
	}
	return p.one(ctx, householdID, `
		WITH upd AS (
			UPDATE shopping_items s SET bought_at = NULL, bought_by = NULL
			WHERE s.id = $1 AND s.household_id = $2 AND s.bought_at IS NOT NULL
			RETURNING *
		)
		SELECT `+cols+` FROM upd s`, id, householdID)
}

// one roda um UPDATE ... RETURNING no escopo do casal. Sem linha, é ErrNotFound; item igual já aberto, ErrDuplicate.
func (p *PostgresRepo) one(ctx context.Context, householdID, query string, args ...any) (Item, error) {
	var out Item
	err := db.InTx(ctx, p.db, db.Scope{HouseholdID: householdID}, func(tx *sql.Tx) error {
		var err error
		out, err = scan(tx.QueryRowContext(ctx, query, args...))
		return err
	})
	if errors.Is(err, sql.ErrNoRows) {
		return Item{}, ErrNotFound
	}
	if db.IsUniqueViolation(err) {
		return Item{}, ErrDuplicate
	}
	return out, err
}

func (p *PostgresRepo) Delete(ctx context.Context, householdID, id string) error {
	return db.InTx(ctx, p.db, db.Scope{HouseholdID: householdID}, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `DELETE FROM shopping_items WHERE id = $1 AND household_id = $2`, id, householdID)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrNotFound
		}
		return nil
	})
}

// Restock reabre só um item por (nome, unidade, categoria) e só se não houver um igual já aberto,
// para o índice único dos abertos nunca ser violado.
func (p *PostgresRepo) Restock(ctx context.Context, householdID string) (int64, error) {
	var n int64
	err := db.InTx(ctx, p.db, db.Scope{HouseholdID: householdID}, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `
			UPDATE shopping_items s SET bought_at = NULL, bought_by = NULL, source = 'recurring'
			WHERE s.id IN (
				SELECT DISTINCT ON (r.name_key, r.unit, r.category) r.id
				FROM shopping_items r
				WHERE r.household_id = $1 AND r.recurring AND r.bought_at IS NOT NULL
				  AND NOT EXISTS (
					SELECT 1 FROM shopping_items o
					WHERE o.household_id = r.household_id AND o.bought_at IS NULL
					  AND o.name_key = r.name_key AND o.unit = r.unit AND o.category = r.category)
				ORDER BY r.name_key, r.unit, r.category, r.bought_at DESC
			)`, householdID)
		if err != nil {
			return err
		}
		n, _ = res.RowsAffected()
		return nil
	})
	return n, err
}

func (p *PostgresRepo) PurgeBought(ctx context.Context, cutoff time.Time) (int64, error) {
	var n int64
	err := db.InTx(ctx, p.db, db.Scope{System: true}, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `
			DELETE FROM shopping_items WHERE bought_at IS NOT NULL AND bought_at < $1 AND NOT recurring`, cutoff)
		if err != nil {
			return err
		}
		n, _ = res.RowsAffected()
		return nil
	})
	return n, err
}
