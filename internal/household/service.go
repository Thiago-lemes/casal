package household

import (
	"context"
	"time"

	"casal/internal/auth"
)

const inviteTTL = 7 * 24 * time.Hour

type Service struct {
	repo Repository
	now  func() time.Time
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo, now: time.Now}
}

func (s *Service) Create(ctx context.Context, userID string) (string, error) {
	return s.repo.CreateHousehold(ctx, userID)
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
	raw, hash, err := auth.NewToken()
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
	return s.repo.AcceptInvite(ctx, auth.HashToken(rawToken), userID, s.now())
}
