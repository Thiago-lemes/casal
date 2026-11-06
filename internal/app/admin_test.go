package app

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"casal/internal/admin"
	"casal/internal/config"
	"casal/internal/testutil"
)

// ---- ajudantes do painel ----

func adminConfig() config.Config { return config.Config{SessionTTL: time.Hour, AdminEnabled: true} }

// newAdminAccount cria uma conta de administração como o comando faz e a remove no fim do teste.
func newAdminAccount(t *testing.T, conn *sql.DB, tag string) (email, pass, secret string) {
	t.Helper()
	svc, err := admin.NewService(admin.NewPostgresRepo(conn), admin.Deps{})
	if err != nil {
		t.Fatal(err)
	}
	c, err := svc.CreateAdmin(context.Background(), tag+"-"+time.Now().Format("150405.000000")+"@admin.teste.local")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Exec(`DELETE FROM admin_users WHERE id = $1`, c.Admin.ID) })
	return c.Admin.Email, c.Password, c.Secret
}

type adminClient struct {
	t     *testing.T
	base  string
	http  *http.Client
	email string // a conta do painel deste cliente (a auditoria é global; os testes olham só as próprias linhas)
}

func newAdminClient(t *testing.T, base string) *adminClient {
	jar, _ := cookiejar.New(nil)
	return &adminClient{t: t, base: base, http: &http.Client{Jar: jar}}
}

// do envia a requisição; csrf diz se vai o cabeçalho X-Requested-With (o front sempre manda nas escritas).
func (c *adminClient) do(method, path string, body any, csrf bool) (int, map[string]any, []byte, *http.Response) {
	c.t.Helper()
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, c.base+path, rd)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if csrf {
		req.Header.Set("X-Requested-With", "zoog-admin")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var obj map[string]any
	_ = json.Unmarshal(raw, &obj)
	return resp.StatusCode, obj, raw, resp
}

func (c *adminClient) must(want int, method, path string, body any) map[string]any {
	c.t.Helper()
	got, obj, raw, _ := c.do(method, path, body, true)
	if got != want {
		c.t.Fatalf("%s %s: esperava %d, veio %d: %s", method, path, want, got, raw)
	}
	return obj
}

// login entra com o código do instante (offset 0 = agora; 30s pula para o passo seguinte).
func (c *adminClient) login(email, pass, secret string, offset time.Duration) (int, map[string]any) {
	c.t.Helper()
	code, err := admin.Code(secret, time.Now().Add(offset))
	if err != nil {
		c.t.Fatal(err)
	}
	st, obj, _, _ := c.do("POST", "/admin/v1/auth/login", map[string]string{"email": email, "password": pass, "code": code}, true)
	return st, obj
}

func loggedInAdmin(t *testing.T, srv *httptest.Server, conn *sql.DB) *adminClient {
	t.Helper()
	email, pass, secret := newAdminAccount(t, conn, "painel")
	c := newAdminClient(t, srv.URL)
	c.email = email
	if st, obj := c.login(email, pass, secret, 0); st != 200 {
		t.Fatalf("login do painel: %d %v", st, obj)
	}
	return c
}

func auditActions(t *testing.T, c *adminClient) []string {
	t.Helper()
	_, _, raw, _ := c.do("GET", "/admin/v1/audit?limit=500", nil, false)
	var rows []struct {
		Action     string `json:"action"`
		AdminEmail string `json:"admin_email"`
	}
	_ = json.Unmarshal(raw, &rows)
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		if r.AdminEmail == c.email {
			out = append(out, r.Action)
		}
	}
	return out
}

func count(list []string, want string) int {
	n := 0
	for _, a := range list {
		if a == want {
			n++
		}
	}
	return n
}

// ---- testes ----

func TestPainelDesligadoResponde404(t *testing.T) {
	srv, _ := startApp(t) // ADMIN_ENABLED não definido
	for _, tc := range [][2]string{
		{"POST", "/admin/v1/auth/login"}, {"GET", "/admin/v1/households?q=ab"}, {"GET", "/admin/v1/audit"}, {"GET", "/admin/v1/me"},
	} {
		st, _, _, _ := newAdminClient(t, srv.URL).do(tc[0], tc[1], nil, true)
		if st != 404 {
			t.Errorf("%s %s com o painel desligado: %d, esperado 404", tc[0], tc[1], st)
		}
	}
}

func TestLoginDoPainelExigeSenhaETotp(t *testing.T) {
	conn := testutil.OpenDB(t)
	srv, _ := startAppWith(t, adminConfig())
	email, pass, secret := newAdminAccount(t, conn, "login")
	c := newAdminClient(t, srv.URL)

	fail := func(what string, st int, obj map[string]any) {
		t.Helper()
		if st != 401 || obj["code"] != "admin_login_failed" {
			t.Errorf("%s: %d %v (esperado 401 admin_login_failed)", what, st, obj)
		}
	}
	code, _ := admin.Code(secret, time.Now())

	st, obj, _, _ := c.do("POST", "/admin/v1/auth/login", map[string]string{"email": email, "password": "senha-errada", "code": code}, true)
	fail("senha errada", st, obj)
	st, obj, _, _ = c.do("POST", "/admin/v1/auth/login", map[string]string{"email": email, "password": pass}, true)
	fail("senha certa sem TOTP", st, obj)
	st, obj, _, _ = c.do("POST", "/admin/v1/auth/login", map[string]string{"email": email, "password": pass, "code": "000000"}, true)
	fail("TOTP errado", st, obj)

	// tudo certo: entra, com o cookie separado, restrito ao /admin e sem JavaScript
	st, obj, _, resp := c.do("POST", "/admin/v1/auth/login", map[string]string{"email": email, "password": pass, "code": code}, true)
	if st != 200 || obj["email"] != email {
		t.Fatalf("login correto: %d %v", st, obj)
	}
	var cookie *http.Cookie
	for _, ck := range resp.Cookies() {
		if ck.Name == "admin_session" {
			cookie = ck
		}
	}
	if cookie == nil || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != "/admin" {
		t.Fatalf("cookie do painel fora do combinado: %+v", cookie)
	}
	if d := time.Until(cookie.Expires); d < 7*time.Hour || d > 8*time.Hour+time.Minute {
		t.Errorf("a sessão do painel deve durar 8 h, dura %v", d)
	}
	c.must(200, "GET", "/admin/v1/me", nil)

	// o mesmo código não entra de novo (outro aparelho, mesmo instante)
	st, obj, _, _ = newAdminClient(t, srv.URL).do("POST", "/admin/v1/auth/login", map[string]string{"email": email, "password": pass, "code": code}, true)
	fail("TOTP reaproveitado", st, obj)
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestSessoesDeCasalEDoPainelSaoSeparadas(t *testing.T) {
	conn := testutil.OpenDB(t)
	srv, _ := startAppWith(t, adminConfig())
	ana := newClient(t, srv.URL)
	ana.register("sep-ana")
	ana.mustStatus(200, "GET", "/v1/me", nil)

	// a sessão do casal (casal_session, enviada ao /admin também por ter Path=/) não abre o painel
	if st, _, _, _ := ana.call("GET", "/admin/v1/me", nil); st != 401 {
		t.Errorf("sessão de casal no /admin: %d, esperado 401", st)
	}
	// e o valor do cookie do casal, apresentado como se fosse do painel, também não
	var casalCookie string
	for _, ck := range ana.http.Jar.Cookies(mustURL(t, srv.URL)) {
		if ck.Name == "casal_session" {
			casalCookie = ck.Value
		}
	}
	req, _ := http.NewRequest("GET", srv.URL+"/admin/v1/me", nil)
	req.AddCookie(&http.Cookie{Name: "admin_session", Value: casalCookie})
	if resp, err := http.DefaultClient.Do(req); err != nil || resp.StatusCode != 401 {
		t.Errorf("valor do cookie do casal como admin_session: %v %v", resp, err)
	}

	// a sessão do painel não abre o app
	adm := loggedInAdmin(t, srv, conn)
	var adminCookie string
	for _, ck := range adm.http.Jar.Cookies(mustURL(t, srv.URL+"/admin")) {
		if ck.Name == "admin_session" {
			adminCookie = ck.Value
		}
	}
	if adminCookie == "" {
		t.Fatal("sem cookie do painel")
	}
	req, _ = http.NewRequest("GET", srv.URL+"/v1/me", nil)
	req.AddCookie(&http.Cookie{Name: "admin_session", Value: adminCookie})
	req.AddCookie(&http.Cookie{Name: "casal_session", Value: adminCookie})
	if resp, err := http.DefaultClient.Do(req); err != nil || resp.StatusCode != 401 {
		t.Errorf("sessão do painel no /v1: %v %v", resp, err)
	}
}

func TestEscritasDoPainelExigemCabecalho(t *testing.T) {
	conn := testutil.OpenDB(t)
	srv, _ := startAppWith(t, adminConfig())
	adm := loggedInAdmin(t, srv, conn)
	st, obj, _, _ := adm.do("POST", "/admin/v1/signup-codes", map[string]any{"note": "x"}, false)
	if st != 403 || obj["code"] != "csrf" {
		t.Errorf("escrita sem X-Requested-With: %d %v", st, obj)
	}
	// sem sessão: 401 antes de tudo
	if st, _, _, _ := newAdminClient(t, srv.URL).do("GET", "/admin/v1/households?q=ab", nil, false); st != 401 {
		t.Errorf("sem sessão: %d", st)
	}
}

func TestFichaDoCasalMostraSoMetadados(t *testing.T) {
	const segredo = "segredo-de-teste-ficha"
	conn := testutil.OpenDB(t)
	srv, _ := startAppWith(t, adminConfig())

	ana, bia := newClient(t, srv.URL), newClient(t, srv.URL)
	anaID := ana.register("ficha-ana")
	ana.mustStatus(201, "POST", "/v1/household", nil)
	invite := ana.mustStatus(201, "POST", "/v1/household/invites", nil)["token"].(string)
	biaID := bia.register("ficha-bia")
	bia.mustStatus(200, "POST", "/v1/household/invites/accept", map[string]string{"token": invite})
	ana.mustStatus(201, "POST", "/v1/notes", map[string]any{"title": segredo, "kind": "text", "body": segredo, "visibility": "private"})
	ana.mustStatus(201, "POST", "/v1/reminders", map[string]any{
		"recipient_id": biaID, "body": segredo, "remind_at": time.Now().UTC().Add(time.Hour).Format(time.RFC3339),
	})

	adm := loggedInAdmin(t, srv, conn)

	// busca por e-mail (parte) e por nome; curta demais é recusada
	found := func(q string) []any {
		_, _, raw, _ := adm.do("GET", "/admin/v1/households?q="+q, nil, false)
		var list []any
		_ = json.Unmarshal(raw, &list)
		return list
	}
	if list := found("ficha-ana"); len(list) != 1 {
		t.Fatalf("busca por e-mail: %d resultados", len(list))
	}
	if list := found("Ficha-Bia"); len(list) != 1 {
		t.Fatalf("busca por nome: %d resultados", len(list))
	}
	adm.must(400, "GET", "/admin/v1/households?q=a", nil)

	hid := found("ficha-ana")[0].(map[string]any)["id"].(string)
	_, _, raw, _ := adm.do("GET", "/admin/v1/households/"+hid, nil, false)
	if strings.Contains(string(raw), segredo) {
		t.Fatalf("a ficha vazou o conteúdo do casal: %s", raw)
	}
	var file struct {
		Household struct {
			Members []struct{ Email, Name string }
			Counts  map[string]int
		}
	}
	_ = json.Unmarshal(raw, &file)
	if len(file.Household.Members) != 2 {
		t.Errorf("a ficha deveria listar as duas pessoas: %s", raw)
	}
	// a nota privada da Ana entra na contagem do total (a ficha não distingue pessoal de casal), sem mostrar o texto
	if file.Household.Counts["notes"] != 1 || file.Household.Counts["reminders"] != 1 {
		t.Errorf("contagens: %v", file.Household.Counts)
	}

	// ficha da pessoa
	_, _, rawUser, _ := adm.do("GET", "/admin/v1/users/"+anaID, nil, false)
	if strings.Contains(string(rawUser), segredo) || !strings.Contains(string(rawUser), "ficha-ana") {
		t.Errorf("ficha da pessoa: %s", rawUser)
	}
	adm.must(404, "GET", "/admin/v1/households/00000000-0000-0000-0000-000000000000", nil)
	adm.must(404, "GET", "/admin/v1/households/nao-e-uuid", nil)
	adm.must(404, "GET", "/admin/v1/users/00000000-0000-0000-0000-000000000000", nil)

	// abrir as fichas é auditado
	actions := auditActions(t, adm)
	if count(actions, "household_viewed") != 1 || count(actions, "user_viewed") != 1 {
		t.Errorf("auditoria das leituras: %v", actions)
	}
}

func TestPainelLigaModuloPeloMesmoCaminhoDoApp(t *testing.T) {
	conn := testutil.OpenDB(t)
	srv, _ := startAppWith(t, adminConfig())
	ana := newClient(t, srv.URL)
	ana.register("mod-ana")
	ana.mustStatus(201, "POST", "/v1/household", nil)
	ana.mustStatus(403, "GET", "/v1/finance/categories", nil) // módulo desligado

	adm := loggedInAdmin(t, srv, conn)
	_, _, raw, _ := adm.do("GET", "/admin/v1/households?q=mod-ana", nil, false)
	var list []struct{ ID string }
	_ = json.Unmarshal(raw, &list)
	if len(list) != 1 {
		t.Fatalf("busca: %s", raw)
	}
	id := list[0].ID

	adm.must(200, "PUT", "/admin/v1/households/"+id+"/modules/finance", map[string]any{"enabled": true})
	// o gancho OnEnable do module.Service rodou: as categorias padrão existem
	_, _, cats, _ := ana.call("GET", "/v1/finance/categories", nil)
	if !bytes.Contains(cats, []byte(`"name"`)) {
		t.Errorf("ligar pelo painel deveria semear as categorias, veio %s", cats)
	}
	adm.must(200, "PUT", "/admin/v1/households/"+id+"/modules/finance", map[string]any{"enabled": false})
	ana.mustStatus(403, "GET", "/v1/finance/categories", nil)

	// módulo que não existe e corpo sem enabled
	adm.must(404, "PUT", "/admin/v1/households/"+id+"/modules/inexistente", map[string]any{"enabled": true})
	adm.must(400, "PUT", "/admin/v1/households/"+id+"/modules/finance", map[string]any{})
	adm.must(404, "PUT", "/admin/v1/households/00000000-0000-0000-0000-000000000000/modules/finance", map[string]any{"enabled": true})

	actions := auditActions(t, adm)
	if count(actions, "module_enabled") != 1 || count(actions, "module_disabled") != 1 {
		t.Errorf("auditoria dos módulos: %v", actions)
	}
	// antes e depois ficam registrados, só com valores estruturados
	_, _, auditRaw, _ := adm.do("GET", "/admin/v1/audit", nil, false)
	if !strings.Contains(string(auditRaw), `"module":"finance"`) || !strings.Contains(string(auditRaw), `"enabled":true`) {
		t.Errorf("auditoria sem antes/depois: %s", auditRaw)
	}
}

func TestPainelBloqueiaSemApagarNada(t *testing.T) {
	conn := testutil.OpenDB(t)
	srv, _ := startAppWith(t, adminConfig())
	ana := newClient(t, srv.URL)
	anaID := ana.register("blq-ana")
	ana.mustStatus(201, "POST", "/v1/household", nil)
	ana.mustStatus(201, "POST", "/v1/notes", map[string]any{"title": "minha nota", "kind": "text", "body": "x", "visibility": "private"})
	ana.mustStatus(200, "GET", "/v1/me", nil)

	adm := loggedInAdmin(t, srv, conn)
	got := adm.must(200, "POST", "/admin/v1/users/"+anaID+"/block", nil)
	if got["blocked_at"] == nil {
		t.Errorf("a ficha devia mostrar o bloqueio: %v", got)
	}

	// a sessão que já estava aberta cai na requisição seguinte
	ana.mustStatus(401, "GET", "/v1/me", nil)
	// o login responde account_blocked (senha certa); com senha errada não revela nada
	st, obj, _, _ := newClient(t, srv.URL).call("POST", "/v1/auth/login", map[string]string{"email": "blq-ana" + e2eDomain, "password": "senha-forte-123"})
	if st != 403 || obj["code"] != "account_blocked" {
		t.Errorf("login bloqueado: %d %v", st, obj)
	}
	st, obj, _, _ = newClient(t, srv.URL).call("POST", "/v1/auth/login", map[string]string{"email": "blq-ana" + e2eDomain, "password": "errada-errada"})
	if st != 401 || obj["code"] != "invalid_credentials" {
		t.Errorf("senha errada em conta bloqueada: %d %v", st, obj)
	}

	// nada foi apagado: a pessoa, o casal e a nota continuam no banco
	var users, members, notes int
	testutil.SystemScan(t, conn, `SELECT (SELECT count(*) FROM users WHERE id = $1),
		(SELECT count(*) FROM household_members WHERE user_id = $1),
		(SELECT count(*) FROM notes WHERE owner_id = $1)`, []any{anaID}, &users, &members, &notes)
	if users != 1 || members != 1 || notes != 1 {
		t.Errorf("bloquear apagou dados: users=%d members=%d notes=%d", users, members, notes)
	}

	// desbloquear devolve o acesso
	adm.must(200, "POST", "/admin/v1/users/"+anaID+"/unblock", nil)
	if st, _, _, _ := newClient(t, srv.URL).call("POST", "/v1/auth/login", map[string]string{"email": "blq-ana" + e2eDomain, "password": "senha-forte-123"}); st != 200 {
		t.Errorf("depois do desbloqueio o login deve voltar, deu %d", st)
	}

	actions := auditActions(t, adm)
	if count(actions, "user_blocked") != 1 || count(actions, "user_unblocked") != 1 {
		t.Errorf("auditoria do bloqueio: %v", actions)
	}
}

func TestPainelAdministraCodigosDeCadastro(t *testing.T) {
	conn := testutil.OpenDB(t)
	cfg := adminConfig()
	cfg.SignupMode = "invite"
	srv, _ := startAppWith(t, cfg)
	adm := loggedInAdmin(t, srv, conn)

	created := adm.must(201, "POST", "/admin/v1/signup-codes", map[string]any{"note": "amiga da Carla", "max_uses": 1, "days": 7})
	raw, _ := created["code"].(string)
	id := created["signup_code"].(map[string]any)["id"].(string)
	t.Cleanup(func() { conn.Exec(`DELETE FROM signup_codes WHERE id = $1`, id) })
	if raw == "" {
		t.Fatalf("o código bruto deve aparecer na criação: %v", created)
	}
	// a listagem nunca mostra o valor
	_, _, list, _ := adm.do("GET", "/admin/v1/signup-codes", nil, false)
	if bytes.Contains(list, []byte(raw)) || !bytes.Contains(list, []byte("amiga da Carla")) {
		t.Errorf("listagem: %s", list)
	}

	reg := func(name string) int {
		st, _, _, _ := newClient(t, srv.URL).call("POST", "/v1/auth/register", map[string]string{
			"email": name + e2eDomain, "name": name, "password": "senha-forte-123", "signup_code": raw})
		return st
	}
	if st := reg("cod-ana"); st != 201 {
		t.Fatalf("cadastro com o código criado pelo painel: %d", st)
	}
	adm.must(400, "POST", "/admin/v1/signup-codes", map[string]any{"max_uses": 500})

	adm.must(204, "DELETE", "/admin/v1/signup-codes/"+id, nil)
	adm.must(404, "DELETE", "/admin/v1/signup-codes/00000000-0000-0000-0000-000000000000", nil)
	if st := reg("cod-bia"); st != 400 {
		t.Errorf("código revogado deveria ser recusado, deu %d", st)
	}

	actions := auditActions(t, adm)
	if count(actions, "invite_created") != 1 || count(actions, "invite_revoked") != 1 {
		t.Errorf("auditoria dos convites: %v", actions)
	}
}

// A auditoria só aceita inserção: o papel do app não consegue mudar nem apagar o que já foi registrado.
func TestAuditoriaSoAceitaInsercao(t *testing.T) {
	conn := testutil.OpenDB(t)
	var ins, sel, upd, del, trunc bool
	if err := conn.QueryRow(`SELECT
		has_table_privilege('casal_app', 'admin_audit', 'INSERT'),
		has_table_privilege('casal_app', 'admin_audit', 'SELECT'),
		has_table_privilege('casal_app', 'admin_audit', 'UPDATE'),
		has_table_privilege('casal_app', 'admin_audit', 'DELETE'),
		has_table_privilege('casal_app', 'admin_audit', 'TRUNCATE')`).Scan(&ins, &sel, &upd, &del, &trunc); err != nil {
		t.Fatal(err)
	}
	if !ins || !sel || upd || del || trunc {
		t.Errorf("casal_app em admin_audit: insert=%v select=%v update=%v delete=%v truncate=%v (esperado true true false false false)",
			ins, sel, upd, del, trunc)
	}

	// e na prática, quando o teste roda como o papel do app
	var user string
	_ = conn.QueryRow(`SELECT current_user`).Scan(&user)
	if user == "casal_app" {
		if _, err := conn.Exec(`UPDATE admin_audit SET action = 'admin_login' WHERE false`); err == nil {
			t.Error("UPDATE na auditoria deveria ser negado a casal_app")
		}
		if _, err := conn.Exec(`DELETE FROM admin_audit WHERE false`); err == nil {
			t.Error("DELETE na auditoria deveria ser negado a casal_app")
		}
	}
}

// A mensagem escrita no app chega ao painel; é a única exceção ao "nunca ver conteúdo", e só vive na tabela de feedback.
func TestFeedbackDoAppAoPainel(t *testing.T) {
	const msg = "mensagem-de-teste-feedback: o botão sumiu"
	conn := testutil.OpenDB(t)
	cfg := adminConfig()
	cfg.MetricsToken = "token-feedback"
	srv, _ := startAppWith(t, cfg)

	ana := newClient(t, srv.URL)
	anaID := ana.register("fb-ana")
	// sem casal ainda: 403
	ana.mustStatus(403, "POST", "/v1/feedback", map[string]string{"message": msg})
	ana.mustStatus(201, "POST", "/v1/household", nil)

	ana.mustStatus(400, "POST", "/v1/feedback", map[string]string{"message": "   "})
	ana.mustStatus(400, "POST", "/v1/feedback", map[string]string{"message": "oi", "screen": "tela-inventada"})
	ana.mustStatus(204, "POST", "/v1/feedback", map[string]string{"message": msg, "screen": "tasks"})
	// o casal não tem como ler (nem o próprio, nem o dos outros): a rota de leitura não existe
	if st, _, _, _ := ana.call("GET", "/v1/feedback", nil); st != 404 && st != 405 {
		t.Errorf("GET /v1/feedback não deveria existir, deu %d", st)
	}

	adm := loggedInAdmin(t, srv, conn)
	_, _, raw, _ := adm.do("GET", "/admin/v1/feedback?unread=true", nil, false)
	var items []struct {
		ID      string `json:"id"`
		Author  string `json:"author"`
		Message string `json:"message"`
		Screen  string `json:"screen"`
		ReadAt  any    `json:"read_at"`
	}
	_ = json.Unmarshal(raw, &items)
	var id string
	for _, it := range items {
		if it.Message == msg {
			id = it.ID
			if it.Author != "fb-ana" || it.Screen != "tasks" || it.ReadAt != nil {
				t.Errorf("mensagem no painel: %+v", it)
			}
		}
	}
	if id == "" {
		t.Fatalf("o painel não achou a mensagem: %s", raw)
	}

	adm.must(204, "POST", "/admin/v1/feedback/"+id+"/read", nil)
	_, _, raw, _ = adm.do("GET", "/admin/v1/feedback?unread=true", nil, false)
	if bytes.Contains(raw, []byte(msg)) {
		t.Errorf("depois de lida, a mensagem não pode estar nas não lidas: %s", raw)
	}
	adm.must(404, "POST", "/admin/v1/feedback/00000000-0000-0000-0000-000000000000/read", nil)

	// a auditoria registra a leitura sem copiar o texto
	_, _, audit, _ := adm.do("GET", "/admin/v1/audit?limit=500", nil, false)
	if count(auditActions(t, adm), "feedback_read") != 1 || bytes.Contains(audit, []byte(msg)) {
		t.Errorf("auditoria do feedback: %v / vazou o texto: %v", auditActions(t, adm), bytes.Contains(audit, []byte(msg)))
	}

	// o texto não vai para métricas nem para eventos
	if _, out := scrape(t, srv.URL, "token-feedback"); strings.Contains(out, "mensagem-de-teste") {
		t.Errorf("o /metrics vazou o feedback:\n%s", out)
	}
	names := testutil.SystemStrings(t, conn, `SELECT name FROM product_events WHERE user_id = $1`, anaID)
	if count(names, "feedback_sent") != 1 {
		t.Errorf("evento feedback_sent: %v", names)
	}
}
