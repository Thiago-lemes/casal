package admin

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"
)

// ---- o que o painel precisa das outras features (o internal/app liga) ----

// Person é uma pessoa, com o que o suporte precisa para localizar a conta.
type Person struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

// HouseholdSummary é um casal nos resultados da busca.
type HouseholdSummary struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	Members   []Person  `json:"members"`
}

// HouseholdDetail é a ficha do casal: só metadados e contagens, nunca conteúdo. As contagens são o total do módulo,
// sem distinguir itens pessoais de itens do casal.
type HouseholdDetail struct {
	HouseholdSummary
	Modules     []string       `json:"modules"`
	LastUsedAt  *time.Time     `json:"last_used_at"`
	PushDevices int            `json:"push_devices"`
	Counts      map[string]int `json:"counts"`
}

// UserDetail é a ficha de uma pessoa.
type UserDetail struct {
	Person
	CreatedAt   time.Time  `json:"created_at"`
	BlockedAt   *time.Time `json:"blocked_at"`
	HouseholdID *string    `json:"household_id"`
	LastUsedAt  *time.Time `json:"last_used_at"`
	PushDevices int        `json:"push_devices"`
}

// Directory lê os metadados de casais e pessoas (contagens, nunca conteúdo).
type Directory interface {
	SearchHouseholds(ctx context.Context, q string) ([]HouseholdSummary, error)
	Household(ctx context.Context, id string) (HouseholdDetail, error) // ErrNotFound se não existe
	User(ctx context.Context, id string) (UserDetail, error)           // ErrNotFound se não existe
}

// ModuleState é o estado de um módulo de um casal.
type ModuleState struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

// Modules liga e desliga módulos pelo mesmo caminho do app (mesmos ganchos e regras do module.Service).
type Modules interface {
	List(ctx context.Context, householdID string) ([]ModuleState, error)
	Set(ctx context.Context, householdID, name string, enabled bool) error
}

// Users bloqueia e desbloqueia contas.
type Users interface {
	SetBlocked(ctx context.Context, userID string, blocked bool) error
}

// SignupCode é um código de cadastro do beta, sem o valor bruto.
type SignupCode struct {
	ID        string     `json:"id"`
	Note      string     `json:"note"`
	ExpiresAt time.Time  `json:"expires_at"`
	MaxUses   int        `json:"max_uses"`
	Uses      int        `json:"uses"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// SignupCodes administra os códigos de cadastro.
type SignupCodes interface {
	Create(ctx context.Context, note string, maxUses int, ttl time.Duration, createdBy string) (SignupCode, string, error)
	List(ctx context.Context) ([]SignupCode, error)
	Revoke(ctx context.Context, id string) error
}

// FeedbackItem é uma mensagem que a pessoa escreveu para o time. É a única exceção a "nunca ver conteúdo".
type FeedbackItem struct {
	ID          string     `json:"id"`
	HouseholdID string     `json:"household_id"`
	Author      string     `json:"author"` // nome da pessoa
	Message     string     `json:"message"`
	Screen      string     `json:"screen"`
	CreatedAt   time.Time  `json:"created_at"`
	ReadAt      *time.Time `json:"read_at"`
}

// Feedback lista as mensagens e as marca como lidas.
type Feedback interface {
	List(ctx context.Context, onlyUnread bool) ([]FeedbackItem, error)
	MarkRead(ctx context.Context, id string) error
}

// ---- ações do painel: cada uma passa pelo serviço de domínio e grava auditoria ----

const maxSearch = 100

// Search procura casais por e-mail ou nome de quem pertence a eles.
func (s *Service) Search(ctx context.Context, q string) ([]HouseholdSummary, error) {
	if len([]rune(q)) < 2 || len(q) > maxSearch {
		return nil, fmt.Errorf("%w: digite ao menos 2 letras do e-mail ou do nome", ErrInvalid)
	}
	return s.dir.SearchHouseholds(ctx, q)
}

// HouseholdFile devolve a ficha do casal e grava household_viewed. Sem a auditoria, nada é devolvido.
func (s *Service) HouseholdFile(ctx context.Context, a Admin, id string) (HouseholdDetail, []ModuleState, error) {
	d, err := s.dir.Household(ctx, id)
	if err != nil {
		return HouseholdDetail{}, nil, err
	}
	states, err := s.modules.List(ctx, id)
	if err != nil {
		return HouseholdDetail{}, nil, err
	}
	if err := s.repo.Audit(ctx, a, Entry{Action: ActionHouseholdView, TargetType: TargetHousehold, TargetID: id}); err != nil {
		return HouseholdDetail{}, nil, err
	}
	return d, states, nil
}

// UserFile devolve a ficha da pessoa e grava user_viewed.
func (s *Service) UserFile(ctx context.Context, a Admin, id string) (UserDetail, error) {
	u, err := s.dir.User(ctx, id)
	if err != nil {
		return UserDetail{}, err
	}
	if err := s.repo.Audit(ctx, a, Entry{Action: ActionUserView, TargetType: TargetUser, TargetID: id}); err != nil {
		return UserDetail{}, err
	}
	return u, nil
}

// SetModule liga ou desliga um módulo do casal, pelo module.Service (mesmos ganchos e regras do app).
func (s *Service) SetModule(ctx context.Context, a Admin, householdID, name string, enabled bool) ([]ModuleState, error) {
	if _, err := s.dir.Household(ctx, householdID); err != nil {
		return nil, err
	}
	before, err := s.modules.List(ctx, householdID)
	if err != nil {
		return nil, err
	}
	was := false
	for _, m := range before {
		if m.Name == name {
			was = m.Enabled
		}
	}
	if err := s.modules.Set(ctx, householdID, name, enabled); err != nil {
		return nil, err
	}
	action := ActionModuleDisabled
	if enabled {
		action = ActionModuleEnabled
	}
	err = s.repo.Audit(ctx, a, Entry{
		Action: action, TargetType: TargetHousehold, TargetID: householdID,
		Before: map[string]any{"module": name, "enabled": was},
		After:  map[string]any{"module": name, "enabled": enabled},
	})
	if err != nil {
		return nil, err
	}
	return s.modules.List(ctx, householdID)
}

// SetBlocked bloqueia ou desbloqueia a pessoa. Não apaga dado nenhum.
func (s *Service) SetBlocked(ctx context.Context, a Admin, userID string, blocked bool) (UserDetail, error) {
	u, err := s.dir.User(ctx, userID)
	if err != nil {
		return UserDetail{}, err
	}
	if err := s.users.SetBlocked(ctx, userID, blocked); err != nil {
		return UserDetail{}, err
	}
	action := ActionUserUnblocked
	if blocked {
		action = ActionUserBlocked
	}
	err = s.repo.Audit(ctx, a, Entry{
		Action: action, TargetType: TargetUser, TargetID: userID,
		Before: map[string]any{"blocked": u.BlockedAt != nil},
		After:  map[string]any{"blocked": blocked},
	})
	if err != nil {
		return UserDetail{}, err
	}
	return s.dir.User(ctx, userID)
}

// CreateSignupCode cria um código de cadastro. O valor bruto só aparece nesta resposta.
func (s *Service) CreateSignupCode(ctx context.Context, a Admin, note string, maxUses, days int) (SignupCode, string, error) {
	c, raw, err := s.codes.Create(ctx, note, maxUses, time.Duration(days)*24*time.Hour, a.ID)
	if err != nil {
		return SignupCode{}, "", err
	}
	err = s.repo.Audit(ctx, a, Entry{
		Action: ActionInviteCreated, TargetType: TargetInvite, TargetID: c.ID,
		After: map[string]any{"max_uses": c.MaxUses, "days": days},
	})
	if err != nil {
		return SignupCode{}, "", err
	}
	return c, raw, nil
}

func (s *Service) SignupCodes(ctx context.Context) ([]SignupCode, error) { return s.codes.List(ctx) }

func (s *Service) RevokeSignupCode(ctx context.Context, a Admin, id string) error {
	if err := s.codes.Revoke(ctx, id); err != nil {
		return err
	}
	return s.repo.Audit(ctx, a, Entry{Action: ActionInviteRevoked, TargetType: TargetInvite, TargetID: id})
}

// FeedbackList lista as mensagens (mais novas primeiro).
func (s *Service) FeedbackList(ctx context.Context, onlyUnread bool) ([]FeedbackItem, error) {
	if s.feedback == nil {
		return []FeedbackItem{}, nil
	}
	return s.feedback.List(ctx, onlyUnread)
}

func (s *Service) FeedbackRead(ctx context.Context, a Admin, id string) error {
	if s.feedback == nil {
		return ErrNotFound
	}
	if err := s.feedback.MarkRead(ctx, id); err != nil {
		return err
	}
	return s.repo.Audit(ctx, a, Entry{Action: ActionFeedbackRead, TargetType: TargetFeedback, TargetID: id})
}

// randomPassword gera uma senha de 24 caracteres para o comando de criação de conta.
func randomPassword() (string, error) {
	b := make([]byte, 18)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
