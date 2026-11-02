// Package module liga e desliga os módulos opcionais de cada casal. Desligar esconde e não apaga.
package module

import (
	"context"
	"time"

	"casal/internal/apperr"
)

// Nomes dos módulos que existem. A tabela só aceita estes.
const (
	Finance = "finance"
	Goals   = "goals"
)

var known = []string{Finance, Goals}

var (
	ErrNotFound = apperr.NewCoded(apperr.NotFound, "module_not_found", "módulo não encontrado")
	ErrDisabled = apperr.NewCoded(apperr.Forbidden, "module_disabled", "esse módulo está desligado para o casal")
)

type Status struct {
	Name      string     `json:"name"`
	Enabled   bool       `json:"enabled"`
	EnabledAt *time.Time `json:"enabled_at,omitempty"`
}

type Service struct {
	repo  Repository
	hooks map[string][]Hook
	after map[string][]func(ctx context.Context, householdID string, enabled bool)
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo, hooks: map[string][]Hook{}, after: map[string][]func(context.Context, string, bool){}}
}

// OnEnable registra um gancho que roda quando o módulo passa de desligado a ligado, na mesma transação.
// Como religar também conta, o gancho precisa ser idempotente (por exemplo, as categorias padrão de Finanças
// usam a chave e ignoram as que já existem). Registre na montagem do app, antes de atender requisições.
func (s *Service) OnEnable(name string, h Hook) { s.hooks[name] = append(s.hooks[name], h) }

// AfterChange registra uma função que roda depois de o módulo ser ligado ou desligado, já fora da transação
// (é onde se mexe em outras features, como os lembretes). Não devolve erro: quem registra trata e registra a falha,
// e um job de reconciliação cobre o que não deu certo.
func (s *Service) AfterChange(name string, f func(ctx context.Context, householdID string, enabled bool)) {
	s.after[name] = append(s.after[name], f)
}

func (s *Service) List(ctx context.Context, householdID string) ([]Status, error) {
	on, err := s.repo.Enabled(ctx, householdID)
	if err != nil {
		return nil, err
	}
	out := make([]Status, 0, len(known))
	for _, name := range known {
		st := Status{Name: name}
		if at, ok := on[name]; ok {
			st.Enabled, st.EnabledAt = true, &at
		}
		out = append(out, st)
	}
	return out, nil
}

func (s *Service) Set(ctx context.Context, householdID, name string, enabled bool) (Status, error) {
	if !isKnown(name) {
		return Status{}, ErrNotFound
	}
	if enabled {
		if _, err := s.repo.Enable(ctx, householdID, name, s.hooks[name]...); err != nil {
			return Status{}, err
		}
	} else if err := s.repo.Disable(ctx, householdID, name); err != nil {
		return Status{}, err
	}
	for _, f := range s.after[name] {
		f(ctx, householdID, enabled)
	}
	return s.status(ctx, householdID, name)
}

func (s *Service) status(ctx context.Context, householdID, name string) (Status, error) {
	on, err := s.repo.Enabled(ctx, householdID)
	if err != nil {
		return Status{}, err
	}
	st := Status{Name: name}
	if at, ok := on[name]; ok {
		st.Enabled, st.EnabledAt = true, &at
	}
	return st, nil
}

// IsEnabled diz se o casal tem o módulo ligado.
func (s *Service) IsEnabled(ctx context.Context, householdID, name string) (bool, error) {
	return s.repo.IsEnabled(ctx, householdID, name)
}

func isKnown(name string) bool {
	for _, k := range known {
		if k == name {
			return true
		}
	}
	return false
}
