package admin

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"casal/internal/apperr"
	"casal/internal/db"
)

var (
	// ErrLoginFailed é a única resposta para e-mail, senha ou código errados: não diz qual dos três.
	ErrLoginFailed = apperr.NewCoded(apperr.Unauthorized, "admin_login_failed", "e-mail, senha ou código inválidos")
	ErrNotFound    = apperr.New(apperr.NotFound, "não encontrado")
	ErrInvalid     = apperr.New(apperr.Invalid, "dados inválidos")
	ErrEmailTaken  = apperr.NewCoded(apperr.Conflict, "email_taken", "e-mail já cadastrado")
)

// Admin é uma conta de administração.
type Admin struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

type account struct {
	Admin
	passwordHash string
	totpSecret   string
	lastStep     int64
}

// Entry é uma linha da auditoria. Before e After só recebem valores estruturados (estado do módulo, bloqueio):
// nunca texto de casal.
type Entry struct {
	Action     string
	TargetType string
	TargetID   string // vazio quando não há alvo
	Before     map[string]any
	After      map[string]any
}

// AuditRow é uma linha lida da auditoria.
type AuditRow struct {
	ID         int64           `json:"id"`
	OccurredAt time.Time       `json:"occurred_at"`
	AdminEmail string          `json:"admin_email"`
	Action     string          `json:"action"`
	TargetType string          `json:"target_type"`
	TargetID   *string         `json:"target_id"`
	Before     json.RawMessage `json:"before"`
	After      json.RawMessage `json:"after"`
}

// Repository é o que o service precisa do banco.
type Repository interface {
	CreateAdmin(ctx context.Context, email, passwordHash, totpSecret string) (Admin, error)
	AccountByEmail(ctx context.Context, email string) (account, error)
	// AcceptStep grava o passo TOTP só se for maior que o último aceito (um código não vale duas vezes).
	AcceptStep(ctx context.Context, adminID string, step int64) (bool, error)
	CreateSession(ctx context.Context, adminID, tokenHash string, expiresAt time.Time) error
	AdminBySession(ctx context.Context, tokenHash string, now time.Time) (Admin, error)
	DeleteSession(ctx context.Context, tokenHash string) error
	DeleteExpiredSessions(ctx context.Context, now time.Time) (int64, error)
	Audit(ctx context.Context, a Admin, e Entry) error
	ListAudit(ctx context.Context, limit int) ([]AuditRow, error)
}

type PostgresRepo struct{ db *sql.DB }

func NewPostgresRepo(db *sql.DB) *PostgresRepo { return &PostgresRepo{db: db} }

func (r *PostgresRepo) CreateAdmin(ctx context.Context, email, passwordHash, totpSecret string) (Admin, error) {
	var a Admin
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO admin_users (email, password_hash, totp_secret) VALUES ($1, $2, $3)
		RETURNING id, email`, email, passwordHash, totpSecret).Scan(&a.ID, &a.Email)
	if db.IsUniqueViolation(err) {
		return Admin{}, ErrEmailTaken
	}
	return a, err
}

func (r *PostgresRepo) AccountByEmail(ctx context.Context, email string) (account, error) {
	var a account
	err := r.db.QueryRowContext(ctx, `
		SELECT id, email, password_hash, totp_secret, totp_last_step FROM admin_users WHERE lower(email) = lower($1)`, email).
		Scan(&a.ID, &a.Email, &a.passwordHash, &a.totpSecret, &a.lastStep)
	if errors.Is(err, sql.ErrNoRows) {
		return account{}, ErrNotFound
	}
	return a, err
}

func (r *PostgresRepo) AcceptStep(ctx context.Context, adminID string, step int64) (bool, error) {
	res, err := r.db.ExecContext(ctx,
		`UPDATE admin_users SET totp_last_step = $2 WHERE id = $1 AND totp_last_step < $2`, adminID, step)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}

func (r *PostgresRepo) CreateSession(ctx context.Context, adminID, tokenHash string, expiresAt time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO admin_sessions (admin_id, token_hash, expires_at) VALUES ($1, $2, $3)`, adminID, tokenHash, expiresAt)
	return err
}

func (r *PostgresRepo) AdminBySession(ctx context.Context, tokenHash string, now time.Time) (Admin, error) {
	var a Admin
	err := r.db.QueryRowContext(ctx, `
		SELECT u.id, u.email FROM admin_sessions s JOIN admin_users u ON u.id = s.admin_id
		WHERE s.token_hash = $1 AND s.expires_at > $2`, tokenHash, now).Scan(&a.ID, &a.Email)
	if errors.Is(err, sql.ErrNoRows) {
		return Admin{}, ErrNotFound
	}
	return a, err
}

func (r *PostgresRepo) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM admin_sessions WHERE token_hash = $1`, tokenHash)
	return err
}

func (r *PostgresRepo) DeleteExpiredSessions(ctx context.Context, now time.Time) (int64, error) {
	res, err := r.db.ExecContext(ctx, `DELETE FROM admin_sessions WHERE expires_at <= $1`, now)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (r *PostgresRepo) Audit(ctx context.Context, a Admin, e Entry) error {
	before, err := jsonOrNil(e.Before)
	if err != nil {
		return err
	}
	after, err := jsonOrNil(e.After)
	if err != nil {
		return err
	}
	var target any
	if e.TargetID != "" {
		target = e.TargetID
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO admin_audit (admin_id, admin_email, action, target_type, target_id, before, after)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		a.ID, a.Email, e.Action, e.TargetType, target, before, after)
	return err
}

func (r *PostgresRepo) ListAudit(ctx context.Context, limit int) ([]AuditRow, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, occurred_at, admin_email, action, target_type, target_id::text, before, after
		FROM admin_audit ORDER BY id DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AuditRow{}
	for rows.Next() {
		var a AuditRow
		var before, after []byte
		if err := rows.Scan(&a.ID, &a.OccurredAt, &a.AdminEmail, &a.Action, &a.TargetType, &a.TargetID, &before, &after); err != nil {
			return nil, err
		}
		a.Before, a.After = before, after
		out = append(out, a)
	}
	return out, rows.Err()
}

func jsonOrNil(m map[string]any) (any, error) {
	if m == nil {
		return nil, nil
	}
	b, err := json.Marshal(m)
	return string(b), err
}
