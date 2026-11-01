package reminder

import (
	"log/slog"
	"net/http"
	"time"

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
	mux.Handle("POST /v1/reminders", wrap(h.create))
	mux.Handle("GET /v1/reminders", wrap(h.list))
	mux.Handle("POST /v1/reminders/{id}/resolve", wrap(h.resolve))
	mux.Handle("POST /v1/reminders/{id}/snooze", wrap(h.snooze))
	mux.Handle("DELETE /v1/reminders/{id}", wrap(h.cancel))
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var in struct {
		RecipientID string    `json:"recipient_id"`
		Body        string    `json:"body"`
		RemindAt    time.Time `json:"remind_at"`
	}
	if !httpx.Bind(w, r, &in) {
		return
	}
	p, _ := identity.From(r.Context())
	out, err := h.svc.Create(r.Context(), p.HouseholdID, p.UserID, in.RecipientID, in.Body, in.RemindAt)
	if err != nil {
		httpx.Fail(w, r, h.log, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, out)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	p, _ := identity.From(r.Context())
	out, err := h.svc.List(r.Context(), p.HouseholdID, p.UserID)
	if err != nil {
		httpx.Fail(w, r, h.log, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handler) resolve(w http.ResponseWriter, r *http.Request) {
	id, ok := h.idParam(w, r)
	if !ok {
		return
	}
	p, _ := identity.From(r.Context())
	out, err := h.svc.Resolve(r.Context(), p.HouseholdID, id, p.UserID)
	if err != nil {
		httpx.Fail(w, r, h.log, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handler) snooze(w http.ResponseWriter, r *http.Request) {
	id, ok := h.idParam(w, r)
	if !ok {
		return
	}
	var in struct {
		Minutes int `json:"minutes"`
	}
	if !httpx.Bind(w, r, &in) {
		return
	}
	p, _ := identity.From(r.Context())
	out, err := h.svc.Snooze(r.Context(), p.HouseholdID, id, p.UserID, in.Minutes)
	if err != nil {
		httpx.Fail(w, r, h.log, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handler) cancel(w http.ResponseWriter, r *http.Request) {
	id, ok := h.idParam(w, r)
	if !ok {
		return
	}
	p, _ := identity.From(r.Context())
	if err := h.svc.Cancel(r.Context(), p.HouseholdID, id, p.UserID); err != nil {
		httpx.Fail(w, r, h.log, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// idParam lê o {id} da URL. Um id malformado responde 404, igual a "não existe".
func (h *Handler) idParam(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("id")
	if !httpx.IsUUID(id) {
		httpx.Fail(w, r, h.log, ErrNotFound)
		return "", false
	}
	return id, true
}
