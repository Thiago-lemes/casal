package finance

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"casal/internal/identity"
)

const syncLimit = 2000

type BillInput struct {
	Title         string
	AmountCents   *int64
	DueOn         string
	ResponsibleID string
	CategoryID    string
	NotifyBoth    bool
	Repeat        string
}

type PayInput struct {
	AmountCents int64 // 0 usa o valor da conta
	PaidOn      string
	PaidBy      string
}

type PayResult struct {
	Bill     Bill    `json:"bill"`
	Expense  Expense `json:"expense"`
	NextBill *Bill   `json:"next_bill,omitempty"`
	Fallback bool    `json:"split_fallback,omitempty"`
}

// reminderPlan é um lembrete que a conta deve ter. O ref carrega o vencimento e a pessoa, então mudar um dos dois
// muda os refs e o sincronismo cancela os velhos e cria os novos.
type reminderPlan struct {
	Ref       string
	Recipient string
	Body      string
	At        time.Time
}

// billPlans: 3 dias antes e no dia, às 09:00 na hora local de quem recebe. Com notify_both, também para o parceiro.
func billPlans(b Bill, ps []Person) []reminderPlan {
	due, err := parseDate(b.DueOn)
	if err != nil || b.PaidAt != nil {
		return nil
	}
	who := []string{b.ResponsibleID}
	if b.NotifyBoth {
		for _, p := range ps {
			if p.ID != b.ResponsibleID {
				who = append(who, p.ID)
			}
		}
	}
	var out []reminderPlan
	for _, id := range who {
		var person *Person
		for i := range ps {
			if ps[i].ID == id {
				person = &ps[i]
			}
		}
		if person == nil {
			continue // quem saiu do casal não recebe
		}
		loc, err := time.LoadLocation(person.Timezone)
		if err != nil || person.Timezone == "" {
			loc, _ = time.LoadLocation("America/Sao_Paulo")
			if loc == nil {
				loc = time.UTC
			}
		}
		at := func(days int) time.Time {
			return time.Date(due.Year(), due.Month(), due.Day()-days, 9, 0, 0, 0, loc)
		}
		out = append(out,
			reminderPlan{fmt.Sprintf("bill:%s:%s:%s:d3", b.ID, b.DueOn, id), id, "Conta «" + b.Title + "» vence em 3 dias", at(3)},
			reminderPlan{fmt.Sprintf("bill:%s:%s:%s:d0", b.ID, b.DueOn, id), id, "Conta «" + b.Title + "» vence hoje", at(0)})
	}
	return out
}

// syncReminders deixa os lembretes da conta como ela pede agora: cancela os refs que já não valem e cria os que
// faltam (só os que ainda estão no futuro). Os lembretes são de outra feature e ficam fora da transação da conta;
// se algo falhar, devolve false e o job de reconciliação refaz por recálculo, não por incremento.
func (s *Service) syncReminders(ctx context.Context, b Bill, ps []Person) bool {
	plans := billPlans(b, ps)
	want := map[string]bool{}
	for _, pl := range plans {
		want[pl.Ref] = true
	}
	have := map[string]bool{}
	for _, r := range b.ReminderRefs {
		have[r] = true
	}
	ok := true
	kept := map[string]bool{}
	for r := range have {
		if want[r] {
			kept[r] = true
			continue
		}
		if err := s.reminders.CancelByRef(ctx, b.HouseholdID, r); err != nil {
			s.log.Warn("falha ao cancelar lembrete da conta", "bill", b.ID, "ref", r, "err", err)
			kept[r], ok = true, false // continua anotado para tentar de novo
		}
	}
	for _, pl := range plans {
		if have[pl.Ref] {
			continue
		}
		// o horário que já passou fica anotado como tratado, sem criar o lembrete
		if pl.At.After(s.now()) {
			if err := s.reminders.CreateWithRef(ctx, b.HouseholdID, pl.Recipient, pl.Recipient, pl.Body, pl.At, pl.Ref); err != nil {
				s.log.Warn("falha ao criar lembrete da conta", "bill", b.ID, "ref", pl.Ref, "err", err)
				ok = false
				continue
			}
		}
		kept[pl.Ref] = true
	}
	refs := make([]string, 0, len(kept))
	for r := range kept {
		refs = append(refs, r)
	}
	sort.Strings(refs)
	if strings.Join(refs, ",") != strings.Join(sortedCopy(b.ReminderRefs), ",") {
		if err := s.repo.SetBillRefs(ctx, b.HouseholdID, b.ID, refs); err != nil {
			s.log.Warn("falha ao anotar os lembretes da conta", "bill", b.ID, "err", err)
			ok = false
		}
	}
	return ok
}

func sortedCopy(in []string) []string {
	out := append([]string{}, in...)
	sort.Strings(out)
	return out
}

func (s *Service) billDraft(ctx context.Context, p identity.Principal, ps []Person, in BillInput) (Bill, error) {
	title, err := checkText(in.Title, maxTitle, "o título")
	if err != nil {
		return Bill{}, err
	}
	if title == "" {
		return Bill{}, fmt.Errorf("%w: o título é obrigatório", ErrInvalidInput)
	}
	due, err := parseDate(in.DueOn)
	if err != nil {
		return Bill{}, err
	}
	if in.AmountCents != nil && !validAmount(*in.AmountCents) {
		return Bill{}, ErrAmountInvalid
	}
	b := Bill{
		HouseholdID: p.HouseholdID, CreatedBy: p.UserID, Title: title, AmountCents: in.AmountCents,
		DueOn: fmtDate(due), DueDay: due.Day(), ResponsibleID: in.ResponsibleID, CategoryID: in.CategoryID,
		NotifyBoth: in.NotifyBoth, Repeat: in.Repeat,
	}
	if b.ResponsibleID == "" {
		b.ResponsibleID = p.UserID
	}
	if !isMember(ps, b.ResponsibleID) {
		return Bill{}, fmt.Errorf("%w: o responsável precisa ser do casal", ErrInvalidInput)
	}
	if b.Repeat == "" {
		b.Repeat = "none"
	}
	if b.Repeat != "none" && b.Repeat != "monthly" {
		return Bill{}, fmt.Errorf("%w: repeat é none ou monthly", ErrInvalidInput)
	}
	return b, nil
}

func (s *Service) CreateBill(ctx context.Context, p identity.Principal, in BillInput) (Bill, error) {
	ps, err := s.members(ctx, p.HouseholdID)
	if err != nil {
		return Bill{}, err
	}
	b, err := s.billDraft(ctx, p, ps, in)
	if err != nil {
		return Bill{}, err
	}
	if b.CategoryID == "" {
		c, err := s.repo.CategoryByKey(ctx, p.HouseholdID, KeyBills)
		if err != nil {
			return Bill{}, err
		}
		b.CategoryID = c.ID
	}
	if err := s.usableCategory(ctx, p.HouseholdID, b.CategoryID); err != nil {
		return Bill{}, err
	}
	out, err := s.repo.InsertBill(ctx, b)
	if err != nil {
		return Bill{}, err
	}
	out.SyncPending = !s.syncReminders(ctx, out, ps)
	return out, nil
}

func (s *Service) GetBill(ctx context.Context, p identity.Principal, id string) (Bill, error) {
	return s.repo.GetBill(ctx, p.HouseholdID, id)
}

// ListBills: status open (padrão, as vencidas primeiro) ou paid. due=today traz as abertas que vencem hoje ou já
// venceram, para a tela Hoje.
func (s *Service) ListBills(ctx context.Context, p identity.Principal, status, dueBefore string, dueToday bool) ([]Bill, error) {
	if status != "" && status != "open" && status != "paid" {
		return nil, fmt.Errorf("%w: status é open ou paid", ErrInvalidInput)
	}
	if dueBefore != "" {
		d, err := parseDate(dueBefore)
		if err != nil {
			return nil, err
		}
		dueBefore = fmtDate(d)
	}
	if dueToday {
		status, dueBefore = "open", fmtDate(today(s.now(), p.Timezone))
	}
	return s.repo.ListBills(ctx, p.HouseholdID, BillFilter{Status: status, DueBefore: dueBefore})
}

func (s *Service) UpdateBill(ctx context.Context, p identity.Principal, id string, in BillInput) (Bill, error) {
	cur, err := s.repo.GetBill(ctx, p.HouseholdID, id)
	if err != nil {
		return Bill{}, err
	}
	if cur.PaidAt != nil {
		return Bill{}, ErrBillAlreadyPaid
	}
	ps, err := s.members(ctx, p.HouseholdID)
	if err != nil {
		return Bill{}, err
	}
	b, err := s.billDraft(ctx, p, ps, in)
	if err != nil {
		return Bill{}, err
	}
	b.ID, b.CreatedBy = cur.ID, cur.CreatedBy
	if b.CategoryID == "" {
		b.CategoryID = cur.CategoryID
	} else if b.CategoryID != cur.CategoryID {
		if err := s.usableCategory(ctx, p.HouseholdID, b.CategoryID); err != nil {
			return Bill{}, err
		}
	}
	if b.DueOn == cur.DueOn { // o dia âncora do vencimento só muda se a data mudar
		b.DueDay = cur.DueDay
	}
	out, err := s.repo.UpdateBill(ctx, b)
	if isNoRows(err) {
		return Bill{}, ErrBillAlreadyPaid // existia e estava aberta um instante atrás: foi paga no meio
	}
	if err != nil {
		return Bill{}, err
	}
	out.SyncPending = !s.syncReminders(ctx, out, ps)
	return out, nil
}

func (s *Service) DeleteBill(ctx context.Context, p identity.Principal, id string) error {
	b, err := s.repo.DeleteBill(ctx, p.HouseholdID, id)
	if err != nil {
		return err
	}
	for _, r := range b.ReminderRefs {
		if err := s.reminders.CancelByRef(ctx, p.HouseholdID, r); err != nil {
			s.log.Warn("falha ao cancelar lembrete de conta apagada", "bill", id, "ref", r, "err", err)
		}
	}
	return nil
}

// PayBill marca a conta como paga e cria o gasto do casal na mesma transação; conta mensal cria a próxima.
func (s *Service) PayBill(ctx context.Context, p identity.Principal, id string, in PayInput) (PayResult, error) {
	b, err := s.repo.GetBill(ctx, p.HouseholdID, id)
	if err != nil {
		return PayResult{}, err
	}
	if b.PaidAt != nil {
		return PayResult{}, ErrBillAlreadyPaid
	}
	amount := in.AmountCents
	if amount == 0 && b.AmountCents != nil {
		amount = *b.AmountCents
	}
	if !validAmount(amount) {
		return PayResult{}, ErrAmountInvalid
	}
	ps, err := s.members(ctx, p.HouseholdID)
	if err != nil {
		return PayResult{}, err
	}
	paidBy := in.PaidBy
	if paidBy == "" {
		paidBy = p.UserID
	}
	if !isMember(ps, paidBy) {
		return PayResult{}, fmt.Errorf("%w: quem pagou precisa ser do casal", ErrInvalidInput)
	}
	spent := fmtDate(today(s.now(), p.Timezone))
	if in.PaidOn != "" {
		d, err := parseDate(in.PaidOn)
		if err != nil {
			return PayResult{}, err
		}
		spent = fmtDate(d)
	}
	split, fallback, err := s.defaultSplit(ctx, p.HouseholdID, ps)
	if err != nil {
		return PayResult{}, err
	}
	paid, exp, next, err := s.repo.PayBill(ctx, p.HouseholdID, id, Expense{
		CreatedBy: p.UserID, PaidBy: paidBy, Scope: ScopeHousehold, Visibility: VisHousehold, AmountCents: amount,
		CategoryID: b.CategoryID, SpentOn: spent, Description: b.Title, Split: split, Source: SourceBill,
	})
	if err != nil {
		return PayResult{}, err
	}
	s.syncReminders(ctx, paid, ps) // conta paga: cancela os pendentes
	res := PayResult{Bill: paid, Expense: exp, NextBill: next, Fallback: fallback}
	if next != nil {
		res.NextBill.SyncPending = !s.syncReminders(ctx, *next, ps)
	}
	s.checkBudget(ctx, p, exp.CategoryID, []Expense{exp})
	return res, nil
}

// ReconcileReminders é o job de hora em hora: recalcula os lembretes das contas abertas dos casais com o módulo
// ligado e refaz o que faltou. Devolve quantas contas mudaram.
func (s *Service) ReconcileReminders(ctx context.Context) (int64, error) {
	bills, err := s.repo.OpenBillsForSync(ctx, syncLimit)
	if err != nil {
		return 0, err
	}
	people := map[string][]Person{}
	var n int64
	for _, b := range bills {
		ps, ok := people[b.HouseholdID]
		if !ok {
			if ps, err = s.members(ctx, b.HouseholdID); err != nil {
				s.log.Warn("falha ao listar o casal na reconciliação", "household", b.HouseholdID, "err", err)
				continue
			}
			people[b.HouseholdID] = ps
		}
		before := strings.Join(sortedCopy(b.ReminderRefs), ",")
		want := []string{}
		for _, pl := range billPlans(b, ps) {
			want = append(want, pl.Ref)
		}
		sort.Strings(want)
		if before == strings.Join(want, ",") {
			continue
		}
		s.syncReminders(ctx, b, ps)
		n++
	}
	return n, nil
}

// ModuleChanged é chamado depois de o módulo ser ligado ou desligado: desligado, os lembretes das contas são
// cancelados (sem o módulo, nenhum lembrete de Finanças existe); religado, são refeitos. As contas ficam.
func (s *Service) ModuleChanged(ctx context.Context, householdID string, enabled bool) {
	if enabled {
		bills, err := s.repo.ListBills(ctx, householdID, BillFilter{Status: "open"})
		ps, err2 := s.members(ctx, householdID)
		if err != nil || err2 != nil {
			s.log.Warn("falha ao refazer lembretes ao religar Finanças", "household", householdID, "err", err, "err2", err2)
			return
		}
		for _, b := range bills {
			s.syncReminders(ctx, b, ps)
		}
		return
	}
	bills, err := s.repo.BillsWithRefs(ctx, householdID)
	if err != nil {
		s.log.Warn("falha ao cancelar lembretes ao desligar Finanças", "household", householdID, "err", err)
		return
	}
	for _, b := range bills {
		failed := false
		for _, r := range b.ReminderRefs {
			if err := s.reminders.CancelByRef(ctx, householdID, r); err != nil {
				s.log.Warn("falha ao cancelar lembrete", "ref", r, "err", err)
				failed = true
			}
		}
		if !failed {
			_ = s.repo.SetBillRefs(ctx, householdID, b.ID, nil)
		}
	}
}
