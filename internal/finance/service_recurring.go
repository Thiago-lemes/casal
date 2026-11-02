package finance

import (
	"context"
	"fmt"
	"time"

	"casal/internal/identity"
)

type RecurringInput struct {
	Scope       string
	Visibility  string
	PaidBy      string
	AmountCents int64
	CategoryID  string
	Description string
	Split       map[string]int
	DayOfMonth  int
	StartOn     string
	EndOn       string
}

func (s *Service) recurringDraft(ctx context.Context, p identity.Principal, ps []Person, in RecurringInput) (Recurring, error) {
	if !validAmount(in.AmountCents) {
		return Recurring{}, ErrAmountInvalid
	}
	if in.DayOfMonth < 1 || in.DayOfMonth > 31 {
		return Recurring{}, fmt.Errorf("%w: o dia do mês vai de 1 a 31", ErrInvalidInput)
	}
	// reaproveita as regras do gasto (escopo, quem pagou, visibilidade, descrição)
	d, err := s.expenseDraft(ctx, p, ps, ExpenseInput{
		Scope: in.Scope, Visibility: in.Visibility, PaidBy: in.PaidBy, Description: in.Description,
		CategoryID: in.CategoryID, Split: in.Split,
	})
	if err != nil {
		return Recurring{}, err
	}
	r := Recurring{
		HouseholdID: p.HouseholdID, CreatedBy: p.UserID, PaidBy: d.PaidBy, AmountCents: in.AmountCents,
		CategoryID: in.CategoryID, Description: d.Description, Scope: d.Scope, Visibility: d.Visibility,
		DayOfMonth: in.DayOfMonth, Active: true,
	}
	if in.EndOn != "" {
		end, err := parseDate(in.EndOn)
		if err != nil {
			return Recurring{}, err
		}
		r.EndOn = fmtDate(end)
	}
	return r, nil
}

func (s *Service) CreateRecurring(ctx context.Context, p identity.Principal, in RecurringInput) (Recurring, error) {
	ps, err := s.members(ctx, p.HouseholdID)
	if err != nil {
		return Recurring{}, err
	}
	r, err := s.recurringDraft(ctx, p, ps, in)
	if err != nil {
		return Recurring{}, err
	}
	if err := s.usableCategory(ctx, p.HouseholdID, r.CategoryID); err != nil {
		return Recurring{}, err
	}
	now := today(s.now(), p.Timezone)
	start := now
	if in.StartOn != "" {
		if start, err = parseDate(in.StartOn); err != nil {
			return Recurring{}, err
		}
	}
	// não há retroatividade além do mês corrente
	if start.Before(time0(now)) {
		return Recurring{}, fmt.Errorf("%w: o início não pode ser anterior ao mês atual", ErrInvalidInput)
	}
	r.StartOn = fmtDate(start)
	if r.EndOn != "" && r.EndOn < r.StartOn {
		return Recurring{}, fmt.Errorf("%w: o fim é anterior ao início", ErrInvalidInput)
	}
	if r.Scope == ScopeHousehold {
		if r.Split, _, err = s.resolveSplit(ctx, p.HouseholdID, in.Split, ps); err != nil {
			return Recurring{}, err
		}
	}
	out, err := s.repo.InsertRecurring(ctx, r)
	if err != nil {
		return Recurring{}, err
	}
	// se o dia deste mês já chegou, o gasto aparece agora em vez de esperar a próxima rodada do job
	for _, m := range ps {
		if m.ID == out.PaidBy {
			s.generateOne(ctx, DueRecurring{Recurring: out, Timezone: m.Timezone})
		}
	}
	return out, nil
}

// time0 é o primeiro dia do mês da data.
func time0(d time.Time) time.Time { return DayInMonth(d, 1) }

func (s *Service) ListRecurring(ctx context.Context, p identity.Principal) ([]Recurring, error) {
	return s.repo.ListRecurring(ctx, p.HouseholdID, p.UserID)
}

// readRecurring devolve o gasto fixo se a pessoa o vê, e confere se pode alterá-lo.
func (s *Service) mutableRecurring(ctx context.Context, p identity.Principal, id string) (Recurring, error) {
	cur, err := s.repo.GetRecurring(ctx, p.HouseholdID, id, p.UserID)
	if err != nil {
		return Recurring{}, err
	}
	if !cur.CanEdit {
		return Recurring{}, ErrExpenseForbidden
	}
	return cur, nil
}

// UpdateRecurring muda o gasto fixo dos próximos meses em diante; o que já foi gerado não muda.
func (s *Service) UpdateRecurring(ctx context.Context, p identity.Principal, id string, in RecurringInput) (Recurring, error) {
	cur, err := s.mutableRecurring(ctx, p, id)
	if err != nil {
		return Recurring{}, err
	}
	ps, err := s.members(ctx, p.HouseholdID)
	if err != nil {
		return Recurring{}, err
	}
	in.Scope = cur.Scope
	if in.Visibility == "" {
		in.Visibility = cur.Visibility
	}
	r, err := s.recurringDraft(ctx, p, ps, in)
	if err != nil {
		return Recurring{}, err
	}
	if in.CategoryID != cur.CategoryID {
		if err := s.usableCategory(ctx, p.HouseholdID, in.CategoryID); err != nil {
			return Recurring{}, err
		}
	}
	r.ID, r.StartOn = cur.ID, cur.StartOn
	if r.EndOn != "" && r.EndOn < r.StartOn {
		return Recurring{}, fmt.Errorf("%w: o fim é anterior ao início", ErrInvalidInput)
	}
	if r.Scope == ScopeHousehold {
		r.Split = cur.Split
		if in.Split != nil {
			if r.Split, err = NormalizeSplit(in.Split, ids(ps)); err != nil {
				return Recurring{}, err
			}
		}
	}
	return s.repo.UpdateRecurring(ctx, r, p.UserID)
}

func (s *Service) DeleteRecurring(ctx context.Context, p identity.Principal, id string) error {
	if _, err := s.mutableRecurring(ctx, p, id); err != nil {
		return err
	}
	ok, err := s.repo.DeleteRecurring(ctx, p.HouseholdID, id, p.UserID)
	if err == nil && !ok {
		return ErrRecurringNotFound
	}
	return err
}

func (s *Service) SetRecurringActive(ctx context.Context, p identity.Principal, id string, active bool) (Recurring, error) {
	if _, err := s.mutableRecurring(ctx, p, id); err != nil {
		return Recurring{}, err
	}
	return s.repo.SetRecurringActive(ctx, p.HouseholdID, id, p.UserID, active)
}

// generateOne cria o gasto do mês de um fixo, se o dia já chegou no fuso de quem paga e ainda não foi gerado.
// Sem retroatividade: só o mês corrente, e nunca antes do início. Idempotente.
func (s *Service) generateOne(ctx context.Context, d DueRecurring) bool {
	t := today(s.now(), d.Timezone)
	target := DayInMonth(t, d.DayOfMonth)
	if target.After(t) {
		return false
	}
	start, err := parseDate(d.StartOn)
	if err != nil || target.Before(start) {
		return false
	}
	if d.EndOn != "" {
		if end, err := parseDate(d.EndOn); err != nil || target.After(end) {
			return false
		}
	}
	day := fmtDate(target)
	if d.LastGenerated != "" && day <= d.LastGenerated {
		return false // já foi gerado (e se a pessoa apagou o gasto, ele não volta)
	}
	e, inserted, err := s.repo.GenerateRecurring(ctx, d.Recurring, day)
	if err != nil {
		s.log.Error("falha ao gerar gasto fixo", "recurring", d.ID, "err", err)
		return false
	}
	if inserted && e.Scope == ScopeHousehold {
		s.checkBudgetMonth(ctx, d.HouseholdID, fmtDate(t)[:7], e.CategoryID, []Expense{e})
	}
	return inserted
}

// GenerateRecurring é o job de hora em hora: gera os gastos fixos do dia que chegou. Devolve quantos gastos criou.
func (s *Service) GenerateRecurring(ctx context.Context) (int64, error) {
	due, err := s.repo.DueRecurring(ctx)
	if err != nil {
		return 0, err
	}
	var n int64
	for _, d := range due {
		if s.generateOne(ctx, d) {
			n++
		}
	}
	return n, nil
}
