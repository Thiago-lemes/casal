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

	if err := SetScope(ctx, tx, scope); err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("db: confirmar transação: %w", err)
	}
	return nil
}

// SetScope informa o escopo ao Postgres numa transação já aberta (vale só dentro dela).
// InTx chama isto por você; use direto só em quem recebe uma transação pronta, como os ganchos de Handoff.
func SetScope(ctx context.Context, tx *sql.Tx, scope Scope) error {
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
	return nil
}

// Handoff descreve uma pessoa mudando de casal. As features recebem isto para levar consigo o que é
// pessoal e passar adiante o que é compartilhado, sem importar umas às outras: quem liga é o app.
//
//   - Ao sair: UserID saiu de From; RemainingUserID é quem ficou (vazio se a pessoa era a única).
//   - Ao entrar (criar ou aceitar convite): UserID passou a ser de To.
type Handoff struct {
	UserID          string
	From            string
	To              string
	RemainingUserID string
}

// Hook roda dentro da transação da mudança de casal: ou tudo acontece, ou nada.
// Quem usa Hook decide o próprio escopo (normalmente db.SetScope com System, pois atravessa casais).
type Hook func(ctx context.Context, tx *sql.Tx, h Handoff) error

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
