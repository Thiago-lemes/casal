package app

import (
	"encoding/json"
	"testing"

	"casal/internal/testutil"
)

// list lê uma resposta que é uma lista JSON.
func (c *client) list(path string) []map[string]any {
	c.t.Helper()
	status, _, raw, _ := c.call("GET", path, nil)
	if status != 200 {
		c.t.Fatalf("GET %s: esperava 200, veio %d: %s", path, status, raw)
	}
	var out []map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		c.t.Fatalf("GET %s: não é uma lista: %s", path, raw)
	}
	return out
}

func titles(items []map[string]any, key string) []string {
	out := []string{}
	for _, it := range items {
		out = append(out, it[key].(string))
	}
	return out
}

// A jornada de sair do casal, de ponta a ponta: o que era compartilhado fica com quem permanece,
// o que era pessoal acompanha quem sai, e a pessoa pode criar outro casal ou voltar por um convite.
func TestLeavingTheHouseholdAndComingBack(t *testing.T) {
	srv, _ := startApp(t)
	conn := testutil.OpenDB(t)
	ana, bia := newClient(t, srv.URL), newClient(t, srv.URL)

	anaID := ana.register("leave-ana")
	biaID := bia.register("leave-bia")

	hhOld := ana.mustStatus(201, "POST", "/v1/household", nil)["id"].(string)
	t.Cleanup(func() { conn.Exec(`DELETE FROM households WHERE id = $1`, hhOld) })
	invite := ana.mustStatus(201, "POST", "/v1/household/invites", nil)["token"].(string)
	bia.mustStatus(200, "POST", "/v1/household/invites/accept", map[string]string{"token": invite})

	// --- a Ana monta um pouco de tudo
	ana.mustStatus(201, "POST", "/v1/notes", map[string]any{"title": "ideias que a Bia lê", "kind": "text", "body": "x", "visibility": "view"})
	ana.mustStatus(201, "POST", "/v1/notes", map[string]any{"title": "privada da Ana", "kind": "text", "body": "y"})
	ana.mustStatus(201, "POST", "/v1/notes", map[string]any{"title": "do casal", "kind": "text", "body": "z", "visibility": "household"})
	ana.mustStatus(201, "POST", "/v1/routine/blocks", map[string]any{
		"title": "Trabalho", "category": "work", "recurrence": "weekly", "weekdays": []int{1, 2, 3, 4, 5},
		"start_date": "2026-11-02", "start": "09:00", "end": "18:00", "visibility": "view",
	})
	ana.mustStatus(201, "POST", "/v1/tasks", map[string]any{"title": "tirar o lixo", "assignee_id": biaID})
	ana.mustStatus(201, "POST", "/v1/shopping/items", map[string]any{"name": "leite"})

	if got := titles(bia.list("/v1/notes"), "title"); len(got) != 2 {
		t.Fatalf("a Bia deveria ver 2 notas (a de leitura e a do casal): %v", got)
	}
	if len(bia.list("/v1/routine/blocks")) != 1 {
		t.Fatal("a Bia deveria ver o bloco de trabalho da Ana")
	}

	// --- fuso: a Ana diz onde está (e um fuso inválido é recusado, com código estável)
	ana.mustStatus(200, "PATCH", "/v1/me", map[string]string{"timezone": "America/Manaus"})
	if _, body, _, _ := ana.call("PATCH", "/v1/me", map[string]string{"timezone": "Marte/Olympus"}); body["code"] != "invalid" {
		t.Errorf("fuso inválido deveria trazer code=invalid: %v", body)
	}

	// --- a Ana sai do casal
	ana.mustStatus(204, "POST", "/v1/household/leave", nil)
	status, body, _, _ := ana.call("GET", "/v1/notes", nil)
	if status != 403 || body["code"] != "no_household" {
		t.Errorf("sem casal: esperava 403 com code=no_household, veio %d %v", status, body)
	}
	ana.mustStatus(403, "POST", "/v1/household/leave", nil) // já saiu

	// o que era compartilhado fica com a Bia; o que era da Ana saiu com ela
	notes := titles(bia.list("/v1/notes"), "title")
	if len(notes) != 1 || notes[0] != "do casal" {
		t.Errorf("a Bia deveria ficar só com a nota do casal: %v", notes)
	}
	if len(bia.list("/v1/routine/blocks")) != 0 {
		t.Error("o bloco que a Bia via era da Ana e deveria ter saído com ela")
	}
	tasks := bia.list("/v1/tasks?status=open")
	if len(tasks) != 1 || tasks[0]["assignee_id"] != biaID || tasks[0]["created_by"] != biaID {
		t.Errorf("a tarefa deveria passar para a Bia: %v", tasks)
	}
	if got := titles(bia.list("/v1/shopping/items"), "name"); len(got) != 1 || got[0] != "leite" {
		t.Errorf("a lista de compras é do casal e fica com a Bia: %v", got)
	}
	members := bia.mustStatus(200, "GET", "/v1/household", nil)["members"].([]any)
	if len(members) != 1 {
		t.Errorf("o casal da Bia agora tem 1 pessoa: %d", len(members))
	}

	// --- a Ana cria um casal só dela: as notas e a rotina dela vêm junto, privadas
	hhNew := ana.mustStatus(201, "POST", "/v1/household", nil)["id"].(string)
	t.Cleanup(func() { conn.Exec(`DELETE FROM households WHERE id = $1`, hhNew) })
	mine := titles(ana.list("/v1/notes"), "title")
	if len(mine) != 2 {
		t.Fatalf("a Ana deveria ter as 2 notas dela de volta: %v", mine)
	}
	blocks := ana.list("/v1/routine/blocks")
	if len(blocks) != 1 || blocks[0]["visibility"] != "private" || blocks[0]["owner_id"] != anaID {
		t.Errorf("o bloco volta privado: %v", blocks)
	}
	if got := titles(bia.list("/v1/notes"), "title"); len(got) != 1 {
		t.Errorf("a Bia não pode ver as notas da Ana no casal novo: %v", got)
	}

	// --- a Ana sai do casal só dela e volta para o da Bia, por convite
	ana.mustStatus(204, "POST", "/v1/household/leave", nil)
	again := bia.mustStatus(201, "POST", "/v1/household/invites", nil)["token"].(string)
	ana.mustStatus(200, "POST", "/v1/household/invites/accept", map[string]string{"token": again})

	if got := titles(ana.list("/v1/notes"), "title"); len(got) != 3 {
		t.Errorf("a Ana deveria ver as 2 dela mais a do casal: %v", got)
	}
	if got := titles(bia.list("/v1/notes"), "title"); len(got) != 1 || got[0] != "do casal" {
		t.Errorf("de volta ao mesmo casal, as notas da Ana continuam privadas: %v", got)
	}
}

// Um convite criado por quem saiu não vale mais, e quem entra por um convite de quem ficou, vale.
func TestInviteOfWhoLeftIsRevoked(t *testing.T) {
	srv, _ := startApp(t)
	conn := testutil.OpenDB(t)
	ana, bia, cris := newClient(t, srv.URL), newClient(t, srv.URL), newClient(t, srv.URL)

	ana.register("revoga-ana")
	bia.register("revoga-bia")
	cris.register("revoga-cris")
	hh := ana.mustStatus(201, "POST", "/v1/household", nil)["id"].(string)
	t.Cleanup(func() { conn.Exec(`DELETE FROM households WHERE id = $1`, hh) })
	forBia := ana.mustStatus(201, "POST", "/v1/household/invites", nil)["token"].(string)
	bia.mustStatus(200, "POST", "/v1/household/invites/accept", map[string]string{"token": forBia})

	// com o casal cheio, a Ana não gera convite; mas a Bia sai e a Ana convida a Cris antes de sair também
	bia.mustStatus(204, "POST", "/v1/household/leave", nil)
	forCris := ana.mustStatus(201, "POST", "/v1/household/invites", nil)["token"].(string)
	ana.mustStatus(204, "POST", "/v1/household/leave", nil)
	cris.mustStatus(404, "POST", "/v1/household/invites/accept", map[string]string{"token": forCris})
}
