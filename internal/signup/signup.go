// Package signup controla quem pode criar conta durante o beta: códigos de cadastro com validade, limite de usos
// e revogação. O código é mostrado uma vez; só o hash fica no banco, como o token de convite do casal.
package signup

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"casal/internal/apperr"
	"casal/internal/token"
)

var (
	ErrInvalidInput = apperr.New(apperr.Invalid, "dados inválidos")
	ErrNotFound     = apperr.NewCoded(apperr.NotFound, "signup_code_not_found", "código de cadastro não encontrado")
)

// Code é um código de cadastro, sem o valor bruto (que só existe no momento da criação).
type Code struct {
	ID        string     `json:"id"`
	Note      string     `json:"note"`
	ExpiresAt time.Time  `json:"expires_at"`
	MaxUses   int        `json:"max_uses"`
	Uses      int        `json:"uses"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// Service cria, lista, revoga e consome códigos.
type Service struct {
	db  *sql.DB
	now func() time.Time
}

func NewService(db *sql.DB) *Service { return &Service{db: db, now: time.Now} }

const (
	maxNote      = 100
	maxTTL       = 90 * 24 * time.Hour
	maxUsesLimit = 100
)

// Create gera um código novo. Devolve o valor bruto, que não pode ser recuperado depois. createdBy é a conta de
// administração (vazio quando vem da linha de comando).
func (s *Service) Create(ctx context.Context, note string, maxUses int, ttl time.Duration, createdBy string) (Code, string, error) {
	note = strings.TrimSpace(note)
	if utf8.RuneCountInString(note) > maxNote {
		return Code{}, "", fmt.Errorf("%w: o rótulo tem até %d caracteres", ErrInvalidInput, maxNote)
	}
	if maxUses < 1 || maxUses > maxUsesLimit {
		return Code{}, "", fmt.Errorf("%w: os usos vão de 1 a %d", ErrInvalidInput, maxUsesLimit)
	}
	if ttl <= 0 || ttl > maxTTL {
		return Code{}, "", fmt.Errorf("%w: a validade vai até 90 dias", ErrInvalidInput)
	}
	raw, hash, err := token.New()
	if err != nil {
		return Code{}, "", err
	}
	var by any
	if createdBy != "" {
		by = createdBy
	}
	var c Code
	err = s.db.QueryRowContext(ctx, `
		INSERT INTO signup_codes (code_hash, note, expires_at, max_uses, created_by)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, note, expires_at, max_uses, uses, revoked_at, created_at`,
		hash, note, s.now().Add(ttl), maxUses, by).
		Scan(&c.ID, &c.Note, &c.ExpiresAt, &c.MaxUses, &c.Uses, &c.RevokedAt, &c.CreatedAt)
	return c, raw, err
}

// List devolve os códigos, os mais novos primeiro.
func (s *Service) List(ctx context.Context) ([]Code, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, note, expires_at, max_uses, uses, revoked_at, created_at
		FROM signup_codes ORDER BY created_at DESC LIMIT 200`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Code{}
	for rows.Next() {
		var c Code
		if err := rows.Scan(&c.ID, &c.Note, &c.ExpiresAt, &c.MaxUses, &c.Uses, &c.RevokedAt, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Revoke impede novos usos. Quem já se cadastrou com o código continua com a conta.
func (s *Service) Revoke(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE signup_codes SET revoked_at = coalesce(revoked_at, $2) WHERE id = $1`, id, s.now())
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// Consume reserva um uso do código. ok=false quando o código não existe, expirou, foi revogado ou esgotou: quem
// chama não distingue os casos (a resposta é a mesma). release devolve o uso se o cadastro falhar depois (e-mail já usado).
// O UPDATE é atômico, então dois cadastros ao mesmo tempo com o último uso não passam os dois.
func (s *Service) Consume(ctx context.Context, raw string) (release func(context.Context), ok bool, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > 200 {
		return nil, false, nil
	}
	hash := token.Hash(raw)
	var id string
	err = s.db.QueryRowContext(ctx, `
		UPDATE signup_codes SET uses = uses + 1
		WHERE code_hash = $1 AND revoked_at IS NULL AND expires_at > $2 AND uses < max_uses
		RETURNING id`, hash, s.now()).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return func(ctx context.Context) {
		_, _ = s.db.ExecContext(ctx, `UPDATE signup_codes SET uses = uses - 1 WHERE id = $1 AND uses > 0`, id)
	}, true, nil
}
