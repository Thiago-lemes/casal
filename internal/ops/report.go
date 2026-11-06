package ops

import (
	"fmt"
	"net/http"
	"time"

	"github.com/getsentry/sentry-go"

	"casal/internal/httpx"
	"casal/internal/identity"
)

// Reporter envia erros inesperados ao Sentry (ou GlitchTip) sem nenhum conteúdo da pessoa.
//
// O que sai: método, padrão da rota (/v1/reminders/{id}, nunca o caminho real), status, request_id, o tipo Go do
// erro, a pilha de chamadas e o user_id. Nunca: corpo, cookies, cabeçalhos, query string, e-mail, nome nem o texto do
// erro (um erro do banco pode trazer o valor que a pessoa digitou).
type Reporter struct {
	client *sentry.Client
}

// ReporterOptions configura o envio. Transport existe para os testes trocarem a rede por um fake.
type ReporterOptions struct {
	DSN         string
	Environment string
	Release     string
	Transport   sentry.Transport
}

// NewReporter devolve nil (sem erro) quando não há DSN: o relatório fica desligado.
func NewReporter(o ReporterOptions) (*Reporter, error) {
	if o.DSN == "" && o.Transport == nil {
		return nil, nil
	}
	client, err := sentry.NewClient(sentry.ClientOptions{
		Dsn:              o.DSN,
		Environment:      o.Environment,
		Release:          o.Release,
		Transport:        o.Transport,
		AttachStacktrace: false,
		MaxBreadcrumbs:   -1,
		BeforeSend:       scrub,
	})
	if err != nil {
		return nil, fmt.Errorf("sentry: %w", err)
	}
	return &Reporter{client: client}, nil
}

// Hook liga o reporter ao ponto em que o servidor já loga o erro inesperado.
func (rp *Reporter) Hook() httpx.ServerErrorHook {
	return func(r *http.Request, err error, panicValue any) {
		rp.Capture(r, err, panicValue)
	}
}

// Capture monta e envia o evento. Nunca falha nem bloqueia a resposta.
func (rp *Reporter) Capture(r *http.Request, err error, panicValue any) {
	if rp == nil {
		return
	}
	route := r.Pattern
	if route == "" {
		route = httpx.RouteFrom(r.Context())
	}
	if route == "" {
		route = "desconhecida"
	}
	kind := "erro"
	typ := "erro"
	if panicValue != nil {
		kind, typ = "panic", fmt.Sprintf("%T", panicValue)
	} else if err != nil {
		typ = fmt.Sprintf("%T", err)
	}

	ev := sentry.NewEvent()
	ev.Level = sentry.LevelError
	ev.Message = kind + " inesperado em " + route // texto fixo: não vem de err.Error()
	ev.Fingerprint = []string{kind, route, typ}
	ev.Tags = map[string]string{
		"request_id": httpx.RequestIDFrom(r.Context()),
		"route":      route,
		"method":     r.Method,
		"error_type": typ,
	}
	if p, ok := identity.From(r.Context()); ok && p.UserID != "" {
		ev.User = sentry.User{ID: p.UserID}
	}
	if st := sentry.NewStacktrace(); st != nil {
		ev.Threads = []sentry.Thread{{Stacktrace: st, Crashed: true, Current: true}}
	}
	rp.client.CaptureEvent(ev, nil, nil)
}

// Flush espera o envio pendente (chame ao desligar o servidor).
func (rp *Reporter) Flush(timeout time.Duration) {
	if rp != nil {
		rp.client.Flush(timeout)
	}
}

// scrub é a última barreira antes do envio: mesmo que alguém acrescente dados ao evento, tudo o que pode ter
// conteúdo da pessoa é removido aqui. Há teste que prova isso.
func scrub(ev *sentry.Event, _ *sentry.EventHint) *sentry.Event {
	ev.Request = nil
	ev.Contexts = nil
	ev.Breadcrumbs = nil
	ev.Modules = nil
	ev.Exception = nil // as mensagens de exceção podem trazer texto digitado
	ev.User = sentry.User{ID: ev.User.ID}
	ev.ServerName = ""
	return ev
}
