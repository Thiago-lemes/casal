package goal

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"time"

	"casal/internal/identity"
)

// ---- lembrete de hábito ----
//
// Há no máximo um lembrete por dia que ainda importa, com o ref goal:<id>:<AAAA-MM-DD> (o dia é o do dono). Nenhum
// nos dias fora de weekdays, depois de marcado o dia, depois de cumprida a meta do período, com a meta pausada ou
// depois de o horário passar. O job recalcula de hora em hora; criar de novo com o mesmo ref é idempotente.

// syncHabitFor recalcula o lembrete de hoje do hábito. ps pode ser nulo (os membros são lidos).
func (s *Service) syncHabitFor(ctx context.Context, g Goal, ps []Person) {
	if g.Kind != KindHabit || g.Habit == nil || g.OwnerID == nil {
		return
	}
	if ps == nil {
		var err error
		if ps, err = s.members(ctx, g.HouseholdID); err != nil {
			s.log.Warn("falha ao listar o casal para o lembrete do hábito", "goal", g.ID, "err", err)
			return
		}
	}
	if err := s.syncHabit(ctx, g, ps); err != nil {
		s.log.Warn("falha ao sincronizar o lembrete do hábito", "goal", g.ID, "err", err)
	}
}

func (s *Service) syncHabit(ctx context.Context, g Goal, ps []Person) error {
	owner := *g.OwnerID
	tz := ""
	if pe, ok := person(ps, owner); ok {
		tz = pe.Timezone
	}
	now := s.now()
	day := today(now, tz)
	ref := habitRef(g.ID, day)

	h := g.Habit
	want := g.Status == StatusActive && h.RemindEnabled && dayCounts(h.Weekdays, day)
	var at time.Time
	if want {
		start, end := periodOf(h.Period, day)
		marks, err := s.repo.Checkins(ctx, g.HouseholdID, owner, []string{g.ID}, start, end)
		if err != nil {
			return err
		}
		markedToday := false
		for _, m := range marks[g.ID] {
			if m.UserID == owner && m.Day == fmtDay(day) {
				markedToday = true
			}
		}
		done := habitDays(marks[g.ID], owner, h.Weekdays, start, end)
		at = time.Date(day.Year(), day.Month(), day.Day(), h.RemindMin/60, h.RemindMin%60, 0, 0, location(tz))
		// já marcou, já cumpriu o período ou o horário passou: não insiste
		want = !markedToday && done < h.TargetCount && at.After(now)
	}
	if !want {
		return s.reminders.CancelByRef(ctx, g.HouseholdID, ref)
	}
	return s.reminders.CreateWithRef(ctx, g.HouseholdID, owner, owner, "Hábito: "+g.Title, at, ref)
}

func (s *Service) cancelHabitToday(ctx context.Context, g Goal) {
	s.cancelHabitRefs(ctx, g, 0, 0)
}

// cancelHabitDays cancela os lembretes de ontem, hoje e amanhã (a folga cobre a virada de dia em fusos diferentes).
func (s *Service) cancelHabitDays(ctx context.Context, g Goal) { s.cancelHabitRefs(ctx, g, -1, 1) }

func (s *Service) cancelHabitRefs(ctx context.Context, g Goal, from, to int) {
	if g.OwnerID == nil {
		return
	}
	tz := ""
	if ps, err := s.members(ctx, g.HouseholdID); err == nil {
		if pe, ok := person(ps, *g.OwnerID); ok {
			tz = pe.Timezone
		}
	}
	day := today(s.now(), tz)
	for i := from; i <= to; i++ {
		if err := s.reminders.CancelByRef(ctx, g.HouseholdID, habitRef(g.ID, day.AddDate(0, 0, i))); err != nil {
			s.log.Warn("falha ao cancelar o lembrete do hábito", "goal", g.ID, "err", err)
		}
	}
}

// ReconcileHabitReminders é o job de hora em hora: garante o lembrete do dia dos hábitos ativos com lembrete, nos
// casais com o módulo ligado, e cancela o que já não vale. Rodar duas vezes não duplica nada.
func (s *Service) ReconcileHabitReminders(ctx context.Context) (int64, error) {
	people := map[string][]Person{}
	after := firstID
	for {
		habits, err := s.repo.HabitsForReminders(ctx, after, jobPage)
		if err != nil {
			return 0, err
		}
		for _, g := range habits {
			after = g.ID
			s.reconcileHabit(ctx, g, people)
		}
		if len(habits) < jobPage {
			return 0, nil
		}
	}
}

func (s *Service) reconcileHabit(ctx context.Context, g Goal, people map[string][]Person) {
	ps, ok := people[g.HouseholdID]
	if !ok {
		var err error
		if ps, err = s.members(ctx, g.HouseholdID); err != nil {
			s.log.Warn("falha ao listar o casal na reconciliação de hábitos", "household", g.HouseholdID, "err", err)
			return
		}
		people[g.HouseholdID] = ps
	}
	if err := s.syncHabit(ctx, g, ps); err != nil {
		s.log.Warn("falha ao sincronizar o lembrete do hábito", "goal", g.ID, "err", err)
	}
}

// ReconcileSteps é o job de hora em hora: cria as tarefas das etapas que ficaram sem elas (sync_pending).
func (s *Service) ReconcileSteps(ctx context.Context) (int64, error) {
	var n int64
	people := map[string][]Person{}
	after := firstID
	for {
		pending, err := s.repo.PendingSteps(ctx, after, jobPage)
		if err != nil {
			return n, err
		}
		for _, ps := range pending {
			after = ps.StepID
			if s.reconcileStep(ctx, ps, people) {
				n++
			}
		}
		if len(pending) < jobPage {
			return n, nil
		}
	}
}

func (s *Service) reconcileStep(ctx context.Context, ps PendingStep, people map[string][]Person) bool {
	members, ok := people[ps.HouseholdID]
	if !ok {
		var err error
		if members, err = s.members(ctx, ps.HouseholdID); err != nil {
			s.log.Warn("falha ao listar o casal na reconciliação de etapas", "household", ps.HouseholdID, "err", err)
			return false
		}
		people[ps.HouseholdID] = members
	}
	// quem criou a meta pode já ter saído do casal: a tarefa precisa de alguém que ainda seja do casal
	actor := ps.CreatedBy
	if _, ok := person(members, actor); !ok {
		if len(members) == 0 {
			return false
		}
		actor = members[0].ID
	}
	g, err := s.repo.GetGoalSystem(ctx, ps.HouseholdID, ps.GoalID)
	if err != nil {
		return false
	}
	st, err := s.repo.GetStep(ctx, ps.HouseholdID, ps.GoalID, ps.StepID)
	if err != nil {
		return false
	}
	return s.ensureTask(ctx, actor, g, st, members)
}

// ModuleChanged é chamado depois de o módulo ser ligado ou desligado. Desligado, nenhum lembrete de Metas existe
// para o casal; religado, os lembretes de hoje voltam. As metas ficam.
func (s *Service) ModuleChanged(ctx context.Context, householdID string, enabled bool) {
	habits, err := s.repo.HabitsOfHousehold(ctx, householdID)
	if err != nil {
		s.log.Warn("falha ao listar os hábitos ao mudar o módulo Metas", "household", householdID, "err", err)
		return
	}
	ps, err := s.members(ctx, householdID)
	if err != nil {
		s.log.Warn("falha ao listar o casal ao mudar o módulo Metas", "household", householdID, "err", err)
		return
	}
	for _, g := range habits {
		if enabled {
			s.syncHabitFor(ctx, g, ps)
		} else {
			s.cancelHabitDays(ctx, g)
		}
	}
}

// ---- exportação ----

// ExportGoal é uma meta com tudo o que a pessoa pode ver dela.
type ExportGoal struct {
	Goal          Goal           `json:"goal"`
	Contributions []Contribution `json:"contributions,omitempty"`
	Checkins      []Checkin      `json:"checkins,omitempty"`
}

type Export struct {
	ExportedAt time.Time    `json:"exported_at"`
	Goals      []ExportGoal `json:"goals"`
}

// Export reúne as metas que a pessoa pode ver. Quem só enxerga uma meta pessoal "view" recebe o resumo dela
// (título, prazo e progresso), sem notas, aportes, marcações nem etapas.
func (s *Service) Export(ctx context.Context, p identity.Principal) (Export, error) {
	goals, err := s.repo.ListGoals(ctx, p.HouseholdID, p.UserID, ListFilter{Status: "all"})
	if err != nil {
		return Export{}, err
	}
	goals, err = s.decorate(ctx, p, goals, true)
	if err != nil {
		return Export{}, err
	}
	var contribIDs, markIDs []string
	for _, g := range goals {
		if g.Access != AccessFull {
			continue
		}
		switch g.Kind {
		case KindFinancial:
			contribIDs = append(contribIDs, g.ID)
		case KindHabit, KindShared:
			markIDs = append(markIDs, g.ID)
		}
	}
	contribs, err := s.repo.AllContributions(ctx, p.HouseholdID, p.UserID, contribIDs)
	if err != nil {
		return Export{}, err
	}
	marks, err := s.repo.Checkins(ctx, p.HouseholdID, p.UserID, markIDs, time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2200, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		return Export{}, err
	}
	out := Export{ExportedAt: s.now().UTC(), Goals: make([]ExportGoal, 0, len(goals))}
	for _, g := range goals {
		eg := ExportGoal{Goal: g}
		if g.Access == AccessFull {
			eg.Contributions = contribs[g.ID]
			eg.Checkins = marks[g.ID]
		}
		out.Goals = append(out.Goals, eg)
	}
	return out, nil
}

// WriteCSV escreve a exportação em CSV: uma linha por meta, aporte, marcação e etapa.
func (e Export) WriteCSV(w io.Writer) error {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"record", "goal_id", "kind", "scope", "status", "title", "date", "user_id", "amount_cents", "text"}); err != nil {
		return err
	}
	for _, eg := range e.Goals {
		g := eg.Goal
		due := ""
		if g.DueOn != nil {
			due = *g.DueOn
		}
		rows := [][]string{{"goal", g.ID, g.Kind, g.Scope, g.Status, g.Title, due, "", "", g.Notes}}
		for _, c := range eg.Contributions {
			rows = append(rows, []string{"contribution", g.ID, g.Kind, g.Scope, g.Status, g.Title, c.MadeOn, c.CreatedBy, strconv.FormatInt(c.AmountCents, 10), c.Note})
		}
		for _, m := range eg.Checkins {
			rows = append(rows, []string{"checkin", g.ID, g.Kind, g.Scope, g.Status, g.Title, m.Day, m.UserID, "", ""})
		}
		for _, st := range g.Steps {
			state := "open"
			if st.Done {
				state = "done"
			}
			stDue := ""
			if st.DueOn != nil {
				stDue = *st.DueOn
			}
			rows = append(rows, []string{"step", g.ID, g.Kind, g.Scope, g.Status, g.Title, stDue, fmt.Sprint(deref(st.AssigneeID)), "", state + ": " + st.Title})
		}
		for _, r := range rows {
			if err := cw.Write(csvSafe(r)); err != nil {
				return err
			}
		}
	}
	cw.Flush()
	return cw.Error()
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// csvSafe neutraliza células que uma planilha leria como fórmula (título e notas vêm de quem digitou).
func csvSafe(row []string) []string {
	out := make([]string, len(row))
	for i, c := range row {
		if len(c) > 0 && (c[0] == '=' || c[0] == '+' || c[0] == '-' || c[0] == '@' || c[0] == '\t' || c[0] == '\r') && !isNumber(c) {
			c = "'" + c
		}
		out[i] = c
	}
	return out
}

func isNumber(s string) bool {
	_, err := strconv.ParseInt(s, 10, 64)
	return err == nil
}
