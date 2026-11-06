package admin

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"casal/internal/httpx"
)

// CookieName é o cookie da sessão do painel. É outro nome, outra tabela e outro Path: a sessão de um casal não abre o
// painel, e a do painel não abre o app.
const CookieName = "admin_session"

// Options reúne os ajustes de segurança do painel.
type Options struct {
	CookieSecure bool
	TrustProxy   bool
}

type Handler struct {
	svc  *Service
	log  *slog.Logger
	opts Options

	// mais duros que os do app: o painel é a superfície de maior risco
	loginByIP    *httpx.Limiter
	loginByEmail *httpx.Limiter
}

func NewHandler(svc *Service, log *slog.Logger, opts Options) *Handler {
	return &Handler{
		svc: svc, log: log, opts: opts,
		loginByIP:    httpx.NewLimiter(5, time.Minute),   // 5 tentativas seguidas, depois 1 por minuto
		loginByEmail: httpx.NewLimiter(5, 5*time.Minute), // 5 por conta, depois 1 a cada 5 minutos
	}
}

type adminKey struct{}

// FromContext devolve a conta de administração da requisição (dentro das rotas protegidas).
func FromContext(ctx context.Context) (Admin, bool) {
	a, ok := ctx.Value(adminKey{}).(Admin)
	return a, ok
}

// Routes liga o painel em /admin/v1. Sem isto (ADMIN_ENABLED=false) o app liga NotFound no lugar.
func (h *Handler) Routes(mux *http.ServeMux) {
	guard := func(f http.HandlerFunc) http.Handler { return noStore(h.requireAdmin(http.HandlerFunc(f))) }

	mux.Handle("POST /admin/v1/auth/login", noStore(http.HandlerFunc(h.login)))
	mux.Handle("POST /admin/v1/auth/logout", noStore(http.HandlerFunc(h.logout)))
	mux.Handle("GET /admin/v1/me", guard(h.me))

	mux.Handle("GET /admin/v1/households", guard(h.searchHouseholds))
	mux.Handle("GET /admin/v1/households/{id}", guard(h.household))
	mux.Handle("PUT /admin/v1/households/{id}/modules/{name}", guard(h.setModule))
	mux.Handle("GET /admin/v1/users/{id}", guard(h.user))
	mux.Handle("POST /admin/v1/users/{id}/block", guard(h.block(true)))
	mux.Handle("POST /admin/v1/users/{id}/unblock", guard(h.block(false)))

	mux.Handle("GET /admin/v1/signup-codes", guard(h.listCodes))
	mux.Handle("POST /admin/v1/signup-codes", guard(h.createCode))
	mux.Handle("DELETE /admin/v1/signup-codes/{id}", guard(h.revokeCode))

	mux.Handle("GET /admin/v1/feedback", guard(h.listFeedback))
	mux.Handle("POST /admin/v1/feedback/{id}/read", guard(h.readFeedback))

	mux.Handle("GET /admin/v1/audit", guard(h.audit))
}

// Disabled responde 404 a tudo em /admin/v1 quando o painel está desligado (ADMIN_ENABLED=false, o padrão).
func Disabled() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		httpx.Error(w, http.StatusNotFound, "não encontrado")
	})
}

// noStore impede que navegador ou proxy guardem respostas do painel.
func noStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

// requireAdmin exige a sessão do painel (cookie admin_session) e, nas escritas, o cabeçalho X-Requested-With: um
// formulário de outro site não consegue enviá-lo, o que complementa o SameSite=Strict do cookie.
func (h *Handler) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(CookieName)
		if err != nil {
			httpx.Error(w, http.StatusUnauthorized, "faça login")
			return
		}
		a, err := h.svc.AdminFromToken(r.Context(), c.Value)
		if err != nil {
			httpx.Error(w, http.StatusUnauthorized, "sessão inválida ou expirada")
			return
		}
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
		default:
			if r.Header.Get("X-Requested-With") == "" {
				httpx.ErrorCode(w, http.StatusForbidden, "csrf", "requisição não permitida")
				return
			}
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), adminKey{}, a)))
	})
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	if !h.allow(w, h.loginByIP, httpx.ClientIP(r, h.opts.TrustProxy)) {
		return
	}
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Code     string `json:"code"`
	}
	if !httpx.Bind(w, r, &in) {
		return
	}
	if !h.allow(w, h.loginByEmail, strings.ToLower(strings.TrimSpace(in.Email))) {
		return
	}
	a, session, err := h.svc.Login(r.Context(), in.Email, in.Password, in.Code)
	if err != nil {
		httpx.Fail(w, r, h.log, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: CookieName, Value: session, Path: "/admin",
		Expires:  time.Now().Add(SessionTTL),
		HttpOnly: true, Secure: h.opts.CookieSecure, SameSite: http.SameSiteStrictMode,
	})
	httpx.JSON(w, http.StatusOK, a)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(CookieName); err == nil {
		_ = h.svc.Logout(r.Context(), c.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name: CookieName, Value: "", Path: "/admin", MaxAge: -1,
		HttpOnly: true, Secure: h.opts.CookieSecure, SameSite: http.SameSiteStrictMode,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) allow(w http.ResponseWriter, l *httpx.Limiter, key string) bool {
	ok, retry := l.Allow(key)
	if !ok {
		httpx.TooManyRequests(w, retry)
	}
	return ok
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	a, _ := FromContext(r.Context())
	httpx.JSON(w, http.StatusOK, a)
}

func (h *Handler) searchHouseholds(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.Search(r.Context(), strings.TrimSpace(r.URL.Query().Get("q")))
	if err != nil {
		httpx.Fail(w, r, h.log, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

// uuidParam responde 404 a um id malformado, em vez de mandá-lo ao banco.
func (h *Handler) uuidParam(w http.ResponseWriter, r *http.Request, name string) (string, bool) {
	id := r.PathValue(name)
	if !httpx.IsUUID(id) {
		httpx.Fail(w, r, h.log, ErrNotFound)
		return "", false
	}
	return id, true
}

func (h *Handler) household(w http.ResponseWriter, r *http.Request) {
	id, ok := h.uuidParam(w, r, "id")
	if !ok {
		return
	}
	a, _ := FromContext(r.Context())
	d, mods, err := h.svc.HouseholdFile(r.Context(), a, id)
	if err != nil {
		httpx.Fail(w, r, h.log, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"household": d, "modules": mods})
}

func (h *Handler) setModule(w http.ResponseWriter, r *http.Request) {
	id, ok := h.uuidParam(w, r, "id")
	if !ok {
		return
	}
	var in struct {
		Enabled *bool `json:"enabled"`
	}
	if !httpx.Bind(w, r, &in) {
		return
	}
	if in.Enabled == nil {
		httpx.Error(w, http.StatusBadRequest, "informe enabled")
		return
	}
	a, _ := FromContext(r.Context())
	mods, err := h.svc.SetModule(r.Context(), a, id, r.PathValue("name"), *in.Enabled)
	if err != nil {
		httpx.Fail(w, r, h.log, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"modules": mods})
}

func (h *Handler) user(w http.ResponseWriter, r *http.Request) {
	id, ok := h.uuidParam(w, r, "id")
	if !ok {
		return
	}
	a, _ := FromContext(r.Context())
	u, err := h.svc.UserFile(r.Context(), a, id)
	if err != nil {
		httpx.Fail(w, r, h.log, err)
		return
	}
	httpx.JSON(w, http.StatusOK, u)
}

func (h *Handler) block(blocked bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := h.uuidParam(w, r, "id")
		if !ok {
			return
		}
		a, _ := FromContext(r.Context())
		u, err := h.svc.SetBlocked(r.Context(), a, id, blocked)
		if err != nil {
			httpx.Fail(w, r, h.log, err)
			return
		}
		httpx.JSON(w, http.StatusOK, u)
	}
}

func (h *Handler) listCodes(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.SignupCodes(r.Context())
	if err != nil {
		httpx.Fail(w, r, h.log, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handler) createCode(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Note    string `json:"note"`
		MaxUses int    `json:"max_uses"`
		Days    int    `json:"days"`
	}
	if !httpx.Bind(w, r, &in) {
		return
	}
	if in.MaxUses == 0 {
		in.MaxUses = 1
	}
	if in.Days == 0 {
		in.Days = 14
	}
	a, _ := FromContext(r.Context())
	c, raw, err := h.svc.CreateSignupCode(r.Context(), a, in.Note, in.MaxUses, in.Days)
	if err != nil {
		httpx.Fail(w, r, h.log, err)
		return
	}
	// o código bruto só aparece nesta resposta
	httpx.JSON(w, http.StatusCreated, map[string]any{"code": raw, "signup_code": c})
}

func (h *Handler) revokeCode(w http.ResponseWriter, r *http.Request) {
	id, ok := h.uuidParam(w, r, "id")
	if !ok {
		return
	}
	a, _ := FromContext(r.Context())
	if err := h.svc.RevokeSignupCode(r.Context(), a, id); err != nil {
		httpx.Fail(w, r, h.log, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) listFeedback(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.FeedbackList(r.Context(), r.URL.Query().Get("unread") == "true")
	if err != nil {
		httpx.Fail(w, r, h.log, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handler) readFeedback(w http.ResponseWriter, r *http.Request) {
	id, ok := h.uuidParam(w, r, "id")
	if !ok {
		return
	}
	a, _ := FromContext(r.Context())
	if err := h.svc.FeedbackRead(r.Context(), a, id); err != nil {
		httpx.Fail(w, r, h.log, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) audit(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	out, err := h.svc.Audit(r.Context(), limit)
	if err != nil {
		httpx.Fail(w, r, h.log, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}
