package app

import (
	"encoding/json"
	"strings"
	"testing"
)

func num(t *testing.T, obj map[string]any, key string) float64 {
	t.Helper()
	v, ok := obj[key].(float64)
	if !ok {
		t.Fatalf("campo %q ausente ou não numérico em %v", key, obj)
	}
	return v
}

// Fecha um mês de acerto de contas pela API: é a saída da Fase 2.
func TestFinanceJourney(t *testing.T) {
	srv, _ := startApp(t)
	ana, bia := newClient(t, srv.URL), newClient(t, srv.URL)

	anaID := ana.register("fin-ana")
	ana.mustStatus(201, "POST", "/v1/household", nil)
	invite := ana.mustStatus(201, "POST", "/v1/household/invites", nil)["token"].(string)
	biaID := bia.register("fin-bia")
	bia.mustStatus(200, "POST", "/v1/household/invites/accept", map[string]string{"token": invite})

	// --- módulo desligado: nada de Finanças existe para o casal
	_, body, _, _ := ana.call("GET", "/v1/finance/categories", nil)
	if body["code"] != "module_disabled" {
		t.Fatalf("esperava module_disabled, veio %v", body)
	}
	ana.mustStatus(403, "GET", "/v1/finance/categories", nil)
	ana.mustStatus(403, "GET", "/v1/finance/balance", nil)
	ana.mustStatus(404, "PUT", "/v1/modules/marmita", map[string]bool{"enabled": true})
	ana.mustStatus(400, "PUT", "/v1/modules/finance", map[string]string{"nope": "x"})

	// --- liga (qualquer um dos dois) e as categorias padrão nascem uma vez
	st := ana.mustStatus(200, "PUT", "/v1/modules/finance", map[string]bool{"enabled": true})
	if st["enabled"] != true {
		t.Fatalf("módulo: %v", st)
	}
	bia.mustStatus(200, "PUT", "/v1/modules/finance", map[string]bool{"enabled": true})
	_, _, raw, _ := ana.call("GET", "/v1/finance/categories", nil)
	if n := strings.Count(string(raw), `"key"`); n != 7 {
		t.Fatalf("esperava 7 categorias padrão, vieram %d: %s", n, raw)
	}
	var groceries string
	for _, c := range decodeList(t, raw) {
		if c["key"] == "groceries" {
			groceries = c["id"].(string)
		}
	}

	// --- gastos: a Ana paga o mercado e o aluguel dividido; a Bia, a internet
	spend := func(c *client, cents int, on string) map[string]any {
		return c.mustStatus(201, "POST", "/v1/finance/expenses", map[string]any{
			"amount_cents": cents, "category_id": groceries, "spent_on": on, "description": "mercado",
		})
	}
	first := spend(ana, 30000, "2026-10-03")
	spend(bia, 10000, "2026-10-04")
	if first["split"].(map[string]any)[anaID] != float64(5000) || first["version"] != float64(1) {
		t.Fatalf("gasto: %v", first)
	}
	// parcelado: 3x de R$ 100,00 a partir do mês passado (todas vencidas ou futuras conforme a data)
	inst := ana.mustStatus(201, "POST", "/v1/finance/expenses", map[string]any{
		"amount_cents": 30001, "category_id": groceries, "spent_on": "2026-10-10",
		"installments": map[string]any{"count": 3},
	})
	parts := inst["expenses"].([]any)
	if len(parts) != 3 || parts[0].(map[string]any)["amount_cents"] != float64(10001) {
		t.Fatalf("parcelas: %v", inst)
	}
	// gasto pessoal da Ana: a Bia nunca o vê
	secret := ana.mustStatus(201, "POST", "/v1/finance/expenses", map[string]any{
		"scope": "personal", "amount_cents": 99999, "category_id": groceries, "description": "presente",
	})
	bia.mustStatus(404, "GET", "/v1/finance/expenses/"+secret["id"].(string), nil)
	if _, _, raw, _ := bia.call("GET", "/v1/finance/expenses?scope=mine", nil); strings.Contains(string(raw), "99999") {
		t.Errorf("a Bia viu o gasto pessoal da Ana: %s", raw)
	}

	// --- renda: privada de cada um; a proporcional exige a concordância dos dois
	ana.mustStatus(400, "PUT", "/v1/finance/settings", map[string]string{"split_mode": "proportional"})
	ana.mustStatus(200, "PUT", "/v1/finance/income", map[string]any{"monthly_income_cents": 8_765_432})
	_, inc, _, _ := bia.call("GET", "/v1/finance/income", nil)
	if inc["monthly_income_cents"] != nil {
		t.Errorf("a Bia não lê a renda da Ana: %v", inc)
	}
	if _, _, raw, _ := bia.call("GET", "/v1/finance/settings", nil); strings.Contains(string(raw), "8765432") {
		t.Errorf("a renda da Ana apareceu para a Bia: %s", raw)
	}
	ana.mustStatus(400, "PUT", "/v1/finance/income", map[string]any{"monthly_income_cents": 0})

	// --- saldo: a Ana pagou 30000 + 30001 e a Bia 10000; a parte de cada um é metade do total
	bal := ana.mustStatus(200, "GET", "/v1/finance/balance?through=2026-12-31", nil)
	// total 70001: a parte da Ana = 35001 (o centavo extra vai ao menor id em cada gasto), então o valor sai das contas do servidor
	owed := num(t, bal, "amount_cents")
	if owed <= 0 || bal["from_user"] != biaID || bal["to_user"] != anaID {
		t.Fatalf("saldo: %v", bal)
	}
	if again := bia.mustStatus(200, "GET", "/v1/finance/balance?through=2026-12-31", nil); again["amount_cents"] != bal["amount_cents"] {
		t.Errorf("o saldo é o mesmo para os dois: %v e %v", bal["amount_cents"], again["amount_cents"])
	}

	// acerto com valor errado: nada muda
	bia.mustStatus(409, "POST", "/v1/finance/settlements", map[string]any{"through_on": "2026-12-31", "amount_cents": int(owed) + 1})
	_, serr, _, _ := bia.call("POST", "/v1/finance/settlements", map[string]any{"through_on": "2026-12-31", "amount_cents": 1})
	if serr["code"] != "settlement_stale" {
		t.Errorf("esperava settlement_stale: %v", serr)
	}
	if hist := ana.mustStatus(200, "GET", "/v1/finance/settlements", nil); hist != nil {
		_ = hist
	}

	// acerto certo: o saldo volta a zero e os gastos travam
	done := bia.mustStatus(201, "POST", "/v1/finance/settlements", map[string]any{"through_on": "2026-12-31", "amount_cents": int(owed)})
	if done["kind"] != "manual" || done["from_user"] != biaID {
		t.Fatalf("acerto: %v", done)
	}
	if after := ana.mustStatus(200, "GET", "/v1/finance/balance?through=2026-12-31", nil); after["amount_cents"] != float64(0) {
		t.Errorf("o saldo deveria voltar a zero: %v", after)
	}
	ana.mustStatus(409, "DELETE", "/v1/finance/expenses/"+first["id"].(string), nil)
	_, lock, _, _ := ana.call("PUT", "/v1/finance/expenses/"+first["id"].(string), map[string]any{
		"amount_cents": 1, "category_id": groceries, "spent_on": "2026-10-03", "version": 1,
	})
	if lock["code"] != "expense_settled" {
		t.Errorf("esperava expense_settled: %v", lock)
	}
	bia.mustStatus(400, "POST", "/v1/finance/settlements", map[string]any{"through_on": "2026-12-31", "amount_cents": 100})

	// --- contas a pagar, orçamento, resumo e exportação respondem
	bill := ana.mustStatus(201, "POST", "/v1/finance/bills", map[string]any{"title": "Luz", "due_on": "2026-12-20", "repeat": "monthly"})
	paid := bia.mustStatus(200, "POST", "/v1/finance/bills/"+bill["id"].(string)+"/pay", map[string]any{"amount_cents": 18990, "paid_on": "2026-12-19"})
	if paid["next_bill"].(map[string]any)["due_on"] != "2027-01-20" {
		t.Errorf("próxima conta: %v", paid)
	}
	bia.mustStatus(409, "POST", "/v1/finance/bills/"+bill["id"].(string)+"/pay", map[string]any{"amount_cents": 18990})
	ana.mustStatus(200, "PUT", "/v1/finance/budgets", map[string]any{"category_id": groceries, "limit_cents": 100000})
	sum := ana.mustStatus(200, "GET", "/v1/finance/summary?month=2026-10", nil)
	if num(t, sum, "total_cents") <= 0 {
		t.Errorf("resumo: %v", sum)
	}
	_, _, csv, hdr := ana.call("GET", "/v1/finance/export?format=csv", nil)
	if !strings.HasPrefix(hdr.Get("Content-Type"), "text/csv") || !strings.Contains(string(csv), "tipo,data") || strings.Contains(string(csv), "99999") == false {
		t.Errorf("CSV da Ana (inclui o pessoal dela): %s", csv)
	}
	if _, _, csvB, _ := bia.call("GET", "/v1/finance/export?format=csv", nil); strings.Contains(string(csvB), "99999") {
		t.Errorf("o CSV da Bia trouxe o gasto pessoal da Ana: %s", csvB)
	}

	// --- desligar esconde e não apaga; religar traz tudo de volta
	ana.mustStatus(200, "PUT", "/v1/modules/finance", map[string]bool{"enabled": false})
	ana.mustStatus(403, "GET", "/v1/finance/expenses", nil)
	bia.mustStatus(403, "POST", "/v1/finance/expenses", map[string]any{"amount_cents": 1, "category_id": groceries})
	ana.mustStatus(200, "PUT", "/v1/modules/finance", map[string]bool{"enabled": true})
	page := ana.mustStatus(200, "GET", "/v1/finance/expenses?month=2026-10", nil)
	if n := len(page["expenses"].([]any)); n != 3 { // 2 avulsos + a 1ª parcela (as outras caem em nov e dez)
		t.Errorf("religar traz os gastos de volta: %d", n)
	}
	_, _, raw, _ = ana.call("GET", "/v1/finance/categories", nil)
	if n := strings.Count(string(raw), `"key"`); n != 7 {
		t.Errorf("religar não recria categorias: %d", n)
	}
}

func decodeList(t *testing.T, raw []byte) []map[string]any {
	t.Helper()
	var out []map[string]any
	if err := jsonUnmarshal(raw, &out); err != nil {
		t.Fatalf("lista inválida: %v: %s", err, raw)
	}
	return out
}

var jsonUnmarshal = json.Unmarshal
