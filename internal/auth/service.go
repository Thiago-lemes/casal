package auth

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"runtime"
	"strings"
	"time"

	"casal/internal/token"
)

type Service struct {
	repo       Repository
	sessionTTL time.Duration
	now        func() time.Time
	dummyHash  string        // usado para gastar o mesmo tempo quando o e-mail não existe
	hashSlots  chan struct{} // limita quantos hashes de senha rodam ao mesmo tempo
}

func NewService(repo Repository, sessionTTL time.Duration) (*Service, error) {
	dummy, err := HashPassword("senha-que-nunca-sera-usada")
	if err != nil {
		return nil, err
	}
	// o PBKDF2 é caro de propósito; sem limite, muitas tentativas em paralelo esgotam a CPU
	slots := make(chan struct{}, max(2, runtime.NumCPU()))
	return &Service{repo: repo, sessionTTL: sessionTTL, now: time.Now, dummyHash: dummy, hashSlots: slots}, nil
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

	hash, err := s.hashPassword(ctx, password)
	if err != nil {
		return User{}, "", err
	}
	u, err := s.repo.CreateUser(ctx, email, name, hash)
	if err != nil {
		return User{}, "", err
	}
	session, err := s.openSession(ctx, u.ID)
	return u, session, err
}

func (s *Service) Login(ctx context.Context, email, password string) (User, string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	u, hash, err := s.repo.UserByEmail(ctx, email)
	if errors.Is(err, ErrNotFound) {
		_, _ = s.verifyPassword(ctx, password, s.dummyHash) // mesmo custo de tempo, resposta igual
		return User{}, "", ErrInvalidCredentials
	}
	if err != nil {
		return User{}, "", err
	}
	ok, err := s.verifyPassword(ctx, password, hash)
	if err != nil {
		return User{}, "", err
	}
	if !ok {
		return User{}, "", ErrInvalidCredentials
	}
	session, err := s.openSession(ctx, u.ID)
	return u, session, err
}

func (s *Service) Logout(ctx context.Context, rawToken string) error {
	return s.repo.DeleteSession(ctx, token.Hash(rawToken))
}

func (s *Service) UserFromToken(ctx context.Context, rawToken string) (User, error) {
	return s.repo.UserBySession(ctx, token.Hash(rawToken), s.now())
}

func (s *Service) SessionTTL() time.Duration { return s.sessionTTL }

func (s *Service) openSession(ctx context.Context, userID string) (string, error) {
	raw, hash, err := token.New()
	if err != nil {
		return "", err
	}
	if err := s.repo.CreateSession(ctx, userID, hash, s.now().Add(s.sessionTTL)); err != nil {
		return "", err
	}
	return raw, nil
}

// acquire espera uma vaga para calcular um hash; desiste se o contexto acabar.
func (s *Service) acquire(ctx context.Context) (release func(), err error) {
	select {
	case s.hashSlots <- struct{}{}:
		return func() { <-s.hashSlots }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *Service) hashPassword(ctx context.Context, password string) (string, error) {
	release, err := s.acquire(ctx)
	if err != nil {
		return "", err
	}
	defer release()
	return HashPassword(password)
}

func (s *Service) verifyPassword(ctx context.Context, password, hash string) (bool, error) {
	release, err := s.acquire(ctx)
	if err != nil {
		return false, err
	}
	defer release()
	return VerifyPassword(password, hash), nil
}

// LogoutAll encerra as sessões da pessoa em todos os aparelhos.
func (s *Service) LogoutAll(ctx context.Context, userID string) error {
	return s.repo.DeleteUserSessions(ctx, userID)
}

// PurgeExpired apaga as sessões vencidas. Rode periodicamente.
func (s *Service) PurgeExpired(ctx context.Context) (int64, error) {
	return s.repo.DeleteExpiredSessions(ctx, s.now())
}
