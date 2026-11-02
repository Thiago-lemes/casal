package module

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"casal/internal/identity"
)

func serve(on bool, p *identity.Principal) (rec *httptest.ResponseRecorder, passed bool) {
	repo := &fakeRepo{on: map[string]time.Time{}}
	if on {
		repo.on[Finance] = time.Now()
	}
	h := Require(NewService(repo), slog.New(slog.NewTextHandler(io.Discard, nil)), Finance)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { passed = true }))

	req := httptest.NewRequest("GET", "/", nil)
	if p != nil {
		req = req.WithContext(identity.With(req.Context(), *p))
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec, passed
}

func TestRequireLetsEnabledModuleThrough(t *testing.T) {
	rec, passed := serve(true, &identity.Principal{UserID: "u1", HouseholdID: "h1"})
	if rec.Code != http.StatusOK || !passed {
		t.Errorf("status %d, seguiu=%v", rec.Code, passed)
	}
}

func TestRequireBlocksDisabledModuleWithStableCode(t *testing.T) {
	rec, passed := serve(false, &identity.Principal{UserID: "u1", HouseholdID: "h1"})
	if passed || rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), `"code":"module_disabled"`) {
		t.Errorf("esperava 403 module_disabled sem seguir, veio %d %s (seguiu=%v)", rec.Code, rec.Body.String(), passed)
	}
}

func TestRequireWithoutHouseholdOrLoginIsUnauthorized(t *testing.T) {
	for name, p := range map[string]*identity.Principal{
		"sem casal no contexto": {UserID: "u1"},
		"sem login":             nil,
	} {
		rec, passed := serve(true, p)
		if passed || rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: esperava 401 sem seguir, veio %d (seguiu=%v)", name, rec.Code, passed)
		}
	}
}
