package household

import (
	"context"
	"time"

	"casal/internal/db"
	"casal/internal/token"
)

const inviteTTL = 7 * 24 * time.Hour

type Service struct {
	repo Repository

	// ganchos de mudança de casal; registre-os na montagem do app, antes de atender requisições
	onLeave []db.Hook
	onJoin  []db.Hook
	now     func() time.Time
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo, now: time.Now}
}

func (s *Service) Create(ctx context.Context, userID string) (string, error) {
	return s.repo.CreateHousehold(ctx, userID, s.onJoin...)
}

func (s *Service) HouseholdOf(ctx context.Context, userID string) (string, error) {
	return s.repo.HouseholdOf(ctx, userID)
}

func (s *Service) Members(ctx context.Context, householdID string) ([]Member, error) {
	return s.repo.Members(ctx, householdID)
}

// AreMembers confere se todas as pessoas informadas pertencem ao casal.
// É a função pública que outras features usam, em vez de ler as tabelas daqui.
func (s *Service) AreMembers(ctx context.Context, householdID string, userIDs ...string) (bool, error) {
	for _, id := range userIDs {
		ok, err := s.repo.IsMember(ctx, householdID, id)
		if err != nil || !ok {
			return false, err
		}
	}
	return true, nil
}

// Invite gera um convite. Devolve o token bruto, que só existe neste momento.
func (s *Service) Invite(ctx context.Context, householdID, userID string) (string, time.Time, error) {
	members, err := s.repo.Members(ctx, householdID)
	if err != nil {
		return "", time.Time{}, err
	}
	if len(members) >= MaxMembers {
		return "", time.Time{}, ErrHouseholdFull
	}
	raw, hash, err := token.New()
	if err != nil {
		return "", time.Time{}, err
	}
	expires := s.now().Add(inviteTTL)
	if err := s.repo.CreateInvite(ctx, householdID, userID, hash, expires); err != nil {
		return "", time.Time{}, err
	}
	return raw, expires, nil
}

func (s *Service) Accept(ctx context.Context, userID, rawToken string) (string, error) {
	return s.repo.AcceptInvite(ctx, token.Hash(rawToken), userID, s.now(), s.onJoin...)
}

// PurgeStaleInvites apaga os convites que já não servem para nada (usados ou vencidos há 30 dias).
// Rode periodicamente.
func (s *Service) PurgeStaleInvites(ctx context.Context) (int64, error) {
	return s.repo.DeleteStaleInvites(ctx, s.now(), 30*24*time.Hour)
}

// OtherMember devolve a outra pessoa do casal (para o rodízio de tarefas). ok é falso quando
// o casal só tem a própria pessoa. É a função pública que outras features usam.
func (s *Service) OtherMember(ctx context.Context, householdID, userID string) (string, bool, error) {
	members, err := s.repo.Members(ctx, householdID)
	if err != nil {
		return "", false, err
	}
	for _, m := range members {
		if m.ID != userID {
			return m.ID, true, nil
		}
	}
	return "", false, nil
}

// OnLeave registra um gancho que roda quando alguém sai do casal, na mesma transação da saída.
// É assim que cada feature passa adiante o que era compartilhado, sem que o household as conheça.
func (s *Service) OnLeave(h db.Hook) { s.onLeave = append(s.onLeave, h) }

// OnJoin registra um gancho que roda quando alguém passa a ter um casal (criando ou aceitando um convite),
// na mesma transação. É assim que o que é pessoal acompanha a pessoa.
func (s *Service) OnJoin(h db.Hook) { s.onJoin = append(s.onJoin, h) }

// Leave tira a pessoa do casal. O que era compartilhado fica com quem permanece; o que é pessoal
// acompanha quem sai (as features fazem isso pelos ganchos). Quem sai fica sem casal e pode criar outro
// ou aceitar um convite.
func (s *Service) Leave(ctx context.Context, userID string) error {
	return s.repo.LeaveHousehold(ctx, userID, s.onLeave...)
}
