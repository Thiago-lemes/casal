package finance

import (
	"net/http"

	"casal/internal/httpx"
)

type cardBody struct {
	Name       string `json:"name"`
	Brand      string `json:"brand"`
	Last4      string `json:"last4"`
	ClosingDay *int   `json:"closing_day"`
	DueDay     *int   `json:"due_day"`
}

func (b cardBody) input() CardInput { return CardInput(b) }

func (h *Handler) listCards(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.ListCards(r.Context(), who(r), r.URL.Query().Get("archived") == "true")
	reply(h, w, r, out, err)
}

func (h *Handler) createCard(w http.ResponseWriter, r *http.Request) {
	var in cardBody
	if !httpx.Bind(w, r, &in) {
		return
	}
	c, err := h.svc.CreateCard(r.Context(), who(r), in.input())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, c)
}

func (h *Handler) updateCard(w http.ResponseWriter, r *http.Request) {
	id, ok := h.id(w, r, ErrCardNotFound)
	if !ok {
		return
	}
	var in cardBody
	if !httpx.Bind(w, r, &in) {
		return
	}
	c, err := h.svc.UpdateCard(r.Context(), who(r), id, in.input())
	reply(h, w, r, c, err)
}

func (h *Handler) archiveCard(archived bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := h.id(w, r, ErrCardNotFound)
		if !ok {
			return
		}
		c, err := h.svc.ArchiveCard(r.Context(), who(r), id, archived)
		reply(h, w, r, c, err)
	}
}
