package auth

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"casal/internal/httpx"
)

// Options reúne os ajustes de segurança do handler.
type Options struct {
	CookieSecure bool
	TrustProxy   bool // confia no X-Forwarded-For para identificar o IP
}

type Handler struct {
	svc    *Service
	log    *slog.Logger
	secure bool
	opts   Options

	// limites contra força bruta, em memória: por IP e, no login, também por e-mail
	registerByIP *httpx.Limiter
	loginByIP    *httpx.Limiter
	loginByEmail *httpx.Limiter
}

func NewHandler(svc *Service, log *slog.Logger, opts Options) *Handler {
	return &Handler{
		svc: svc, log: log, secure: opts.CookieSecure, opts: opts,
		registerByIP: httpx.NewLimiter(5, 2*time.Minute),  // 5 cadastros seguidos, depois 1 a cada 2 min
		loginByIP:    httpx.NewLimiter(20, 6*time.Second), // 20 tentativas seguidas, depois 10 por minuto
		loginByEmail: httpx.NewLimiter(5, 3*time.Minute),  // 5 tentativas por conta, depois 1 a cada 3 min
	}
}

func (h *Handler) Routes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/auth/register", h.register)
	mux.HandleFunc("POST /v1/auth/login", h.login)
	mux.HandleFunc("POST /v1/auth/logout", h.logout)
	mux.Handle("GET /v1/me", Require(h.svc)(http.HandlerFunc(h.me)))
}

type credentials struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	if !h.allow(w, h.registerByIP, httpx.ClientIP(r, h.opts.TrustProxy)) {
		return
	}
	var in credentials
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.Error(w, http.StatusBadRequest, "JSON inválido")
		return
	}
	u, token, err := h.svc.Register(r.Context(), in.Email, in.Name, in.Password)
	if err != nil {
		h.fail(w, err)
		return
	}
	h.setCookie(w, token)
	httpx.JSON(w, http.StatusCreated, u)
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	if !h.allow(w, h.loginByIP, httpx.ClientIP(r, h.opts.TrustProxy)) {
		return
	}
	var in credentials
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.Error(w, http.StatusBadRequest, "JSON inválido")
		return
	}
	if !h.allow(w, h.loginByEmail, strings.ToLower(strings.TrimSpace(in.Email))) {
		return
	}
	u, token, err := h.svc.Login(r.Context(), in.Email, in.Password)
	if err != nil {
		h.fail(w, err)
		return
	}
	h.setCookie(w, token)
	httpx.JSON(w, http.StatusOK, u)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(CookieName); err == nil {
		_ = h.svc.Logout(r.Context(), c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: CookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: h.secure, SameSite: http.SameSiteLaxMode})
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	httpx.JSON(w, http.StatusOK, u)
}

func (h *Handler) setCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		Expires:  time.Now().Add(h.svc.SessionTTL()),
		HttpOnly: true,
		Secure:   h.secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *Handler) fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidInput):
		httpx.Error(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, ErrEmailTaken):
		httpx.Error(w, http.StatusConflict, err.Error())
	case errors.Is(err, ErrInvalidCredentials):
		httpx.Error(w, http.StatusUnauthorized, err.Error())
	default:
		h.log.Error("erro em auth", "err", err)
		httpx.Error(w, http.StatusInternalServerError, "erro interno")
	}
}

// allow gasta uma tentativa da chave; se o limite acabou, já responde 429.
func (h *Handler) allow(w http.ResponseWriter, l *httpx.Limiter, key string) bool {
	ok, retry := l.Allow(key)
	if !ok {
		httpx.TooManyRequests(w, retry)
	}
	return ok
}
