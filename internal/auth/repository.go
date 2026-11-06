package auth

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"casal/internal/apperr"
	"casal/internal/db"
)

type User struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	Timezone string `json:"timezone"`
}

var (
	ErrEmailTaken         = apperr.NewCoded(apperr.Conflict, "email_taken", "e-mail já cadastrado")
	ErrNotFound           = apperr.New(apperr.NotFound, "não encontrado")
	ErrInvalidCredentials = apperr.NewCoded(apperr.Unauthorized, "invalid_credentials", "e-mail ou senha inválidos")
	ErrInvalidInput       = apperr.New(apperr.Invalid, "dados inválidos")
	// ErrSignupCodeInvalid cobre código ausente, errado, vencido, revogado ou esgotado: a resposta não diz qual.
	ErrSignupCodeInvalid = apperr.NewCoded(apperr.Invalid, "signup_code_invalid", "código de cadastro inválido")
)

// Repository é o que o service precisa do banco. Em testes, troque por um fake.
type Repository interface {
	CreateUser(ctx context.Context, email, name, passwordHash string) (User, error)
	UserByEmail(ctx context.Context, email string) (User, string, error) // usuário e hash da senha
	CreateSession(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error
	UserBySession(ctx context.Context, tokenHash string, now time.Time) (User, error)
	DeleteSession(ctx context.Context, tokenHash string) error
	DeleteUserSessions(ctx context.Context, userID string) error
	DeleteExpiredSessions(ctx context.Context, now time.Time) (int64, error)
	UpdateProfile(ctx context.Context, userID, name, timezone string) (User, error)
}

type PostgresRepo struct{ db *sql.DB }

func NewPostgresRepo(db *sql.DB) *PostgresRepo { return &PostgresRepo{db: db} }

func (r *PostgresRepo) CreateUser(ctx context.Context, email, name, passwordHash string) (User, error) {
	var u User
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO users (email, name, password_hash)
		VALUES ($1, $2, $3)
		RETURNING id, email, name, timezone`,
		email, name, passwordHash).Scan(&u.ID, &u.Email, &u.Name, &u.Timezone)

	if db.IsUniqueViolation(err) {
		return User{}, ErrEmailTaken
	}
	return u, err
}

func (r *PostgresRepo) UserByEmail(ctx context.Context, email string) (User, string, error) {
	var u User
	var hash string
	err := r.db.QueryRowContext(ctx, `
		SELECT id, email, name, timezone, password_hash
		FROM users WHERE lower(email) = lower($1)`, email).
		Scan(&u.ID, &u.Email, &u.Name, &u.Timezone, &hash)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, "", ErrNotFound
	}
	return u, hash, err
}

func (r *PostgresRepo) CreateSession(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO sessions (user_id, token_hash, expires_at) VALUES ($1, $2, $3)`,
		userID, tokenHash, expiresAt)
	return err
}

func (r *PostgresRepo) UserBySession(ctx context.Context, tokenHash string, now time.Time) (User, error) {
	var u User
	// Uma ida só ao banco: acha a sessão e, no máximo uma vez por hora, marca o último uso e o dia de atividade
	// da pessoa (base das métricas de ativos e retenção). Nas demais requisições o UPDATE não toca em nada.
	err := r.db.QueryRowContext(ctx, `
		WITH s AS (
			SELECT id, user_id FROM sessions WHERE token_hash = $1 AND expires_at > $2
		), touched AS (
			UPDATE sessions SET last_used_at = $2
			WHERE id IN (SELECT id FROM s) AND (last_used_at IS NULL OR last_used_at < $2 - interval '1 hour')
			RETURNING user_id
		), activity AS (
			INSERT INTO user_activity (user_id, day)
			SELECT user_id, ($2 AT TIME ZONE 'UTC')::date FROM touched
			ON CONFLICT DO NOTHING
		)
		SELECT u.id, u.email, u.name, u.timezone
		FROM s JOIN users u ON u.id = s.user_id`, tokenHash, now).
		Scan(&u.ID, &u.Email, &u.Name, &u.Timezone)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}

func (r *PostgresRepo) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = $1`, tokenHash)
	return err
}

// DeleteUserSessions encerra todas as sessões da pessoa (todos os aparelhos).
func (r *PostgresRepo) DeleteUserSessions(ctx context.Context, userID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = $1`, userID)
	return err
}

// DeleteExpiredSessions apaga as sessões vencidas e devolve quantas eram.
func (r *PostgresRepo) DeleteExpiredSessions(ctx context.Context, now time.Time) (int64, error) {
	res, err := r.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= $1`, now)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// UpdateProfile grava nome e fuso (o service já validou e completou os dois).
func (r *PostgresRepo) UpdateProfile(ctx context.Context, userID, name, timezone string) (User, error) {
	var u User
	err := r.db.QueryRowContext(ctx, `
		UPDATE users SET name = $2, timezone = $3 WHERE id = $1
		RETURNING id, email, name, timezone`, userID, name, timezone).
		Scan(&u.ID, &u.Email, &u.Name, &u.Timezone)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}
