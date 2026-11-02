package app

import (
	"strings"
	"testing"

	"casal/internal/testutil"
)

func progress(t *testing.T, g map[string]any) map[string]any {
	t.Helper()
	p, ok := g["progress"].(map[string]any)
	if !ok {
		t.Fatalf("meta sem progress: %v", g)
	}
	return p
}

func ids(t *testing.T, raw []byte) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	for _, g := range decodeList(t, raw) {
		out[g["id"].(string)] = true
	}
	return out
}

// Um casal cria uma meta de cada tipo, registra aporte, marcações e etapas, e confere progresso, tarefas e
// privacidade pela API: é a saída da Fase 3.
func TestGoalsJourney(t *testing.T) {
	srv, _ := startApp(t)
	ana, bia := newClient(t, srv.URL), newClient(t, srv.URL)

	anaID := ana.register("goal-ana")
	ana.mustStatus(201, "POST", "/v1/household", nil)
	invite := ana.mustStatus(201, "POST", "/v1/household/invites", nil)["token"].(string)
	biaID := bia.register("goal-bia")
	bia.mustStatus(200, "POST", "/v1/household/invites/accept", map[string]string{"token": invite})

	// --- módulo desligado: nada de Metas existe para o casal
	_, body, _, _ := ana.call("GET", "/v1/goals", nil)
	if body["code"] != "module_disabled" {
		t.Fatalf("esperava module_disabled: %v", body)
	}
	bia.mustStatus(403, "POST", "/v1/goals", map[string]any{"kind": "project", "scope": "household", "title": "x"})
	ana.mustStatus(200, "PUT", "/v1/modules/goals", map[string]bool{"enabled": true})
	bia.mustStatus(200, "GET", "/v1/goals", nil)

	// --- uma meta de cada tipo
	habit := ana.mustStatus(201, "POST", "/v1/goals", map[string]any{
		"kind": "habit", "scope": "personal", "title": "Terapia", "notes": "só eu",
		"habit": map[string]any{"target_count": 3, "period": "week", "remind_enabled": true},
	})
	shared := ana.mustStatus(201, "POST", "/v1/goals", map[string]any{
		"kind": "shared", "scope": "household", "title": "Jantar fora",
		"shared": map[string]any{"target_count": 2, "period": "month"},
	})
	saving := bia.mustStatus(201, "POST", "/v1/goals", map[string]any{
		"kind": "financial", "scope": "household", "title": "Casa nova", "due_on": "2099-12-31",
		"financial": map[string]any{"target_cents": 1_000_000},
	})
	project := ana.mustStatus(201, "POST", "/v1/goals", map[string]any{
		"kind": "project", "scope": "household", "title": "Reforma da cozinha",
		"steps": []map[string]any{
			{"title": "Comprar tinta", "assignee_id": biaID, "due_on": "2099-12-01", "task": true},
			{"title": "Chamar o pintor"},
		},
	})
	habitID, sharedID, savingID, projectID := habit["id"].(string), shared["id"].(string), saving["id"].(string), project["id"].(string)
	if progress(t, habit)["unit"] != "count" || progress(t, saving)["unit"] != "cents" || progress(t, project)["unit"] != "steps" {
		t.Errorf("unidades do progresso: %v %v %v", habit["progress"], saving["progress"], project["progress"])
	}
	ana.mustStatus(400, "POST", "/v1/goals", map[string]any{"kind": "habit", "scope": "personal", "title": "", "habit": map[string]any{"target_count": 1, "period": "week"}})
	_, bad, _, _ := ana.call("POST", "/v1/goals", map[string]any{"kind": "financial", "scope": "personal", "title": "x", "financial": map[string]any{"target_cents": 0}})
	if bad["code"] != "amount_invalid" {
		t.Errorf("esperava amount_invalid: %v", bad)
	}

	// --- privacidade: a Bia vê as do casal e nunca a pessoal da Ana
	_, _, raw, _ := bia.call("GET", "/v1/goals", nil)
	seen := ids(t, raw)
	if !seen[sharedID] || !seen[savingID] || !seen[projectID] || seen[habitID] {
		t.Errorf("a Bia vê: %v", seen)
	}
	if strings.Contains(string(raw), "Terapia") || strings.Contains(string(raw), "só eu") {
		t.Errorf("a meta privada vazou na lista: %s", raw)
	}
	_, nf, _, _ := bia.call("GET", "/v1/goals/"+habitID, nil)
	if nf["code"] != "goal_not_found" {
		t.Errorf("esperava goal_not_found: %v", nf)
	}
	bia.mustStatus(404, "POST", "/v1/goals/"+habitID+"/checkins", nil)
	bia.mustStatus(404, "GET", "/v1/goals/nao-e-uuid", nil)

	// --- hábito: marca hoje (idempotente) e o progresso vem do servidor
	r1 := ana.mustStatus(200, "POST", "/v1/goals/"+habitID+"/checkins", nil)
	r2 := ana.mustStatus(200, "POST", "/v1/goals/"+habitID+"/checkins", map[string]any{})
	if r1["created"] != true || r2["created"] != false || progress(t, r2["goal"].(map[string]any))["done"] != float64(1) {
		t.Errorf("marcação idempotente: %v / %v", r1, r2)
	}
	_, ck, _, _ := ana.call("POST", "/v1/goals/"+habitID+"/checkins", map[string]any{"day": "2099-01-01"})
	if ck["code"] != "checkin_date_invalid" {
		t.Errorf("esperava checkin_date_invalid: %v", ck)
	}
	hist := ana.mustStatus(200, "GET", "/v1/goals/"+habitID+"/history", nil)
	if len(hist["days"].([]any)) != 1 || len(hist["periods"].([]any)) == 0 {
		t.Errorf("histórico: %v", hist)
	}
	ana.mustStatus(400, "POST", "/v1/goals/"+savingID+"/checkins", nil) // goal_kind_mismatch

	// --- meta do casal: a Bia marca, a Ana também; o progresso é do casal
	bia.mustStatus(200, "POST", "/v1/goals/"+sharedID+"/checkins", nil)
	g := ana.mustStatus(200, "POST", "/v1/goals/"+sharedID+"/checkins", nil)["goal"].(map[string]any)
	if progress(t, g)["done"] != float64(1) { // os dois marcaram o mesmo dia: uma ocorrência
		t.Errorf("meta do casal: %v", g["progress"])
	}

	// --- meta financeira: aporte não é gasto
	c := bia.mustStatus(201, "POST", "/v1/goals/"+savingID+"/contributions", map[string]any{"amount_cents": 250_000, "note": "13º"})
	if progress(t, c["goal"].(map[string]any))["done"] != float64(250_000) || progress(t, c["goal"].(map[string]any))["percent"] != float64(25) {
		t.Errorf("progresso do aporte: %v", c["goal"])
	}
	if prog := progress(t, c["goal"].(map[string]any)); prog["required_per_month_cents"] == nil || prog["on_track"] == nil {
		t.Errorf("com prazo, o servidor calcula o ritmo: %v", prog)
	}
	_, neg, _, _ := ana.call("POST", "/v1/goals/"+savingID+"/contributions", map[string]any{"amount_cents": -250_001})
	if neg["code"] != "contribution_negative" {
		t.Errorf("esperava contribution_negative: %v", neg)
	}
	ana.mustStatus(400, "POST", "/v1/goals/"+habitID+"/contributions", map[string]any{"amount_cents": 100})
	ana.mustStatus(403, "GET", "/v1/finance/expenses", nil) // Finanças continua desligado
	ana.mustStatus(200, "PUT", "/v1/modules/finance", map[string]bool{"enabled": true})
	page := ana.mustStatus(200, "GET", "/v1/finance/expenses", nil)
	if n := len(page["expenses"].([]any)); n != 0 {
		t.Errorf("o aporte criou %d gasto(s) em Finanças", n)
	}

	// --- economia do orçamento: lê Finanças, só gasto do casal
	_, _, catRaw, _ := ana.call("GET", "/v1/finance/categories", nil)
	var groceries string
	for _, cat := range decodeList(t, catRaw) {
		if cat["key"] == "groceries" {
			groceries = cat["id"].(string)
		}
	}
	ana.mustStatus(200, "PUT", "/v1/finance/budgets", map[string]any{"category_id": groceries, "limit_cents": 80_000})
	econ := ana.mustStatus(201, "POST", "/v1/goals", map[string]any{
		"kind": "financial", "scope": "household", "title": "Economizar no mercado",
		"financial": map[string]any{"target_cents": 400_000, "source": "budget_savings", "category_id": groceries},
	})
	econID := econ["id"].(string)
	if p := progress(t, econ); p["done"] != float64(80_000) || p["provisional"] != true {
		t.Errorf("sem gastos, sobra o limite inteiro (provisório): %v", p)
	}
	ana.mustStatus(201, "POST", "/v1/finance/expenses", map[string]any{"amount_cents": 30_000, "category_id": groceries})
	ana.mustStatus(201, "POST", "/v1/finance/expenses", map[string]any{"scope": "personal", "amount_cents": 99_000, "category_id": groceries})
	if p := progress(t, ana.mustStatus(200, "GET", "/v1/goals/"+econID, nil)); p["done"] != float64(50_000) {
		t.Errorf("a sobra considera só o gasto do casal: %v", p)
	}
	ana.mustStatus(400, "POST", "/v1/goals/"+econID+"/contributions", map[string]any{"amount_cents": 100})
	_, mod, _, _ := ana.call("POST", "/v1/goals", map[string]any{
		"kind": "financial", "scope": "personal", "title": "x",
		"financial": map[string]any{"target_cents": 1000, "source": "budget_savings", "category_id": groceries}})
	if mod["code"] != "goal_invalid" {
		t.Errorf("meta pessoal não lê o gasto do casal: %v", mod)
	}
	ana.mustStatus(200, "PUT", "/v1/modules/finance", map[string]bool{"enabled": false})
	if p := progress(t, ana.mustStatus(200, "GET", "/v1/goals/"+econID, nil)); p["unavailable"] != true {
		t.Errorf("com Finanças desligado: %v", p)
	}
	_, req, _, _ := ana.call("POST", "/v1/goals", map[string]any{
		"kind": "financial", "scope": "household", "title": "y",
		"financial": map[string]any{"target_cents": 1000, "source": "budget_savings", "category_id": groceries}})
	if req["code"] != "module_required" {
		t.Errorf("esperava module_required: %v", req)
	}
	ana.mustStatus(200, "PUT", "/v1/modules/finance", map[string]bool{"enabled": true})

	// --- projeto: a etapa com tarefa é a tarefa; o estado vem de um lugar só
	_, _, taskRaw, _ := bia.call("GET", "/v1/tasks", nil)
	var taskID string
	for _, tk := range decodeList(t, taskRaw) {
		if ref, _ := tk["ref"].(string); strings.HasPrefix(ref, "goal:"+projectID+":step:") {
			taskID = tk["id"].(string)
			if tk["assignee_id"] != biaID {
				t.Errorf("a tarefa é da Bia: %v", tk)
			}
		}
	}
	if taskID == "" {
		t.Fatalf("a etapa deveria ter virado tarefa: %s", taskRaw)
	}
	proj := ana.mustStatus(200, "GET", "/v1/goals/"+projectID, nil)
	steps := proj["steps"].([]any)
	if progress(t, proj)["done"] != float64(0) || progress(t, proj)["total"] != float64(2) || steps[0].(map[string]any)["has_task"] != true {
		t.Fatalf("projeto: %v", proj)
	}
	taskStep, checklist := steps[0].(map[string]any), steps[1].(map[string]any)
	ana.mustStatus(409, "POST", "/v1/goals/"+projectID+"/steps/"+taskStep["id"].(string)+"/done", nil) // step_has_task

	bia.mustStatus(200, "POST", "/v1/tasks/"+taskID+"/complete", nil)
	proj = ana.mustStatus(200, "GET", "/v1/goals/"+projectID, nil)
	if progress(t, proj)["done"] != float64(1) || proj["steps"].([]any)[0].(map[string]any)["done"] != true {
		t.Errorf("concluir a tarefa conclui a etapa: %v", proj["progress"])
	}
	proj = ana.mustStatus(200, "POST", "/v1/goals/"+projectID+"/steps/"+checklist["id"].(string)+"/done", nil)
	if p := progress(t, proj); p["percent"] != float64(100) || p["suggest_complete"] != true || proj["status"] != "active" {
		t.Errorf("todas prontas: o app sugere, não conclui: %v status=%v", p, proj["status"])
	}
	ana.mustStatus(200, "POST", "/v1/goals/"+projectID+"/complete", nil)
	ana.mustStatus(409, "POST", "/v1/goals/"+projectID+"/steps", map[string]any{"title": "tarde demais"}) // goal_closed

	// --- edição a dois: versão velha dá 409
	ver := saving["version"]
	bia.mustStatus(200, "PUT", "/v1/goals/"+savingID, map[string]any{"version": ver, "title": "Casa nova!"})
	_, cf, _, _ := ana.call("PUT", "/v1/goals/"+savingID, map[string]any{"version": ver, "title": "Casa velha"})
	if cf["code"] != "goal_conflict" {
		t.Errorf("esperava goal_conflict: %v", cf)
	}

	// --- exportação: cada um leva o que pode ver
	_, _, anaJSON, hdr := ana.call("GET", "/v1/goals/export", nil)
	if !strings.Contains(string(anaJSON), "Terapia") || !strings.Contains(hdr.Get("Content-Disposition"), "metas.json") {
		t.Errorf("exportação da Ana: %s", anaJSON)
	}
	_, _, biaJSON, _ := bia.call("GET", "/v1/goals/export", nil)
	if strings.Contains(string(biaJSON), "Terapia") || !strings.Contains(string(biaJSON), "Jantar fora") {
		t.Errorf("a exportação da Bia: %s", biaJSON)
	}
	_, _, csv, hdr := ana.call("GET", "/v1/goals/export?format=csv", nil)
	if !strings.HasPrefix(hdr.Get("Content-Type"), "text/csv") || !strings.Contains(string(csv), "record,goal_id") {
		t.Errorf("CSV: %s", csv)
	}

	// --- desligar esconde e não apaga; religar traz tudo de volta
	ana.mustStatus(200, "PUT", "/v1/modules/goals", map[string]bool{"enabled": false})
	ana.mustStatus(403, "GET", "/v1/goals", nil)
	bia.mustStatus(403, "POST", "/v1/goals/"+sharedID+"/checkins", nil)
	bia.mustStatus(200, "PUT", "/v1/modules/goals", map[string]bool{"enabled": true})
	_, _, raw, _ = ana.call("GET", "/v1/goals?status=all", nil)
	if all := ids(t, raw); !all[habitID] || !all[sharedID] || !all[savingID] || !all[projectID] || !all[econID] {
		t.Errorf("religar traz as metas de volta: %v", all)
	}
	if p := progress(t, ana.mustStatus(200, "GET", "/v1/goals/"+savingID, nil)); p["done"] != float64(250_000) {
		t.Errorf("os aportes continuam: %v", p)
	}

	// --- apagar: só quem pode; a pessoal do outro continua 404
	bia.mustStatus(404, "DELETE", "/v1/goals/"+habitID, nil)
	ana.mustStatus(204, "DELETE", "/v1/goals/"+habitID, nil)
	ana.mustStatus(404, "GET", "/v1/goals/"+habitID, nil)
	_ = anaID
}

// Sair do casal: as metas do casal ficam com quem permanece; as pessoais acompanham quem sai e chegam privadas
// ao casal novo (onde o módulo precisa ser ligado de novo: é por casal).
func TestGoalsFollowThePersonWhoLeaves(t *testing.T) {
	srv, _ := startApp(t)
	conn := testutil.OpenDB(t)
	ana, bia := newClient(t, srv.URL), newClient(t, srv.URL)

	ana.register("goal-leave-ana")
	hhOld := ana.mustStatus(201, "POST", "/v1/household", nil)["id"].(string)
	t.Cleanup(func() { conn.Exec(`DELETE FROM households WHERE id = $1`, hhOld) })
	invite := ana.mustStatus(201, "POST", "/v1/household/invites", nil)["token"].(string)
	bia.register("goal-leave-bia")
	bia.mustStatus(200, "POST", "/v1/household/invites/accept", map[string]string{"token": invite})
	ana.mustStatus(200, "PUT", "/v1/modules/goals", map[string]bool{"enabled": true})

	view := ana.mustStatus(201, "POST", "/v1/goals", map[string]any{
		"kind": "financial", "scope": "personal", "visibility": "view", "title": "Viagem",
		"financial": map[string]any{"target_cents": 100_000}})
	both := ana.mustStatus(201, "POST", "/v1/goals", map[string]any{
		"kind": "shared", "scope": "household", "title": "Cozinhar juntos",
		"shared": map[string]any{"target_count": 2, "period": "month", "requires_both": true}})
	viewID, bothID := view["id"].(string), both["id"].(string)
	ana.mustStatus(201, "POST", "/v1/goals/"+viewID+"/contributions", map[string]any{"amount_cents": 40_000})
	bia.mustStatus(200, "GET", "/v1/goals/"+viewID, nil) // enquanto era "view", a Bia lia o progresso

	ana.mustStatus(204, "POST", "/v1/household/leave", nil)
	bia.mustStatus(404, "GET", "/v1/goals/"+viewID, nil) // virou privada e saiu com a Ana
	left := bia.mustStatus(200, "GET", "/v1/goals/"+bothID, nil)
	if left["shared"].(map[string]any)["requires_both"] != false {
		t.Errorf("a meta do casal deixa de exigir os dois: %v", left["shared"])
	}

	hhNew := ana.mustStatus(201, "POST", "/v1/household", nil)["id"].(string)
	t.Cleanup(func() { conn.Exec(`DELETE FROM households WHERE id = $1`, hhNew) })
	ana.mustStatus(403, "GET", "/v1/goals", nil) // módulo é por casal: no casal novo está desligado
	ana.mustStatus(200, "PUT", "/v1/modules/goals", map[string]bool{"enabled": true})
	got := ana.mustStatus(200, "GET", "/v1/goals/"+viewID, nil)
	if got["visibility"] != "private" || progress(t, got)["done"] != float64(40_000) {
		t.Errorf("a meta pessoal vem junto, privada, com os aportes: %v", got)
	}
	_, _, raw, _ := ana.call("GET", "/v1/goals", nil)
	if all := ids(t, raw); all[bothID] || !all[viewID] {
		t.Errorf("no casal novo: só as pessoais: %v", all)
	}
}
