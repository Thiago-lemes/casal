package goal

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"casal/internal/identity"
)

// ---- aportes (meta financeira) ----

// ContributionResult é o aporte gravado e a meta com o progresso já recalculado.
type ContributionResult struct {
	Contribution Contribution `json:"contribution"`
	Goal         Goal         `json:"goal"`
}

// AddContribution registra um aporte (positivo) ou uma retirada (negativo). É um registro da meta: não cria gasto
// em Finanças nem mexe no saldo do acerto.
func (s *Service) AddContribution(ctx context.Context, p identity.Principal, goalID string, in ContributionInput) (ContributionResult, error) {
	if in.AmountCents == 0 || in.AmountCents > MaxAmountCents || in.AmountCents < -MaxAmountCents {
		return ContributionResult{}, ErrAmountInvalid
	}
	todayD := today(s.now(), p.Timezone)
	made := todayD
	if in.MadeOn != "" {
		d, err := parseDay(in.MadeOn)
		if err != nil {
			return ContributionResult{}, err
		}
		made = d
	}
	if made.After(todayD) {
		return ContributionResult{}, fmt.Errorf("%w: o aporte não pode ser de uma data futura", ErrInvalid)
	}
	note := strings.TrimSpace(in.Note)
	if utf8.RuneCountInString(note) > 200 {
		return ContributionResult{}, fmt.Errorf("%w: a observação tem até 200 caracteres", ErrInvalid)
	}
	c, err := s.repo.AddContribution(ctx, p.HouseholdID, p.UserID, goalID, Contribution{AmountCents: in.AmountCents, MadeOn: fmtDay(made), Note: note})
	if err != nil {
		return ContributionResult{}, err
	}
	g, err := s.Get(ctx, p, goalID)
	return ContributionResult{Contribution: c, Goal: g}, err
}

func (s *Service) ListContributions(ctx context.Context, p identity.Principal, goalID, before string, limit int) (ContributionPage, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	items, err := s.repo.ListContributions(ctx, p.HouseholdID, p.UserID, goalID, before, limit)
	if err != nil {
		return ContributionPage{}, err
	}
	page := ContributionPage{Items: items}
	if len(items) == limit {
		page.NextBefore = items[len(items)-1].ID
	}
	return page, nil
}

// DeleteContribution apaga um aporte e devolve a meta recalculada.
func (s *Service) DeleteContribution(ctx context.Context, p identity.Principal, goalID, id string) (Goal, error) {
	if err := s.repo.DeleteContribution(ctx, p.HouseholdID, p.UserID, goalID, id); err != nil {
		return Goal{}, err
	}
	return s.Get(ctx, p, goalID)
}

// ---- marcações (hábito e meta do casal) ----

// CheckinResult diz se a marcação é nova (marcar de novo o mesmo dia não duplica) e traz a meta recalculada.
type CheckinResult struct {
	Created bool `json:"created"`
	Goal    Goal `json:"goal"`
}

// Checkin marca um dia (padrão: hoje no fuso de quem marca). Não aceita o futuro nem mais de 31 dias atrás.
func (s *Service) Checkin(ctx context.Context, p identity.Principal, goalID, day string) (CheckinResult, error) {
	todayD := today(s.now(), p.Timezone)
	d := todayD
	if day != "" {
		var err error
		if d, err = parseDay(day); err != nil {
			return CheckinResult{}, ErrCheckinDate
		}
	}
	if d.After(todayD) || d.Before(todayD.AddDate(0, 0, -MaxBackfillDays)) {
		return CheckinResult{}, ErrCheckinDate
	}
	created, err := s.repo.AddCheckin(ctx, p.HouseholdID, p.UserID, goalID, fmtDay(d))
	if err != nil {
		return CheckinResult{}, err
	}
	g, err := s.repo.GetGoal(ctx, p.HouseholdID, p.UserID, goalID)
	if err != nil {
		return CheckinResult{}, err
	}
	if created && d.Equal(todayD) {
		switch g.Kind {
		case KindHabit:
			s.syncHabitFor(ctx, g, nil) // marcou: o lembrete do dia não faz mais sentido
		case KindShared:
			s.noticeShared(ctx, p, g, d)
		}
	}
	out, err := s.present(ctx, p, g, true)
	return CheckinResult{Created: created, Goal: out}, err
}

// UndoCheckin desfaz a PRÓPRIA marcação de um dia.
func (s *Service) UndoCheckin(ctx context.Context, p identity.Principal, goalID, day string) (Goal, error) {
	d, err := parseDay(day)
	if err != nil {
		return Goal{}, ErrCheckinDate
	}
	if err := s.repo.RemoveCheckin(ctx, p.HouseholdID, p.UserID, goalID, fmtDay(d)); err != nil {
		return Goal{}, err
	}
	g, err := s.repo.GetGoal(ctx, p.HouseholdID, p.UserID, goalID)
	if err != nil {
		return Goal{}, err
	}
	if g.Kind == KindHabit {
		s.syncHabitFor(ctx, g, nil)
	}
	return s.present(ctx, p, g, true)
}

// noticeShared avisa a outra pessoa, no máximo uma vez por meta por dia, sem cobrar: o aviso é só um fato.
func (s *Service) noticeShared(ctx context.Context, p identity.Principal, g Goal, day time.Time) {
	if g.Status != StatusActive {
		return
	}
	ps, err := s.members(ctx, g.HouseholdID)
	if err != nil {
		return
	}
	other, ok := otherThan(ps, p.UserID)
	if !ok {
		return
	}
	first, err := s.repo.ClaimSharedNotice(ctx, g.HouseholdID, g.ID, fmtDay(day))
	if err != nil || !first {
		return
	}
	name := "Seu par"
	if me, ok := person(ps, p.UserID); ok && me.Name != "" {
		name = me.Name
	}
	s.notify(ctx, other.ID, "Meta do casal", name+" marcou: "+g.Title)
}

// History devolve as marcações por dia e o resumo por período, só para quem tem o detalhe da meta.
func (s *Service) History(ctx context.Context, p identity.Principal, goalID, from, to string) (History, error) {
	g, err := s.repo.GetGoal(ctx, p.HouseholdID, p.UserID, goalID)
	if err != nil {
		return History{}, err
	}
	if g.Kind != KindHabit && g.Kind != KindShared {
		return History{}, ErrKindMismatch
	}
	if g.Access != AccessFull {
		return History{}, ErrForbidden
	}
	todayD := today(s.now(), p.Timezone)
	end, start := todayD, todayD.AddDate(0, 0, -90)
	if to != "" {
		if end, err = parseDay(to); err != nil {
			return History{}, err
		}
	}
	if from != "" {
		if start, err = parseDay(from); err != nil {
			return History{}, err
		}
	}
	if end.Before(start) || end.Sub(start) > 366*24*time.Hour {
		return History{}, fmt.Errorf("%w: o intervalo vai de 1 a 366 dias", ErrInvalid)
	}

	period, target := "", 0
	if g.Habit != nil {
		period, target = g.Habit.Period, g.Habit.TargetCount
	} else {
		period, target = g.Shared.Period, g.Shared.TargetCount
	}
	// os períodos inteiros que tocam o intervalo
	firstStart, _ := periodOf(period, start)
	_, lastEnd := periodOf(period, end)
	marks, err := s.repo.Checkins(ctx, g.HouseholdID, p.UserID, []string{g.ID}, firstStart, lastEnd)
	if err != nil {
		return History{}, err
	}
	all := marks[g.ID]
	ps, err := s.members(ctx, g.HouseholdID)
	if err != nil {
		return History{}, err
	}
	members := map[string]bool{}
	for _, m := range ps {
		members[m.ID] = true
	}

	h := History{From: fmtDay(start), To: fmtDay(end), Days: []DayMarks{}, Periods: []PeriodSummary{}}
	var inRange []Checkin
	for _, m := range all {
		if d := mustDay(m.Day); !d.Before(start) && !d.After(end) {
			inRange = append(inRange, m)
		}
	}
	h.Days = s.dayMarks(ctx, &g, inRange)

	owner := ""
	if g.OwnerID != nil {
		owner = *g.OwnerID
	}
	required := 1
	if g.Shared != nil && g.Shared.RequiresBoth {
		required = max(1, min(2, len(members)))
	}
	for cur := firstStart; cur.Before(lastEnd); {
		pStart, pEnd := periodOf(period, cur)
		count := 0
		if g.Habit != nil {
			count = habitDays(all, owner, g.Habit.Weekdays, pStart, pEnd)
		} else {
			count, _ = sharedDays(all, required, members, pStart, pEnd)
		}
		h.Periods = append(h.Periods, PeriodSummary{Start: fmtDay(pStart), End: fmtDay(pEnd.AddDate(0, 0, -1)), Count: count, Target: target})
		cur = pEnd
	}
	return h, nil
}
