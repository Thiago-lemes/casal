package goal

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"

	"casal/internal/httpx"
	"casal/internal/identity"
)

type Handler struct {
	svc *Service
	log *slog.Logger
}

func NewHandler(svc *Service, log *slog.Logger) *Handler { return &Handler{svc: svc, log: log} }

// Routes recebe os middlewares de login, casal e módulo ligado: toda rota de Metas exige os três.
func (h *Handler) Routes(mux *http.ServeMux, requireUser, requireHousehold, requireModule httpx.Middleware) {
	wrap := func(f http.HandlerFunc) http.Handler {
		return httpx.Chain(f, requireUser, requireHousehold, requireModule)
	}
	const v = "/v1/goals"
	mux.Handle("POST "+v, wrap(h.create))
	mux.Handle("GET "+v, wrap(h.list))
	mux.Handle("GET "+v+"/export", wrap(h.export))
	mux.Handle("GET "+v+"/{id}", wrap(h.get))
	mux.Handle("PUT "+v+"/{id}", wrap(h.update))
	mux.Handle("DELETE "+v+"/{id}", wrap(h.delete))
	mux.Handle("POST "+v+"/{id}/pause", wrap(h.state((*Service).Pause)))
	mux.Handle("POST "+v+"/{id}/resume", wrap(h.state((*Service).Resume)))
	mux.Handle("POST "+v+"/{id}/complete", wrap(h.state((*Service).Complete)))
	mux.Handle("POST "+v+"/{id}/archive", wrap(h.state((*Service).Archive)))
	mux.Handle("POST "+v+"/{id}/unarchive", wrap(h.state((*Service).Unarchive)))

	mux.Handle("POST "+v+"/{id}/contributions", wrap(h.addContribution))
	mux.Handle("GET "+v+"/{id}/contributions", wrap(h.listContributions))
	mux.Handle("DELETE "+v+"/{id}/contributions/{cid}", wrap(h.deleteContribution))

	mux.Handle("POST "+v+"/{id}/checkins", wrap(h.checkin))
	mux.Handle("DELETE "+v+"/{id}/checkins/{day}", wrap(h.undoCheckin))
	mux.Handle("GET "+v+"/{id}/history", wrap(h.history))

	mux.Handle("POST "+v+"/{id}/steps", wrap(h.addStep))
	mux.Handle("PUT "+v+"/{id}/steps/{sid}", wrap(h.updateStep))
	mux.Handle("DELETE "+v+"/{id}/steps/{sid}", wrap(h.deleteStep))
	mux.Handle("POST "+v+"/{id}/steps/{sid}/done", wrap(h.stepDone(true)))
	mux.Handle("POST "+v+"/{id}/steps/{sid}/undone", wrap(h.stepDone(false)))
	mux.Handle("POST "+v+"/{id}/steps/{sid}/task", wrap(h.makeTask))
	mux.Handle("DELETE "+v+"/{id}/steps/{sid}/task", wrap(h.unlinkTask))
}

func who(r *http.Request) identity.Principal {
	p, _ := identity.From(r.Context())
	return p
}

// goalID lê o {id} da URL. Um id malformado responde 404, igual a "não existe".
func (h *Handler) goalID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("id")
	if !httpx.IsUUID(id) {
		httpx.Fail(w, r, h.log, ErrNotFound)
		return "", false
	}
	return id, true
}

func (h *Handler) stepID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("sid")
	if !httpx.IsUUID(id) {
		httpx.Fail(w, r, h.log, ErrStepNotFound)
		return "", false
	}
	return id, true
}

func (h *Handler) reply(w http.ResponseWriter, r *http.Request, status int, v any, err error) {
	if err != nil {
		httpx.Fail(w, r, h.log, err)
		return
	}
	httpx.JSON(w, status, v)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var in CreateInput
	if !httpx.Bind(w, r, &in) {
		return
	}
	out, err := h.svc.Create(r.Context(), who(r), in)
	h.reply(w, r, http.StatusCreated, out, err)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	out, err := h.svc.List(r.Context(), who(r), ListFilter{Scope: q.Get("scope"), Kind: q.Get("kind"), Status: q.Get("status")})
	h.reply(w, r, http.StatusOK, out, err)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, ok := h.goalID(w, r)
	if !ok {
		return
	}
	out, err := h.svc.Get(r.Context(), who(r), id)
	h.reply(w, r, http.StatusOK, out, err)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id, ok := h.goalID(w, r)
	if !ok {
		return
	}
	var in UpdateInput
	if !httpx.Bind(w, r, &in) {
		return
	}
	out, err := h.svc.Update(r.Context(), who(r), id, in)
	h.reply(w, r, http.StatusOK, out, err)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	id, ok := h.goalID(w, r)
	if !ok {
		return
	}
	if err := h.svc.Delete(r.Context(), who(r), id); err != nil {
		httpx.Fail(w, r, h.log, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) state(f func(*Service, context.Context, identity.Principal, string) (Goal, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := h.goalID(w, r)
		if !ok {
			return
		}
		out, err := f(h.svc, r.Context(), who(r), id)
		h.reply(w, r, http.StatusOK, out, err)
	}
}

func (h *Handler) addContribution(w http.ResponseWriter, r *http.Request) {
	id, ok := h.goalID(w, r)
	if !ok {
		return
	}
	var in ContributionInput
	if !httpx.Bind(w, r, &in) {
		return
	}
	out, err := h.svc.AddContribution(r.Context(), who(r), id, in)
	h.reply(w, r, http.StatusCreated, out, err)
}

func (h *Handler) listContributions(w http.ResponseWriter, r *http.Request) {
	id, ok := h.goalID(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	before := q.Get("before")
	if before != "" && !httpx.IsUUID(before) {
		httpx.Fail(w, r, h.log, ErrInvalid)
		return
	}
	limit, _ := strconv.Atoi(q.Get("limit"))
	out, err := h.svc.ListContributions(r.Context(), who(r), id, before, limit)
	h.reply(w, r, http.StatusOK, out, err)
}

func (h *Handler) deleteContribution(w http.ResponseWriter, r *http.Request) {
	id, ok := h.goalID(w, r)
	if !ok {
		return
	}
	cid := r.PathValue("cid")
	if !httpx.IsUUID(cid) {
		httpx.Fail(w, r, h.log, ErrContributionNotFound)
		return
	}
	out, err := h.svc.DeleteContribution(r.Context(), who(r), id, cid)
	h.reply(w, r, http.StatusOK, out, err)
}

func (h *Handler) checkin(w http.ResponseWriter, r *http.Request) {
	id, ok := h.goalID(w, r)
	if !ok {
		return
	}
	var in struct {
		Day string `json:"day"`
	}
	if r.ContentLength != 0 && !httpx.Bind(w, r, &in) {
		return
	}
	out, err := h.svc.Checkin(r.Context(), who(r), id, in.Day)
	h.reply(w, r, http.StatusOK, out, err)
}

func (h *Handler) undoCheckin(w http.ResponseWriter, r *http.Request) {
	id, ok := h.goalID(w, r)
	if !ok {
		return
	}
	out, err := h.svc.UndoCheckin(r.Context(), who(r), id, r.PathValue("day"))
	h.reply(w, r, http.StatusOK, out, err)
}

func (h *Handler) history(w http.ResponseWriter, r *http.Request) {
	id, ok := h.goalID(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	out, err := h.svc.History(r.Context(), who(r), id, q.Get("from"), q.Get("to"))
	h.reply(w, r, http.StatusOK, out, err)
}

func (h *Handler) addStep(w http.ResponseWriter, r *http.Request) {
	id, ok := h.goalID(w, r)
	if !ok {
		return
	}
	var in StepInput
	if !httpx.Bind(w, r, &in) {
		return
	}
	out, err := h.svc.AddStep(r.Context(), who(r), id, in)
	h.reply(w, r, http.StatusCreated, out, err)
}

func (h *Handler) updateStep(w http.ResponseWriter, r *http.Request) {
	id, ok := h.goalID(w, r)
	if !ok {
		return
	}
	sid, ok := h.stepID(w, r)
	if !ok {
		return
	}
	var in StepUpdate
	if !httpx.Bind(w, r, &in) {
		return
	}
	out, err := h.svc.UpdateStep(r.Context(), who(r), id, sid, in)
	h.reply(w, r, http.StatusOK, out, err)
}

func (h *Handler) deleteStep(w http.ResponseWriter, r *http.Request) {
	id, ok := h.goalID(w, r)
	if !ok {
		return
	}
	sid, ok := h.stepID(w, r)
	if !ok {
		return
	}
	out, err := h.svc.DeleteStep(r.Context(), who(r), id, sid)
	h.reply(w, r, http.StatusOK, out, err)
}

func (h *Handler) stepDone(done bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := h.goalID(w, r)
		if !ok {
			return
		}
		sid, ok := h.stepID(w, r)
		if !ok {
			return
		}
		out, err := h.svc.SetStepDone(r.Context(), who(r), id, sid, done)
		h.reply(w, r, http.StatusOK, out, err)
	}
}

func (h *Handler) makeTask(w http.ResponseWriter, r *http.Request) {
	id, ok := h.goalID(w, r)
	if !ok {
		return
	}
	sid, ok := h.stepID(w, r)
	if !ok {
		return
	}
	out, err := h.svc.MakeStepTask(r.Context(), who(r), id, sid)
	h.reply(w, r, http.StatusOK, out, err)
}

func (h *Handler) unlinkTask(w http.ResponseWriter, r *http.Request) {
	id, ok := h.goalID(w, r)
	if !ok {
		return
	}
	sid, ok := h.stepID(w, r)
	if !ok {
		return
	}
	out, err := h.svc.UnlinkStepTask(r.Context(), who(r), id, sid)
	h.reply(w, r, http.StatusOK, out, err)
}

func (h *Handler) export(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.Export(r.Context(), who(r))
	if err != nil {
		httpx.Fail(w, r, h.log, err)
		return
	}
	if r.URL.Query().Get("format") == "csv" {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="metas.csv"`)
		if err := out.WriteCSV(w); err != nil {
			h.log.Error("falha ao escrever o CSV de metas", "err", err)
		}
		return
	}
	w.Header().Set("Content-Disposition", `attachment; filename="metas.json"`)
	httpx.JSON(w, http.StatusOK, out)
}
