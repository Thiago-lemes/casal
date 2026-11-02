package task

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
	mux.Handle("POST /v1/tasks", wrap(h.create))
	mux.Handle("GET /v1/tasks", wrap(h.list))
	mux.Handle("POST /v1/tasks/{id}/complete", wrap(h.complete))
	mux.Handle("POST /v1/tasks/{id}/decline", wrap(h.decline))
	mux.Handle("POST /v1/tasks/{id}/assign", wrap(h.assign))
	mux.Handle("DELETE /v1/tasks/{id}", wrap(h.cancel))
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Title      string     `json:"title"`
		AssigneeID string     `json:"assignee_id"`
		Shared     bool       `json:"shared"`
		DueAt      *time.Time `json:"due_at"`
		Repeat     string     `json:"repeat"`
		Rotation   bool       `json:"rotation"`
	}
	if !httpx.Bind(w, r, &in) {
		return
	}
	p, _ := identity.From(r.Context())
	out, err := h.svc.Create(r.Context(), p.HouseholdID, p.UserID, CreateInput{
		Title: in.Title, AssigneeID: in.AssigneeID, Shared: in.Shared, DueAt: in.DueAt,
		Repeat: in.Repeat, Rotation: in.Rotation,
	})
	if err != nil {
		httpx.Fail(w, r, h.log, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, out)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	p, _ := identity.From(r.Context())
	out, err := h.svc.List(r.Context(), p.HouseholdID, r.URL.Query().Get("status"))
	if err != nil {
		httpx.Fail(w, r, h.log, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handler) complete(w http.ResponseWriter, r *http.Request) {
	id, ok := h.idParam(w, r)
	if !ok {
		return
	}
	p, _ := identity.From(r.Context())
	out, err := h.svc.Complete(r.Context(), p.HouseholdID, id, p.UserID)
	if err != nil {
		httpx.Fail(w, r, h.log, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handler) decline(w http.ResponseWriter, r *http.Request) {
	id, ok := h.idParam(w, r)
	if !ok {
		return
	}
	var in struct {
		Reason string `json:"reason"`
	}
	if !httpx.Bind(w, r, &in) {
		return
	}
	p, _ := identity.From(r.Context())
	out, err := h.svc.Decline(r.Context(), p.HouseholdID, id, p.UserID, in.Reason)
	if err != nil {
		httpx.Fail(w, r, h.log, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handler) assign(w http.ResponseWriter, r *http.Request) {
	id, ok := h.idParam(w, r)
	if !ok {
		return
	}
	var in struct {
		AssigneeID string `json:"assignee_id"`
		Shared     bool   `json:"shared"`
	}
	if !httpx.Bind(w, r, &in) {
		return
	}
	p, _ := identity.From(r.Context())
	out, err := h.svc.Reassign(r.Context(), p.HouseholdID, id, p.UserID, in.AssigneeID, in.Shared)
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
