package goal

import (
	"fmt"
	"testing"
	"time"
)

func ref(g Goal, st Step) string { return "goal:" + g.ID + ":step:" + st.ID }

func (e *env) get(id string) Goal {
	e.t.Helper()
	g, err := e.svc.Get(ctx(), e.ana, id)
	mustNoErr(e.t, err)
	return g
}

// taskEvent faz o que o app faz quando a tela de Tarefas muda uma tarefa com ref de meta.
func (e *env) taskEvent(g Goal, st Step, status string) {
	e.t.Helper()
	e.tasks.set(ref(g, st), status)
	e.svc.TaskChanged(ctx(), e.hh, ref(g, st), status)
}

func TestStepWithTaskIsDoneOnlyWhenTheTaskIs(t *testing.T) {
	e := newEnv(t)
	g := e.project(e.ana, ScopeHousehold, "Reforma", StepInput{Title: "Comprar tinta", AssigneeID: e.beto.UserID, DueOn: "2026-11-20", Task: true}, StepInput{Title: "Pintar"})
	if len(g.Steps) != 2 || !g.Steps[0].HasTask || g.Progress.Done != 0 || g.Progress.Total != 2 {
		t.Fatalf("criada: %+v", g.Progress)
	}
	st := g.Steps[0]
	task := e.tasks.get(ref(g, st))
	if task == nil || task.Assignee != e.beto.UserID || task.Title != "Reforma: Comprar tinta" {
		t.Fatalf("tarefa criada: %+v", task)
	}
	// 09:00 em São Paulo do dia 20 = 12:00 UTC
	if task.Due == nil || !task.Due.Equal(time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)) {
		t.Errorf("prazo da tarefa: %v", task.Due)
	}

	// enquanto a tarefa está aberta, a etapa está aberta, e o estado não vem de lugar nenhum a não ser a tarefa
	if got := e.get(g.ID); got.Steps[0].Done || got.Progress.Done != 0 {
		t.Errorf("tarefa aberta: %+v", got.Steps[0])
	}
	e.taskEvent(g, st, "done")
	got := e.get(g.ID)
	if !got.Steps[0].Done || got.Steps[0].TaskStatus != "done" || got.Steps[0].DoneAt == nil || got.Progress.Done != 1 || got.Progress.Percent != 50 {
		t.Errorf("tarefa concluída: %+v %+v", got.Steps[0], got.Progress)
	}
	// o banco não guarda estado próprio da etapa com tarefa
	raw, err := e.repo.GetStep(ctx(), e.hh, g.ID, st.ID)
	mustNoErr(t, err)
	if raw.rawDone != nil || !raw.wantTask {
		t.Errorf("a etapa com tarefa não pode ter done_at próprio: %+v", raw)
	}
	// e /done na etapa com tarefa é recusado
	_, err = e.svc.SetStepDone(ctx(), e.ana, g.ID, st.ID, true)
	wantErr(t, err, ErrStepHasTask, "concluir etapa que tem tarefa")
	_, err = e.svc.SetStepDone(ctx(), e.ana, g.ID, st.ID, false)
	wantErr(t, err, ErrStepHasTask, "reabrir etapa que tem tarefa")
}

func TestDeclinedTaskLeavesTheStepOpenAndUnassigned(t *testing.T) {
	e := newEnv(t)
	g := e.project(e.ana, ScopeHousehold, "Reforma", StepInput{Title: "Comprar tinta", AssigneeID: e.beto.UserID, Task: true})
	st := g.Steps[0]
	e.taskEvent(g, st, "declined")
	got := e.get(g.ID)
	if got.Steps[0].Done || got.Steps[0].AssigneeID != nil || got.Steps[0].TaskStatus != "declined" || !got.Steps[0].HasTask {
		t.Errorf("recusada: %+v", got.Steps[0])
	}
}

func TestReassigningTheTaskChangesTheStepAssignee(t *testing.T) {
	e := newEnv(t)
	g := e.project(e.ana, ScopeHousehold, "Reforma", StepInput{Title: "Comprar tinta", AssigneeID: e.beto.UserID, Task: true})
	st := g.Steps[0]
	e.tasks.reassign(ref(g, st), e.ana.UserID)
	got := e.get(g.ID)
	if got.Steps[0].AssigneeID == nil || *got.Steps[0].AssigneeID != e.ana.UserID {
		t.Errorf("responsável vem da tarefa: %+v", got.Steps[0].AssigneeID)
	}
	// e não se reatribui a etapa por aqui: o responsável é da tarefa
	_, err := e.svc.UpdateStep(ctx(), e.ana, g.ID, st.ID, StepUpdate{AssigneeID: &e.beto.UserID})
	wantErr(t, err, ErrStepHasTask, "reatribuir etapa com tarefa")
}

func TestTaskCanceledOutsideReleasesTheStepButKeepsIt(t *testing.T) {
	e := newEnv(t)
	e.enableModule()
	g := e.project(e.ana, ScopeHousehold, "Reforma", StepInput{Title: "Comprar tinta", Task: true})
	st := g.Steps[0]
	e.taskEvent(g, st, "canceled")

	got := e.get(g.ID)
	if len(got.Steps) != 1 || got.Steps[0].HasTask || got.Steps[0].Done || got.Progress.Total != 1 {
		t.Errorf("a etapa continua, aberta e sem tarefa: %+v", got.Steps)
	}
	raw, _ := e.repo.GetStep(ctx(), e.hh, g.ID, st.ID)
	if raw.wantTask {
		t.Error("a etapa solta não deve pedir tarefa de novo")
	}
	// o job de reconciliação não recria a tarefa que alguém cancelou de propósito
	n, err := e.svc.ReconcileSteps(ctx())
	mustNoErr(t, err)
	if n != 0 || e.tasks.get(ref(g, st)).Status != "canceled" {
		t.Errorf("reconciliação recriou a tarefa cancelada: n=%d", n)
	}
	// agora é checklist: conclui à mão
	done, err := e.svc.SetStepDone(ctx(), e.ana, g.ID, st.ID, true)
	mustNoErr(t, err)
	if !done.Steps[0].Done || done.Progress.Done != 1 {
		t.Errorf("checklist: %+v", done.Progress)
	}
}

func TestDeletingAStepCancelsItsTaskAndShrinksTheProgress(t *testing.T) {
	e := newEnv(t)
	g := e.project(e.ana, ScopeHousehold, "Reforma",
		StepInput{Title: "A", Task: true}, StepInput{Title: "B"}, StepInput{Title: "C"})
	a := g.Steps[0]
	_, err := e.svc.SetStepDone(ctx(), e.ana, g.ID, g.Steps[1].ID, true)
	mustNoErr(t, err)
	got := e.get(g.ID)
	if got.Progress.Done != 1 || got.Progress.Total != 3 || got.Progress.Percent != 33 {
		t.Fatalf("1 de 3: %+v", got.Progress)
	}
	got, err = e.svc.DeleteStep(ctx(), e.beto, g.ID, a.ID)
	mustNoErr(t, err)
	if e.tasks.get(ref(g, a)).Status != "canceled" {
		t.Error("apagar a etapa cancela a tarefa")
	}
	if len(got.Steps) != 2 || got.Progress.Done != 1 || got.Progress.Total != 2 || got.Progress.Percent != 50 {
		t.Errorf("concluídas ÷ ativas: %+v", got.Progress)
	}
	_, err = e.svc.DeleteStep(ctx(), e.ana, g.ID, a.ID)
	wantErr(t, err, ErrStepNotFound, "apagar duas vezes")
}

func TestAllStepsReadyPushesOnceAndNeverCompletesTheGoal(t *testing.T) {
	e := newEnv(t)
	g := e.project(e.ana, ScopeHousehold, "Reforma", StepInput{Title: "A", Task: true}, StepInput{Title: "B", Task: true})
	e.taskEvent(g, g.Steps[0], "done")
	if e.notes.count() != 0 {
		t.Fatalf("ainda falta uma etapa: %+v", e.notes.sent)
	}
	e.taskEvent(g, g.Steps[1], "done")
	for _, u := range []string{e.ana.UserID, e.beto.UserID} {
		got := e.notes.to(u)
		if len(got) != 1 || got[0].Body != "Todas as etapas de Reforma estão prontas" {
			t.Errorf("aviso a %s: %+v", u, got)
		}
	}
	// repetir o evento não avisa de novo
	e.svc.TaskChanged(ctx(), e.hh, ref(g, g.Steps[1]), "done")
	if e.notes.count() != 2 {
		t.Errorf("avisos: %d, quero 2 (um para cada)", e.notes.count())
	}
	got := e.get(g.ID)
	if got.Status != StatusActive || !got.Progress.SuggestComplete || got.Progress.Percent != 100 {
		t.Errorf("sugere concluir, não conclui: %+v %s", got.Progress, got.Status)
	}
}

func TestAllStepsReadyNoticeRearmsWhenAStepReopens(t *testing.T) {
	e := newEnv(t)
	g := e.project(e.ana, ScopeHousehold, "Mudança", StepInput{Title: "A"}, StepInput{Title: "B"})
	for _, s := range g.Steps {
		_, err := e.svc.SetStepDone(ctx(), e.ana, g.ID, s.ID, true)
		mustNoErr(t, err)
	}
	if e.notes.count() != 2 {
		t.Fatalf("primeira vez: %d", e.notes.count())
	}
	_, err := e.svc.SetStepDone(ctx(), e.ana, g.ID, g.Steps[0].ID, false)
	mustNoErr(t, err)
	_, err = e.svc.SetStepDone(ctx(), e.ana, g.ID, g.Steps[0].ID, true)
	mustNoErr(t, err)
	if e.notes.count() != 4 {
		t.Errorf("depois de reabrir e concluir de novo avisa outra vez: %d", e.notes.count())
	}
	// uma etapa nova reabre o projeto
	_, err = e.svc.AddStep(ctx(), e.beto, g.ID, StepInput{Title: "C"})
	mustNoErr(t, err)
	if got := e.get(g.ID); got.Progress.Percent != 66 || got.Progress.SuggestComplete {
		t.Errorf("com a etapa nova: %+v", got.Progress)
	}
}

func TestPersonalProjectIsAChecklistAndNeverMakesTasks(t *testing.T) {
	e := newEnv(t)
	g := e.project(e.ana, ScopePersonal, "Tese", StepInput{Title: "Capítulo 1"}, StepInput{Title: "Capítulo 2"})
	_, err := e.svc.MakeStepTask(ctx(), e.ana, g.ID, g.Steps[0].ID)
	wantErr(t, err, ErrInvalid, "tarefa de etapa pessoal")
	_, err = e.svc.AddStep(ctx(), e.ana, g.ID, StepInput{Title: "Capítulo 3", Task: true})
	wantErr(t, err, ErrInvalid, "etapa pessoal pedindo tarefa")
	if len(e.tasks.byRef) != 0 {
		t.Errorf("meta pessoal não cria tarefa: %v", e.tasks.byRef)
	}

	// o parceiro não vê nada disso, e a conclusão avisa só o dono
	_, err = e.svc.Get(ctx(), e.beto, g.ID)
	wantErr(t, err, ErrNotFound, "projeto privado do parceiro")
	_, err = e.svc.AddStep(ctx(), e.beto, g.ID, StepInput{Title: "invasão"})
	wantErr(t, err, ErrNotFound, "etapa em projeto privado do parceiro")
	_, err = e.svc.SetStepDone(ctx(), e.beto, g.ID, g.Steps[0].ID, true)
	wantErr(t, err, ErrNotFound, "concluir etapa de projeto privado do parceiro")
	for _, s := range g.Steps {
		_, err := e.svc.SetStepDone(ctx(), e.ana, g.ID, s.ID, true)
		mustNoErr(t, err)
	}
	if len(e.notes.to(e.ana.UserID)) != 1 || len(e.notes.to(e.beto.UserID)) != 0 {
		t.Errorf("aviso só ao dono: %+v", e.notes.sent)
	}
}

func TestMakeTaskWhenTasksAreDownSetsSyncPendingAndTheJobRetries(t *testing.T) {
	e := newEnv(t)
	e.enableModule()
	g := e.project(e.ana, ScopeHousehold, "Reforma", StepInput{Title: "Comprar tinta"})
	e.tasks.failed = true
	got, err := e.svc.MakeStepTask(ctx(), e.ana, g.ID, g.Steps[0].ID)
	mustNoErr(t, err) // 200, não erro: a meta foi gravada
	if !got.SyncPending || got.Steps[0].Done || got.Steps[0].HasTask {
		t.Fatalf("sync_pending esperado: %+v", got)
	}
	// ainda pendente em leituras seguintes
	if !e.get(g.ID).SyncPending {
		t.Error("a etapa continua pendente até o job")
	}

	e.tasks.failed = false
	n, err := e.svc.ReconcileSteps(ctx())
	mustNoErr(t, err)
	if n != 1 {
		t.Fatalf("o job deveria refazer 1 tarefa, refez %d", n)
	}
	after := e.get(g.ID)
	if after.SyncPending || !after.Steps[0].HasTask || e.tasks.get(ref(g, g.Steps[0])) == nil {
		t.Errorf("depois do job: %+v", after.Steps[0])
	}
	// de novo: nada a fazer
	if n, _ := e.svc.ReconcileSteps(ctx()); n != 0 {
		t.Errorf("segunda rodada: %d", n)
	}
}

func TestMakeTaskOnTheStepTwiceDoesNotDuplicate(t *testing.T) {
	e := newEnv(t)
	g := e.project(e.ana, ScopeHousehold, "Reforma", StepInput{Title: "Comprar tinta"})
	for i := 0; i < 2; i++ {
		_, err := e.svc.MakeStepTask(ctx(), e.ana, g.ID, g.Steps[0].ID)
		mustNoErr(t, err)
	}
	if e.tasks.seq != 1 {
		t.Errorf("tarefas criadas: %d, quero 1", e.tasks.seq)
	}
}

func TestUnlinkTaskKeepsTheStepAndItsState(t *testing.T) {
	e := newEnv(t)
	g := e.project(e.ana, ScopeHousehold, "Reforma", StepInput{Title: "Aberta", Task: true}, StepInput{Title: "Feita", Task: true})
	e.taskEvent(g, g.Steps[1], "done")

	got, err := e.svc.UnlinkStepTask(ctx(), e.ana, g.ID, g.Steps[0].ID)
	mustNoErr(t, err)
	if got.Steps[0].HasTask || got.Steps[0].Done || e.tasks.get(ref(g, g.Steps[0])).Status != "canceled" {
		t.Errorf("desligada aberta: %+v", got.Steps[0])
	}
	got, err = e.svc.UnlinkStepTask(ctx(), e.ana, g.ID, g.Steps[1].ID)
	mustNoErr(t, err)
	if got.Steps[1].HasTask || !got.Steps[1].Done {
		t.Errorf("desligada concluída continua concluída: %+v", got.Steps[1])
	}
}

func TestDeletingTheGoalCancelsLinkedTasks(t *testing.T) {
	e := newEnv(t)
	g := e.project(e.ana, ScopeHousehold, "Reforma", StepInput{Title: "A", Task: true}, StepInput{Title: "B", Task: true})
	e.taskEvent(g, g.Steps[1], "done")
	mustNoErr(t, e.svc.Delete(ctx(), e.beto, g.ID))
	if e.tasks.get(ref(g, g.Steps[0])).Status != "canceled" {
		t.Error("a tarefa aberta é cancelada")
	}
	if e.tasks.get(ref(g, g.Steps[1])).Status != "done" {
		t.Error("a concluída fica como histórico")
	}
}

func TestStepEditingAndOrder(t *testing.T) {
	e := newEnv(t)
	g := e.project(e.ana, ScopeHousehold, "Reforma", StepInput{Title: "A"}, StepInput{Title: "B"}, StepInput{Title: "C"})
	c := g.Steps[2]
	got, err := e.svc.UpdateStep(ctx(), e.beto, g.ID, c.ID, StepUpdate{Position: intp(1)})
	mustNoErr(t, err)
	order := ""
	for _, s := range got.Steps {
		order += s.Title
	}
	if order != "CAB" {
		t.Errorf("ordem: %s", order)
	}
	title := "  Pintar a sala  "
	due := "2026-12-01"
	got, err = e.svc.UpdateStep(ctx(), e.ana, g.ID, c.ID, StepUpdate{Title: &title, DueOn: &due, AssigneeID: &e.beto.UserID})
	mustNoErr(t, err)
	if got.Steps[0].Title != "Pintar a sala" || got.Steps[0].DueOn == nil || *got.Steps[0].DueOn != due ||
		got.Steps[0].AssigneeID == nil || *got.Steps[0].AssigneeID != e.beto.UserID {
		t.Errorf("edição: %+v", got.Steps[0])
	}
	empty := ""
	got, _ = e.svc.UpdateStep(ctx(), e.ana, g.ID, c.ID, StepUpdate{DueOn: &empty, AssigneeID: &empty})
	if got.Steps[0].DueOn != nil || got.Steps[0].AssigneeID != nil {
		t.Errorf("limpar prazo e responsável: %+v", got.Steps[0])
	}
	bad := ""
	_, err = e.svc.UpdateStep(ctx(), e.ana, g.ID, c.ID, StepUpdate{Title: &bad})
	wantErr(t, err, ErrInvalid, "título vazio")
	_, err = e.svc.UpdateStep(ctx(), e.ana, g.ID, "00000000-0000-4000-8000-0000000000aa", StepUpdate{Title: &title})
	wantErr(t, err, ErrStepNotFound, "etapa que não existe")
}

func intp(n int) *int { return &n }

func TestStepLimitAndClosedGoals(t *testing.T) {
	e := newEnv(t)
	steps := make([]StepInput, MaxSteps)
	for i := range steps {
		steps[i] = StepInput{Title: fmt.Sprintf("etapa %d", i)}
	}
	g := e.project(e.ana, ScopeHousehold, "Enorme", steps...)
	_, err := e.svc.AddStep(ctx(), e.ana, g.ID, StepInput{Title: "a 201ª"})
	wantErr(t, err, ErrLimit, "passou de 200 etapas")
	_, err = e.svc.Create(ctx(), e.ana, CreateInput{Kind: KindProject, Scope: ScopeHousehold, Title: "x", Steps: append(steps, StepInput{Title: "y"})})
	wantErr(t, err, ErrLimit, "criar com mais de 200")

	small := e.project(e.ana, ScopeHousehold, "Pequena", StepInput{Title: "A"})
	_, err = e.svc.Complete(ctx(), e.ana, small.ID)
	mustNoErr(t, err)
	_, err = e.svc.AddStep(ctx(), e.ana, small.ID, StepInput{Title: "B"})
	wantErr(t, err, ErrClosed, "etapa em meta concluída")
	_, err = e.svc.SetStepDone(ctx(), e.ana, small.ID, small.Steps[0].ID, true)
	wantErr(t, err, ErrClosed, "concluir etapa de meta concluída")

	h := e.habit(e.ana, "Correr", HabitInput{})
	_, err = e.svc.AddStep(ctx(), e.ana, h.ID, StepInput{Title: "x"})
	wantErr(t, err, ErrKindMismatch, "etapa em hábito")
}

func TestEitherPartnerEditsAHouseholdProject(t *testing.T) {
	e := newEnv(t)
	g := e.project(e.ana, ScopeHousehold, "Reforma", StepInput{Title: "A"})
	got, err := e.svc.AddStep(ctx(), e.beto, g.ID, StepInput{Title: "B", AssigneeID: e.ana.UserID})
	mustNoErr(t, err)
	if len(got.Steps) != 2 || got.Steps[1].AssigneeID == nil || *got.Steps[1].AssigneeID != e.ana.UserID {
		t.Errorf("o parceiro acrescentou uma etapa: %+v", got.Steps)
	}
}
