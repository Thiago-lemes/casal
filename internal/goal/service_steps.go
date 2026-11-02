package goal

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"casal/internal/identity"
)

// Uma etapa pode ter tarefa ou ser só um item de checklist. Com tarefa, ela não guarda estado próprio: está
// concluída quando a tarefa está, e o responsável é o da tarefa (lidos de Tasks.StatesByRef). Assim a etapa e a
// tarefa nunca discordam. Meta pessoal não gera tarefa (tarefas são do casal): suas etapas são checklist.

// editable carrega a meta, confere que é um projeto e que a pessoa pode alterá-la.
func (s *Service) editableProject(ctx context.Context, p identity.Principal, goalID string) (Goal, error) {
	g, err := s.repo.GetGoal(ctx, p.HouseholdID, p.UserID, goalID)
	if err != nil {
		return Goal{}, err
	}
	if !g.CanEdit {
		return Goal{}, ErrForbidden
	}
	if g.Kind != KindProject {
		return Goal{}, ErrKindMismatch
	}
	return g, nil
}

// afterStep devolve a meta recalculada depois de uma mudança nas etapas.
func (s *Service) afterStep(ctx context.Context, p identity.Principal, goalID string, pending bool) (Goal, error) {
	s.checkAllDone(ctx, p.HouseholdID, goalID)
	g, err := s.repo.GetGoal(ctx, p.HouseholdID, p.UserID, goalID)
	if err != nil {
		return Goal{}, err
	}
	out, err := s.present(ctx, p, g, true)
	out.SyncPending = out.SyncPending || pending
	return out, err
}

func (s *Service) AddStep(ctx context.Context, p identity.Principal, goalID string, in StepInput) (Goal, error) {
	g, err := s.editableProject(ctx, p, goalID)
	if err != nil {
		return Goal{}, err
	}
	ps, err := s.members(ctx, p.HouseholdID)
	if err != nil {
		return Goal{}, err
	}
	ns, err := s.newStep(in, g, ps)
	if err != nil {
		return Goal{}, err
	}
	st, err := s.repo.AddStep(ctx, p.HouseholdID, p.UserID, goalID, ns)
	if err != nil {
		return Goal{}, err
	}
	pending := false
	if ns.WantTask {
		pending = !s.ensureTask(ctx, p.UserID, g, st, ps)
	}
	return s.afterStep(ctx, p, goalID, pending)
}

func (s *Service) UpdateStep(ctx context.Context, p identity.Principal, goalID, stepID string, in StepUpdate) (Goal, error) {
	g, err := s.editableProject(ctx, p, goalID)
	if err != nil {
		return Goal{}, err
	}
	cur, err := s.repo.GetStep(ctx, p.HouseholdID, goalID, stepID)
	if err != nil {
		return Goal{}, err
	}
	var patch StepPatch
	if in.Title != nil {
		t, err := cleanTitle(*in.Title, 200, "o título da etapa")
		if err != nil {
			return Goal{}, err
		}
		patch.Title = &t
	}
	if in.AssigneeID != nil {
		if cur.wantTask { // o responsável de uma etapa com tarefa é o da tarefa: reatribua pela tela de tarefas
			return Goal{}, ErrStepHasTask
		}
		patch.SetAssignee = true
		if *in.AssigneeID != "" {
			ps, err := s.members(ctx, p.HouseholdID)
			if err != nil {
				return Goal{}, err
			}
			ns, err := s.newStep(StepInput{Title: "x", AssigneeID: *in.AssigneeID}, g, ps)
			if err != nil {
				return Goal{}, err
			}
			patch.AssigneeID = ns.AssigneeID
		}
	}
	if in.DueOn != nil {
		patch.SetDue = true
		if *in.DueOn != "" {
			d, err := parseDay(*in.DueOn)
			if err != nil {
				return Goal{}, err
			}
			due := fmtDay(d)
			patch.DueOn = &due
		}
	}
	if in.Position != nil {
		if *in.Position < 1 {
			return Goal{}, fmt.Errorf("%w: a posição começa em 1", ErrInvalid)
		}
		patch.Position = in.Position
	}
	if err := s.repo.UpdateStep(ctx, p.HouseholdID, p.UserID, goalID, stepID, patch); err != nil {
		return Goal{}, err
	}
	if cur.wantTask && (patch.Title != nil || patch.SetDue) {
		// a tarefa acompanha o título e o prazo da etapa, senão a tela de Tarefas mostraria outro prazo
		if upd, err := s.repo.GetStep(ctx, p.HouseholdID, goalID, stepID); err == nil {
			if ps, err := s.members(ctx, p.HouseholdID); err == nil {
				title, due := taskFields(g, upd, p.UserID, ps)
				if err := s.tasks.UpdateByRef(ctx, p.HouseholdID, stepRef(goalID, stepID), title, due); err != nil {
					s.log.Warn("falha ao atualizar a tarefa da etapa", "goal", goalID, "step", stepID, "err", err)
				}
			}
		}
	}
	return s.afterStep(ctx, p, goalID, false)
}

// DeleteStep tira a etapa da meta e cancela a tarefa ligada.
func (s *Service) DeleteStep(ctx context.Context, p identity.Principal, goalID, stepID string) (Goal, error) {
	if _, err := s.editableProject(ctx, p, goalID); err != nil {
		return Goal{}, err
	}
	if err := s.repo.CancelStep(ctx, p.HouseholdID, p.UserID, goalID, stepID); err != nil {
		return Goal{}, err
	}
	if err := s.tasks.CancelByRef(ctx, p.HouseholdID, stepRef(goalID, stepID)); err != nil {
		s.log.Warn("falha ao cancelar a tarefa da etapa apagada", "goal", goalID, "step", stepID, "err", err)
	}
	return s.afterStep(ctx, p, goalID, false)
}

// SetStepDone conclui ou reabre uma etapa que não tem tarefa.
func (s *Service) SetStepDone(ctx context.Context, p identity.Principal, goalID, stepID string, done bool) (Goal, error) {
	if _, err := s.editableProject(ctx, p, goalID); err != nil {
		return Goal{}, err
	}
	if err := s.repo.SetStepDone(ctx, p.HouseholdID, p.UserID, goalID, stepID, done); err != nil {
		return Goal{}, err
	}
	return s.afterStep(ctx, p, goalID, false)
}

// MakeStepTask transforma a etapa em tarefa. Se a tarefa não puder ser criada agora, a resposta é 200 com
// sync_pending e o job de reconciliação refaz.
func (s *Service) MakeStepTask(ctx context.Context, p identity.Principal, goalID, stepID string) (Goal, error) {
	g, err := s.editableProject(ctx, p, goalID)
	if err != nil {
		return Goal{}, err
	}
	if g.Scope != ScopeHousehold {
		return Goal{}, fmt.Errorf("%w: meta pessoal não gera tarefa", ErrInvalid)
	}
	if g.Status == StatusDone || g.Status == StatusArchived {
		return Goal{}, ErrClosed
	}
	st, err := s.repo.GetStep(ctx, p.HouseholdID, goalID, stepID)
	if err != nil {
		return Goal{}, err
	}
	if st.rawDone != nil {
		return Goal{}, fmt.Errorf("%w: a etapa já está concluída", ErrInvalid)
	}
	if !st.wantTask {
		if err := s.repo.SetStepTask(ctx, p.HouseholdID, p.UserID, goalID, stepID, true, nil, false); err != nil {
			return Goal{}, err
		}
		st.wantTask = true
	}
	ps, err := s.members(ctx, p.HouseholdID)
	if err != nil {
		return Goal{}, err
	}
	pending := st.taskID == "" && !s.ensureTask(ctx, p.UserID, g, st, ps)
	return s.afterStep(ctx, p, goalID, pending)
}

// UnlinkStepTask desliga a tarefa (ela é cancelada) e a etapa continua como item de checklist. Se a tarefa já
// estava concluída, a etapa continua concluída.
func (s *Service) UnlinkStepTask(ctx context.Context, p identity.Principal, goalID, stepID string) (Goal, error) {
	if _, err := s.editableProject(ctx, p, goalID); err != nil {
		return Goal{}, err
	}
	st, err := s.repo.GetStep(ctx, p.HouseholdID, goalID, stepID)
	if err != nil {
		return Goal{}, err
	}
	if !st.wantTask {
		return s.afterStep(ctx, p, goalID, false)
	}
	ref := stepRef(goalID, stepID)
	states, err := s.tasks.StatesByRef(ctx, p.HouseholdID, []string{ref})
	if err != nil {
		return Goal{}, err
	}
	wasDone := states[ref].Status == "done"
	if err := s.repo.SetStepTask(ctx, p.HouseholdID, p.UserID, goalID, stepID, false, nil, wasDone); err != nil {
		return Goal{}, err
	}
	if err := s.tasks.CancelByRef(ctx, p.HouseholdID, ref); err != nil {
		s.log.Warn("falha ao cancelar a tarefa desligada", "goal", goalID, "step", stepID, "err", err)
	}
	return s.afterStep(ctx, p, goalID, false)
}

// taskFields monta o título e o prazo da tarefa de uma etapa. O prazo é o da etapa, às 09:00 no fuso do
// responsável (ou de quem agiu, na tarefa do casal).
func taskFields(g Goal, st Step, actor string, ps []Person) (string, *time.Time) {
	var due *time.Time
	if st.DueOn != nil {
		tzOwner := actor
		if st.AssigneeID != nil {
			tzOwner = *st.AssigneeID
		}
		tz := ""
		if pe, ok := person(ps, tzOwner); ok {
			tz = pe.Timezone
		}
		d := mustDay(*st.DueOn)
		at := time.Date(d.Year(), d.Month(), d.Day(), 9, 0, 0, 0, location(tz))
		due = &at
	}
	title := g.Title + ": " + st.Title
	if utf8.RuneCountInString(title) > 200 {
		title = string([]rune(title)[:200])
	}
	return strings.TrimSpace(title), due
}

// ensureTask cria a tarefa da etapa (idempotente pelo ref) e guarda o id. Devolve falso se não deu certo agora.
// Quem recebe a tarefa de outra pessoa é avisado, como em qualquer tarefa atribuída.
func (s *Service) ensureTask(ctx context.Context, actor string, g Goal, st Step, ps []Person) bool {
	assignee := ""
	if st.AssigneeID != nil {
		assignee = *st.AssigneeID
	}
	title, due := taskFields(g, st, actor, ps)
	taskID, err := s.tasks.Create(ctx, g.HouseholdID, actor, assignee, title, due, stepRef(g.ID, st.ID))
	if err != nil {
		s.log.Warn("falha ao criar a tarefa da etapa", "goal", g.ID, "step", st.ID, "err", err)
		return false
	}
	if err := s.repo.AttachTask(ctx, g.HouseholdID, g.ID, st.ID, taskID); err != nil {
		s.log.Warn("falha ao ligar a tarefa à etapa", "goal", g.ID, "step", st.ID, "err", err)
		return false
	}
	if assignee != "" && assignee != actor {
		s.notify(ctx, assignee, "Nova tarefa para você", title)
	}
	return true
}

// createStepTasks cria as tarefas das etapas que nasceram pedindo uma. Devolve verdadeiro se alguma ficou pendente.
func (s *Service) createStepTasks(ctx context.Context, p identity.Principal, g Goal) (pending bool) {
	if g.Kind != KindProject {
		return false
	}
	ps, err := s.members(ctx, g.HouseholdID)
	if err != nil {
		return true
	}
	for _, st := range g.Steps {
		if st.wantTask && st.taskID == "" && !s.ensureTask(ctx, p.UserID, g, st, ps) {
			pending = true
		}
	}
	return pending
}

// TaskChanged é o gancho de tarefa concluída ou cancelada (registrado no app): recalcula o projeto e avisa quando
// todas as etapas ficam prontas. Uma tarefa cancelada por fora solta a etapa, que continua aberta e sem tarefa.
// Nunca devolve erro: a tarefa já mudou, e o estado da etapa é lido da tarefa de qualquer forma.
func (s *Service) TaskChanged(ctx context.Context, householdID, ref, status string) {
	goalID, stepID, ok := parseStepRef(ref)
	if !ok {
		return
	}
	if status == "canceled" {
		if err := s.repo.ReleaseStepTask(ctx, householdID, goalID, stepID); err != nil {
			s.log.Warn("falha ao soltar a etapa de uma tarefa cancelada", "goal", goalID, "step", stepID, "err", err)
		}
	}
	s.checkAllDone(ctx, householdID, goalID)
}

func parseStepRef(ref string) (goalID, stepID string, ok bool) {
	parts := strings.Split(ref, ":")
	if len(parts) != 4 || parts[0] != "goal" || parts[2] != "step" || parts[1] == "" || parts[3] == "" {
		return "", "", false
	}
	return parts[1], parts[3], true
}

// checkAllDone avisa, uma vez, que todas as etapas estão prontas (sem concluir a meta: quem decide é a pessoa).
// O aviso volta a valer quando uma etapa reabre. Meta do casal avisa os dois; pessoal, só o dono.
func (s *Service) checkAllDone(ctx context.Context, householdID, goalID string) {
	g, err := s.repo.GetGoalSystem(ctx, householdID, goalID)
	if err != nil || g.Kind != KindProject {
		return
	}
	refs := []string{}
	for _, st := range g.Steps {
		if st.wantTask && st.taskID != "" {
			refs = append(refs, stepRef(g.ID, st.ID))
		}
	}
	states, err := s.tasks.StatesByRef(ctx, householdID, refs)
	if err != nil {
		s.log.Warn("falha ao ler as tarefas das etapas", "goal", goalID, "err", err)
		return
	}
	steps, _ := stepViews(&g, states)
	done := 0
	for _, st := range steps {
		if st.Done {
			done++
		}
	}
	if len(steps) == 0 || done < len(steps) || g.Status != StatusActive {
		if err := s.repo.ClearAllDone(ctx, householdID, goalID); err != nil {
			s.log.Warn("falha ao limpar o aviso de etapas prontas", "goal", goalID, "err", err)
		}
		return
	}
	first, err := s.repo.ClaimAllDone(ctx, householdID, goalID)
	if err != nil || !first {
		return
	}
	body := "Todas as etapas de " + g.Title + " estão prontas"
	if g.Scope == ScopePersonal && g.OwnerID != nil {
		s.notify(ctx, *g.OwnerID, "Meta", body)
		return
	}
	ps, err := s.members(ctx, householdID)
	if err != nil {
		return
	}
	for _, m := range ps {
		s.notify(ctx, m.ID, "Meta", body)
	}
}
