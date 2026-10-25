package auth

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

type User struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	Timezone string `json:"timezone"`
}

var (
	ErrEmailTaken         = errors.New("e-mail já cadastrado")
	ErrNotFound           = errors.New("não encontrado")
	ErrInvalidCredentials = errors.New("e-mail ou senha inválidos")
	ErrInvalidInput       = errors.New("dados inválidos")
)

// Repository é o que o service precisa do banco. Em testes, troque por um fake.
type Repository interface {
	CreateUser(ctx context.Context, email, name, passwordHash string) (User, error)
	UserByEmail(ctx context.Context, email string) (User, string, error) // usuário e hash da senha
	CreateSession(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error
	UserBySession(ctx context.Context, tokenHash string, now time.Time) (User, error)
	DeleteSession(ctx context.Context, tokenHash string) error
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

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
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
	err := r.db.QueryRowContext(ctx, `
		SELECT u.id, u.email, u.name, u.timezone
		FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = $1 AND s.expires_at > $2`, tokenHash, now).
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
