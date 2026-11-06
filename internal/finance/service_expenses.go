package finance

import (
	"context"
	"fmt"
	"regexp"

	"casal/internal/identity"
)

// InstallmentsInput pede uma compra parcelada. Sem InstallmentAmountCents, AmountCents é o total e o app o reparte;
// com ele, o total é Count × valor da parcela (quem pagou juros).
type InstallmentsInput struct {
	Count                  int
	FirstOn                string
	InstallmentAmountCents int64
}

type ExpenseInput struct {
	Scope        string
	Visibility   string
	PaidBy       string
	AmountCents  int64
	CategoryID   string
	SpentOn      string
	Description  string
	Split        map[string]int
	Installments *InstallmentsInput
	Source       string
	CardID       string // opcional: um cartão de quem pagou
	Version      int    // só na edição: a versão que a pessoa viu
}

type CreateResult struct {
	Expenses []Expense
	// SplitFallback avisa que o casal usa a divisão proporcional, mas ela não estava disponível: caiu em igual.
	SplitFallback bool
}

var cursorRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2},[0-9a-fA-F-]{36}$`)

// expenseDraft valida o que é comum à criação e à edição, já com o gasto montado.
func (s *Service) expenseDraft(ctx context.Context, p identity.Principal, ps []Person, in ExpenseInput) (Expense, error) {
	e := Expense{HouseholdID: p.HouseholdID, CreatedBy: p.UserID, Scope: in.Scope, Source: in.Source}
	if e.Scope == "" {
		e.Scope = ScopeHousehold
	}
	if e.Source == "" {
		e.Source = SourceManual
	}
	var err error
	if e.Description, err = checkText(in.Description, maxDescription, "a descrição"); err != nil {
		return Expense{}, err
	}
	if in.SpentOn == "" {
		e.SpentOn = fmtDate(today(s.now(), p.Timezone))
	} else {
		d, err := parseDate(in.SpentOn)
		if err != nil {
			return Expense{}, err
		}
		e.SpentOn = fmtDate(d)
	}
	e.CategoryID = in.CategoryID

	switch e.Scope {
	case ScopePersonal:
		if in.Split != nil {
			return Expense{}, fmt.Errorf("%w: gasto pessoal não tem divisão", ErrSplitInvalid)
		}
		if in.PaidBy != "" && in.PaidBy != p.UserID {
			return Expense{}, fmt.Errorf("%w: o gasto pessoal é sempre pago por quem o registra", ErrInvalidInput)
		}
		e.PaidBy, e.Visibility = p.UserID, in.Visibility
		if e.Visibility == "" {
			e.Visibility = VisPrivate
		}
		if e.Visibility != VisPrivate && e.Visibility != VisView {
			return Expense{}, fmt.Errorf("%w: visibilidade inválida", ErrInvalidInput)
		}
	case ScopeHousehold:
		e.Visibility = VisHousehold
		e.PaidBy = in.PaidBy
		if e.PaidBy == "" {
			e.PaidBy = p.UserID
		}
		if !isMember(ps, e.PaidBy) {
			return Expense{}, fmt.Errorf("%w: quem pagou precisa ser do casal", ErrInvalidInput)
		}
	default:
		return Expense{}, fmt.Errorf("%w: escopo inválido", ErrInvalidInput)
	}
	return e, nil
}

// CreateExpense registra um gasto ou, com Installments, todas as parcelas de uma compra (ou todas, ou nenhuma).
func (s *Service) CreateExpense(ctx context.Context, p identity.Principal, in ExpenseInput) (CreateResult, error) {
	ps, err := s.members(ctx, p.HouseholdID)
	if err != nil {
		return CreateResult{}, err
	}
	base, err := s.expenseDraft(ctx, p, ps, in)
	if err != nil {
		return CreateResult{}, err
	}
	if err := s.usableCategory(ctx, p.HouseholdID, base.CategoryID); err != nil {
		return CreateResult{}, err
	}
	if err := s.usableCard(ctx, p, base.PaidBy, in.CardID, ""); err != nil {
		return CreateResult{}, err
	}
	base.CardID = in.CardID // as parcelas herdam o cartão da compra

	var res CreateResult
	if base.Scope == ScopeHousehold {
		if base.Split, res.SplitFallback, err = s.resolveSplit(ctx, p.HouseholdID, in.Split, ps); err != nil {
			return CreateResult{}, err
		}
	}

	var drafts []Expense
	if in.Installments == nil {
		if !validAmount(in.AmountCents) {
			return CreateResult{}, ErrAmountInvalid
		}
		base.AmountCents = in.AmountCents
		drafts = []Expense{base}
	} else if drafts, err = s.installmentDrafts(base, in); err != nil {
		return CreateResult{}, err
	}

	if res.Expenses, err = s.repo.InsertExpenses(ctx, p.HouseholdID, drafts); err != nil {
		return CreateResult{}, err
	}
	if base.Scope == ScopeHousehold {
		s.checkBudget(ctx, p, base.CategoryID, res.Expenses)
	}
	return res, nil
}

// installmentDrafts monta uma linha de gasto por parcela, cada uma no seu mês e com a divisão gravada.
func (s *Service) installmentDrafts(base Expense, in ExpenseInput) ([]Expense, error) {
	ins := in.Installments
	if ins.Count < minInstallments || ins.Count > maxInstallments {
		return nil, fmt.Errorf("%w: de %d a %d parcelas", ErrInstallmentsInvalid, minInstallments, maxInstallments)
	}
	firstStr := ins.FirstOn
	if firstStr == "" {
		firstStr = base.SpentOn
	}
	first, err := parseDate(firstStr)
	if err != nil {
		return nil, fmt.Errorf("%w: data da primeira parcela inválida", ErrInstallmentsInvalid)
	}

	var amounts []int64
	if ins.InstallmentAmountCents != 0 {
		if !validAmount(ins.InstallmentAmountCents) {
			return nil, fmt.Errorf("%w: valor da parcela inválido", ErrInstallmentsInvalid)
		}
		if in.AmountCents != 0 && in.AmountCents != ins.InstallmentAmountCents*int64(ins.Count) {
			return nil, fmt.Errorf("%w: o total não bate com parcelas × valor", ErrInstallmentsInvalid)
		}
		amounts = make([]int64, ins.Count)
		for i := range amounts {
			amounts[i] = ins.InstallmentAmountCents
		}
	} else {
		if in.AmountCents <= 0 {
			return nil, ErrAmountInvalid
		}
		if in.AmountCents < int64(ins.Count) {
			return nil, fmt.Errorf("%w: o total é menor que um centavo por parcela", ErrInstallmentsInvalid)
		}
		amounts = SplitInstallments(in.AmountCents, ins.Count)
		if !validAmount(amounts[0]) {
			return nil, ErrAmountInvalid
		}
	}

	group := newUUID()
	out := make([]Expense, ins.Count)
	for i := range out {
		e := base
		e.AmountCents = amounts[i]
		e.SpentOn = fmtDate(AddMonths(first, i, first.Day()))
		e.Installment = &Installment{GroupID: group, No: i + 1, Total: ins.Count}
		out[i] = e
	}
	return out, nil
}

// CreateFromShopping registra o gasto de mercado depois de uma compra: a categoria é a de mercado.
func (s *Service) CreateFromShopping(ctx context.Context, p identity.Principal, in ExpenseInput) (CreateResult, error) {
	c, err := s.repo.CategoryByKey(ctx, p.HouseholdID, KeyGroceries)
	if err != nil {
		return CreateResult{}, err
	}
	in.Scope, in.Source, in.CategoryID, in.Installments, in.Split = ScopeHousehold, SourceShopping, c.ID, nil, nil
	return s.CreateExpense(ctx, p, in)
}

func (s *Service) GetExpense(ctx context.Context, p identity.Principal, id string) (Expense, error) {
	return s.repo.GetExpense(ctx, p.HouseholdID, id, p.UserID)
}

type ExpensePage struct {
	Expenses []Expense `json:"expenses"`
	Next     string    `json:"next,omitempty"`
}

func (s *Service) ListExpenses(ctx context.Context, p identity.Principal, f ExpenseFilter) (ExpensePage, error) {
	if f.Scope != "" && f.Scope != "household" && f.Scope != "mine" {
		return ExpensePage{}, fmt.Errorf("%w: scope é household ou mine", ErrInvalidInput)
	}
	if f.Month != "" {
		if _, err := parseMonth(f.Month); err != nil {
			return ExpensePage{}, err
		}
	}
	if f.Before != "" && !cursorRe.MatchString(f.Before) {
		return ExpensePage{}, fmt.Errorf("%w: cursor inválido", ErrInvalidInput)
	}
	for _, id := range []string{f.CategoryID, f.PaidBy} {
		if id != "" && !uuidRe.MatchString(id) {
			return ExpensePage{}, fmt.Errorf("%w: id inválido", ErrInvalidInput)
		}
	}
	if f.Limit <= 0 {
		f.Limit = pageDefault
	}
	if f.Limit > pageMax {
		f.Limit = pageMax
	}
	want := f.Limit
	f.Limit++ // um a mais diz se há outra página
	out, err := s.repo.ListExpenses(ctx, p.HouseholdID, p.UserID, f)
	if err != nil {
		return ExpensePage{}, err
	}
	page := ExpensePage{Expenses: out}
	if len(out) > want {
		page.Expenses = out[:want]
		last := page.Expenses[want-1]
		page.Next = last.SpentOn + "," + last.ID
	}
	return page, nil
}

var uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// mutable confere se quem pergunta pode alterar o gasto, na ordem da spec: não vê (404), vê mas não altera (403),
// já acertado (409).
func mutable(e Expense, userID string) error {
	if e.Scope == ScopePersonal && e.CreatedBy != userID {
		return ErrExpenseForbidden
	}
	if e.Settled {
		return ErrExpenseSettled
	}
	return nil
}

// UpdateExpense substitui o gasto. Version é a que a pessoa viu: se outra gravou antes, é conflito e nada se perde.
func (s *Service) UpdateExpense(ctx context.Context, p identity.Principal, id string, in ExpenseInput) (Expense, error) {
	cur, err := s.repo.GetExpense(ctx, p.HouseholdID, id, p.UserID)
	if err != nil {
		return Expense{}, err // quem não pode ler o gasto nem sabe que ele existe
	}
	if err := mutable(cur, p.UserID); err != nil {
		return Expense{}, err
	}
	if in.Version != cur.Version {
		return Expense{}, ErrExpenseConflict
	}
	ps, err := s.members(ctx, p.HouseholdID)
	if err != nil {
		return Expense{}, err
	}
	// o escopo e a origem não mudam; a visibilidade do pessoal e a divisão do casal, se não vierem, ficam como estão
	in.Scope = cur.Scope
	if in.Visibility == "" {
		in.Visibility = cur.Visibility
	}
	if in.Source == "" {
		in.Source = cur.Source
	}
	next, err := s.expenseDraft(ctx, p, ps, in)
	if err != nil {
		return Expense{}, err
	}
	if !validAmount(in.AmountCents) {
		return Expense{}, ErrAmountInvalid
	}
	if in.CategoryID == "" {
		return Expense{}, fmt.Errorf("%w: informe a categoria", ErrInvalidInput)
	}
	if in.CategoryID != cur.CategoryID { // categoria arquivada só continua nos gastos antigos
		if err := s.usableCategory(ctx, p.HouseholdID, in.CategoryID); err != nil {
			return Expense{}, err
		}
	} else if _, err := s.repo.GetCategory(ctx, p.HouseholdID, in.CategoryID); err != nil {
		return Expense{}, err
	}
	next.ID, next.AmountCents, next.UpdatedBy = cur.ID, in.AmountCents, p.UserID
	if next.CardID, err = s.cardOnUpdate(ctx, p, cur, next.PaidBy, in.CardID); err != nil {
		return Expense{}, err
	}
	if cur.Scope == ScopeHousehold {
		next.Split = cur.Split
		if in.Split != nil {
			if next.Split, err = NormalizeSplit(in.Split, ids(ps)); err != nil {
				return Expense{}, err
			}
		}
	}
	out, err := s.repo.UpdateExpense(ctx, p.HouseholdID, next, in.Version, p.UserID)
	if isNoRows(err) {
		return Expense{}, s.whyNotMutated(ctx, p, id)
	}
	if err != nil {
		return Expense{}, err
	}
	if out.Scope == ScopeHousehold {
		s.checkBudget(ctx, p, out.CategoryID, []Expense{out})
	}
	return out, nil
}

// whyNotMutated descobre por que um UPDATE/DELETE não achou linha: sumiu, foi acertado ou alguém gravou no meio.
func (s *Service) whyNotMutated(ctx context.Context, p identity.Principal, id string) error {
	cur, err := s.repo.GetExpense(ctx, p.HouseholdID, id, p.UserID)
	if err != nil {
		return err
	}
	if err := mutable(cur, p.UserID); err != nil {
		return err
	}
	return ErrExpenseConflict
}

func (s *Service) DeleteExpense(ctx context.Context, p identity.Principal, id string) error {
	cur, err := s.repo.GetExpense(ctx, p.HouseholdID, id, p.UserID)
	if err != nil {
		return err
	}
	if err := mutable(cur, p.UserID); err != nil {
		return err
	}
	ok, err := s.repo.DeleteExpense(ctx, p.HouseholdID, id)
	if err != nil {
		return err
	}
	if !ok {
		return s.whyNotMutated(ctx, p, id)
	}
	return nil
}

// ---- compras parceladas ----

type Group struct {
	GroupID  string    `json:"group_id"`
	Expenses []Expense `json:"expenses"`
	Settled  int       `json:"settled"` // quantas parcelas já foram acertadas
}

func (s *Service) GetGroup(ctx context.Context, p identity.Principal, groupID string) (Group, error) {
	es, err := s.repo.GetGroup(ctx, p.HouseholdID, groupID, p.UserID)
	if err != nil {
		return Group{}, err
	}
	if len(es) == 0 {
		return Group{}, ErrExpenseNotFound
	}
	g := Group{GroupID: groupID, Expenses: es}
	for _, e := range es {
		if e.Settled {
			g.Settled++
		}
	}
	return g, nil
}

type GroupInput struct {
	Description *string
	CategoryID  *string
	PaidBy      *string
	Split       map[string]int
	CardID      *string // nil = não mexe; "" = tira o cartão
}

// UpdateGroup muda descrição, categoria, quem pagou e divisão nas parcelas ainda abertas. Valor e número de
// parcelas não mudam: apaga-se o grupo e registra-se de novo.
func (s *Service) UpdateGroup(ctx context.Context, p identity.Principal, groupID string, in GroupInput) (Group, error) {
	g, err := s.GetGroup(ctx, p, groupID)
	if err != nil {
		return Group{}, err
	}
	first := g.Expenses[0]
	if err := mutable(Expense{Scope: first.Scope, CreatedBy: first.CreatedBy}, p.UserID); err != nil {
		return Group{}, err
	}
	ps, err := s.members(ctx, p.HouseholdID)
	if err != nil {
		return Group{}, err
	}
	patch := GroupPatch{UpdatedBy: p.UserID, Description: in.Description, CategoryID: in.CategoryID, PaidBy: in.PaidBy}
	if in.Description != nil {
		d, err := checkText(*in.Description, maxDescription, "a descrição")
		if err != nil {
			return Group{}, err
		}
		patch.Description = &d
	}
	if in.CategoryID != nil {
		if err := s.usableCategory(ctx, p.HouseholdID, *in.CategoryID); err != nil {
			return Group{}, err
		}
	}
	if first.Scope == ScopePersonal && (in.PaidBy != nil || in.Split != nil) {
		return Group{}, fmt.Errorf("%w: gasto pessoal não tem divisão nem outro pagador", ErrInvalidInput)
	}
	if in.PaidBy != nil && !isMember(ps, *in.PaidBy) {
		return Group{}, fmt.Errorf("%w: quem pagou precisa ser do casal", ErrInvalidInput)
	}
	if in.Split != nil {
		if patch.Split, err = NormalizeSplit(in.Split, ids(ps)); err != nil {
			return Group{}, err
		}
	}
	if in.CardID != nil { // o cartão é de quem pagou: só ele troca, e vale para a compra inteira
		payer := first.PaidBy
		if in.PaidBy != nil {
			payer = *in.PaidBy
		}
		if payer != p.UserID {
			return Group{}, ErrCardInvalid
		}
		if err := s.usableCard(ctx, p, payer, *in.CardID, first.CardID); err != nil {
			return Group{}, err
		}
		patch.CardSet, patch.CardID = true, *in.CardID
	}
	n, err := s.repo.UpdateGroup(ctx, p.HouseholdID, groupID, p.UserID, patch)
	if err != nil {
		return Group{}, err
	}
	if n == 0 {
		return Group{}, ErrExpenseSettled // todas as parcelas já foram acertadas
	}
	return s.GetGroup(ctx, p, groupID)
}

type GroupDeleted struct {
	Deleted   int64 `json:"deleted"`
	Remaining int64 `json:"remaining"` // as parcelas acertadas, que são imutáveis
}

func (s *Service) DeleteGroup(ctx context.Context, p identity.Principal, groupID string) (GroupDeleted, error) {
	g, err := s.GetGroup(ctx, p, groupID)
	if err != nil {
		return GroupDeleted{}, err
	}
	first := g.Expenses[0]
	if first.Scope == ScopePersonal && first.CreatedBy != p.UserID {
		return GroupDeleted{}, ErrExpenseForbidden
	}
	del, rem, err := s.repo.DeleteGroup(ctx, p.HouseholdID, groupID, p.UserID)
	if err != nil {
		return GroupDeleted{}, err
	}
	if del == 0 && rem > 0 {
		return GroupDeleted{}, ErrExpenseSettled
	}
	return GroupDeleted{Deleted: del, Remaining: rem}, nil
}
