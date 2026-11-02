package module

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

func (h *Handler) Routes(mux *http.ServeMux, requireUser, requireHousehold httpx.Middleware) {
	wrap := func(f http.HandlerFunc) http.Handler {
		return httpx.Chain(f, requireUser, requireHousehold)
	}
	mux.Handle("GET /v1/modules", wrap(h.list))
	mux.Handle("PUT /v1/modules/{name}", wrap(h.set))
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	p, _ := identity.From(r.Context())
	out, err := h.svc.List(r.Context(), p.HouseholdID)
	if err != nil {
		httpx.Fail(w, r, h.log, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handler) set(w http.ResponseWriter, r *http.Request) {
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
	p, _ := identity.From(r.Context())
	out, err := h.svc.Set(r.Context(), p.HouseholdID, r.PathValue("name"), *in.Enabled)
	if err != nil {
		httpx.Fail(w, r, h.log, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}
