// Package admin é o painel de suporte do beta (/admin): contas de administração separadas das dos casais, com
// 2FA obrigatório, sessão curta e auditoria de tudo. Só mostra metadados e contagens, nunca o conteúdo dos casais.
//
// O painel não lê as tabelas de negócio: usa interfaces pequenas declaradas aqui (Directory, Modules, Users,
// SignupCodes, Feedback), que o internal/app liga às features.
package admin

import (
	"context"
	"fmt"
	"net/mail"
	"runtime"
	"strings"
	"time"

	"casal/internal/password"
	"casal/internal/token"
)

// SessionTTL é a validade da sessão do painel: curta de propósito.
const SessionTTL = 8 * time.Hour

// Ações da auditoria (a tabela tem um CHECK com a mesma lista).
const (
	ActionLogin          = "admin_login"
	ActionHouseholdView  = "household_viewed"
	ActionUserView       = "user_viewed"
	ActionModuleEnabled  = "module_enabled"
	ActionModuleDisabled = "module_disabled"
	ActionUserBlocked    = "user_blocked"
	ActionUserUnblocked  = "user_unblocked"
	ActionInviteCreated  = "invite_created"
	ActionInviteRevoked  = "invite_revoked"
	ActionFeedbackRead   = "feedback_read"
)

// Tipos de alvo da auditoria.
const (
	TargetAdmin     = "admin"
	TargetHousehold = "household"
	TargetUser      = "user"
	TargetInvite    = "invite"
	TargetFeedback  = "feedback"
)

type Service struct {
	repo      Repository
	now       func() time.Time
	dummyHash string
	hashSlots chan struct{}

	dir      Directory
	modules  Modules
	users    Users
	codes    SignupCodes
	feedback Feedback
}

// Deps reúne o que o painel pede às outras features (ligado no internal/app).
type Deps struct {
	Directory   Directory
	Modules     Modules
	Users       Users
	SignupCodes SignupCodes
	Feedback    Feedback
}

func NewService(repo Repository, d Deps) (*Service, error) {
	dummy, err := password.Hash("senha-que-nunca-sera-usada")
	if err != nil {
		return nil, err
	}
	return &Service{
		repo: repo, now: time.Now, dummyHash: dummy,
		hashSlots: make(chan struct{}, max(2, runtime.NumCPU())),
		dir:       d.Directory, modules: d.Modules, users: d.Users, codes: d.SignupCodes, feedback: d.Feedback,
	}, nil
}

// Created é o que o comando de criação mostra uma única vez.
type Created struct {
	Admin    Admin
	Password string
	Secret   string
	URI      string
}

// CreateAdmin cria uma conta de administração com senha aleatória e segredo TOTP novo. Não há cadastro pela API:
// só quem tem acesso ao servidor cria contas, pelo comando go run ./cmd/admin admin create.
func (s *Service) CreateAdmin(ctx context.Context, email string) (Created, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if addr, err := mail.ParseAddress(email); err != nil || addr.Address != email {
		return Created{}, fmt.Errorf("%w: e-mail inválido", ErrInvalid)
	}
	pass, err := randomPassword()
	if err != nil {
		return Created{}, err
	}
	hash, err := s.hash(ctx, pass)
	if err != nil {
		return Created{}, err
	}
	secret, err := NewSecret()
	if err != nil {
		return Created{}, err
	}
	a, err := s.repo.CreateAdmin(ctx, email, hash, secret)
	if err != nil {
		return Created{}, err
	}
	return Created{Admin: a, Password: pass, Secret: secret, URI: OtpauthURI(email, secret)}, nil
}

// Login confere e-mail, senha e código TOTP. Qualquer falha dá o mesmo erro, e o tempo gasto é parecido
// com e sem conta. Devolve o token bruto da sessão (para o cookie).
func (s *Service) Login(ctx context.Context, email, pass, code string) (Admin, string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	acc, err := s.repo.AccountByEmail(ctx, email)
	if err != nil {
		if err == ErrNotFound {
			_, _ = s.verify(ctx, pass, s.dummyHash)
			return Admin{}, "", ErrLoginFailed
		}
		return Admin{}, "", err
	}
	ok, err := s.verify(ctx, pass, acc.passwordHash)
	if err != nil {
		return Admin{}, "", err
	}
	step, codeOK := verifyTOTP(acc.totpSecret, code, s.now(), acc.lastStep)
	if !ok || !codeOK {
		return Admin{}, "", ErrLoginFailed
	}
	// dois logins ao mesmo tempo com o mesmo código: só o primeiro grava o passo
	fresh, err := s.repo.AcceptStep(ctx, acc.ID, step)
	if err != nil {
		return Admin{}, "", err
	}
	if !fresh {
		return Admin{}, "", ErrLoginFailed
	}
	raw, hash, err := token.New()
	if err != nil {
		return Admin{}, "", err
	}
	if err := s.repo.CreateSession(ctx, acc.ID, hash, s.now().Add(SessionTTL)); err != nil {
		return Admin{}, "", err
	}
	if err := s.repo.Audit(ctx, acc.Admin, Entry{Action: ActionLogin, TargetType: TargetAdmin, TargetID: acc.ID}); err != nil {
		return Admin{}, "", err
	}
	return acc.Admin, raw, nil
}

func (s *Service) Logout(ctx context.Context, rawToken string) error {
	return s.repo.DeleteSession(ctx, token.Hash(rawToken))
}

// AdminFromToken devolve a conta dona da sessão, ou ErrNotFound se ela não existe ou venceu.
func (s *Service) AdminFromToken(ctx context.Context, rawToken string) (Admin, error) {
	return s.repo.AdminBySession(ctx, token.Hash(rawToken), s.now())
}

// PurgeExpired apaga as sessões do painel vencidas. Rode periodicamente.
func (s *Service) PurgeExpired(ctx context.Context) (int64, error) {
	return s.repo.DeleteExpiredSessions(ctx, s.now())
}

// Audit lista as últimas ações.
func (s *Service) Audit(ctx context.Context, limit int) ([]AuditRow, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	return s.repo.ListAudit(ctx, limit)
}

func (s *Service) hash(ctx context.Context, pass string) (string, error) {
	release, err := s.acquire(ctx)
	if err != nil {
		return "", err
	}
	defer release()
	return password.Hash(pass)
}

func (s *Service) verify(ctx context.Context, pass, hash string) (bool, error) {
	release, err := s.acquire(ctx)
	if err != nil {
		return false, err
	}
	defer release()
	return password.Verify(pass, hash), nil
}

func (s *Service) acquire(ctx context.Context) (func(), error) {
	select {
	case s.hashSlots <- struct{}{}:
		return func() { <-s.hashSlots }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
