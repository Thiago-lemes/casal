package db

import (
	"context"
	"database/sql"
)

type Scope struct {
	HouseholdID string // casal da requisição
	System      bool   // só para processos internos, como o agendador
}

// InTx abre uma transação, informa o escopo ao Postgres (vale só dentro dela) e roda fn.
func InTx(ctx context.Context, db *sql.DB, scope Scope, fn func(tx *sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.household_id', $1, true)`, scope.HouseholdID); err != nil {
		return err
	}
	system := "off"
	if scope.System {
		system = "on"
	}
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.system', $1, true)`, system); err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}
