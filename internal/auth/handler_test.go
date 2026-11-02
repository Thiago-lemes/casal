package auth

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newTestServer(t *testing.T, secure bool) *http.ServeMux {
	t.Helper()
	svc, err := NewService(newFakeRepo(), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	NewHandler(svc, slog.New(slog.NewTextHandler(io.Discard, nil)), Options{CookieSecure: secure}).Routes(mux)
	return mux
}

func post(mux http.Handler, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", path, strings.NewReader(body))
	req.RemoteAddr = "203.0.113.9:1234"
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestRegisterSetsSafeCookie(t *testing.T) {
	mux := newTestServer(t, true)
	rec := post(mux, "/v1/auth/register", `{"email":"ana@exemplo.com","name":"Ana","password":"senha-forte-123"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("esperava 201, veio %d: %s", rec.Code, rec.Body)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("esperava 1 cookie, veio %d", len(cookies))
	}
	c := cookies[0]
	if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode {
		t.Errorf("cookie inseguro: HttpOnly=%v Secure=%v SameSite=%v", c.HttpOnly, c.Secure, c.SameSite)
	}
}

func TestMeRequiresLogin(t *testing.T) {
	mux := newTestServer(t, false)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/me", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("sem cookie deveria dar 401, veio %d", rec.Code)
	}
}

func TestLoginIsRateLimitedPerEmail(t *testing.T) {
	mux := newTestServer(t, false)
	post(mux, "/v1/auth/register", `{"email":"ana@exemplo.com","name":"Ana","password":"senha-forte-123"}`)

	body := `{"email":"ana@exemplo.com","password":"errada-errada"}`
	for i := 0; i < 5; i++ {
		if rec := post(mux, "/v1/auth/login", body); rec.Code != http.StatusUnauthorized {
			t.Fatalf("tentativa %d: esperava 401, veio %d", i+1, rec.Code)
		}
	}
	rec := post(mux, "/v1/auth/login", body)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("a 6ª tentativa deveria dar 429, veio %d", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("o 429 deveria informar o Retry-After")
	}
	// bloqueada a conta, mesmo a senha certa espera: não dá para testar senhas em sequência
	if rec := post(mux, "/v1/auth/login", `{"email":"ana@exemplo.com","password":"senha-forte-123"}`); rec.Code != http.StatusTooManyRequests {
		t.Errorf("a senha certa também deveria esperar, veio %d", rec.Code)
	}
}

func TestRegisterIsRateLimitedPerIP(t *testing.T) {
	mux := newTestServer(t, false)
	var last int
	for i := 0; i < 6; i++ {
		last = post(mux, "/v1/auth/register", `{"email":"x","name":"x","password":"curta"}`).Code
	}
	if last != http.StatusTooManyRequests {
		t.Errorf("o 6º cadastro seguido do mesmo IP deveria dar 429, veio %d", last)
	}
}

func TestPatchMeChangesTimezoneAndExposesCode(t *testing.T) {
	mux := newTestServer(t, false)
	reg := post(mux, "/v1/auth/register", `{"email":"ana@exemplo.com","name":"Ana","password":"senha-forte-123"}`)
	cookie := reg.Result().Cookies()[0]

	patch := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("PATCH", "/v1/me", strings.NewReader(body))
		req.AddCookie(cookie)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec
	}
	ok := patch(`{"timezone":"America/Manaus"}`)
	if ok.Code != 200 || !strings.Contains(ok.Body.String(), `"timezone":"America/Manaus"`) {
		t.Errorf("PATCH deveria mudar o fuso: %d %s", ok.Code, ok.Body)
	}
	bad := patch(`{"timezone":"Marte/Olympus"}`)
	if bad.Code != 400 || !strings.Contains(bad.Body.String(), `"code":"invalid"`) {
		t.Errorf("fuso inválido deveria dar 400 com code: %d %s", bad.Code, bad.Body)
	}
	dup := post(mux, "/v1/auth/register", `{"email":"ana@exemplo.com","name":"Outra","password":"senha-forte-123"}`)
	if dup.Code != 409 || !strings.Contains(dup.Body.String(), `"code":"email_taken"`) {
		t.Errorf("e-mail repetido deveria trazer code email_taken: %d %s", dup.Code, dup.Body)
	}
}
