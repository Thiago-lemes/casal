package household

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"casal/internal/identity"
)

type memberRepo struct {
	Repository
	households map[string]string // userID -> householdID
}

func (m memberRepo) HouseholdOf(_ context.Context, userID string) (string, error) {
	if id, ok := m.households[userID]; ok {
		return id, nil
	}
	return "", ErrNoHousehold
}

func serve(t *testing.T, principal *identity.Principal) (*httptest.ResponseRecorder, identity.Principal) {
	t.Helper()
	svc := NewService(memberRepo{households: map[string]string{"u1": "h1"}})
	var got identity.Principal
	h := RequireHousehold(svc, slog.New(slog.NewTextHandler(io.Discard, nil)))(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { got, _ = identity.From(r.Context()) }))

	req := httptest.NewRequest("GET", "/", nil)
	if principal != nil {
		req = req.WithContext(identity.With(req.Context(), *principal))
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec, got
}

func TestRequireHouseholdFillsPrincipal(t *testing.T) {
	rec, got := serve(t, &identity.Principal{UserID: "u1"})
	if rec.Code != http.StatusOK || got.HouseholdID != "h1" || got.UserID != "u1" {
		t.Errorf("status %d, principal %+v", rec.Code, got)
	}
}

func TestRequireHouseholdWithoutHousehold(t *testing.T) {
	if rec, _ := serve(t, &identity.Principal{UserID: "sem-casal"}); rec.Code != http.StatusForbidden {
		t.Errorf("quem não tem casal deveria receber 403, veio %d", rec.Code)
	}
}

func TestRequireHouseholdWithoutLogin(t *testing.T) {
	if rec, _ := serve(t, nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("sem login deveria dar 401, veio %d", rec.Code)
	}
}
