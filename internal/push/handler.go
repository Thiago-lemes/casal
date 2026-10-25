package push

import (
	"errors"
	"log/slog"
	"net/http"

	"casal/internal/auth"
	"casal/internal/httpx"
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
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.Error(w, http.StatusBadRequest, "JSON inválido")
		return
	}
	u, _ := auth.UserFromContext(r.Context())
	sub := Subscription{Endpoint: in.Endpoint, P256dh: in.Keys.P256dh, Auth: in.Keys.Auth}
	if err := h.svc.Subscribe(r.Context(), u.ID, sub, r.UserAgent()); err != nil {
		h.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) unsubscribe(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Endpoint string `json:"endpoint"`
	}
	if err := httpx.Decode(w, r, &in); err != nil || in.Endpoint == "" {
		httpx.Error(w, http.StatusBadRequest, "JSON inválido")
		return
	}
	u, _ := auth.UserFromContext(r.Context())
	if err := h.svc.Unsubscribe(r.Context(), u.ID, in.Endpoint); err != nil {
		h.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// test manda um aviso para os aparelhos da própria pessoa. Útil para validar o push.
func (h *Handler) test(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFromContext(r.Context())
	if err := h.svc.Notify(r.Context(), u.ID, "Teste", "Funcionou! Este aviso veio do servidor."); err != nil {
		h.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidSubscription):
		httpx.Error(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, ErrNoDevices):
		httpx.Error(w, http.StatusConflict, err.Error())
	default:
		h.log.Warn("erro em push", "err", err)
		httpx.Error(w, http.StatusBadGateway, "não consegui enviar o aviso")
	}
}
