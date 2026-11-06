package httpx

import (
	"context"
	"net/http"
	"sync/atomic"
)

type routeKey struct{}

// RouteHolder guarda o padrão da rota atendida (GET /v1/reminders/{id}) num lugar que os middlewares mais externos
// também enxergam: o mux só preenche o padrão na cópia da requisição que ele recebe, e quem está acima dele (o Recover,
// por exemplo) não veria nada. Deve ficar perto do topo da cadeia.
func RouteHolder() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			holder := new(atomic.Value)
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), routeKey{}, holder)))
		})
	}
}

// SetRoute registra o padrão da rota. Quem chama é o código que já passou pelo mux (as métricas).
func SetRoute(ctx context.Context, pattern string) {
	if h, ok := ctx.Value(routeKey{}).(*atomic.Value); ok {
		h.Store(pattern)
	}
}

// RouteFrom devolve o padrão da rota, ou "" se ainda não houve roteamento.
func RouteFrom(ctx context.Context) string {
	if h, ok := ctx.Value(routeKey{}).(*atomic.Value); ok {
		if s, ok := h.Load().(string); ok {
			return s
		}
	}
	return ""
}

// ServerErrorHook é chamado para cada erro inesperado (500) e cada panic, no mesmo ponto em que ele já vai para o log.
// err é nulo no panic, e panicValue só vem preenchido nele. Quem registra não deve guardar o texto do erro nem da
// requisição: eles podem conter dados digitados pela pessoa.
type ServerErrorHook func(r *http.Request, err error, panicValue any)

var serverErrorHook atomic.Pointer[ServerErrorHook]

// OnServerError registra o gancho (uma vez, na montagem do app). Sem registro, nada acontece.
func OnServerError(h ServerErrorHook) { serverErrorHook.Store(&h) }

func reportServerError(r *http.Request, err error, panicValue any) {
	if h := serverErrorHook.Load(); h != nil && *h != nil {
		(*h)(r, err, panicValue)
	}
}
