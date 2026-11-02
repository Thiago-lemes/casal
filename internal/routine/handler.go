package routine

import (
	"log/slog"
	"net/http"
	"strconv"
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
	mux.Handle("POST /v1/routine/blocks", wrap(h.create))
	mux.Handle("GET /v1/routine/blocks", wrap(h.list))
	mux.Handle("GET /v1/routine/blocks/{id}", wrap(h.get))
	mux.Handle("PUT /v1/routine/blocks/{id}", wrap(h.update))
	mux.Handle("DELETE /v1/routine/blocks/{id}", wrap(h.delete))
	mux.Handle("PUT /v1/routine/blocks/{id}/exceptions/{date}", wrap(h.setException))
	mux.Handle("DELETE /v1/routine/blocks/{id}/exceptions/{date}", wrap(h.deleteException))
	mux.Handle("GET /v1/routine/occurrences", wrap(h.occurrences))
	mux.Handle("GET /v1/routine/free", wrap(h.free))
	mux.Handle("GET /v1/routine/now", wrap(h.now))
}

// ---- formato JSON ----

type blockView struct {
	ID            string   `json:"id"`
	OwnerID       string   `json:"owner_id"`
	OwnerName     string   `json:"owner_name"`
	Title         string   `json:"title"`
	Category      string   `json:"category"`
	Location      string   `json:"location"`
	Visibility    string   `json:"visibility"`
	Busy          bool     `json:"busy"` // verdadeiro: o parceiro marcou só "ocupado", sem detalhes
	Recurrence    string   `json:"recurrence"`
	Weekdays      []int    `json:"weekdays"`
	IntervalWeeks int      `json:"interval_weeks"`
	StartDate     string   `json:"start_date"`
	EndDate       *string  `json:"end_date,omitempty"`
	Start         string   `json:"start"`
	End           string   `json:"end"`
	Version       int      `json:"version"`
	Overlaps      []string `json:"overlaps,omitempty"`
}

func view(b Block) blockView {
	v := blockView{
		ID: b.ID, OwnerID: b.OwnerID, OwnerName: b.OwnerName, Title: b.Title, Category: b.Category,
		Location: b.Location, Visibility: b.Visibility, Busy: b.Busy, Recurrence: b.Recurrence,
		Weekdays: DaysFromMask(b.Weekdays), IntervalWeeks: b.IntervalWeeks, StartDate: DateKey(b.StartDate),
		Start: FormatClock(b.StartMin), End: FormatClock(b.EndMin), Version: b.Version,
	}
	if b.EndDate != nil {
		e := DateKey(*b.EndDate)
		v.EndDate = &e
	}
	return v
}

type occurrenceView struct {
	BlockID   string    `json:"block_id"`
	OwnerID   string    `json:"owner_id"`
	OwnerName string    `json:"owner_name"`
	Title     string    `json:"title"`
	Category  string    `json:"category"`
	Location  string    `json:"location"`
	Busy      bool      `json:"busy"`
	Start     time.Time `json:"start_at"`
	End       time.Time `json:"end_at"`
	Exception bool      `json:"exception"`
}

func occurrenceViews(os []Occurrence) []occurrenceView {
	out := make([]occurrenceView, 0, len(os))
	for _, o := range os {
		out = append(out, occurrenceView{
			BlockID: o.BlockID, OwnerID: o.OwnerID, OwnerName: o.OwnerName, Title: o.Title,
			Category: o.Category, Location: o.Location, Busy: o.Busy,
			Start: o.Start.UTC(), End: o.End.UTC(), Exception: o.Exception,
		})
	}
	return out
}

// body é o JSON de criação e de edição de um bloco.
type body struct {
	Title         string `json:"title"`
	Category      string `json:"category"`
	Location      string `json:"location"`
	Visibility    string `json:"visibility"`
	Recurrence    string `json:"recurrence"`
	Weekdays      []int  `json:"weekdays"`
	IntervalWeeks int    `json:"interval_weeks"`
	StartDate     string `json:"start_date"`
	EndDate       string `json:"end_date"`
	Start         string `json:"start"`
	End           string `json:"end"`
	Version       int    `json:"version"`
}

func (b body) input() Input {
	return Input{
		Title: b.Title, Category: b.Category, Location: b.Location, Visibility: b.Visibility,
		Recurrence: b.Recurrence, Weekdays: b.Weekdays, IntervalWeeks: b.IntervalWeeks,
		StartDate: b.StartDate, EndDate: b.EndDate, Start: b.Start, End: b.End,
	}
}

func savedView(s Saved) blockView {
	v := view(s.Block)
	v.Overlaps = s.Overlaps
	return v
}

// ---- rotas ----

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var in body
	if !httpx.Bind(w, r, &in) {
		return
	}
	p, _ := identity.From(r.Context())
	out, err := h.svc.Create(r.Context(), p.HouseholdID, p.UserID, in.input())
	if err != nil {
		httpx.Fail(w, r, h.log, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, savedView(out))
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	p, _ := identity.From(r.Context())
	blocks, err := h.svc.List(r.Context(), p.HouseholdID, p.UserID)
	if err != nil {
		httpx.Fail(w, r, h.log, err)
		return
	}
	out := make([]blockView, 0, len(blocks))
	for _, b := range blocks {
		out = append(out, view(b))
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, ok := h.idParam(w, r)
	if !ok {
		return
	}
	p, _ := identity.From(r.Context())
	b, err := h.svc.Get(r.Context(), p.HouseholdID, id, p.UserID)
	if err != nil {
		httpx.Fail(w, r, h.log, err)
		return
	}
	httpx.JSON(w, http.StatusOK, view(b))
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
	out, err := h.svc.Update(r.Context(), p.HouseholdID, id, p.UserID, in.input(), in.Version)
	if err != nil {
		httpx.Fail(w, r, h.log, err)
		return
	}
	httpx.JSON(w, http.StatusOK, savedView(out))
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

func (h *Handler) setException(w http.ResponseWriter, r *http.Request) {
	id, ok := h.idParam(w, r)
	if !ok {
		return
	}
	var in struct {
		Skip  bool   `json:"skip"`
		Start string `json:"start"`
		End   string `json:"end"`
	}
	if !httpx.Bind(w, r, &in) {
		return
	}
	p, _ := identity.From(r.Context())
	if err := h.svc.SetException(r.Context(), p.HouseholdID, id, p.UserID, r.PathValue("date"), in.Skip, in.Start, in.End); err != nil {
		httpx.Fail(w, r, h.log, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) deleteException(w http.ResponseWriter, r *http.Request) {
	id, ok := h.idParam(w, r)
	if !ok {
		return
	}
	p, _ := identity.From(r.Context())
	if err := h.svc.DeleteException(r.Context(), p.HouseholdID, id, p.UserID, r.PathValue("date")); err != nil {
		httpx.Fail(w, r, h.log, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) occurrences(w http.ResponseWriter, r *http.Request) {
	p, _ := identity.From(r.Context())
	q := r.URL.Query()
	out, err := h.svc.Occurrences(r.Context(), p.HouseholdID, p.UserID, q.Get("from"), q.Get("to"))
	if err != nil {
		httpx.Fail(w, r, h.log, err)
		return
	}
	httpx.JSON(w, http.StatusOK, occurrenceViews(out))
}

// free: GET /v1/routine/free?date=AAAA-MM-DD[&min=30][&from_hour=7&to_hour=22]. O dia é no fuso de quem pergunta.
func (h *Handler) free(w http.ResponseWriter, r *http.Request) {
	p, _ := identity.From(r.Context())
	q := r.URL.Query()
	in := FreeInput{Date: q.Get("date"), Timezone: p.Timezone}
	var bad bool
	atoi := func(key string) (int, bool) {
		s := q.Get(key)
		if s == "" {
			return 0, false
		}
		n, err := strconv.Atoi(s)
		if err != nil {
			bad = true
		}
		return n, true
	}
	in.MinMinutes, _ = atoi("min")
	from, hasFrom := atoi("from_hour")
	to, hasTo := atoi("to_hour")
	if hasFrom != hasTo {
		bad = true // um sem o outro não diz a janela
	}
	in.FromHour, in.ToHour, in.HoursPassed = from, to, hasFrom && hasTo
	if bad {
		httpx.Fail(w, r, h.log, ErrInvalidInput)
		return
	}
	out, err := h.svc.FreeWindows(r.Context(), p.HouseholdID, p.UserID, in)
	if err != nil {
		httpx.Fail(w, r, h.log, err)
		return
	}
	type window struct {
		Start time.Time `json:"start_at"`
		End   time.Time `json:"end_at"`
	}
	res := make([]window, 0, len(out))
	for _, iv := range out {
		res = append(res, window{iv.Start.UTC(), iv.End.UTC()})
	}
	httpx.JSON(w, http.StatusOK, res)
}

func (h *Handler) now(w http.ResponseWriter, r *http.Request) {
	p, _ := identity.From(r.Context())
	out, err := h.svc.Now(r.Context(), p.HouseholdID, p.UserID)
	if err != nil {
		httpx.Fail(w, r, h.log, err)
		return
	}
	type presence struct {
		UserID string          `json:"user_id"`
		Name   string          `json:"name"`
		Now    *occurrenceView `json:"now"`
	}
	res := make([]presence, 0, len(out))
	for _, pr := range out {
		item := presence{UserID: pr.UserID, Name: pr.Name}
		if pr.Now != nil {
			v := occurrenceViews([]Occurrence{*pr.Now})[0]
			item.Now = &v
		}
		res = append(res, item)
	}
	httpx.JSON(w, http.StatusOK, res)
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
