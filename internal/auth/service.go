package auth

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"
)

type Service struct {
	repo       Repository
	sessionTTL time.Duration
	now        func() time.Time
	dummyHash  string // usado para gastar o mesmo tempo quando o e-mail não existe
}

func NewService(repo Repository, sessionTTL time.Duration) (*Service, error) {
	dummy, err := HashPassword("senha-que-nunca-sera-usada")
	if err != nil {
		return nil, err
	}
	return &Service{repo: repo, sessionTTL: sessionTTL, now: time.Now, dummyHash: dummy}, nil
}

// Register cria a conta e já abre uma sessão. Devolve o token bruto para o cookie.
func (s *Service) Register(ctx context.Context, email, name, password string) (User, string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	name = strings.TrimSpace(name)
	if addr, err := mail.ParseAddress(email); err != nil || addr.Address != email {
		return User{}, "", fmt.Errorf("%w: e-mail inválido", ErrInvalidInput)
	}
	if name == "" || len(name) > 80 {
		return User{}, "", fmt.Errorf("%w: nome é obrigatório (até 80 caracteres)", ErrInvalidInput)
	}
	if len(password) < 8 || len(password) > 128 {
		return User{}, "", fmt.Errorf("%w: a senha precisa ter de 8 a 128 caracteres", ErrInvalidInput)
	}

	hash, err := HashPassword(password)
	if err != nil {
		return User{}, "", err
	}
	u, err := s.repo.CreateUser(ctx, email, name, hash)
	if err != nil {
		return User{}, "", err
	}
	token, err := s.openSession(ctx, u.ID)
	return u, token, err
}

func (s *Service) Login(ctx context.Context, email, password string) (User, string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	u, hash, err := s.repo.UserByEmail(ctx, email)
	if errors.Is(err, ErrNotFound) {
		VerifyPassword(password, s.dummyHash) // mesmo custo de tempo, resposta igual
		return User{}, "", ErrInvalidCredentials
	}
	if err != nil {
		return User{}, "", err
	}
	if !VerifyPassword(password, hash) {
		return User{}, "", ErrInvalidCredentials
	}
	token, err := s.openSession(ctx, u.ID)
	return u, token, err
}

func (s *Service) Logout(ctx context.Context, rawToken string) error {
	return s.repo.DeleteSession(ctx, HashToken(rawToken))
}

func (s *Service) UserFromToken(ctx context.Context, rawToken string) (User, error) {
	return s.repo.UserBySession(ctx, HashToken(rawToken), s.now())
}

func (s *Service) SessionTTL() time.Duration { return s.sessionTTL }

func (s *Service) openSession(ctx context.Context, userID string) (string, error) {
	raw, hash, err := NewToken()
	if err != nil {
		return "", err
	}
	if err := s.repo.CreateSession(ctx, userID, hash, s.now().Add(s.sessionTTL)); err != nil {
		return "", err
	}
	return raw, nil
}
