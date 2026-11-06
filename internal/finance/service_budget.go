package finance

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"casal/internal/identity"
)

// ---- orçamento ----

func (s *Service) ListBudgets(ctx context.Context, p identity.Principal) ([]Budget, error) {
	return s.repo.ListBudgets(ctx, p.HouseholdID)
}

func (s *Service) SaveBudget(ctx context.Context, p identity.Principal, b Budget) (Budget, error) {
	if !validAmount(b.LimitCents) {
		return Budget{}, ErrAmountInvalid
	}
	if _, err := s.repo.GetCategory(ctx, p.HouseholdID, b.CategoryID); err != nil {
		return Budget{}, err
	}
	return b, s.repo.SaveBudget(ctx, p.HouseholdID, b)
}

func (s *Service) DeleteBudget(ctx context.Context, p identity.Principal, categoryID string) error {
	ok, err := s.repo.DeleteBudget(ctx, p.HouseholdID, categoryID)
	if err == nil && !ok {
		return ErrCategoryNotFound
	}
	return err
}

// checkBudget confere, depois de gravar (fora da transação), se o gasto fez a categoria cruzar 80% ou 100% do limite
// do mês corrente. Cada nível é avisado uma vez por mês: a chave primária de budget_alerts decide quem avisa, mesmo
// com dois celulares lançando gastos ao mesmo tempo.
func (s *Service) checkBudget(ctx context.Context, p identity.Principal, categoryID string, es []Expense) {
	month := fmtDate(today(s.now(), p.Timezone))[:7]
	s.checkBudgetMonth(ctx, p.HouseholdID, month, categoryID, es)
}

func (s *Service) checkBudgetMonth(ctx context.Context, hh, nowMonth, categoryID string, es []Expense) {
	inMonth := false
	for _, e := range es {
		if e.Scope == ScopeHousehold && len(e.SpentOn) >= 7 && e.SpentOn[:7] == nowMonth {
			inMonth = true
		}
	}
	if !inMonth { // gasto de outro mês (parcela futura, lançamento antigo) não avisa nada agora
		return
	}
	limit, ok, err := s.repo.BudgetLimit(ctx, hh, categoryID)
	if err != nil || !ok {
		if err != nil {
			s.log.Warn("falha ao ler o orçamento", "err", err)
		}
		return
	}
	used, err := s.repo.MonthUsage(ctx, hh, categoryID, nowMonth)
	if err != nil {
		s.log.Warn("falha ao somar o uso do orçamento", "err", err)
		return
	}
	top := 0
	for _, level := range []int{80, 100} {
		if used*100 < limit*int64(level) {
			continue
		}
		first, err := s.repo.TryAlert(ctx, hh, categoryID, nowMonth, level)
		if err != nil {
			s.log.Warn("falha ao gravar o aviso do orçamento", "err", err)
			return
		}
		if first {
			top = level
		}
	}
	if top == 0 {
		return
	}
	name := "uma categoria"
	if c, err := s.repo.GetCategory(ctx, hh, categoryID); err == nil {
		name = "«" + c.Name + "»"
	}
	ps, err := s.members(ctx, hh)
	if err != nil {
		return
	}
	body := fmt.Sprintf("O orçamento de %s chegou a %d%% do limite deste mês.", name, top)
	for _, m := range ps {
		s.notify(ctx, m.ID, "Orçamento do mês", body)
	}
}

// ---- resumo do mês ----

type CategorySummary struct {
	CategoryID    string `json:"category_id"`
	TotalCents    int64  `json:"total_cents"`
	PreviousCents int64  `json:"previous_cents"`
	LimitCents    *int64 `json:"limit_cents,omitempty"`
	UsedPercent   *int   `json:"used_percent,omitempty"`
}

type Summary struct {
	Month              string            `json:"month"`
	Scope              string            `json:"scope"`
	TotalCents         int64             `json:"total_cents"`
	PreviousTotalCents int64             `json:"previous_total_cents"`
	Categories         []CategorySummary `json:"categories"`
	PaidBy             map[string]int64  `json:"paid_by,omitempty"`
	// Cards são os totais nos cartões de quem pergunta e WithoutCardCents o que ficou sem cartão. O cartão do parceiro
	// não aparece em nenhum dos dois (a diferença para o total é o que ele pagou com cartão).
	Cards            []CardSummary `json:"cards"`
	WithoutCardCents int64         `json:"without_card_cents"`
}

// Summary soma o mês por categoria, compara com o mês anterior e mostra o uso do orçamento. Em "mine" só entram os
// gastos pessoais de quem pergunta; os do parceiro nunca contam nos totais.
func (s *Service) Summary(ctx context.Context, p identity.Principal, month, scope string) (Summary, error) {
	if scope == "" {
		scope = "household"
	}
	if scope != "household" && scope != "mine" {
		return Summary{}, fmt.Errorf("%w: scope é household ou mine", ErrInvalidInput)
	}
	if month == "" {
		month = fmtDate(today(s.now(), p.Timezone))[:7]
	}
	m, err := parseMonth(month)
	if err != nil {
		return Summary{}, err
	}
	prevMonth := m.AddDate(0, -1, 0).Format(monthLayout)
	cur, err := s.repo.CategoryTotals(ctx, p.HouseholdID, p.UserID, scope, month)
	if err != nil {
		return Summary{}, err
	}
	prev, err := s.repo.CategoryTotals(ctx, p.HouseholdID, p.UserID, scope, prevMonth)
	if err != nil {
		return Summary{}, err
	}
	limits := map[string]int64{}
	out := Summary{Month: month, Scope: scope, Categories: []CategorySummary{}, Cards: []CardSummary{}}
	byCard, without, err := s.repo.CardTotals(ctx, p.HouseholdID, p.UserID, scope, month)
	if err != nil {
		return Summary{}, err
	}
	out.WithoutCardCents = without
	for id, total := range byCard {
		out.Cards = append(out.Cards, CardSummary{CardID: id, TotalCents: total})
	}
	sort.Slice(out.Cards, func(i, j int) bool {
		a, b := out.Cards[i], out.Cards[j]
		if a.TotalCents != b.TotalCents {
			return a.TotalCents > b.TotalCents
		}
		return a.CardID < b.CardID
	})
	if scope == "household" {
		if out.PaidBy, err = s.repo.PaidTotals(ctx, p.HouseholdID, month); err != nil {
			return Summary{}, err
		}
		bs, err := s.repo.ListBudgets(ctx, p.HouseholdID)
		if err != nil {
			return Summary{}, err
		}
		for _, b := range bs {
			limits[b.CategoryID] = b.LimitCents
		}
	}
	seen := map[string]bool{}
	for _, set := range []map[string]int64{cur, prev, limits} {
		for id := range set {
			seen[id] = true
		}
	}
	for id := range seen {
		c := CategorySummary{CategoryID: id, TotalCents: cur[id], PreviousCents: prev[id]}
		if l, ok := limits[id]; ok {
			pct := int(c.TotalCents * 100 / l)
			c.LimitCents, c.UsedPercent = &l, &pct
		}
		out.TotalCents += c.TotalCents
		out.PreviousTotalCents += c.PreviousCents
		out.Categories = append(out.Categories, c)
	}
	sort.Slice(out.Categories, func(i, j int) bool {
		a, b := out.Categories[i], out.Categories[j]
		if a.TotalCents != b.TotalCents {
			return a.TotalCents > b.TotalCents
		}
		return a.CategoryID < b.CategoryID
	})
	return out, nil
}

// ---- exportação ----

const exportLimit = 20000

// ExportData é a cópia dos dados que a pessoa pode ver. A renda entra só a dela.
type ExportData struct {
	Categories    []Category   `json:"categories"`
	Expenses      []Expense    `json:"expenses"`
	Settlements   []Settlement `json:"settlements"`
	Cards         []Card       `json:"cards"` // só os meus
	MyIncomeCents *int64       `json:"my_monthly_income_cents,omitempty"`
}

func (s *Service) Export(ctx context.Context, p identity.Principal) (ExportData, error) {
	var d ExportData
	var err error
	if d.Categories, err = s.repo.ListCategories(ctx, p.HouseholdID, true); err != nil {
		return d, err
	}
	if d.Cards, err = s.repo.ListCards(ctx, p.HouseholdID, p.UserID, true); err != nil {
		return d, err
	}
	if d.Expenses, err = s.repo.VisibleExpenses(ctx, p.HouseholdID, p.UserID, exportLimit); err != nil {
		return d, err
	}
	if d.Settlements, err = s.repo.ListSettlements(ctx, p.HouseholdID); err != nil {
		return d, err
	}
	d.MyIncomeCents, err = s.GetIncome(ctx, p)
	return d, err
}

// WriteCSV escreve gastos e acertos numa tabela só (coluna "tipo"). Texto que começa com = + - @ ganha um apóstrofo,
// para a planilha não executá-lo como fórmula.
func (d ExportData) WriteCSV(w io.Writer) error {
	names := map[string]string{}
	for _, c := range d.Categories {
		names[c.ID] = c.Name
	}
	cards := map[string]string{}
	for _, c := range d.Cards {
		cards[c.ID] = c.Name
	}
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"tipo", "data", "descricao", "categoria", "valor_centavos", "pago_por", "escopo", "parcela", "acertado", "de", "para", "cartao"})
	for _, e := range d.Expenses {
		inst := ""
		if e.Installment != nil {
			inst = fmt.Sprintf("%d/%d", e.Installment.No, e.Installment.Total)
		}
		_ = cw.Write([]string{"gasto", e.SpentOn, safeCell(e.Description), safeCell(names[e.CategoryID]),
			strconv.FormatInt(e.AmountCents, 10), e.PaidBy, e.Scope, inst, strconv.FormatBool(e.Settled), "", "", safeCell(cards[e.MyCardID])})
	}
	for _, st := range d.Settlements {
		_ = cw.Write([]string{"acerto", st.ThroughOn, st.Kind, "", strconv.FormatInt(st.AmountCents, 10), "", "", "", "", st.FromUser, st.ToUser, ""})
	}
	cw.Flush()
	return cw.Error()
}

func safeCell(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}

// CategoryMonthlySpend é o que o módulo Metas lê (por um adaptador no app): o limite atual do orçamento da categoria
// e o que o CASAL gastou nela em cada mês entre from e to. Categoria sem limite (ou de outro casal) devolve
// hasLimit falso. O gasto pessoal nunca entra.
func (s *Service) CategoryMonthlySpend(ctx context.Context, householdID, categoryID string, from, to time.Time) (limit int64, hasLimit bool, spent map[string]int64, err error) {
	limit, hasLimit, err = s.repo.BudgetLimit(ctx, householdID, categoryID)
	if err != nil || !hasLimit {
		return 0, false, nil, err
	}
	spent, err = s.repo.SpentByMonth(ctx, householdID, categoryID, from, to)
	return limit, true, spent, err
}
