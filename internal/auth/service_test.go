package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

type fakeRepo struct {
	users    map[string]User // por e-mail
	hashes   map[string]string
	sessions map[string]fakeSession // por hash do token
	nextID   int
}

type fakeSession struct {
	userID  string
	expires time.Time
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{users: map[string]User{}, hashes: map[string]string{}, sessions: map[string]fakeSession{}}
}

func (f *fakeRepo) CreateUser(_ context.Context, email, name, hash string) (User, error) {
	if _, ok := f.users[email]; ok {
		return User{}, ErrEmailTaken
	}
	f.nextID++
	u := User{ID: fmt.Sprintf("u%d", f.nextID), Email: email, Name: name, Timezone: "America/Sao_Paulo"}
	f.users[email] = u
	f.hashes[email] = hash
	return u, nil
}

func (f *fakeRepo) UserByEmail(_ context.Context, email string) (User, string, error) {
	u, ok := f.users[email]
	if !ok {
		return User{}, "", ErrNotFound
	}
	return u, f.hashes[email], nil
}

func (f *fakeRepo) CreateSession(_ context.Context, userID, tokenHash string, expiresAt time.Time) error {
	f.sessions[tokenHash] = fakeSession{userID: userID, expires: expiresAt}
	return nil
}

func (f *fakeRepo) UserBySession(_ context.Context, tokenHash string, now time.Time) (User, error) {
	s, ok := f.sessions[tokenHash]
	if !ok || !s.expires.After(now) {
		return User{}, ErrNotFound
	}
	for _, u := range f.users {
		if u.ID == s.userID {
			return u, nil
		}
	}
	return User{}, ErrNotFound
}

func (f *fakeRepo) DeleteSession(_ context.Context, tokenHash string) error {
	delete(f.sessions, tokenHash)
	return nil
}

func newTestService(t *testing.T) *Service {
	t.Helper()
	svc, err := NewService(newFakeRepo(), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

func TestRegisterAndLogin(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)

	u, token, err := svc.Register(ctx, "  Ana@Exemplo.com ", "Ana", "senha-forte-123")
	if err != nil {
		t.Fatal(err)
	}
	if u.Email != "ana@exemplo.com" {
		t.Errorf("o e-mail deveria ser normalizado, veio %q", u.Email)
	}
	got, err := svc.UserFromToken(ctx, token)
	if err != nil || got.ID != u.ID {
		t.Fatalf("a sessão do cadastro deveria valer: %v", err)
	}

	if _, _, err := svc.Login(ctx, "ana@exemplo.com", "senha-forte-123"); err != nil {
		t.Errorf("login correto falhou: %v", err)
	}
	if _, _, err := svc.Login(ctx, "ana@exemplo.com", "errada"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("senha errada deveria dar ErrInvalidCredentials, deu %v", err)
	}
	if _, _, err := svc.Login(ctx, "ninguem@exemplo.com", "qualquer"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("e-mail inexistente deveria dar o mesmo erro, deu %v", err)
	}
}

func TestRegisterValidation(t *testing.T) {
	svc := newTestService(t)
	cases := []struct{ name, email, user, pass string }{
		{"e-mail inválido", "nao-e-email", "Ana", "senha-forte-123"},
		{"e-mail com nome", "Ana <ana@exemplo.com>", "Ana", "senha-forte-123"},
		{"nome vazio", "ana@exemplo.com", "  ", "senha-forte-123"},
		{"senha curta", "ana@exemplo.com", "Ana", "1234567"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, _, err := svc.Register(context.Background(), c.email, c.user, c.pass)
			if !errors.Is(err, ErrInvalidInput) {
				t.Errorf("esperava ErrInvalidInput, veio %v", err)
			}
		})
	}
}

func TestRegisterDuplicateEmail(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)
	if _, _, err := svc.Register(ctx, "ana@exemplo.com", "Ana", "senha-forte-123"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Register(ctx, "ANA@exemplo.com", "Outra", "senha-forte-123"); !errors.Is(err, ErrEmailTaken) {
		t.Errorf("esperava ErrEmailTaken, veio %v", err)
	}
}

func TestLogoutEndsSession(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)
	_, token, err := svc.Register(ctx, "ana@exemplo.com", "Ana", "senha-forte-123")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Logout(ctx, token); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UserFromToken(ctx, token); err == nil {
		t.Error("a sessão deveria ter sido encerrada")
	}
}

func TestExpiredSession(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)
	_, token, err := svc.Register(ctx, "ana@exemplo.com", "Ana", "senha-forte-123")
	if err != nil {
		t.Fatal(err)
	}
	svc.now = func() time.Time { return time.Now().Add(2 * time.Hour) } // além do TTL de 1h
	if _, err := svc.UserFromToken(ctx, token); err == nil {
		t.Error("uma sessão expirada não deveria valer")
	}
}

func TestPasswordHashingHonorsContext(t *testing.T) {
	svc := newTestService(t)
	for i := 0; i < cap(svc.hashSlots); i++ {
		svc.hashSlots <- struct{}{} // ocupa todas as vagas
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := svc.Register(ctx, "ana@exemplo.com", "Ana", "senha-forte-123"); !errors.Is(err, context.Canceled) {
		t.Errorf("sem vaga e com contexto cancelado, esperava context.Canceled, veio %v", err)
	}
}

func (f *fakeRepo) DeleteUserSessions(_ context.Context, userID string) error {
	for h, s := range f.sessions {
		if s.userID == userID {
			delete(f.sessions, h)
		}
	}
	return nil
}

func (f *fakeRepo) DeleteExpiredSessions(_ context.Context, now time.Time) (int64, error) {
	var n int64
	for h, s := range f.sessions {
		if !s.expires.After(now) {
			delete(f.sessions, h)
			n++
		}
	}
	return n, nil
}

func TestLogoutAllEndsEverySession(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)
	u, phone, err := svc.Register(ctx, "ana@exemplo.com", "Ana", "senha-forte-123")
	if err != nil {
		t.Fatal(err)
	}
	_, laptop, err := svc.Login(ctx, "ana@exemplo.com", "senha-forte-123")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.LogoutAll(ctx, u.ID); err != nil {
		t.Fatal(err)
	}
	for name, tk := range map[string]string{"celular": phone, "notebook": laptop} {
		if _, err := svc.UserFromToken(ctx, tk); err == nil {
			t.Errorf("a sessão do %s deveria ter acabado", name)
		}
	}
}

func TestPurgeExpiredKeepsLiveSessions(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)
	base := time.Now()
	svc.now = func() time.Time { return base }
	_, old, _ := svc.Register(ctx, "ana@exemplo.com", "Ana", "senha-forte-123")
	svc.now = func() time.Time { return base.Add(30 * time.Minute) }
	_, fresh, _ := svc.Login(ctx, "ana@exemplo.com", "senha-forte-123")

	svc.now = func() time.Time { return base.Add(80 * time.Minute) } // a 1ª (1h) venceu, a 2ª ainda não
	n, err := svc.PurgeExpired(ctx)
	if err != nil || n != 1 {
		t.Fatalf("esperava 1 sessão apagada, veio %d (%v)", n, err)
	}
	if _, err := svc.UserFromToken(ctx, fresh); err != nil {
		t.Errorf("a sessão viva deveria continuar: %v", err)
	}
	if _, err := svc.UserFromToken(ctx, old); err == nil {
		t.Error("a sessão vencida deveria ter sumido")
	}
}

func (f *fakeRepo) UpdateProfile(_ context.Context, userID, name, timezone string) (User, error) {
	for email, u := range f.users {
		if u.ID == userID {
			u.Name, u.Timezone = name, timezone
			f.users[email] = u
			return u, nil
		}
	}
	return User{}, ErrNotFound
}

func TestUpdateProfile(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()
	u, _, err := svc.Register(ctx, "ana@exemplo.com", "Ana", "senha-forte-123")
	if err != nil {
		t.Fatal(err)
	}
	str := func(s string) *string { return &s }

	got, err := svc.UpdateProfile(ctx, u, nil, str(" America/Manaus "))
	if err != nil || got.Timezone != "America/Manaus" || got.Name != "Ana" {
		t.Errorf("fuso não mudou ou o nome mudou junto: %+v %v", got, err)
	}
	got, err = svc.UpdateProfile(ctx, got, str("  Ana Maria "), nil)
	if err != nil || got.Name != "Ana Maria" || got.Timezone != "America/Manaus" {
		t.Errorf("nome não mudou ou o fuso mudou junto: %+v %v", got, err)
	}

	bad := []struct {
		name string
		n, z *string
	}{
		{"fuso inventado", nil, str("Marte/Olympus")},
		{"fuso vazio", nil, str("")},
		{"Local depende do servidor", nil, str("Local")},
		{"fuso sem região", nil, str("EST5EDT")},
		{"nome vazio", str("  "), nil},
		{"nome longo", str(strings.Repeat("a", 81)), nil},
	}
	for _, c := range bad {
		if _, err := svc.UpdateProfile(ctx, got, c.n, c.z); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%s: esperava ErrInvalidInput, veio %v", c.name, err)
		}
	}
	if !ValidTimezone("UTC") || !ValidTimezone("America/Sao_Paulo") {
		t.Error("UTC e America/Sao_Paulo deveriam valer")
	}
}
