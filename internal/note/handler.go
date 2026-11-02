package note

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
	mux.Handle("POST /v1/notes", wrap(h.create))
	mux.Handle("GET /v1/notes", wrap(h.list))
	mux.Handle("GET /v1/notes/{id}", wrap(h.get))
	mux.Handle("PUT /v1/notes/{id}", wrap(h.update))
	mux.Handle("DELETE /v1/notes/{id}", wrap(h.delete))
}

// body é o JSON de criação e de edição. Kind só vale na criação; Version só na edição.
type body struct {
	Title      string   `json:"title"`
	Kind       string   `json:"kind"`
	Body       string   `json:"body"`
	Items      []Item   `json:"items"`
	Tags       []string `json:"tags"`
	Visibility string   `json:"visibility"`
	Pinned     bool     `json:"pinned"`
	Version    int      `json:"version"`
}

func (b body) content() Content {
	return Content{
		Title: b.Title, Kind: b.Kind, Body: b.Body, Items: b.Items, Tags: b.Tags,
		Visibility: b.Visibility, Pinned: b.Pinned,
	}
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var in body
	if !httpx.Bind(w, r, &in) {
		return
	}
	p, _ := identity.From(r.Context())
	out, err := h.svc.Create(r.Context(), p.HouseholdID, p.UserID, in.content())
	if err != nil {
		httpx.Fail(w, r, h.log, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, out)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	p, _ := identity.From(r.Context())
	q := r.URL.Query()
	out, err := h.svc.List(r.Context(), p.HouseholdID, p.UserID, q.Get("q"), q.Get("tag"))
	if err != nil {
		httpx.Fail(w, r, h.log, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, ok := h.idParam(w, r)
	if !ok {
		return
	}
	p, _ := identity.From(r.Context())
	out, err := h.svc.Get(r.Context(), p.HouseholdID, id, p.UserID)
	if err != nil {
		httpx.Fail(w, r, h.log, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id, ok := h.idParam(w, r)
	if !ok {
		return
	}
	var in body
	if !httpx.Bind(w, r, &in) {
		return
	}
	p, _ := identity.From(r.Context())
	out, err := h.svc.Update(r.Context(), p.HouseholdID, id, p.UserID, in.content(), in.Version)
	if err != nil {
		httpx.Fail(w, r, h.log, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	id, ok := h.idParam(w, r)
	if !ok {
		return
	}
	p, _ := identity.From(r.Context())
	if err := h.svc.Delete(r.Context(), p.HouseholdID, id, p.UserID); err != nil {
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
