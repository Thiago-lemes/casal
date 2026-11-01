package household

import (
	"errors"
	"log/slog"
	"net/http"

	"casal/internal/auth"
	"casal/internal/httpx"
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
	requireHH := RequireHousehold(h.svc)
	mux.Handle("POST /v1/household", httpx.Chain(http.HandlerFunc(h.create), requireUser))
	mux.Handle("GET /v1/household", httpx.Chain(http.HandlerFunc(h.get), requireUser, requireHH))
	mux.Handle("POST /v1/household/invites", httpx.Chain(http.HandlerFunc(h.invite), requireUser, requireHH))
	mux.Handle("POST /v1/household/invites/accept", httpx.Chain(http.HandlerFunc(h.accept), requireUser))
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFromContext(r.Context())
	id, err := h.svc.Create(r.Context(), u.ID)
	if err != nil {
		h.fail(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]string{"id": id})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, _ := IDFromContext(r.Context())
	members, err := h.svc.Members(r.Context(), id)
	if err != nil {
		h.fail(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"id": id, "members": members})
}

func (h *Handler) invite(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFromContext(r.Context())
	hid, _ := IDFromContext(r.Context())
	token, expires, err := h.svc.Invite(r.Context(), hid, u.ID)
	if err != nil {
		h.fail(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"token": token, "expires_at": expires})
}

func (h *Handler) accept(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Token string `json:"token"`
	}
	if err := httpx.Decode(w, r, &in); err != nil || in.Token == "" {
		httpx.Error(w, http.StatusBadRequest, "informe o token do convite")
		return
	}
	u, _ := auth.UserFromContext(r.Context())
	id, err := h.svc.Accept(r.Context(), u.ID, in.Token)
	if err != nil {
		h.fail(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"household_id": id})
}

func (h *Handler) fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInviteInvalid):
		httpx.Error(w, http.StatusNotFound, err.Error())
	case errors.Is(err, ErrAlreadyInHousehold), errors.Is(err, ErrHouseholdFull):
		httpx.Error(w, http.StatusConflict, err.Error())
	case errors.Is(err, ErrNoHousehold):
		httpx.Error(w, http.StatusForbidden, err.Error())
	default:
		h.log.Error("erro em household", "err", err)
		httpx.Error(w, http.StatusInternalServerError, "erro interno")
	}
}
