package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib" // registra o driver "pgx" no database/sql
)

type Scope struct {
	HouseholdID string // casal da requisição
	System      bool   // só para processos internos, como o agendador
}

// InTx abre uma transação, informa o escopo ao Postgres (vale só dentro dela) e roda fn.
// Erros de fn voltam como estão; só os de infraestrutura ganham o prefixo "db:".
func InTx(ctx context.Context, db *sql.DB, scope Scope, fn func(tx *sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("db: iniciar transação: %w", err)
	}
	defer tx.Rollback()

	system := "off"
	if scope.System {
		system = "on"
	}
	// uma única ida ao banco para os dois ajustes
	if _, err := tx.ExecContext(ctx,
		`SELECT set_config('app.household_id', $1, true), set_config('app.system', $2, true)`,
		scope.HouseholdID, system); err != nil {
		return fmt.Errorf("db: definir escopo: %w", err)
	}
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("db: confirmar transação: %w", err)
	}
	return nil
}

// IsUniqueViolation diz se o erro é de chave duplicada (UNIQUE ou PRIMARY KEY).
func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// Open abre o pool de conexões, já com os limites definidos, e confere que o banco responde.
func Open(ctx context.Context, url string) (*sql.DB, error) {
	conn, err := sql.Open("pgx", url)
	if err != nil {
		return nil, fmt.Errorf("db: abrir: %w", err)
	}
	conn.SetMaxOpenConns(10)
	conn.SetMaxIdleConns(5)
	conn.SetConnMaxLifetime(30 * time.Minute)

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := conn.PingContext(pingCtx); err != nil {
		conn.Close()
		return nil, fmt.Errorf("db: banco indisponível: %w", err)
	}
	return conn, nil
}
