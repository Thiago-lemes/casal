package httpx

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"casal/internal/apperr"
)

func TestFailMapsKindsToStatus(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		status int
		body   string // trecho esperado na resposta
		hidden string // trecho que NÃO pode vazar para o cliente
	}{
		{"inválido", apperr.New(apperr.Invalid, "dados inválidos"), 400, "dados inválidos", ""},
		{"inválido com detalhe", fmt.Errorf("%w: o texto é obrigatório", apperr.New(apperr.Invalid, "dados inválidos")), 400, "o texto é obrigatório", ""},
		{"sem login", apperr.New(apperr.Unauthorized, "e-mail ou senha inválidos"), 401, "e-mail ou senha", ""},
		{"proibido", apperr.New(apperr.Forbidden, "sem casal"), 403, "sem casal", ""},
		{"não achou", apperr.New(apperr.NotFound, "não existe"), 404, "não existe", ""},
		{"conflito", apperr.New(apperr.Conflict, "já existe"), 409, "já existe", ""},
		{"serviço externo", fmt.Errorf("%w: apple 403 token xyz", apperr.New(apperr.Upstream, "não consegui enviar")), 502, "não consegui enviar", "token xyz"},
		{"erro desconhecido", errors.New("pq: conexão com senha=secreta falhou"), 500, "erro interno", "secreta"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			Fail(rec, httptest.NewRequest("GET", "/x", nil), slog.New(slog.NewTextHandler(io.Discard, nil)), c.err)
			if rec.Code != c.status {
				t.Errorf("status %d, esperava %d", rec.Code, c.status)
			}
			if !strings.Contains(rec.Body.String(), c.body) {
				t.Errorf("resposta %q deveria conter %q", rec.Body, c.body)
			}
			if c.hidden != "" && strings.Contains(rec.Body.String(), c.hidden) {
				t.Errorf("a resposta vazou %q: %s", c.hidden, rec.Body)
			}
		})
	}
}

func TestBindRejectsBadJSON(t *testing.T) {
	var in struct{ A string }
	rec := httptest.NewRecorder()
	if Bind(rec, httptest.NewRequest("POST", "/", strings.NewReader(`{"a":`)), &in) || rec.Code != http.StatusBadRequest {
		t.Errorf("JSON quebrado deveria dar 400 e false, deu %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	if Bind(rec, httptest.NewRequest("POST", "/", strings.NewReader(`{"campo_novo":1}`)), &in) {
		t.Error("campo desconhecido deveria ser rejeitado")
	}
}

func TestIsUUID(t *testing.T) {
	if !IsUUID("3c9649c1-0000-4000-8000-000000000000") {
		t.Error("uuid válido")
	}
	for _, bad := range []string{"", "abc", "3c9649c1-0000-4000-8000-00000000000g", "1; DROP TABLE x"} {
		if IsUUID(bad) {
			t.Errorf("%q não é uuid", bad)
		}
	}
}

func TestRequestIDIsGeneratedAndIgnoresClientValue(t *testing.T) {
	var seen string
	h := RequestID()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { seen = RequestIDFrom(r.Context()) }))
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Request-Id", "injetado\nlinha-falsa")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if seen == "" || seen != rec.Header().Get("X-Request-Id") {
		t.Errorf("o id do contexto (%q) deveria ser o devolvido no cabeçalho (%q)", seen, rec.Header().Get("X-Request-Id"))
	}
	if strings.Contains(seen, "injetado") {
		t.Error("o valor do cliente não pode ser usado")
	}
}
