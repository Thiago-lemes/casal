package app

import (
	"bytes"
	"context"
	"database/sql"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"casal/internal/config"
	"casal/internal/db"
	"casal/internal/testutil"
)

func scrape(t *testing.T, base, token string) (int, string) {
	t.Helper()
	req, _ := http.NewRequest("GET", base+"/metrics", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(raw)
}

// Sem METRICS_TOKEN nem METRICS_ADDR a rota não existe.
func TestMetricsNaoExisteSemConfiguracao(t *testing.T) {
	srv, _ := startApp(t)
	if code, _ := scrape(t, srv.URL, ""); code != 404 {
		t.Errorf("sem configuração o /metrics deve dar 404, deu %d", code)
	}
}

func TestMetricsExigeTokenENaoLevaConteudo(t *testing.T) {
	const token = "token-de-teste-do-metrics"
	const segredo = "segredo-de-teste-metrics"
	srv, jobs := startAppWith(t, config.Config{SessionTTL: time.Hour, MetricsToken: token})

	if code, _ := scrape(t, srv.URL, ""); code != 401 {
		t.Errorf("sem token deve dar 401, deu %d", code)
	}
	if code, _ := scrape(t, srv.URL, "errado"); code != 401 {
		t.Errorf("token errado deve dar 401, deu %d", code)
	}

	// uma pessoa usa o app com texto reconhecível em tudo o que digita
	ana := newClient(t, srv.URL)
	anaID := ana.register("metrics-" + segredo[:6])
	ana.mustStatus(201, "POST", "/v1/household", nil)
	rem := ana.mustStatus(201, "POST", "/v1/reminders", map[string]any{
		"recipient_id": anaID, "body": segredo,
		"remind_at": time.Now().UTC().Add(time.Hour).Format(time.RFC3339),
	})
	// a API não aceita horário passado; atrasa-se direto no banco (escopo de sistema, que atravessa o RLS)
	testutil.RunHook(t, testutil.OpenDB(t), func(ctx context.Context, tx *sql.Tx, _ db.Handoff) error {
		if err := db.SetScope(ctx, tx, db.Scope{System: true}); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `UPDATE reminders SET remind_at = now() - interval '10 minutes' WHERE id = $1`, rem["id"])
		return err
	}, db.Handoff{})
	ana.call("GET", "/v1/reminders/"+segredo+"?q="+segredo, nil)

	code, out := scrape(t, srv.URL, token)
	if code != 200 {
		t.Fatalf("com token deve dar 200, deu %d", code)
	}
	if strings.Contains(out, segredo) || strings.Contains(out, anaID) || strings.Contains(out, "@e2e") {
		t.Errorf("o /metrics vazou conteúdo ou identificador:\n%s", out)
	}
	if !strings.Contains(out, `http_requests_total{route="POST /v1/reminders",status="201"}`) {
		t.Errorf("faltou a contagem da rota pelo padrão:\n%s", out)
	}
	// a fila atrasada vê o lembrete criado acima (outros testes em paralelo podem somar mais)
	m := regexp.MustCompile(`(?m)^reminder_queue_due (\d+)$`).FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("faltou reminder_queue_due:\n%s", out)
	}
	if n, _ := strconv.Atoi(m[1]); n < 1 {
		t.Errorf("reminder_queue_due = %d, esperava ao menos 1", n)
	}
	if !strings.Contains(out, "db_connections_in_use") {
		t.Errorf("faltou db_connections_in_use")
	}

	// o agendador vivo avança o relógio de prova de vida e entrega o lembrete
	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	go jobs[0](ctx)
	waitFor(t, 5*time.Second, "o agendador registrar o ciclo", func() bool {
		_, out := scrape(t, srv.URL, token)
		return regexp.MustCompile(`(?m)^reminder_worker_last_tick_timestamp \d+`).MatchString(out)
	})
	waitFor(t, 5*time.Second, "o envio do lembrete ser contado", func() bool {
		_, out := scrape(t, srv.URL, token)
		return bytes.Contains([]byte(out), []byte(`reminder_send_total{result="`))
	})
}

// Os eventos de produto nascem das ações reais e, junto com as views de agregados, nunca carregam o que a pessoa escreveu.
func TestEventosDeProdutoSemConteudo(t *testing.T) {
	const segredo = "segredo-de-teste-eventos"
	srv, _ := startApp(t)
	conn := testutil.OpenDB(t)

	ana, bia := newClient(t, srv.URL), newClient(t, srv.URL)
	anaID := ana.register("ev-ana")
	ana.mustStatus(201, "POST", "/v1/household", nil)
	invite := ana.mustStatus(201, "POST", "/v1/household/invites", nil)["token"].(string)
	biaID := bia.register("ev-bia")
	bia.mustStatus(200, "POST", "/v1/household/invites/accept", map[string]string{"token": invite})
	ana.mustStatus(201, "POST", "/v1/reminders", map[string]any{
		"recipient_id": biaID, "body": segredo, "remind_at": time.Now().UTC().Add(time.Hour).Format(time.RFC3339),
	})
	ana.mustStatus(201, "POST", "/v1/notes", map[string]any{"title": segredo, "kind": "text", "body": segredo})
	ana.mustStatus(200, "PUT", "/v1/modules/finance", map[string]any{"enabled": true})
	ana.mustStatus(200, "PUT", "/v1/modules/finance", map[string]any{"enabled": false})

	got := map[string]int{}
	for _, name := range testutil.SystemStrings(t, conn,
		`SELECT name FROM product_events WHERE user_id IN ($1, $2)`, anaID, biaID) {
		got[name]++
	}
	for name, want := range map[string]int{
		"user_registered": 2, "household_created": 1, "partner_joined": 1,
		"reminder_created_for_partner": 1, "module_enabled": 1, "module_disabled": 1,
	} {
		if got[name] != want {
			t.Errorf("evento %s: %d, esperado %d (todos: %v)", name, got[name], want, got)
		}
	}

	// o que chega ao Grafana: nenhuma view devolve o texto digitado
	ops := testutil.OpenOpsDB(t)
	for _, view := range []string{"daily_activation", "weekly_active", "module_adoption", "module_usage", "retention", "push_coverage"} {
		rows, err := ops.Query(`SELECT row_to_json(v)::text FROM ops.` + view + ` v`)
		if err != nil {
			t.Fatalf("ops.%s: %v", view, err)
		}
		for rows.Next() {
			var line string
			_ = rows.Scan(&line)
			if strings.Contains(line, segredo) || strings.Contains(line, anaID) || strings.Contains(line, "@e2e") {
				t.Errorf("ops.%s vazou conteúdo: %s", view, line)
			}
		}
		rows.Close()
	}
}
