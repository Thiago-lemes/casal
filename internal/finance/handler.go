package finance

import (
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

// Routes recebe os middlewares de login, casal e módulo ligado: toda rota de Finanças exige os três.
func (h *Handler) Routes(mux *http.ServeMux, requireUser, requireHousehold, requireModule httpx.Middleware) {
	wrap := func(f http.HandlerFunc) http.Handler {
		return httpx.Chain(f, requireUser, requireHousehold, requireModule)
	}
	const v = "/v1/finance"
	mux.Handle("GET "+v+"/categories", wrap(h.listCategories))
	mux.Handle("POST "+v+"/categories", wrap(h.createCategory))
	mux.Handle("PUT "+v+"/categories/{id}", wrap(h.renameCategory))
	mux.Handle("POST "+v+"/categories/{id}/archive", wrap(h.archiveCategory(true)))
	mux.Handle("POST "+v+"/categories/{id}/unarchive", wrap(h.archiveCategory(false)))

	mux.Handle("GET "+v+"/cards", wrap(h.listCards))
	mux.Handle("POST "+v+"/cards", wrap(h.createCard))
	mux.Handle("PUT "+v+"/cards/{id}", wrap(h.updateCard))
	mux.Handle("POST "+v+"/cards/{id}/archive", wrap(h.archiveCard(true)))
	mux.Handle("POST "+v+"/cards/{id}/unarchive", wrap(h.archiveCard(false)))

	mux.Handle("POST "+v+"/expenses", wrap(h.createExpense))
	mux.Handle("GET "+v+"/expenses", wrap(h.listExpenses))
	mux.Handle("POST "+v+"/expenses/from-shopping", wrap(h.fromShopping))
	mux.Handle("GET "+v+"/expenses/{id}", wrap(h.getExpense))
	mux.Handle("PUT "+v+"/expenses/{id}", wrap(h.updateExpense))
	mux.Handle("DELETE "+v+"/expenses/{id}", wrap(h.deleteExpense))
	mux.Handle("GET "+v+"/expense-groups/{id}", wrap(h.getGroup))
	mux.Handle("PUT "+v+"/expense-groups/{id}", wrap(h.updateGroup))
	mux.Handle("DELETE "+v+"/expense-groups/{id}", wrap(h.deleteGroup))

	mux.Handle("POST "+v+"/recurring", wrap(h.createRecurring))
	mux.Handle("GET "+v+"/recurring", wrap(h.listRecurring))
	mux.Handle("PUT "+v+"/recurring/{id}", wrap(h.updateRecurring))
	mux.Handle("DELETE "+v+"/recurring/{id}", wrap(h.deleteRecurring))
	mux.Handle("POST "+v+"/recurring/{id}/pause", wrap(h.setRecurringActive(false)))
	mux.Handle("POST "+v+"/recurring/{id}/resume", wrap(h.setRecurringActive(true)))

	mux.Handle("GET "+v+"/settings", wrap(h.getSettings))
	mux.Handle("PUT "+v+"/settings", wrap(h.putSettings))
	mux.Handle("POST "+v+"/settings/proportional/consent", wrap(h.consent))
	mux.Handle("DELETE "+v+"/settings/proportional/consent", wrap(h.withdrawConsent))
	mux.Handle("GET "+v+"/income", wrap(h.getIncome))
	mux.Handle("PUT "+v+"/income", wrap(h.putIncome))
	mux.Handle("DELETE "+v+"/income", wrap(h.deleteIncome))
	mux.Handle("GET "+v+"/balance", wrap(h.balance))
	mux.Handle("POST "+v+"/settlements", wrap(h.settle))
	mux.Handle("GET "+v+"/settlements", wrap(h.listSettlements))

	mux.Handle("POST "+v+"/bills", wrap(h.createBill))
	mux.Handle("GET "+v+"/bills", wrap(h.listBills))
	mux.Handle("GET "+v+"/bills/{id}", wrap(h.getBill))
	mux.Handle("PUT "+v+"/bills/{id}", wrap(h.updateBill))
	mux.Handle("DELETE "+v+"/bills/{id}", wrap(h.deleteBill))
	mux.Handle("POST "+v+"/bills/{id}/pay", wrap(h.payBill))

	mux.Handle("GET "+v+"/budgets", wrap(h.listBudgets))
	mux.Handle("PUT "+v+"/budgets", wrap(h.putBudget))
	mux.Handle("DELETE "+v+"/budgets/{id}", wrap(h.deleteBudget))
	mux.Handle("GET "+v+"/summary", wrap(h.summary))
	mux.Handle("GET "+v+"/export", wrap(h.export))
}

func who(r *http.Request) identity.Principal {
	p, _ := identity.From(r.Context())
	return p
}

// id lê o {id} da URL. Um id malformado responde como "não existe" do tipo do recurso.
func (h *Handler) id(w http.ResponseWriter, r *http.Request, notFound error) (string, bool) {
	id := r.PathValue("id")
	if !httpx.IsUUID(id) {
		httpx.Fail(w, r, h.log, notFound)
		return "", false
	}
	return id, true
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	httpx.Fail(w, r, h.log, err)
}

// reply responde 200 com o valor ou o erro; serve a quase todas as rotas.
func reply[T any](h *Handler, w http.ResponseWriter, r *http.Request, v T, err error) {
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, v)
}

// ---- categorias ----

func (h *Handler) listCategories(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.ListCategories(r.Context(), who(r), r.URL.Query().Get("archived") == "true")
	reply(h, w, r, out, err)
}

type nameBody struct {
	Name string `json:"name"`
}

func (h *Handler) createCategory(w http.ResponseWriter, r *http.Request) {
	var in nameBody
	if !httpx.Bind(w, r, &in) {
		return
	}
	c, err := h.svc.CreateCategory(r.Context(), who(r), in.Name)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, c)
}

func (h *Handler) renameCategory(w http.ResponseWriter, r *http.Request) {
	id, ok := h.id(w, r, ErrCategoryNotFound)
	if !ok {
		return
	}
	var in nameBody
	if !httpx.Bind(w, r, &in) {
		return
	}
	c, err := h.svc.RenameCategory(r.Context(), who(r), id, in.Name)
	reply(h, w, r, c, err)
}

func (h *Handler) archiveCategory(archived bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := h.id(w, r, ErrCategoryNotFound)
		if !ok {
			return
		}
		c, err := h.svc.ArchiveCategory(r.Context(), who(r), id, archived)
		reply(h, w, r, c, err)
	}
}

// ---- gastos ----

type expenseBody struct {
	Scope        string         `json:"scope"`
	Visibility   string         `json:"visibility"`
	PaidBy       string         `json:"paid_by"`
	AmountCents  int64          `json:"amount_cents"`
	CategoryID   string         `json:"category_id"`
	SpentOn      string         `json:"spent_on"`
	Description  string         `json:"description"`
	Split        map[string]int `json:"split"`
	CardID       string         `json:"card_id"`
	Version      int            `json:"version"`
	Installments *struct {
		Count                  int    `json:"count"`
		FirstOn                string `json:"first_on"`
		InstallmentAmountCents int64  `json:"installment_amount_cents"`
	} `json:"installments"`
}

func (b expenseBody) input() ExpenseInput {
	in := ExpenseInput{
		Scope: b.Scope, Visibility: b.Visibility, PaidBy: b.PaidBy, AmountCents: b.AmountCents, CategoryID: b.CategoryID,
		SpentOn: b.SpentOn, Description: b.Description, Split: b.Split, CardID: b.CardID, Version: b.Version,
	}
	if b.Installments != nil {
		in.Installments = &InstallmentsInput{
			Count: b.Installments.Count, FirstOn: b.Installments.FirstOn,
			InstallmentAmountCents: b.Installments.InstallmentAmountCents,
		}
	}
	return in
}

func (h *Handler) writeCreated(w http.ResponseWriter, r *http.Request, res CreateResult, installments bool) {
	if installments {
		httpx.JSON(w, http.StatusCreated, struct {
			Expenses      []Expense `json:"expenses"`
			SplitFallback bool      `json:"split_fallback,omitempty"`
		}{res.Expenses, res.SplitFallback})
		return
	}
	httpx.JSON(w, http.StatusCreated, struct {
		Expense
		SplitFallback bool `json:"split_fallback,omitempty"`
	}{res.Expenses[0], res.SplitFallback})
}

func (h *Handler) createExpense(w http.ResponseWriter, r *http.Request) {
	var in expenseBody
	if !httpx.Bind(w, r, &in) {
		return
	}
	res, err := h.svc.CreateExpense(r.Context(), who(r), in.input())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.writeCreated(w, r, res, in.Installments != nil)
}

func (h *Handler) fromShopping(w http.ResponseWriter, r *http.Request) {
	var in struct {
		AmountCents int64  `json:"amount_cents"`
		PaidBy      string `json:"paid_by"`
		Description string `json:"description"`
		SpentOn     string `json:"spent_on"`
	}
	if !httpx.Bind(w, r, &in) {
		return
	}
	res, err := h.svc.CreateFromShopping(r.Context(), who(r), ExpenseInput{
		AmountCents: in.AmountCents, PaidBy: in.PaidBy, Description: in.Description, SpentOn: in.SpentOn,
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.writeCreated(w, r, res, false)
}

func (h *Handler) listExpenses(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	out, err := h.svc.ListExpenses(r.Context(), who(r), ExpenseFilter{
		Month: q.Get("month"), Scope: q.Get("scope"), CategoryID: q.Get("category"), PaidBy: q.Get("paid_by"), CardID: q.Get("card"),
		Before: q.Get("before"), Limit: limit,
	})
	reply(h, w, r, out, err)
}

func (h *Handler) getExpense(w http.ResponseWriter, r *http.Request) {
	id, ok := h.id(w, r, ErrExpenseNotFound)
	if !ok {
		return
	}
	e, err := h.svc.GetExpense(r.Context(), who(r), id)
	reply(h, w, r, e, err)
}

func (h *Handler) updateExpense(w http.ResponseWriter, r *http.Request) {
	id, ok := h.id(w, r, ErrExpenseNotFound)
	if !ok {
		return
	}
	var in expenseBody
	if !httpx.Bind(w, r, &in) {
		return
	}
	in.Installments = nil // as parcelas não se refazem por aqui
	e, err := h.svc.UpdateExpense(r.Context(), who(r), id, in.input())
	reply(h, w, r, e, err)
}

func (h *Handler) deleteExpense(w http.ResponseWriter, r *http.Request) {
	id, ok := h.id(w, r, ErrExpenseNotFound)
	if !ok {
		return
	}
	if err := h.svc.DeleteExpense(r.Context(), who(r), id); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) getGroup(w http.ResponseWriter, r *http.Request) {
	id, ok := h.id(w, r, ErrExpenseNotFound)
	if !ok {
		return
	}
	g, err := h.svc.GetGroup(r.Context(), who(r), id)
	reply(h, w, r, g, err)
}

func (h *Handler) updateGroup(w http.ResponseWriter, r *http.Request) {
	id, ok := h.id(w, r, ErrExpenseNotFound)
	if !ok {
		return
	}
	var in struct {
		Description *string        `json:"description"`
		CategoryID  *string        `json:"category_id"`
		PaidBy      *string        `json:"paid_by"`
		Split       map[string]int `json:"split"`
		CardID      *string        `json:"card_id"`
	}
	if !httpx.Bind(w, r, &in) {
		return
	}
	g, err := h.svc.UpdateGroup(r.Context(), who(r), id, GroupInput(in))
	reply(h, w, r, g, err)
}

func (h *Handler) deleteGroup(w http.ResponseWriter, r *http.Request) {
	id, ok := h.id(w, r, ErrExpenseNotFound)
	if !ok {
		return
	}
	out, err := h.svc.DeleteGroup(r.Context(), who(r), id)
	reply(h, w, r, out, err)
}

// ---- gastos fixos ----

type recurringBody struct {
	Scope       string         `json:"scope"`
	Visibility  string         `json:"visibility"`
	PaidBy      string         `json:"paid_by"`
	AmountCents int64          `json:"amount_cents"`
	CategoryID  string         `json:"category_id"`
	Description string         `json:"description"`
	Split       map[string]int `json:"split"`
	DayOfMonth  int            `json:"day_of_month"`
	StartOn     string         `json:"start_on"`
	EndOn       string         `json:"end_on"`
}

func (h *Handler) createRecurring(w http.ResponseWriter, r *http.Request) {
	var in recurringBody
	if !httpx.Bind(w, r, &in) {
		return
	}
	out, err := h.svc.CreateRecurring(r.Context(), who(r), RecurringInput(in))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, out)
}

func (h *Handler) listRecurring(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.ListRecurring(r.Context(), who(r))
	reply(h, w, r, out, err)
}

func (h *Handler) updateRecurring(w http.ResponseWriter, r *http.Request) {
	id, ok := h.id(w, r, ErrRecurringNotFound)
	if !ok {
		return
	}
	var in recurringBody
	if !httpx.Bind(w, r, &in) {
		return
	}
	out, err := h.svc.UpdateRecurring(r.Context(), who(r), id, RecurringInput(in))
	reply(h, w, r, out, err)
}

func (h *Handler) deleteRecurring(w http.ResponseWriter, r *http.Request) {
	id, ok := h.id(w, r, ErrRecurringNotFound)
	if !ok {
		return
	}
	if err := h.svc.DeleteRecurring(r.Context(), who(r), id); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) setRecurringActive(active bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := h.id(w, r, ErrRecurringNotFound)
		if !ok {
			return
		}
		out, err := h.svc.SetRecurringActive(r.Context(), who(r), id, active)
		reply(h, w, r, out, err)
	}
}

// ---- divisão, renda, saldo e acerto ----

func (h *Handler) getSettings(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.GetSettings(r.Context(), who(r))
	reply(h, w, r, out, err)
}

func (h *Handler) putSettings(w http.ResponseWriter, r *http.Request) {
	var in struct {
		SplitMode  string         `json:"split_mode"`
		FixedSplit map[string]int `json:"fixed_split"`
	}
	if !httpx.Bind(w, r, &in) {
		return
	}
	out, err := h.svc.SaveSettings(r.Context(), who(r), in.SplitMode, in.FixedSplit)
	reply(h, w, r, out, err)
}

func (h *Handler) consent(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.Consent(r.Context(), who(r))
	reply(h, w, r, out, err)
}

func (h *Handler) withdrawConsent(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.WithdrawConsent(r.Context(), who(r))
	reply(h, w, r, out, err)
}

type incomeBody struct {
	MonthlyIncomeCents *int64 `json:"monthly_income_cents"`
}

func (h *Handler) getIncome(w http.ResponseWriter, r *http.Request) {
	c, err := h.svc.GetIncome(r.Context(), who(r))
	reply(h, w, r, incomeBody{c}, err)
}

func (h *Handler) putIncome(w http.ResponseWriter, r *http.Request) {
	var in incomeBody
	if !httpx.Bind(w, r, &in) {
		return
	}
	if in.MonthlyIncomeCents == nil {
		h.fail(w, r, ErrIncomeInvalid)
		return
	}
	c, err := h.svc.SetIncome(r.Context(), who(r), *in.MonthlyIncomeCents)
	reply(h, w, r, incomeBody{&c}, err)
}

func (h *Handler) deleteIncome(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteIncome(r.Context(), who(r)); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) balance(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.Balance(r.Context(), who(r), r.URL.Query().Get("through"))
	reply(h, w, r, out, err)
}

func (h *Handler) settle(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ThroughOn   string `json:"through_on"`
		AmountCents int64  `json:"amount_cents"`
	}
	if !httpx.Bind(w, r, &in) {
		return
	}
	out, err := h.svc.Settle(r.Context(), who(r), in.ThroughOn, in.AmountCents)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, out)
}

func (h *Handler) listSettlements(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.ListSettlements(r.Context(), who(r))
	reply(h, w, r, out, err)
}

// ---- contas a pagar ----

type billBody struct {
	Title         string `json:"title"`
	AmountCents   *int64 `json:"amount_cents"`
	DueOn         string `json:"due_on"`
	ResponsibleID string `json:"responsible_id"`
	CategoryID    string `json:"category_id"`
	NotifyBoth    bool   `json:"notify_both"`
	Repeat        string `json:"repeat"`
}

func (h *Handler) createBill(w http.ResponseWriter, r *http.Request) {
	var in billBody
	if !httpx.Bind(w, r, &in) {
		return
	}
	out, err := h.svc.CreateBill(r.Context(), who(r), BillInput(in))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, out)
}

func (h *Handler) listBills(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	out, err := h.svc.ListBills(r.Context(), who(r), q.Get("status"), q.Get("due_before"), q.Get("due") == "today")
	reply(h, w, r, out, err)
}

func (h *Handler) getBill(w http.ResponseWriter, r *http.Request) {
	id, ok := h.id(w, r, ErrBillNotFound)
	if !ok {
		return
	}
	out, err := h.svc.GetBill(r.Context(), who(r), id)
	reply(h, w, r, out, err)
}

func (h *Handler) updateBill(w http.ResponseWriter, r *http.Request) {
	id, ok := h.id(w, r, ErrBillNotFound)
	if !ok {
		return
	}
	var in billBody
	if !httpx.Bind(w, r, &in) {
		return
	}
	out, err := h.svc.UpdateBill(r.Context(), who(r), id, BillInput(in))
	reply(h, w, r, out, err)
}

func (h *Handler) deleteBill(w http.ResponseWriter, r *http.Request) {
	id, ok := h.id(w, r, ErrBillNotFound)
	if !ok {
		return
	}
	if err := h.svc.DeleteBill(r.Context(), who(r), id); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) payBill(w http.ResponseWriter, r *http.Request) {
	id, ok := h.id(w, r, ErrBillNotFound)
	if !ok {
		return
	}
	var in struct {
		AmountCents int64  `json:"amount_cents"`
		PaidOn      string `json:"paid_on"`
		PaidBy      string `json:"paid_by"`
	}
	if !httpx.Bind(w, r, &in) {
		return
	}
	out, err := h.svc.PayBill(r.Context(), who(r), id, PayInput(in))
	reply(h, w, r, out, err)
}

// ---- orçamento, resumo e exportação ----

func (h *Handler) listBudgets(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.ListBudgets(r.Context(), who(r))
	reply(h, w, r, out, err)
}

func (h *Handler) putBudget(w http.ResponseWriter, r *http.Request) {
	var in Budget
	if !httpx.Bind(w, r, &in) {
		return
	}
	if !httpx.IsUUID(in.CategoryID) {
		h.fail(w, r, ErrCategoryNotFound)
		return
	}
	out, err := h.svc.SaveBudget(r.Context(), who(r), in)
	reply(h, w, r, out, err)
}

func (h *Handler) deleteBudget(w http.ResponseWriter, r *http.Request) {
	id, ok := h.id(w, r, ErrCategoryNotFound)
	if !ok {
		return
	}
	if err := h.svc.DeleteBudget(r.Context(), who(r), id); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) summary(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	out, err := h.svc.Summary(r.Context(), who(r), q.Get("month"), q.Get("scope"))
	reply(h, w, r, out, err)
}

func (h *Handler) export(w http.ResponseWriter, r *http.Request) {
	d, err := h.svc.Export(r.Context(), who(r))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if r.URL.Query().Get("format") == "csv" {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="financas.csv"`)
		if err := d.WriteCSV(w); err != nil {
			h.log.Warn("falha ao escrever o CSV", "err", err)
		}
		return
	}
	httpx.JSON(w, http.StatusOK, d)
}
