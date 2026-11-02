package shopping

import (
	"context"
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
	mux.Handle("POST /v1/shopping/items", wrap(h.create))
	mux.Handle("GET /v1/shopping/items", wrap(h.list))
	mux.Handle("PUT /v1/shopping/items/{id}", wrap(h.update))
	mux.Handle("POST /v1/shopping/items/{id}/bought", wrap(h.bought))
	mux.Handle("POST /v1/shopping/items/{id}/unbought", wrap(h.unbought))
	mux.Handle("DELETE /v1/shopping/items/{id}", wrap(h.delete))
	mux.Handle("POST /v1/shopping/restock", wrap(h.restock))
}

type body struct {
	Name      string   `json:"name"`
	Quantity  *float64 `json:"quantity"`
	Unit      string   `json:"unit"`
	Category  string   `json:"category"`
	Recurring bool     `json:"recurring"`
}

func (b body) input() Input {
	return Input(b)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var in body
	if !httpx.Bind(w, r, &in) {
		return
	}
	p, _ := identity.From(r.Context())
	it, merged, err := h.svc.Create(r.Context(), p.HouseholdID, p.UserID, in.input())
	if err != nil {
		httpx.Fail(w, r, h.log, err)
		return
	}
	// merged avisa que o item já estava na lista e a quantidade foi somada nele
	status := http.StatusCreated
	if merged {
		status = http.StatusOK
	}
	httpx.JSON(w, status, struct {
		Item
		Merged bool `json:"merged"`
	}{it, merged})
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
	out, err := h.svc.Update(r.Context(), p.HouseholdID, id, in.input())
	if err != nil {
		httpx.Fail(w, r, h.log, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handler) bought(w http.ResponseWriter, r *http.Request) {
	h.mark(w, r, h.svc.MarkBought)
}

func (h *Handler) unbought(w http.ResponseWriter, r *http.Request) {
	h.mark(w, r, h.svc.Unmark)
}

func (h *Handler) mark(w http.ResponseWriter, r *http.Request, f func(ctx context.Context, householdID, id, userID string) (Item, error)) {
	id, ok := h.idParam(w, r)
	if !ok {
		return
	}
	p, _ := identity.From(r.Context())
	out, err := f(r.Context(), p.HouseholdID, id, p.UserID)
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
	if err := h.svc.Delete(r.Context(), p.HouseholdID, id); err != nil {
		httpx.Fail(w, r, h.log, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) restock(w http.ResponseWriter, r *http.Request) {
	p, _ := identity.From(r.Context())
	n, err := h.svc.Restock(r.Context(), p.HouseholdID)
	if err != nil {
		httpx.Fail(w, r, h.log, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]int64{"restocked": n})
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
