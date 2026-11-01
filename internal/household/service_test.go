package household

import (
	"context"
	"errors"
	"testing"
	"time"

	"casal/internal/token"
)

// Embute a interface: só implementamos o que o teste usa. Chamar outro método causa panic.
type fakeRepo struct {
	Repository
	members  []Member
	invites  map[string]time.Time // hash do token -> validade
	inMember map[string]bool
}

func (f *fakeRepo) Members(context.Context, string) ([]Member, error) { return f.members, nil }

func (f *fakeRepo) CreateInvite(_ context.Context, _, _, tokenHash string, expiresAt time.Time) error {
	f.invites[tokenHash] = expiresAt
	return nil
}

func (f *fakeRepo) IsMember(_ context.Context, _, userID string) (bool, error) {
	return f.inMember[userID], nil
}

func TestInviteStoresOnlyTheHash(t *testing.T) {
	repo := &fakeRepo{members: []Member{{ID: "u1"}}, invites: map[string]time.Time{}}
	svc := NewService(repo)

	raw, expires, err := svc.Invite(context.Background(), "h1", "u1")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := repo.invites[raw]; ok {
		t.Error("o token bruto não pode ser guardado")
	}
	if _, ok := repo.invites[token.Hash(raw)]; !ok {
		t.Error("o hash do token deveria ter sido guardado")
	}
	if !expires.After(time.Now().Add(6 * 24 * time.Hour)) {
		t.Error("o convite deveria valer cerca de 7 dias")
	}
}

func TestInviteRefusedWhenHouseholdIsFull(t *testing.T) {
	repo := &fakeRepo{members: []Member{{ID: "u1"}, {ID: "u2"}}, invites: map[string]time.Time{}}
	_, _, err := NewService(repo).Invite(context.Background(), "h1", "u1")
	if !errors.Is(err, ErrHouseholdFull) {
		t.Errorf("esperava ErrHouseholdFull, veio %v", err)
	}
}

func TestAreMembers(t *testing.T) {
	repo := &fakeRepo{inMember: map[string]bool{"u1": true, "u2": true}}
	svc := NewService(repo)

	if ok, _ := svc.AreMembers(context.Background(), "h1", "u1", "u2"); !ok {
		t.Error("os dois são membros")
	}
	if ok, _ := svc.AreMembers(context.Background(), "h1", "u1", "u9"); ok {
		t.Error("u9 não é membro")
	}
}
