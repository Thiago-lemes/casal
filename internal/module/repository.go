package module

import (
	"context"
	"database/sql"
	"time"

	"casal/internal/db"
)

// Hook roda na mesma transação que liga o módulo, dentro do escopo do casal.
type Hook func(ctx context.Context, tx *sql.Tx, householdID string) error

type Repository interface {
	// Enabled devolve os módulos ligados e quando foram ligados.
	Enabled(ctx context.Context, householdID string) (map[string]time.Time, error)
	IsEnabled(ctx context.Context, householdID, name string) (bool, error)
	// Enable liga o módulo e roda os ganchos na mesma transação, só se ele estava desligado
	// (duas chamadas simultâneas rodam os ganchos uma vez). Devolve se mudou.
	Enable(ctx context.Context, householdID, name string, hooks ...Hook) (changed bool, err error)
	// Disable desliga o módulo sem apagar os dados dele.
	Disable(ctx context.Context, householdID, name string) error
}

type PostgresRepo struct{ db *sql.DB }

func NewPostgresRepo(db *sql.DB) *PostgresRepo { return &PostgresRepo{db: db} }

func (p *PostgresRepo) Enabled(ctx context.Context, householdID string) (map[string]time.Time, error) {
	out := map[string]time.Time{}
	err := db.InTx(ctx, p.db, db.Scope{HouseholdID: householdID}, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx,
			`SELECT module, enabled_at FROM household_modules WHERE household_id = $1`, householdID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var name string
			var at time.Time
			if err := rows.Scan(&name, &at); err != nil {
				return err
			}
			out[name] = at
		}
		return rows.Err()
	})
	return out, err
}

func (p *PostgresRepo) IsEnabled(ctx context.Context, householdID, name string) (bool, error) {
	var ok bool
	err := db.InTx(ctx, p.db, db.Scope{HouseholdID: householdID}, func(tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `
			SELECT EXISTS (SELECT 1 FROM household_modules WHERE household_id = $1 AND module = $2)`,
			householdID, name).Scan(&ok)
	})
	return ok, err
}

func (p *PostgresRepo) Enable(ctx context.Context, householdID, name string, hooks ...Hook) (bool, error) {
	changed := false
	err := db.InTx(ctx, p.db, db.Scope{HouseholdID: householdID}, func(tx *sql.Tx) error {
		// com duas chamadas ao mesmo tempo, a segunda espera a primeira e não insere nada
		res, err := tx.ExecContext(ctx, `
			INSERT INTO household_modules (household_id, module) VALUES ($1, $2)
			ON CONFLICT (household_id, module) DO NOTHING`, householdID, name)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			return nil
		}
		changed = true
		for _, h := range hooks {
			if err := h(ctx, tx, householdID); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		changed = false
	}
	return changed, err
}

func (p *PostgresRepo) Disable(ctx context.Context, householdID, name string) error {
	return db.InTx(ctx, p.db, db.Scope{HouseholdID: householdID}, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			`DELETE FROM household_modules WHERE household_id = $1 AND module = $2`, householdID, name)
		return err
	})
}
