package goal

import (
	"context"
	"sort"
	"time"

	"casal/internal/identity"
)

func stepRef(goalID, stepID string) string { return "goal:" + goalID + ":step:" + stepID }

func habitRef(goalID string, day time.Time) string { return "goal:" + goalID + ":" + fmtDay(day) }

// viewData é o que se lê do banco de uma vez para calcular o progresso de várias metas.
type viewData struct {
	sums      map[string]int64
	marks     map[string][]Checkin
	states    map[string]TaskState
	financeOn bool
	members   []Person
	spend     map[string]spendData // por categoria: uma leitura de Finanças serve a todas as metas dela
}

type spendData struct {
	limit int64
	has   bool
	spent map[string]int64
	err   error
}

// decorate calcula o progresso de cada meta (só aqui, no servidor) e tira de quem apenas enxerga tudo o que não é
// título, prazo e progresso. detail inclui as etapas e o histórico recente.
func (s *Service) decorate(ctx context.Context, p identity.Principal, goals []Goal, detail bool) ([]Goal, error) {
	if len(goals) == 0 {
		return goals, nil
	}
	now := s.now()
	todayV := today(now, p.Timezone)
	d := viewData{}
	var contribIDs, markIDs []string
	var refs []string
	needFinance := false
	earliest := map[string]string{} // categoria -> baseline mais antiga
	for _, g := range goals {
		switch g.Kind {
		case KindFinancial:
			if g.Financial.Source == SourceContributions {
				contribIDs = append(contribIDs, g.ID)
			} else {
				needFinance = true
				if f := g.Financial; f.CategoryID != nil {
					if cur, ok := earliest[*f.CategoryID]; !ok || f.BaselineOn < cur {
						earliest[*f.CategoryID] = f.BaselineOn
					}
				}
			}
		case KindHabit, KindShared:
			markIDs = append(markIDs, g.ID)
		case KindProject:
			for _, st := range g.Steps {
				if st.wantTask && st.taskID != "" {
					refs = append(refs, stepRef(g.ID, st.ID))
				}
			}
		}
	}
	var err error
	if d.members, err = s.members(ctx, p.HouseholdID); err != nil {
		return nil, err
	}
	if d.sums, err = s.repo.SumContributions(ctx, p.HouseholdID, p.UserID, contribIDs); err != nil {
		return nil, err
	}
	// as marcações cobrem o mês corrente e uma folga de dias nas duas pontas (fusos e semanas que viram o mês)
	first := time.Date(todayV.Year(), todayV.Month(), 1, 0, 0, 0, 0, time.UTC)
	if d.marks, err = s.repo.Checkins(ctx, p.HouseholdID, p.UserID, markIDs, first.AddDate(0, 0, -8), first.AddDate(0, 1, 8)); err != nil {
		return nil, err
	}
	if d.states, err = s.tasks.StatesByRef(ctx, p.HouseholdID, refs); err != nil {
		return nil, err
	}
	if needFinance {
		if d.financeOn, err = s.modules.IsEnabled(ctx, p.HouseholdID, moduleFinance); err != nil {
			return nil, err
		}
		d.spend = map[string]spendData{}
		if d.financeOn {
			// o mês inteiro conta: o gasto de antes do baseline, no mesmo mês, também come o limite
			for cat, base := range earliest {
				limit, has, spent, serr := s.spending.MonthlySpent(ctx, p.HouseholdID, cat, monthStart(mustDay(base)), todayV)
				d.spend[cat] = spendData{limit, has, spent, serr}
			}
		}
	}

	out := make([]Goal, len(goals))
	for i, g := range goals {
		s.fill(ctx, p, &g, d, now)
		if detail && g.Access == AccessFull {
			s.recent(ctx, p, &g, now)
		}
		if g.Access != AccessFull {
			restrict(&g)
		} else if !detail {
			g.Steps = nil
		}
		out[i] = g
	}
	return out, nil
}

// restrict deixa a quem só enxerga título, prazo e progresso: sem notas, parâmetros, etapas, aportes nem marcações.
func restrict(g *Goal) {
	g.Notes = ""
	g.Financial, g.Habit, g.Shared, g.Steps, g.Recent = nil, nil, nil, nil, nil
	g.Progress.MarkedToday = false
	g.Progress.PendingDays = nil
	g.Progress.Period = nil
}

func (s *Service) fill(ctx context.Context, p identity.Principal, g *Goal, d viewData, now time.Time) {
	todayV := today(now, p.Timezone)
	active := g.Status == StatusActive || g.Status == StatusPaused
	switch g.Kind {
	case KindFinancial:
		f := g.Financial
		saved := d.sums[g.ID]
		pr := Progress{Total: f.TargetCents, Unit: "cents"}
		if f.Source == SourceBudgetSavings {
			saved = 0
			switch {
			case !d.financeOn:
				pr.Unavailable = true
			case f.CategoryID == nil:
				pr.Unavailable = true
			default:
				base := mustDay(f.BaselineOn)
				sd := d.spend[*f.CategoryID]
				if sd.err != nil || !sd.has {
					if sd.err != nil {
						s.log.Warn("falha ao ler o orçamento da meta", "goal", g.ID, "err", sd.err)
					}
					pr.Unavailable = true
				} else {
					saved, pr.Provisional = budgetSavings(sd.limit, sd.spent, base, todayV)
				}
			}
		}
		pr.Done = saved
		pr.Percent = min(100, percent(saved, f.TargetCents))
		pr.SuggestComplete = active && !pr.Unavailable && saved >= f.TargetCents
		if g.DueOn != nil && !pr.Unavailable {
			required, onTrack := pace(f.TargetCents, saved, mustDay(f.BaselineOn), todayV, mustDay(*g.DueOn))
			if saved < f.TargetCents {
				pr.RequiredPerMonthCents = &required
			}
			pr.OnTrack = &onTrack
		}
		g.Progress = pr
	case KindHabit:
		h := g.Habit
		owner := ""
		if g.OwnerID != nil {
			owner = *g.OwnerID
		}
		tz := p.Timezone
		if pe, ok := person(d.members, owner); ok && pe.Timezone != "" {
			tz = pe.Timezone
		}
		day := today(now, tz)
		start, end := periodOf(h.Period, day)
		marks := d.marks[g.ID]
		pr := Progress{Total: int64(h.TargetCount), Unit: "count", Period: rangeOf(start, end)}
		pr.Done = int64(habitDays(marks, owner, h.Weekdays, start, end))
		pr.Percent = percent(pr.Done, pr.Total)
		for _, m := range marks {
			if m.UserID == p.UserID && m.Day == fmtDay(day) && owner == p.UserID {
				pr.MarkedToday = true
			}
		}
		g.Progress = pr
	case KindShared:
		sh := g.Shared
		members := map[string]bool{}
		for _, m := range d.members {
			members[m.ID] = true
		}
		required := 1
		if sh.RequiresBoth {
			required = max(1, min(2, len(members)))
		}
		start, end := periodOf(sh.Period, todayV)
		count, pending := sharedDays(d.marks[g.ID], required, members, start, end)
		pr := Progress{Done: int64(count), Total: int64(sh.TargetCount), Unit: "count", Period: rangeOf(start, end), PendingDays: pending}
		pr.Percent = percent(pr.Done, pr.Total)
		for _, m := range d.marks[g.ID] {
			if m.UserID == p.UserID && m.Day == fmtDay(todayV) {
				pr.MarkedToday = true
			}
		}
		g.Progress = pr
	case KindProject:
		g.Steps, g.SyncPending = stepViews(g, d.states)
		var done int64
		for _, st := range g.Steps {
			if st.Done {
				done++
			}
		}
		total := int64(len(g.Steps))
		g.Progress = Progress{Done: done, Total: total, Unit: "steps", Percent: percent(done, total),
			SuggestComplete: active && total > 0 && done == total}
	}
}

// stepViews monta as etapas como a pessoa as vê. Com tarefa, o estado e o responsável são DA TAREFA (um único
// lugar); sem tarefa, a etapa é um item de checklist. pending é verdadeiro se alguma etapa ainda espera a tarefa.
func stepViews(g *Goal, states map[string]TaskState) (out []Step, pending bool) {
	out = make([]Step, 0, len(g.Steps))
	for _, st := range g.Steps {
		v := st
		v.Done, v.DoneAt = st.rawDone != nil, st.rawDone
		if st.wantTask {
			state, ok := states[stepRef(g.ID, st.ID)]
			switch {
			case st.taskID == "":
				pending = true // a meta foi gravada, a tarefa ainda será criada pelo job
				v.Done, v.DoneAt = false, nil
			case ok && state.Status != "canceled":
				v.HasTask, v.TaskStatus = true, state.Status
				id := st.taskID
				v.TaskID = &id
				v.Done, v.DoneAt = state.Status == "done", state.CompletedAt
				v.AssigneeID = state.AssigneeID
				if state.Status == "declined" { // recusada: a etapa fica aberta e sem responsável
					v.AssigneeID = nil
				}
			default: // tarefa cancelada por fora: a etapa continua, aberta e sem tarefa
				v.Done, v.DoneAt = false, nil
			}
		}
		out = append(out, v)
	}
	return out, pending
}

// recent junta o que a tela de uma meta mostra além do progresso: aportes e marcações recentes.
func (s *Service) recent(ctx context.Context, p identity.Principal, g *Goal, now time.Time) {
	rec := &Recent{}
	switch g.Kind {
	case KindFinancial:
		if g.Financial.Source == SourceContributions {
			cs, err := s.repo.ListContributions(ctx, g.HouseholdID, p.UserID, g.ID, "", 10)
			if err == nil {
				rec.Contributions = cs
			}
		}
	case KindHabit, KindShared:
		todayV := today(now, p.Timezone)
		marks, err := s.repo.Checkins(ctx, g.HouseholdID, p.UserID, []string{g.ID}, todayV.AddDate(0, 0, -35), todayV.AddDate(0, 0, 2))
		if err == nil {
			rec.Checkins = s.dayMarks(ctx, g, marks[g.ID])
		}
	}
	if rec.Contributions != nil || rec.Checkins != nil {
		g.Recent = rec
	}
}

// dayMarks agrupa as marcações por dia. A ordem das pessoas em cada dia é a do id: não significa nada.
func (s *Service) dayMarks(ctx context.Context, g *Goal, marks []Checkin) []DayMarks {
	members := map[string]bool{}
	if ps, err := s.members(ctx, g.HouseholdID); err == nil {
		for _, m := range ps {
			members[m.ID] = true
		}
	}
	required := 1
	if g.Shared != nil && g.Shared.RequiresBoth {
		required = max(1, min(2, len(members)))
	}
	byDay := map[string][]string{}
	for _, m := range marks {
		byDay[m.Day] = append(byDay[m.Day], m.UserID)
	}
	days := make([]string, 0, len(byDay))
	for d := range byDay {
		days = append(days, d)
	}
	sort.Strings(days)
	out := make([]DayMarks, 0, len(days))
	for _, d := range days {
		who := byDay[d]
		sort.Strings(who)
		dm := DayMarks{Day: d, By: who}
		day := mustDay(d)
		switch g.Kind {
		case KindHabit:
			dm.Counted = dayCounts(g.Habit.Weekdays, day)
		case KindShared:
			n := 0
			for _, u := range who {
				if members[u] {
					n++
				}
			}
			dm.Counted = n >= required
		}
		out = append(out, dm)
	}
	return out
}
