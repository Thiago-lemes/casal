package ops

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"

	"casal/internal/httpx"
	"casal/internal/identity"
)

// fakeTransport guarda os eventos que iriam para a rede.
type fakeTransport struct {
	mu     sync.Mutex
	events []*sentry.Event
}

func (f *fakeTransport) Configure(sentry.ClientOptions)        {}
func (f *fakeTransport) Flush(time.Duration) bool              { return true }
func (f *fakeTransport) FlushWithContext(context.Context) bool { return true }
func (f *fakeTransport) Close()                                {}
func (f *fakeTransport) SendEvent(e *sentry.Event) {
	f.mu.Lock()
	f.events = append(f.events, e)
	f.mu.Unlock()
}

func TestMetricasEmFormatoPrometheus(t *testing.T) {
	reg := NewRegistry()
	c := reg.Counter("reminder_send_total", "Envios.", "result")
	c.Inc("ok")
	c.Inc("ok")
	c.Inc("error")
	g := reg.Gauge("last_tick", "Último ciclo.")
	g.Set(42)
	h := reg.Histogram("dur_seconds", "Duração.", []float64{0.1, 1}, "route")
	h.Observe(0.05, "GET /x")
	h.Observe(0.5, "GET /x")
	reg.OnScrape(func(w io.Writer) { WriteGauge(w, "fila", "Fila.", 3) })

	var buf bytes.Buffer
	reg.Write(&buf)
	out := buf.String()
	for _, want := range []string{
		`reminder_send_total{result="ok"} 2`,
		`reminder_send_total{result="error"} 1`,
		"last_tick 42",
		`dur_seconds_bucket{route="GET /x",le="0.1"} 1`,
		`dur_seconds_bucket{route="GET /x",le="1"} 2`,
		`dur_seconds_bucket{route="GET /x",le="+Inf"} 2`,
		`dur_seconds_count{route="GET /x"} 2`,
		"fila 3",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("faltou %q na saída:\n%s", want, out)
		}
	}
}

func TestMetricasHTTPUsamOPadraoDaRota(t *testing.T) {
	reg := NewRegistry()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/reminders/{id}", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) })
	h := NewHTTPMetrics(reg).Wrap(mux)

	for _, path := range []string{"/v1/reminders/abc", "/v1/reminders/def", "/nao/existe/segredo-de-teste"} {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", path, nil))
	}
	var buf bytes.Buffer
	reg.Write(&buf)
	out := buf.String()
	if !strings.Contains(out, `http_requests_total{route="GET /v1/reminders/{id}",status="204"} 2`) {
		t.Errorf("as duas requisições deviam cair na mesma série:\n%s", out)
	}
	if strings.Contains(out, "abc") || strings.Contains(out, "segredo-de-teste") {
		t.Errorf("o caminho real vazou para os rótulos:\n%s", out)
	}
}

func TestMetricsExigeToken(t *testing.T) {
	h := RequireBearer("s3nha", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) }))
	for token, want := range map[string]int{"": 401, "Bearer errado": 401, "Bearer s3nha": 200} {
		req := httptest.NewRequest("GET", "/metrics", nil)
		if token != "" {
			req.Header.Set("Authorization", token)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Errorf("Authorization %q: status %d, esperado %d", token, rec.Code, want)
		}
	}
}

// O evento enviado ao Sentry nunca carrega corpo, cookie, cabeçalho, query, e-mail nem o texto do erro.
func TestRelatorioDeErroNaoLevaConteudo(t *testing.T) {
	tr := &fakeTransport{}
	rp, err := NewReporter(ReporterOptions{Transport: tr, DSN: "https://chave@o0.ingest.sentry.io/1"})
	if err != nil {
		t.Fatal(err)
	}

	const segredo = "segredo-de-teste"
	body := strings.NewReader(`{"body":"` + segredo + `"}`)
	req := httptest.NewRequest("POST", "/v1/notes/abc?q="+segredo, body)
	req.Pattern = "POST /v1/notes/{id}"
	req.Header.Set("Cookie", "casal_session="+segredo)
	req.Header.Set("Authorization", "Bearer "+segredo)
	ctx := identity.With(req.Context(), identity.Principal{UserID: "11111111-1111-1111-1111-111111111111"})
	req = req.WithContext(ctx)

	rp.Capture(req, errors.New("ERROR: duplicate key value (email)=("+segredo+"@x.com)"), nil)
	rp.Capture(req, nil, "panic com "+segredo)

	if len(tr.events) != 2 {
		t.Fatalf("esperava 2 eventos, veio %d", len(tr.events))
	}
	for i, ev := range tr.events {
		raw, _ := json.Marshal(ev)
		if strings.Contains(string(raw), segredo) {
			t.Errorf("evento %d vazou o conteúdo: %s", i, raw)
		}
		if ev.Request != nil {
			t.Errorf("evento %d levou dados da requisição", i)
		}
		if ev.User.ID == "" || ev.User.Email != "" || ev.User.Username != "" || ev.User.IPAddress != "" {
			t.Errorf("evento %d: o usuário deve ter só o id, veio %+v", i, ev.User)
		}
		if ev.Tags["route"] != "POST /v1/notes/{id}" || ev.Tags["method"] != "POST" {
			t.Errorf("evento %d sem rota e método: %v", i, ev.Tags)
		}
	}
}

// Uma exceção acrescentada por engano (com texto digitado) é removida pela última barreira.
func TestScrubRemoveOQuePodeTerConteudo(t *testing.T) {
	ev := &sentry.Event{
		Request:   &sentry.Request{URL: "/x?q=segredo", Cookies: "a=b", Data: "segredo"},
		Exception: []sentry.Exception{{Value: "segredo"}},
		User:      sentry.User{ID: "u1", Email: "a@b.c"},
	}
	out := scrub(ev, nil)
	raw, _ := json.Marshal(out)
	if strings.Contains(string(raw), "segredo") || strings.Contains(string(raw), "a@b.c") {
		t.Errorf("sobrou conteúdo: %s", raw)
	}
	if out.User.ID != "u1" {
		t.Errorf("o id do usuário deve ficar")
	}
}

func TestRelatorioDesligadoSemDSN(t *testing.T) {
	rp, err := NewReporter(ReporterOptions{})
	if err != nil || rp != nil {
		t.Fatalf("sem DSN o relatório fica desligado, veio %v, %v", rp, err)
	}
	rp.Capture(httptest.NewRequest("GET", "/", nil), errors.New("x"), nil) // não pode quebrar
}

func TestGanchoDoServidorRecebeErroEPanic(t *testing.T) {
	tr := &fakeTransport{}
	rp, _ := NewReporter(ReporterOptions{Transport: tr, DSN: "https://chave@o0.ingest.sentry.io/1"})
	httpx.OnServerError(rp.Hook())
	t.Cleanup(func() { httpx.OnServerError(nil) })

	log := discardLogger()
	panicky := httpx.Chain(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }),
		httpx.RequestID(), httpx.Recover(log))
	rec := httptest.NewRecorder()
	panicky.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/x", nil))
	if rec.Code != 500 || strings.Contains(rec.Body.String(), "boom") {
		t.Errorf("o cliente deve ver só erro interno: %d %s", rec.Code, rec.Body)
	}

	failing := httptest.NewRequest("GET", "/v1/y", nil)
	httpx.Fail(httptest.NewRecorder(), failing, log, errors.New("falha inesperada"))

	if len(tr.events) != 2 {
		t.Fatalf("esperava 2 eventos (panic e erro), veio %d", len(tr.events))
	}
}
