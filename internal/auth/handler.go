package auth

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"casal/internal/httpx"
	"casal/internal/identity"
)

// Options reúne os ajustes de segurança do handler.
type Options struct {
	CookieSecure bool
	TrustProxy   bool // confia no X-Forwarded-For para identificar o IP
}

type Handler struct {
	svc  *Service
	log  *slog.Logger
	opts Options

	// limites contra força bruta, em memória: por IP e, no login, também por e-mail
	registerByIP *httpx.Limiter
	loginByIP    *httpx.Limiter
	loginByEmail *httpx.Limiter
}

func NewHandler(svc *Service, log *slog.Logger, opts Options) *Handler {
	return &Handler{
		svc: svc, log: log, opts: opts,
		registerByIP: httpx.NewLimiter(5, 2*time.Minute),  // 5 cadastros seguidos, depois 1 a cada 2 min
		loginByIP:    httpx.NewLimiter(20, 6*time.Second), // 20 tentativas seguidas, depois 10 por minuto
		loginByEmail: httpx.NewLimiter(5, 3*time.Minute),  // 5 tentativas por conta, depois 1 a cada 3 min
	}
}

func (h *Handler) Routes(mux *http.ServeMux) {
	requireUser := Require(h.svc)
	mux.HandleFunc("GET /v1/auth/config", h.config)
	mux.HandleFunc("POST /v1/auth/register", h.register)
	mux.HandleFunc("POST /v1/auth/login", h.login)
	mux.HandleFunc("POST /v1/auth/logout", h.logout)
	mux.Handle("POST /v1/auth/logout-all", requireUser(http.HandlerFunc(h.logoutAll)))
	mux.Handle("GET /v1/me", requireUser(http.HandlerFunc(h.me)))
	mux.Handle("PATCH /v1/me", requireUser(http.HandlerFunc(h.updateMe)))
}

type credentials struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password"`
	// SignupCode só vale no cadastro, e só é exigido no modo invite do beta
	SignupCode string `json:"signup_code"`
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	if !h.allow(w, h.registerByIP, httpx.ClientIP(r, h.opts.TrustProxy)) {
		return
	}
	var in credentials
	if !httpx.Bind(w, r, &in) {
		return
	}
	u, session, err := h.svc.RegisterWithCode(r.Context(), in.Email, in.Name, in.Password, in.SignupCode)
	if err != nil {
		httpx.Fail(w, r, h.log, err)
		return
	}
	h.setCookie(w, session)
	httpx.JSON(w, http.StatusCreated, u)
}

// config diz ao front como o cadastro funciona (aberto ou por código), antes de a pessoa preencher o formulário.
func (h *Handler) config(w http.ResponseWriter, _ *http.Request) {
	mode := "open"
	if h.svc.InviteOnly() {
		mode = "invite"
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"signup_mode": mode})
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	if !h.allow(w, h.loginByIP, httpx.ClientIP(r, h.opts.TrustProxy)) {
		return
	}
	var in credentials
	if !httpx.Bind(w, r, &in) {
		return
	}
	if !h.allow(w, h.loginByEmail, strings.ToLower(strings.TrimSpace(in.Email))) {
		return
	}
	u, session, err := h.svc.Login(r.Context(), in.Email, in.Password)
	if err != nil {
		httpx.Fail(w, r, h.log, err)
		return
	}
	h.setCookie(w, session)
	httpx.JSON(w, http.StatusOK, u)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(CookieName); err == nil {
		_ = h.svc.Logout(r.Context(), c.Value)
	}
	h.clearCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

// logoutAll encerra a sessão em todos os aparelhos, por exemplo se um celular foi perdido.
func (h *Handler) logoutAll(w http.ResponseWriter, r *http.Request) {
	p, _ := identity.From(r.Context())
	if err := h.svc.LogoutAll(r.Context(), p.UserID); err != nil {
		httpx.Fail(w, r, h.log, err)
		return
	}
	h.clearCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	httpx.JSON(w, http.StatusOK, u)
}

// allow gasta uma tentativa da chave; se o limite acabou, já responde 429.
func (h *Handler) allow(w http.ResponseWriter, l *httpx.Limiter, key string) bool {
	ok, retry := l.Allow(key)
	if !ok {
		httpx.TooManyRequests(w, retry)
	}
	return ok
}

func (h *Handler) setCookie(w http.ResponseWriter, session string) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    session,
		Path:     "/",
		Expires:  time.Now().Add(h.svc.SessionTTL()),
		HttpOnly: true,
		Secure:   h.opts.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *Handler) clearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: CookieName, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: h.opts.CookieSecure, SameSite: http.SameSiteLaxMode,
	})
}

// updateMe muda o nome e/ou o fuso horário. Campo ausente fica como está.
func (h *Handler) updateMe(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name     *string `json:"name"`
		Timezone *string `json:"timezone"`
	}
	if !httpx.Bind(w, r, &in) {
		return
	}
	cur, _ := UserFromContext(r.Context())
	u, err := h.svc.UpdateProfile(r.Context(), cur, in.Name, in.Timezone)
	if err != nil {
		httpx.Fail(w, r, h.log, err)
		return
	}
	httpx.JSON(w, http.StatusOK, u)
}
