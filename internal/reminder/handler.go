package reminder

import (
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	"casal/internal/auth"
	"casal/internal/household"
	"casal/internal/httpx"
)

var uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

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

// ids devolve o usuário e o casal da requisição (já validados pelos middlewares).
func ids(r *http.Request) (userID, householdID string) {
	u, _ := auth.UserFromContext(r.Context())
	hid, _ := household.IDFromContext(r.Context())
	return u.ID, hid
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var in struct {
		RecipientID string    `json:"recipient_id"`
		Body        string    `json:"body"`
		RemindAt    time.Time `json:"remind_at"`
	}
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.Error(w, http.StatusBadRequest, "JSON inválido")
		return
	}
	uid, hid := ids(r)
	out, err := h.svc.Create(r.Context(), hid, uid, in.RecipientID, in.Body, in.RemindAt)
	if err != nil {
		h.fail(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, out)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	uid, hid := ids(r)
	out, err := h.svc.List(r.Context(), hid, uid)
	if err != nil {
		h.fail(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handler) resolve(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !uuidRe.MatchString(id) {
		httpx.Error(w, http.StatusNotFound, ErrNotFound.Error())
		return
	}
	uid, hid := ids(r)
	out, err := h.svc.Resolve(r.Context(), hid, id, uid)
	if err != nil {
		h.fail(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handler) snooze(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !uuidRe.MatchString(id) {
		httpx.Error(w, http.StatusNotFound, ErrNotFound.Error())
		return
	}
	var in struct {
		Minutes int `json:"minutes"`
	}
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.Error(w, http.StatusBadRequest, "JSON inválido")
		return
	}
	uid, hid := ids(r)
	out, err := h.svc.Snooze(r.Context(), hid, id, uid, in.Minutes)
	if err != nil {
		h.fail(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handler) cancel(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !uuidRe.MatchString(id) {
		httpx.Error(w, http.StatusNotFound, ErrNotFound.Error())
		return
	}
	uid, hid := ids(r)
	if err := h.svc.Cancel(r.Context(), hid, id, uid); err != nil {
		h.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidInput):
		httpx.Error(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, ErrNotFound):
		httpx.Error(w, http.StatusNotFound, err.Error())
	default:
		h.log.Error("erro em reminder", "err", err)
		httpx.Error(w, http.StatusInternalServerError, "erro interno")
	}
}
