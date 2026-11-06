package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"testing"
	"time"

	"casal/internal/config"
	"casal/internal/testutil"
)

// Testes de ponta a ponta: sobem a aplicação inteira (rotas, middlewares, banco real)
// e conversam com ela por HTTP, como o front vai fazer.

const e2eDomain = "@e2e.teste.local"

// client é uma "pessoa" com o próprio cookie, como um aparelho.
type client struct {
	t    *testing.T
	base string
	http *http.Client
}

func newClient(t *testing.T, base string) *client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return &client{t: t, base: base, http: &http.Client{Jar: jar}}
}

// call envia a requisição e devolve o status e o corpo já decodificado (se for JSON).
func (c *client) call(method, path string, body any) (int, map[string]any, []byte, http.Header) {
	c.t.Helper()
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.base+path, rd)
	if err != nil {
		c.t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var obj map[string]any
	_ = json.Unmarshal(raw, &obj)
	return resp.StatusCode, obj, raw, resp.Header
}

func (c *client) mustStatus(want int, method, path string, body any) map[string]any {
	c.t.Helper()
	got, obj, raw, _ := c.call(method, path, body)
	if got != want {
		c.t.Fatalf("%s %s: esperava %d, veio %d: %s", method, path, want, got, raw)
	}
	return obj
}

func (c *client) register(name string) string {
	c.t.Helper()
	obj := c.mustStatus(201, "POST", "/v1/auth/register", map[string]string{
		"email": name + e2eDomain, "name": name, "password": "senha-forte-123",
	})
	return obj["id"].(string)
}

func startApp(t *testing.T) (*httptest.Server, []Job) {
	t.Helper()
	return startAppWith(t, config.Config{SessionTTL: time.Hour})
}

func startAppWith(t *testing.T, cfg config.Config) (*httptest.Server, []Job) {
	t.Helper()
	conn := testutil.OpenDB(t)
	handler, jobs, err := New(cfg, conn, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	// apaga o que o teste criar pela API (casais, e por cascata o resto), mesmo se ele falhar
	t.Cleanup(func() {
		conn.Exec(`DELETE FROM households WHERE id IN (
			SELECT m.household_id FROM household_members m JOIN users u ON u.id = m.user_id WHERE u.email LIKE '%' || $1)`, e2eDomain)
		conn.Exec(`DELETE FROM users WHERE email LIKE '%' || $1`, e2eDomain)
	})
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv, jobs
}

func TestCoupleJourney(t *testing.T) {
	srv, jobs := startApp(t)
	ana, bia, cris := newClient(t, srv.URL), newClient(t, srv.URL), newClient(t, srv.URL)

	// --- sem login, nada funciona
	ana.mustStatus(401, "GET", "/v1/me", nil)
	ana.mustStatus(401, "GET", "/v1/reminders", nil)

	// --- Ana cria a conta e o casal, e convida a Bia
	anaID := ana.register("ana")
	ana.mustStatus(403, "GET", "/v1/reminders", nil) // ainda sem casal
	ana.mustStatus(201, "POST", "/v1/household", nil)
	ana.mustStatus(409, "POST", "/v1/household", nil) // já tem casal
	invite := ana.mustStatus(201, "POST", "/v1/household/invites", nil)["token"].(string)

	biaID := bia.register("bia")
	bia.mustStatus(404, "POST", "/v1/household/invites/accept", map[string]string{"token": "token-que-nao-existe"})
	bia.mustStatus(200, "POST", "/v1/household/invites/accept", map[string]string{"token": invite})
	bia.mustStatus(404, "POST", "/v1/household/invites/accept", map[string]string{"token": invite}) // convite é de uso único

	members := ana.mustStatus(200, "GET", "/v1/household", nil)["members"].([]any)
	if len(members) != 2 {
		t.Fatalf("o casal deveria ter 2 pessoas, tem %d", len(members))
	}

	// --- casal cheio: nem gera convite novo
	crisID := cris.register("cris")
	ana.mustStatus(409, "POST", "/v1/household/invites", nil)

	// --- Ana deixa um lembrete para a Bia, já vencido (o agendador pega na hora)
	rem := ana.mustStatus(201, "POST", "/v1/reminders", map[string]any{
		"recipient_id": biaID, "body": "tirar a marmita do freezer",
		"remind_at": time.Now().UTC().Format(time.RFC3339),
	})
	remID := rem["id"].(string)
	if rem["created_by"] != anaID {
		t.Errorf("created_by = %v", rem["created_by"])
	}

	// validações da API
	ana.mustStatus(400, "POST", "/v1/reminders", map[string]any{"body": "", "remind_at": time.Now().UTC().Add(time.Hour).Format(time.RFC3339)})
	ana.mustStatus(400, "POST", "/v1/reminders", map[string]any{"recipient_id": crisID, "body": "x", "remind_at": time.Now().UTC().Add(time.Hour).Format(time.RFC3339)}) // fora do casal
	ana.mustStatus(404, "POST", "/v1/reminders/nao-e-uuid/resolve", nil)

	// --- o agendador dispara o lembrete uma vez
	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	go jobs[0](ctx) // jobs[0] é o agendador de lembretes
	waitFor(t, 5*time.Second, "o lembrete virar 'sent'", func() bool {
		list, _, raw, _ := bia.call("GET", "/v1/reminders", nil)
		return list == 200 && bytes.Contains(raw, []byte(`"status":"sent"`))
	})

	// --- quem recebeu resolve; quem enviou não pode resolver no lugar
	ana.mustStatus(404, "POST", "/v1/reminders/"+remID+"/resolve", nil)
	got := bia.mustStatus(200, "POST", "/v1/reminders/"+remID+"/resolve", nil)
	if got["status"] != "resolved" {
		t.Errorf("status = %v", got["status"])
	}

	// --- outro casal não enxerga nem mexe em nada
	cris.mustStatus(201, "POST", "/v1/household", nil)
	_, _, raw, _ := cris.call("GET", "/v1/reminders", nil)
	if string(bytes.TrimSpace(raw)) != "[]" {
		t.Errorf("o casal da Cris não deveria ver lembretes, veio %s", raw)
	}
	cris.mustStatus(404, "POST", "/v1/reminders/"+remID+"/resolve", nil)
	cris.mustStatus(404, "DELETE", "/v1/reminders/"+remID, nil)
	cris.mustStatus(404, "POST", "/v1/reminders/"+remID+"/snooze", map[string]int{"minutes": 5})

	// --- encerrar a sessão em todos os aparelhos
	ana.mustStatus(204, "POST", "/v1/auth/logout-all", nil)
	ana.mustStatus(401, "GET", "/v1/me", nil)
}

func TestResponsesCarrySecurityHeadersAndRequestID(t *testing.T) {
	srv, _ := startApp(t)
	c := newClient(t, srv.URL)
	_, _, _, h := c.call("GET", "/healthz", nil)
	for _, name := range []string{"X-Request-Id", "X-Content-Type-Options", "Content-Security-Policy", "X-Frame-Options"} {
		if h.Get(name) == "" {
			t.Errorf("faltou o cabeçalho %s", name)
		}
	}
}

func TestErrorsNeverLeakInternals(t *testing.T) {
	srv, _ := startApp(t)
	c := newClient(t, srv.URL)
	c.register("vazamento")
	c.mustStatus(201, "POST", "/v1/household", nil)

	// corpo com campo desconhecido e JSON quebrado: 400, sem detalhes do decoder
	for _, body := range []string{`{"campo":1}`, `{"body":`} {
		req, _ := http.NewRequest("POST", srv.URL+"/v1/reminders", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.http.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 400 || !bytes.Contains(raw, []byte("JSON inválido")) {
			t.Errorf("corpo %q: status %d, resposta %s", body, resp.StatusCode, raw)
		}
	}
}

func waitFor(t *testing.T, timeout time.Duration, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("passou %v esperando: %s", timeout, what)
}
