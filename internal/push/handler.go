package push

import (
	"log/slog"
	"net/http"

	"casal/internal/httpx"
	"casal/internal/identity"
)

type Handler struct {
	svc         *Service
	vapidPublic string
	log         *slog.Logger
}

func NewHandler(svc *Service, vapidPublic string, log *slog.Logger) *Handler {
	return &Handler{svc: svc, vapidPublic: vapidPublic, log: log}
}

func (h *Handler) Routes(mux *http.ServeMux, requireUser httpx.Middleware) {
	mux.HandleFunc("GET /v1/push/vapid-key", h.vapidKey)
	mux.Handle("POST /v1/push/subscriptions", httpx.Chain(http.HandlerFunc(h.subscribe), requireUser))
	mux.Handle("DELETE /v1/push/subscriptions", httpx.Chain(http.HandlerFunc(h.unsubscribe), requireUser))
	mux.Handle("POST /v1/push/test", httpx.Chain(http.HandlerFunc(h.test), requireUser))
}

func (h *Handler) vapidKey(w http.ResponseWriter, r *http.Request) {
	httpx.JSON(w, http.StatusOK, map[string]string{"public_key": h.vapidPublic})
}

// O corpo é o que o navegador devolve em subscription.toJSON().
func (h *Handler) subscribe(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Endpoint       string `json:"endpoint"`
		ExpirationTime any    `json:"expirationTime"`
		Keys           struct {
			P256dh string `json:"p256dh"`
			Auth   string `json:"auth"`
		} `json:"keys"`
	}
	if !httpx.Bind(w, r, &in) {
		return
	}
	p, _ := identity.From(r.Context())
	sub := Subscription{Endpoint: in.Endpoint, P256dh: in.Keys.P256dh, Auth: in.Keys.Auth}
	if err := h.svc.Subscribe(r.Context(), p.UserID, sub, r.UserAgent()); err != nil {
		httpx.Fail(w, r, h.log, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) unsubscribe(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Endpoint string `json:"endpoint"`
	}
	if !httpx.Bind(w, r, &in) {
		return
	}
	if in.Endpoint == "" {
		httpx.Error(w, http.StatusBadRequest, "informe o endpoint")
		return
	}
	p, _ := identity.From(r.Context())
	if err := h.svc.Unsubscribe(r.Context(), p.UserID, in.Endpoint); err != nil {
		httpx.Fail(w, r, h.log, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// test manda um aviso para os aparelhos da própria pessoa. Útil para validar o push.
func (h *Handler) test(w http.ResponseWriter, r *http.Request) {
	p, _ := identity.From(r.Context())
	if err := h.svc.Notify(r.Context(), p.UserID, "Teste", "Funcionou! Este aviso veio do servidor."); err != nil {
		httpx.Fail(w, r, h.log, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
