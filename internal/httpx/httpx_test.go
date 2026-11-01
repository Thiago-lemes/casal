package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLimiterBlocksAfterBurstAndRecovers(t *testing.T) {
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	l := NewLimiter(3, time.Minute)
	l.now = func() time.Time { return now }

	for i := 0; i < 3; i++ {
		if ok, _ := l.Allow("a"); !ok {
			t.Fatalf("tentativa %d deveria passar", i+1)
		}
	}
	ok, retry := l.Allow("a")
	if ok {
		t.Fatal("a quarta tentativa seguida deveria ser barrada")
	}
	if retry <= 0 || retry > time.Minute {
		t.Errorf("tempo de espera inesperado: %v", retry)
	}
	if ok, _ := l.Allow("b"); !ok {
		t.Error("outra chave não deveria ser afetada")
	}

	now = now.Add(time.Minute) // uma ficha de volta
	if ok, _ := l.Allow("a"); !ok {
		t.Error("depois do intervalo, uma tentativa deveria passar")
	}
	if ok, _ := l.Allow("a"); ok {
		t.Error("mas só uma")
	}
}

func TestLimiterPurgesIdleKeys(t *testing.T) {
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	l := NewLimiter(1, time.Second)
	l.now = func() time.Time { return now }
	for i := 0; i < maxBuckets; i++ {
		l.Allow(string(rune('a'+i%26)) + time.Duration(i).String())
	}
	now = now.Add(time.Hour)
	l.Allow("nova")
	if len(l.buckets) >= maxBuckets {
		t.Errorf("os baldes ociosos deveriam ter sido descartados, sobraram %d", len(l.buckets))
	}
}

func TestClientIP(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "203.0.113.9:5555"
	r.Header.Set("X-Forwarded-For", "1.1.1.1, 198.51.100.7")

	if got := ClientIP(r, false); got != "203.0.113.9" {
		t.Errorf("sem proxy confiável, deveria usar o endereço da conexão, veio %q", got)
	}
	if got := ClientIP(r, true); got != "198.51.100.7" {
		t.Errorf("com proxy confiável, deveria usar o último valor, veio %q", got)
	}
}

func TestSecurityHeaders(t *testing.T) {
	h := SecurityHeaders(true)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))

	for _, name := range []string{"X-Content-Type-Options", "X-Frame-Options", "Content-Security-Policy", "Strict-Transport-Security"} {
		if rec.Header().Get(name) == "" {
			t.Errorf("faltou o cabeçalho %s", name)
		}
	}

	h = SecurityHeaders(false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Header().Get("Strict-Transport-Security") != "" {
		t.Error("sem HTTPS, o HSTS não deve ser enviado")
	}
}

func TestTimeoutCancelsContext(t *testing.T) {
	var deadlineSet bool
	h := Timeout(time.Second)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, deadlineSet = r.Context().Deadline()
	}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	if !deadlineSet {
		t.Error("o contexto da requisição deveria ter prazo")
	}
}
