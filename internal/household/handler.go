package household

import (
	"log/slog"
	"net/http"

	"casal/internal/httpx"
	"casal/internal/identity"
)

type Handler struct {
	svc *Service
	log *slog.Logger
}

func NewHandler(svc *Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// Routes recebe o middleware de login para não depender de como ele é construído.
func (h *Handler) Routes(mux *http.ServeMux, requireUser httpx.Middleware) {
	requireHH := RequireHousehold(h.svc, h.log)
	mux.Handle("POST /v1/household", httpx.Chain(http.HandlerFunc(h.create), requireUser))
	mux.Handle("GET /v1/household", httpx.Chain(http.HandlerFunc(h.get), requireUser, requireHH))
	mux.Handle("POST /v1/household/invites", httpx.Chain(http.HandlerFunc(h.invite), requireUser, requireHH))
	mux.Handle("POST /v1/household/invites/accept", httpx.Chain(http.HandlerFunc(h.accept), requireUser))
	mux.Handle("POST /v1/household/leave", httpx.Chain(http.HandlerFunc(h.leave), requireUser))
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	p, _ := identity.From(r.Context())
	id, err := h.svc.Create(r.Context(), p.UserID)
	if err != nil {
		httpx.Fail(w, r, h.log, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]string{"id": id})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	p, _ := identity.From(r.Context())
	members, err := h.svc.Members(r.Context(), p.HouseholdID)
	if err != nil {
		httpx.Fail(w, r, h.log, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"id": p.HouseholdID, "members": members})
}

func (h *Handler) invite(w http.ResponseWriter, r *http.Request) {
	p, _ := identity.From(r.Context())
	token, expires, err := h.svc.Invite(r.Context(), p.HouseholdID, p.UserID)
	if err != nil {
		httpx.Fail(w, r, h.log, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"token": token, "expires_at": expires})
}

func (h *Handler) accept(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Token string `json:"token"`
	}
	if !httpx.Bind(w, r, &in) {
		return
	}
	if in.Token == "" {
		httpx.Error(w, http.StatusBadRequest, "informe o token do convite")
		return
	}
	p, _ := identity.From(r.Context())
	id, err := h.svc.Accept(r.Context(), p.UserID, in.Token)
	if err != nil {
		httpx.Fail(w, r, h.log, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"household_id": id})
}

// leave tira a pessoa do casal. Não exige estar em um casal pelo middleware: sem casal, a resposta é 403 do próprio serviço.
func (h *Handler) leave(w http.ResponseWriter, r *http.Request) {
	p, _ := identity.From(r.Context())
	if err := h.svc.Leave(r.Context(), p.UserID); err != nil {
		httpx.Fail(w, r, h.log, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
