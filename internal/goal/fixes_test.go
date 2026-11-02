package goal

import (
	"testing"
	"time"
)

func TestBudgetSavingsReadsTheWholeBaselineMonthOncePerCategory(t *testing.T) {
	e := newEnv(t)
	cat := e.category("Mercado")
	e.spend.limit[cat] = 100000
	e.spend.spent[cat] = map[string]int64{"2026-11": 90000}

	// baseline no meio do mês: o que se gastou antes dele, no mesmo mês, também come o limite
	_, err := e.budgetGoal(cat, "2026-11-10")
	mustNoErr(t, err)
	_, err = e.budgetGoal(cat, "2026-11-01")
	mustNoErr(t, err)
	e.spend.calls, e.spend.froms = 0, nil
	gs, err := e.svc.List(ctx(), e.ana, ListFilter{})
	mustNoErr(t, err)
	for _, g := range gs {
		if g.Progress.Done != 10000 {
			t.Errorf("%s: sobra %d, quero 10000", g.ID, g.Progress.Done)
		}
	}
	if e.spend.calls != 1 {
		t.Errorf("duas metas da mesma categoria leem Finanças %d vezes, quero 1", e.spend.calls)
	}
	if want := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC); len(e.spend.froms) != 1 || !e.spend.froms[0].Equal(want) {
		t.Errorf("a leitura começa no primeiro dia do mês do baseline: %v", e.spend.froms)
	}

	_, err = e.budgetGoal(cat, "2019-01-01")
	wantErr(t, err, ErrInvalid, "baseline de mais de 5 anos")
}

func TestEditingAStepWithATaskUpdatesTheTask(t *testing.T) {
	e := newEnv(t)
	g := e.project(e.ana, ScopeHousehold, "Reforma", StepInput{Title: "Comprar tinta", DueOn: "2026-11-20", Task: true})
	st := g.Steps[0]
	title, due := "Comprar tinta branca", "2026-11-25"
	_, err := e.svc.UpdateStep(ctx(), e.ana, g.ID, st.ID, StepUpdate{Title: &title, DueOn: &due})
	mustNoErr(t, err)
	task := e.tasks.get(ref(g, st))
	if task.Title != "Reforma: Comprar tinta branca" || task.Due == nil || !task.Due.Equal(time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)) {
		t.Errorf("a tarefa deveria acompanhar a etapa: %+v", task)
	}
	empty := ""
	_, err = e.svc.UpdateStep(ctx(), e.ana, g.ID, st.ID, StepUpdate{DueOn: &empty})
	mustNoErr(t, err)
	if e.tasks.get(ref(g, st)).Due != nil {
		t.Error("tirar o prazo da etapa tira o da tarefa")
	}
}

func TestAssigneeOfAStepTaskIsNotified(t *testing.T) {
	e := newEnv(t)
	e.project(e.ana, ScopeHousehold, "Reforma",
		StepInput{Title: "Para o Beto", AssigneeID: e.beto.UserID, Task: true},
		StepInput{Title: "Para mim", AssigneeID: e.ana.UserID, Task: true},
		StepInput{Title: "Do casal", Task: true})
	got := e.notes.to(e.beto.UserID)
	if len(got) != 1 || got[0].Title != "Nova tarefa para você" || got[0].Body != "Reforma: Para o Beto" {
		t.Errorf("aviso ao responsável: %+v", got)
	}
	if len(e.notes.to(e.ana.UserID)) != 0 {
		t.Error("quem criou não é avisado da própria atribuição")
	}
}

func TestClosedGoalsRefuseEditsAndFinancialDueMustFollowBaseline(t *testing.T) {
	e := newEnv(t)
	g := e.saving(e.ana, ScopePersonal, "Reserva", 1000)
	done, err := e.svc.Complete(ctx(), e.ana, g.ID)
	mustNoErr(t, err)
	_, err = e.svc.Update(ctx(), e.ana, g.ID, UpdateInput{Version: done.Version, Title: "outro"})
	wantErr(t, err, ErrClosed, "editar meta concluída")
	_, err = e.svc.Archive(ctx(), e.ana, g.ID)
	mustNoErr(t, err)
	cur, _ := e.svc.Get(ctx(), e.ana, g.ID)
	_, err = e.svc.Update(ctx(), e.ana, g.ID, UpdateInput{Version: cur.Version, Title: "outro"})
	wantErr(t, err, ErrClosed, "editar meta arquivada")
	re, err := e.svc.Unarchive(ctx(), e.ana, g.ID)
	mustNoErr(t, err)
	_, err = e.svc.Update(ctx(), e.ana, g.ID, UpdateInput{Version: re.Version, Title: "reaberta"})
	mustNoErr(t, err)

	// com um início no futuro, um prazo entre hoje e o início é recusado
	fut, err := e.svc.Create(ctx(), e.ana, CreateInput{Kind: KindFinancial, Scope: ScopePersonal, Title: "Futura",
		Financial: &FinancialInput{TargetCents: 1000, BaselineOn: "2026-12-10"}})
	mustNoErr(t, err)
	_, err = e.svc.Update(ctx(), e.ana, fut.ID, UpdateInput{Version: fut.Version, Title: "Futura", DueOn: "2026-12-01"})
	wantErr(t, err, ErrInvalid, "prazo antes do início")
}

func TestHabitLeftBehindByWhoLeftGetsNoReminder(t *testing.T) {
	e := newEnv(t)
	e.enableModule()
	e.now = mondayMorning
	g := e.habit(e.ana, "Correr", HabitInput{TargetCount: 3, RemindEnabled: true})
	e.rem.CancelByRef(ctx(), e.hh, "goal:"+g.ID+":2026-11-16")
	// a Ana saiu do casal: o hábito dela fica no casal antigo, mas ela já não é membro dele
	_, err := e.db.Exec(`DELETE FROM household_members WHERE household_id = $1 AND user_id = $2`, e.hh, e.ana.UserID)
	mustNoErr(t, err)
	e.runReminderJob()
	if len(e.rem.active()) != 0 {
		t.Errorf("o job não pode criar lembrete para quem saiu do casal: %v", e.rem.active())
	}
}

func TestJobsWalkEveryPageNotJustTheFirst(t *testing.T) {
	old := jobPage
	jobPage = 2
	t.Cleanup(func() { jobPage = old })

	e := newEnv(t)
	e.enableModule()
	e.now = mondayMorning
	var goals []Goal
	for i := 0; i < 5; i++ {
		goals = append(goals, e.habit(e.ana, "Hábito", HabitInput{TargetCount: 3, RemindEnabled: true}))
	}
	for _, g := range goals {
		e.rem.CancelByRef(ctx(), e.hh, "goal:"+g.ID+":2026-11-16")
	}
	e.runReminderJob()
	if n := len(e.rem.active()); n != 5 {
		t.Errorf("lembretes depois do job: %d, quero 5", n)
	}

	// etapas pendentes: 5 com a tarefa fora do ar, depois o job refaz todas
	var steps []StepInput
	for i := 0; i < 5; i++ {
		steps = append(steps, StepInput{Title: "etapa"})
	}
	p := e.project(e.ana, ScopeHousehold, "Reforma", steps...)
	e.tasks.failed = true
	for _, s := range p.Steps {
		_, err := e.svc.MakeStepTask(ctx(), e.ana, p.ID, s.ID)
		mustNoErr(t, err)
	}
	e.tasks.failed = false
	n, err := e.svc.ReconcileSteps(ctx())
	mustNoErr(t, err)
	if n != 5 {
		t.Errorf("tarefas refeitas: %d, quero 5", n)
	}
}

func TestBudgetCategoryMustBelongToTheHousehold(t *testing.T) {
	a, b := newEnv(t), newEnv(t)
	foreign := b.category("Do outro casal")
	_, err := a.repo.CreateGoal(ctx(), NewGoal{Goal: Goal{
		HouseholdID: a.hh, CreatedBy: a.ana.UserID, Scope: ScopeHousehold, Visibility: VisibilityHousehold,
		Kind: KindFinancial, Title: "x",
		Financial: &Financial{TargetCents: 1000, Source: SourceBudgetSavings, CategoryID: &foreign, BaselineOn: "2026-11-15"}}})
	if err == nil {
		t.Error("o banco deveria recusar a categoria de outro casal")
	}
}

func TestReopeningRespectsTheCapUnderTheRepositoryLock(t *testing.T) {
	e := newEnv(t)
	// 99 ativas + 2 arquivadas: as duas reaberturas ao mesmo tempo passariam de 100 se a conta fosse fora do trinco
	a, b := e.project(e.ana, ScopeHousehold, "A"), e.project(e.ana, ScopeHousehold, "B")
	for _, g := range []Goal{a, b} {
		_, err := e.svc.Archive(ctx(), e.ana, g.ID)
		mustNoErr(t, err)
	}
	for i := 0; i < MaxActiveGoals-1; i++ {
		_, err := e.repo.CreateGoal(ctx(), NewGoal{Goal: Goal{HouseholdID: e.hh, CreatedBy: e.ana.UserID,
			Scope: ScopeHousehold, Visibility: VisibilityHousehold, Kind: KindProject, Title: "meta"}})
		mustNoErr(t, err)
	}
	errs := make(chan error, 2)
	for _, g := range []Goal{a, b} {
		go func() { _, err := e.svc.Unarchive(ctx(), e.ana, g.ID); errs <- err }()
	}
	e1, e2 := <-errs, <-errs
	if (e1 == nil) == (e2 == nil) {
		t.Fatalf("exatamente uma reabertura cabe no teto: %v e %v", e1, e2)
	}
	n, _ := e.repo.CountActive(ctx(), e.hh, e.ana.UserID)
	if n != MaxActiveGoals {
		t.Errorf("ativas: %d, quero %d", n, MaxActiveGoals)
	}
}
