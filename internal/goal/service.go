// Package goal é o módulo Metas: financeira, hábito, do casal e projeto. O progresso é sempre calculado aqui, no
// servidor. A meta pessoal é privada entre os parceiros e essa trava vive só no código (o RLS não separa os dois do
// mesmo casal): todo SQL de leitura usa visibleTo. Nenhuma resposta ou aviso compara, ordena ou cobra as duas pessoas.
package goal

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"casal/internal/identity"
)

// TaskState é o estado de uma tarefa lido pelo ref. Ref sem tarefa não aparece no mapa de StatesByRef.
type TaskState struct {
	Status      string // open, done, declined ou canceled
	AssigneeID  *string
	CompletedAt *time.Time
}

// Tasks é o que Metas precisa da feature task (o app liga as duas).
type Tasks interface {
	// Create cria a tarefa do casal ou do responsável, marcada com o ref; o mesmo ref devolve a que já existe.
	Create(ctx context.Context, householdID, creatorID, assigneeID, title string, dueAt *time.Time, ref string) (taskID string, err error)
	CancelByRef(ctx context.Context, householdID, ref string) error
	StatesByRef(ctx context.Context, householdID string, refs []string) (map[string]TaskState, error)
	// UpdateByRef muda título e prazo da tarefa aberta (a etapa foi editada).
	UpdateByRef(ctx context.Context, householdID, ref, title string, dueAt *time.Time) error
}

// Reminders é o que Metas precisa da feature reminder.
type Reminders interface {
	CreateWithRef(ctx context.Context, householdID, creatorID, recipientID, body string, remindAt time.Time, ref string) error
	CancelByRef(ctx context.Context, householdID, ref string) error
}

// People é o que Metas precisa da feature household.
type People interface {
	Members(ctx context.Context, householdID string) ([]Person, error)
}

// Notifier envia um aviso a uma pessoa. Nunca é chamado dentro de uma transação.
type Notifier interface {
	Notify(ctx context.Context, userID, title, body string) error
}

// Spending é o que Metas lê de Finanças (adaptador no app): o limite atual do orçamento da categoria e o gasto
// do CASAL por mês (AAAA-MM) entre from e to. Gasto pessoal nunca entra. hasLimit falso: categoria sem limite.
type Spending interface {
	MonthlySpent(ctx context.Context, householdID, categoryID string, from, to time.Time) (limitCents int64, hasLimit bool, spent map[string]int64, err error)
}

// Modules diz se um módulo está ligado para o casal.
type Modules interface {
	IsEnabled(ctx context.Context, householdID, name string) (bool, error)
}

const (
	moduleFinance = "finance"
	firstID       = "00000000-0000-0000-0000-000000000000"
)

type Service struct {
	repo      Repository
	tasks     Tasks
	reminders Reminders
	people    People
	notifier  Notifier
	spending  Spending
	modules   Modules
	log       *slog.Logger
	now       func() time.Time
}

func NewService(repo Repository, tasks Tasks, reminders Reminders, people People, notifier Notifier,
	spending Spending, modules Modules, log *slog.Logger) *Service {
	return &Service{repo: repo, tasks: tasks, reminders: reminders, people: people, notifier: notifier,
		spending: spending, modules: modules, log: log, now: time.Now}
}

func (s *Service) members(ctx context.Context, hh string) ([]Person, error) {
	return s.people.Members(ctx, hh)
}

func person(ps []Person, id string) (Person, bool) {
	for _, p := range ps {
		if p.ID == id {
			return p, true
		}
	}
	return Person{}, false
}

func otherThan(ps []Person, me string) (Person, bool) {
	for _, p := range ps {
		if p.ID != me {
			return p, true
		}
	}
	return Person{}, false
}

func (s *Service) notify(ctx context.Context, userID, title, body string) {
	if userID == "" {
		return
	}
	if err := s.notifier.Notify(ctx, userID, title, body); err != nil {
		s.log.Warn("falha ao avisar", "user", userID, "title", title, "err", err)
	}
}

func cleanTitle(v string, max int, what string) (string, error) {
	v = strings.TrimSpace(v)
	if v == "" || utf8.RuneCountInString(v) > max {
		return "", fmt.Errorf("%w: %s tem de 1 a %d caracteres", ErrInvalid, what, max)
	}
	return v, nil
}

func cleanNotes(v string) (string, error) {
	v = strings.TrimSpace(v)
	if utf8.RuneCountInString(v) > 1000 {
		return "", fmt.Errorf("%w: as notas têm até 1000 caracteres", ErrInvalid)
	}
	return v, nil
}

func validAmount(c int64) bool { return c > 0 && c <= MaxAmountCents }

func validTarget(count int, period string) error {
	max := 31
	if period == PeriodWeek {
		max = 7
	}
	if count < 1 || count > max {
		return fmt.Errorf("%w: a quantidade por %s vai de 1 a %d", ErrInvalid, map[string]string{PeriodWeek: "semana", PeriodMonth: "mês"}[period], max)
	}
	return nil
}

func validPeriod(p string) bool { return p == PeriodWeek || p == PeriodMonth }

// ---- criar ----

func (s *Service) Create(ctx context.Context, p identity.Principal, in CreateInput) (Goal, error) {
	now := s.now()
	todayD := today(now, p.Timezone)

	title, err := cleanTitle(in.Title, 120, "o título")
	if err != nil {
		return Goal{}, err
	}
	notes, err := cleanNotes(in.Notes)
	if err != nil {
		return Goal{}, err
	}
	g := Goal{HouseholdID: p.HouseholdID, CreatedBy: p.UserID, Kind: in.Kind, Scope: in.Scope, Title: title, Notes: notes}
	if in.DueOn != "" {
		d, err := parseDay(in.DueOn)
		if err != nil {
			return Goal{}, err
		}
		if d.Before(todayD) {
			return Goal{}, fmt.Errorf("%w: o prazo não pode estar no passado", ErrInvalid)
		}
		g.DueOn = &in.DueOn
	}

	switch in.Scope {
	case ScopeHousehold:
		g.Visibility = VisibilityHousehold
		if in.Visibility != "" && in.Visibility != VisibilityHousehold {
			return Goal{}, fmt.Errorf("%w: meta do casal é dos dois", ErrInvalid)
		}
	case ScopePersonal:
		owner := p.UserID
		g.OwnerID = &owner
		g.Visibility = in.Visibility
		if g.Visibility == "" {
			g.Visibility = VisibilityPrivate
		}
		if g.Visibility != VisibilityPrivate && g.Visibility != VisibilityView {
			return Goal{}, fmt.Errorf("%w: visibilidade deve ser private ou view", ErrInvalid)
		}
	default:
		return Goal{}, fmt.Errorf("%w: scope deve ser household ou personal", ErrInvalid)
	}

	// só o bloco do tipo escolhido
	blocks := 0
	for _, present := range []bool{in.Financial != nil, in.Habit != nil, in.Shared != nil} {
		if present {
			blocks++
		}
	}
	if blocks > 1 || (blocks == 1 && in.Kind == KindProject) || (len(in.Steps) > 0 && in.Kind != KindProject) {
		return Goal{}, fmt.Errorf("%w: envie só os dados do tipo da meta", ErrInvalid)
	}

	ps, err := s.members(ctx, p.HouseholdID)
	if err != nil {
		return Goal{}, err
	}
	var steps []NewStep

	switch in.Kind {
	case KindFinancial:
		f, err := s.financialBlock(ctx, p, in, todayD)
		if err != nil {
			return Goal{}, err
		}
		g.Financial = f
		if g.DueOn != nil {
			if due, _ := parseDay(*g.DueOn); due.Before(mustDay(f.BaselineOn)) {
				return Goal{}, fmt.Errorf("%w: o prazo vem antes do início", ErrInvalid)
			}
		}
	case KindHabit:
		// só o dono marca um hábito: ele precisa ter dono
		if in.Habit == nil || in.Scope != ScopePersonal {
			return Goal{}, fmt.Errorf("%w: hábito é uma meta pessoal e precisa do bloco habit", ErrInvalid)
		}
		h := in.Habit
		if !validPeriod(h.Period) {
			return Goal{}, fmt.Errorf("%w: period deve ser week ou month", ErrInvalid)
		}
		if err := validTarget(h.TargetCount, h.Period); err != nil {
			return Goal{}, err
		}
		if h.Weekdays < 0 || h.Weekdays > 127 {
			return Goal{}, fmt.Errorf("%w: weekdays é uma máscara de 0 a 127", ErrInvalid)
		}
		remind := DefaultRemindMin
		if h.RemindMin != nil {
			remind = *h.RemindMin
		}
		if remind < 0 || remind > 1439 {
			return Goal{}, fmt.Errorf("%w: remind_min vai de 0 a 1439", ErrInvalid)
		}
		g.Habit = &Habit{TargetCount: h.TargetCount, Period: h.Period, Weekdays: h.Weekdays, RemindMin: remind, RemindEnabled: h.RemindEnabled}
	case KindShared:
		if in.Shared == nil || in.Scope != ScopeHousehold {
			return Goal{}, fmt.Errorf("%w: meta do casal é do casal e precisa do bloco shared", ErrInvalid)
		}
		sh := in.Shared
		if !validPeriod(sh.Period) {
			return Goal{}, fmt.Errorf("%w: period deve ser week ou month", ErrInvalid)
		}
		if err := validTarget(sh.TargetCount, sh.Period); err != nil {
			return Goal{}, err
		}
		g.Shared = &Shared{TargetCount: sh.TargetCount, Period: sh.Period, RequiresBoth: sh.RequiresBoth}
	case KindProject:
		if len(in.Steps) > MaxSteps {
			return Goal{}, ErrLimit
		}
		for _, si := range in.Steps {
			ns, err := s.newStep(si, g, ps)
			if err != nil {
				return Goal{}, err
			}
			steps = append(steps, ns)
		}
	default:
		return Goal{}, fmt.Errorf("%w: kind deve ser financial, habit, shared ou project", ErrInvalid)
	}

	created, err := s.repo.CreateGoal(ctx, NewGoal{Goal: g, Steps: steps})
	if err != nil {
		return Goal{}, err
	}
	pending := s.createStepTasks(ctx, p, created)
	if created.Kind == KindHabit {
		s.syncHabitFor(ctx, created, ps)
	}
	if pending || created.Kind == KindProject { // as etapas ganharam o id da tarefa depois de gravadas
		if created, err = s.repo.GetGoal(ctx, p.HouseholdID, p.UserID, created.ID); err != nil {
			return Goal{}, err
		}
	}
	out, err := s.present(ctx, p, created, true)
	out.SyncPending = out.SyncPending || pending
	return out, err
}

func mustDay(s string) time.Time {
	t, _ := parseDay(s)
	return t
}

func (s *Service) financialBlock(ctx context.Context, p identity.Principal, in CreateInput, todayD time.Time) (*Financial, error) {
	if in.Financial == nil {
		return nil, fmt.Errorf("%w: falta o bloco financial", ErrInvalid)
	}
	fi := in.Financial
	if !validAmount(fi.TargetCents) {
		return nil, ErrAmountInvalid
	}
	f := &Financial{TargetCents: fi.TargetCents, Source: fi.Source, BaselineOn: fmtDay(todayD)}
	if f.Source == "" {
		f.Source = SourceContributions
	}
	if fi.BaselineOn != "" {
		d, err := parseDay(fi.BaselineOn)
		if err != nil {
			return nil, err
		}
		if d.Before(todayD.AddDate(-5, 0, 0)) {
			return nil, fmt.Errorf("%w: o início não pode ser de mais de 5 anos atrás", ErrInvalid)
		}
		f.BaselineOn = fmtDay(d)
	}
	switch f.Source {
	case SourceContributions:
		if fi.CategoryID != "" {
			return nil, fmt.Errorf("%w: category_id é só para budget_savings", ErrInvalid)
		}
	case SourceBudgetSavings:
		// gasto pessoal não entra em totais do casal: a sobra do orçamento só vale para meta do casal
		if in.Scope != ScopeHousehold {
			return nil, fmt.Errorf("%w: economia do orçamento é uma meta do casal", ErrInvalid)
		}
		if on, err := s.modules.IsEnabled(ctx, p.HouseholdID, moduleFinance); err != nil {
			return nil, err
		} else if !on {
			return nil, ErrModuleRequired
		}
		if fi.CategoryID == "" {
			return nil, fmt.Errorf("%w: informe a categoria do orçamento", ErrInvalid)
		}
		_, has, _, err := s.spending.MonthlySpent(ctx, p.HouseholdID, fi.CategoryID, todayD, todayD)
		if err != nil {
			return nil, err
		}
		if !has {
			return nil, fmt.Errorf("%w: a categoria precisa ter um limite de orçamento", ErrInvalid)
		}
		c := fi.CategoryID
		f.CategoryID = &c
	default:
		return nil, fmt.Errorf("%w: source deve ser contributions ou budget_savings", ErrInvalid)
	}
	return f, nil
}

// newStep valida uma etapa nova: título, responsável (do casal; na meta pessoal, só o dono) e prazo.
func (s *Service) newStep(si StepInput, g Goal, ps []Person) (NewStep, error) {
	title, err := cleanTitle(si.Title, 200, "o título da etapa")
	if err != nil {
		return NewStep{}, err
	}
	ns := NewStep{Title: title}
	if si.AssigneeID != "" {
		if g.Scope == ScopePersonal {
			if g.OwnerID == nil || si.AssigneeID != *g.OwnerID {
				return NewStep{}, fmt.Errorf("%w: numa meta pessoal a etapa é só do dono", ErrInvalid)
			}
		} else if _, ok := person(ps, si.AssigneeID); !ok {
			return NewStep{}, fmt.Errorf("%w: a pessoa não pertence ao casal", ErrInvalid)
		}
		a := si.AssigneeID
		ns.AssigneeID = &a
	}
	if si.DueOn != "" {
		d, err := parseDay(si.DueOn)
		if err != nil {
			return NewStep{}, err
		}
		due := fmtDay(d)
		ns.DueOn = &due
	}
	if si.Task {
		// as tarefas são do casal e aparecem aos dois: meta pessoal não gera tarefa, suas etapas são checklist
		if g.Scope != ScopeHousehold {
			return NewStep{}, fmt.Errorf("%w: meta pessoal não gera tarefa", ErrInvalid)
		}
		ns.WantTask = true
	}
	return ns, nil
}

// ---- ler ----

func (s *Service) List(ctx context.Context, p identity.Principal, f ListFilter) ([]Goal, error) {
	switch f.Scope {
	case "", ScopeHousehold, "mine":
	default:
		return nil, fmt.Errorf("%w: scope deve ser household ou mine", ErrInvalid)
	}
	switch f.Kind {
	case "", KindFinancial, KindHabit, KindShared, KindProject:
	default:
		return nil, fmt.Errorf("%w: kind inválido", ErrInvalid)
	}
	switch f.Status {
	case "", "all", StatusActive, StatusPaused, StatusDone, StatusArchived:
	default:
		return nil, fmt.Errorf("%w: status inválido", ErrInvalid)
	}
	goals, err := s.repo.ListGoals(ctx, p.HouseholdID, p.UserID, f)
	if err != nil {
		return nil, err
	}
	return s.decorate(ctx, p, goals, false)
}

func (s *Service) Get(ctx context.Context, p identity.Principal, id string) (Goal, error) {
	g, err := s.repo.GetGoal(ctx, p.HouseholdID, p.UserID, id)
	if err != nil {
		return Goal{}, err
	}
	return s.present(ctx, p, g, true)
}

// present decora uma meta só (leitura completa ou resumo de lista).
func (s *Service) present(ctx context.Context, p identity.Principal, g Goal, detail bool) (Goal, error) {
	out, err := s.decorate(ctx, p, []Goal{g}, detail)
	if err != nil {
		return Goal{}, err
	}
	return out[0], nil
}

// ---- editar ----

func (s *Service) Update(ctx context.Context, p identity.Principal, id string, in UpdateInput) (Goal, error) {
	cur, err := s.repo.GetGoal(ctx, p.HouseholdID, p.UserID, id)
	if err != nil {
		return Goal{}, err
	}
	if !cur.CanEdit {
		return Goal{}, ErrForbidden
	}
	if in.Version <= 0 {
		return Goal{}, fmt.Errorf("%w: informe a version lida", ErrInvalid)
	}
	title, err := cleanTitle(in.Title, 120, "o título")
	if err != nil {
		return Goal{}, err
	}
	notes, err := cleanNotes(in.Notes)
	if err != nil {
		return Goal{}, err
	}
	patch := GoalPatch{Version: in.Version, Title: title, Notes: notes, Visibility: cur.Visibility}
	if in.DueOn != "" {
		d, err := parseDay(in.DueOn)
		if err != nil {
			return Goal{}, err
		}
		if (cur.DueOn == nil || *cur.DueOn != in.DueOn) && d.Before(today(s.now(), p.Timezone)) {
			return Goal{}, fmt.Errorf("%w: o prazo não pode estar no passado", ErrInvalid)
		}
		if cur.Financial != nil && d.Before(mustDay(cur.Financial.BaselineOn)) {
			return Goal{}, fmt.Errorf("%w: o prazo vem antes do início", ErrInvalid)
		}
		due := fmtDay(d)
		patch.DueOn = &due
	}
	if in.Visibility != "" && in.Visibility != cur.Visibility {
		if cur.Scope != ScopePersonal || (in.Visibility != VisibilityPrivate && in.Visibility != VisibilityView) {
			return Goal{}, fmt.Errorf("%w: só a meta pessoal muda de visibilidade (private ou view)", ErrInvalid)
		}
		patch.Visibility = in.Visibility
	}

	habitChanged := false
	if in.Financial != nil {
		if cur.Kind != KindFinancial {
			return Goal{}, ErrKindMismatch
		}
		if !validAmount(in.Financial.TargetCents) {
			return Goal{}, ErrAmountInvalid
		}
		t := in.Financial.TargetCents
		patch.TargetCents = &t
	}
	if in.Habit != nil {
		if cur.Kind != KindHabit {
			return Goal{}, ErrKindMismatch
		}
		h := in.Habit
		if err := validTarget(h.TargetCount, cur.Habit.Period); err != nil {
			return Goal{}, err
		}
		if h.Weekdays < 0 || h.Weekdays > 127 {
			return Goal{}, fmt.Errorf("%w: weekdays é uma máscara de 0 a 127", ErrInvalid)
		}
		remind := cur.Habit.RemindMin
		if h.RemindMin != nil {
			remind = *h.RemindMin
		}
		if remind < 0 || remind > 1439 {
			return Goal{}, fmt.Errorf("%w: remind_min vai de 0 a 1439", ErrInvalid)
		}
		patch.Habit = &Habit{TargetCount: h.TargetCount, Period: cur.Habit.Period, Weekdays: h.Weekdays, RemindMin: remind, RemindEnabled: h.RemindEnabled}
		habitChanged = true
	}
	if in.Shared != nil {
		if cur.Kind != KindShared {
			return Goal{}, ErrKindMismatch
		}
		if err := validTarget(in.Shared.TargetCount, cur.Shared.Period); err != nil {
			return Goal{}, err
		}
		patch.Shared = &Shared{TargetCount: in.Shared.TargetCount, Period: cur.Shared.Period, RequiresBoth: in.Shared.RequiresBoth}
	}

	if err := s.repo.UpdateGoal(ctx, p.HouseholdID, p.UserID, id, patch); err != nil {
		return Goal{}, err
	}
	out, err := s.repo.GetGoal(ctx, p.HouseholdID, p.UserID, id)
	if err != nil {
		return Goal{}, err
	}
	if habitChanged {
		// o lembrete do dia pode estar no horário velho: refaz
		s.cancelHabitToday(ctx, out)
		s.syncHabitFor(ctx, out, nil)
	}
	return s.present(ctx, p, out, true)
}

// ---- estados ----

func (s *Service) Pause(ctx context.Context, p identity.Principal, id string) (Goal, error) {
	return s.transition(ctx, p, id, StatusPaused, []string{StatusActive})
}

// Resume retoma uma meta pausada ou reabre uma concluída (limpa quem concluiu e quando).
func (s *Service) Resume(ctx context.Context, p identity.Principal, id string) (Goal, error) {
	return s.transition(ctx, p, id, StatusActive, []string{StatusPaused, StatusDone})
}

func (s *Service) Complete(ctx context.Context, p identity.Principal, id string) (Goal, error) {
	return s.transition(ctx, p, id, StatusDone, []string{StatusActive, StatusPaused})
}

func (s *Service) Archive(ctx context.Context, p identity.Principal, id string) (Goal, error) {
	return s.transition(ctx, p, id, StatusArchived, []string{StatusActive, StatusPaused, StatusDone})
}

func (s *Service) Unarchive(ctx context.Context, p identity.Principal, id string) (Goal, error) {
	return s.transition(ctx, p, id, StatusActive, []string{StatusArchived})
}

// transition muda o estado. Pedir o estado em que a meta já está é idempotente; estados de onde não dá para
// chegar (por exemplo, reabrir uma arquivada por /resume) respondem goal_closed.
func (s *Service) transition(ctx context.Context, p identity.Principal, id, to string, from []string) (Goal, error) {
	cur, err := s.repo.GetGoal(ctx, p.HouseholdID, p.UserID, id)
	if err != nil {
		return Goal{}, err
	}
	if !cur.CanEdit {
		return Goal{}, ErrForbidden
	}
	if cur.Status != to {
		ok := false
		for _, f := range from {
			ok = ok || f == cur.Status
		}
		if !ok {
			return Goal{}, ErrClosed
		}
		if err := s.repo.SetStatus(ctx, p.HouseholdID, p.UserID, id, from, to); err != nil {
			return Goal{}, err
		}
	}
	out, err := s.repo.GetGoal(ctx, p.HouseholdID, p.UserID, id)
	if err != nil {
		return Goal{}, err
	}
	if out.Kind == KindHabit {
		if out.Status == StatusActive {
			s.syncHabitFor(ctx, out, nil)
		} else {
			s.cancelHabitToday(ctx, out) // pausar, concluir e arquivar cancelam o lembrete do dia
		}
	}
	return s.present(ctx, p, out, true)
}

// ---- apagar ----

func (s *Service) Delete(ctx context.Context, p identity.Principal, id string) error {
	cur, err := s.repo.GetGoal(ctx, p.HouseholdID, p.UserID, id)
	if err != nil {
		return err
	}
	if !cur.CanEdit {
		return ErrForbidden
	}
	var stepIDs []string
	if cur.Kind == KindProject {
		if stepIDs, err = s.repo.StepTaskRefs(ctx, p.HouseholdID, id); err != nil {
			return err
		}
	}
	if err := s.repo.DeleteGoal(ctx, p.HouseholdID, p.UserID, id); err != nil {
		return err
	}
	// fora da transação: cancela as tarefas ligadas (as concluídas ficam de histórico, só o ref é solto)
	for _, sid := range stepIDs {
		if err := s.tasks.CancelByRef(ctx, p.HouseholdID, stepRef(id, sid)); err != nil {
			s.log.Warn("falha ao cancelar a tarefa de uma etapa", "goal", id, "step", sid, "err", err)
		}
	}
	if cur.Kind == KindHabit {
		s.cancelHabitDays(ctx, cur)
	}
	return nil
}

// jobPage é o tamanho da página dos jobs, que andam por id até acabar (nada fica de fora). Variável só para o teste.
var jobPage = 500
