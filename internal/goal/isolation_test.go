package goal

import (
	"testing"
)

// populate cria dados do casal em todas as tabelas de Metas e devolve os ids.
type populated struct {
	habit, shared, financial, project, contribution, step string
}

func populate(t *testing.T, e *env) populated {
	t.Helper()
	e.enableModule()
	var p populated
	h := e.habit(e.ana, "Correr", HabitInput{})
	e.mark(e.ana, h.ID, "2026-11-10")
	p.habit = h.ID
	s := e.shared(e.ana, "Jantar fora", SharedInput{})
	e.mark(e.ana, s.ID, "2026-11-10")
	p.shared = s.ID
	f := e.saving(e.ana, ScopeHousehold, "Casa", 100000)
	p.financial = f.ID
	p.contribution = e.pay(f.ID, 5000).Contribution.ID
	pr := e.project(e.ana, ScopeHousehold, "Reforma", StepInput{Title: "A"})
	p.project, p.step = pr.ID, pr.Steps[0].ID
	return p
}

func TestHouseholdsAreIsolated(t *testing.T) {
	a, b := newEnv(t), newEnv(t)
	pa := populate(t, a)

	// o casal B usa os ids do casal A e não acha nada: 404 em tudo
	for _, id := range []string{pa.habit, pa.shared, pa.financial, pa.project} {
		_, err := b.svc.Get(ctx(), b.ana, id)
		wantErr(t, err, ErrNotFound, "ler meta de outro casal")
		_, err = b.svc.Update(ctx(), b.ana, id, UpdateInput{Version: 1, Title: "invadida"})
		wantErr(t, err, ErrNotFound, "editar meta de outro casal")
		_, err = b.svc.Pause(ctx(), b.ana, id)
		wantErr(t, err, ErrNotFound, "pausar meta de outro casal")
		wantErr(t, b.svc.Delete(ctx(), b.ana, id), ErrNotFound, "apagar meta de outro casal")
	}
	_, err := b.svc.Checkin(ctx(), b.ana, pa.shared, "")
	wantErr(t, err, ErrNotFound, "marcar meta de outro casal")
	_, err = b.svc.UndoCheckin(ctx(), b.ana, pa.shared, "2026-11-10")
	wantErr(t, err, ErrNotFound, "desmarcar meta de outro casal")
	_, err = b.svc.History(ctx(), b.ana, pa.shared, "", "")
	wantErr(t, err, ErrNotFound, "histórico de outro casal")
	_, err = b.svc.AddContribution(ctx(), b.ana, pa.financial, ContributionInput{AmountCents: 100})
	wantErr(t, err, ErrNotFound, "aportar em meta de outro casal")
	_, err = b.svc.ListContributions(ctx(), b.ana, pa.financial, "", 10)
	wantErr(t, err, ErrNotFound, "listar aportes de outro casal")
	_, err = b.svc.DeleteContribution(ctx(), b.ana, pa.financial, pa.contribution)
	wantErr(t, err, ErrNotFound, "apagar aporte de outro casal")
	_, err = b.svc.AddStep(ctx(), b.ana, pa.project, StepInput{Title: "invasão"})
	wantErr(t, err, ErrNotFound, "etapa em projeto de outro casal")
	_, err = b.svc.UpdateStep(ctx(), b.ana, pa.project, pa.step, StepUpdate{Position: intp(1)})
	wantErr(t, err, ErrNotFound, "editar etapa de outro casal")
	_, err = b.svc.SetStepDone(ctx(), b.ana, pa.project, pa.step, true)
	wantErr(t, err, ErrNotFound, "concluir etapa de outro casal")
	_, err = b.svc.DeleteStep(ctx(), b.ana, pa.project, pa.step)
	wantErr(t, err, ErrNotFound, "apagar etapa de outro casal")
	_, err = b.svc.MakeStepTask(ctx(), b.ana, pa.project, pa.step)
	wantErr(t, err, ErrNotFound, "tarefa de etapa de outro casal")

	// listas e exportação do casal B não trazem nada do A
	if n := len(b.list(b.ana, ListFilter{Status: "all"})); n != 0 {
		t.Errorf("o casal B viu %d meta(s) do A", n)
	}
	ex, err := b.svc.Export(ctx(), b.ana)
	mustNoErr(t, err)
	if len(ex.Goals) != 0 {
		t.Errorf("exportação do B trouxe %d meta(s) do A", len(ex.Goals))
	}

	// no banco: com o escopo do casal B, as tabelas filhas do A não aparecem (o filtro e o RLS agem juntos)
	if sums, _ := b.repo.SumContributions(ctx(), b.hh, b.ana.UserID, []string{pa.financial}); len(sums) != 0 {
		t.Errorf("soma de aportes de outro casal: %v", sums)
	}
	if marks, _ := b.repo.Checkins(ctx(), b.hh, b.ana.UserID, []string{pa.shared, pa.habit}, testNow.AddDate(-1, 0, 0), testNow.AddDate(1, 0, 0)); len(marks) != 0 {
		t.Errorf("marcações de outro casal: %v", marks)
	}
	if _, err := b.repo.GetStep(ctx(), b.hh, pa.project, pa.step); err == nil {
		t.Error("etapa de outro casal lida")
	}

	// o casal A segue intacto
	g, err := a.svc.Get(ctx(), a.ana, pa.financial)
	mustNoErr(t, err)
	if g.Progress.Done != 5000 {
		t.Errorf("casal A: %+v", g.Progress)
	}
}

// Tabelas filhas nunca são lidas sozinhas: o parceiro que não vê a meta não vê o que há dentro dela, mesmo pelo repositório.
func TestChildRowsInheritThePrivacyOfTheirGoal(t *testing.T) {
	e := newEnv(t)
	h := e.habit(e.ana, "Terapia", HabitInput{})
	e.mark(e.ana, h.ID, "2026-11-10")
	f := e.saving(e.ana, ScopePersonal, "Reserva", 10000)
	e.pay(f.ID, 5000)
	p := e.project(e.ana, ScopePersonal, "Tese", StepInput{Title: "Cap 1"})

	if m, _ := e.repo.Checkins(ctx(), e.hh, e.beto.UserID, []string{h.ID}, testNow.AddDate(-1, 0, 0), testNow.AddDate(1, 0, 0)); len(m[h.ID]) != 0 {
		t.Error("marcações de meta privada vazaram pelo repositório")
	}
	if s, _ := e.repo.SumContributions(ctx(), e.hh, e.beto.UserID, []string{f.ID}); len(s) != 0 {
		t.Error("soma de aportes de meta privada vazou pelo repositório")
	}
	if c, _ := e.repo.AllContributions(ctx(), e.hh, e.beto.UserID, []string{f.ID}); len(c) != 0 {
		t.Error("aportes de meta privada vazaram pelo repositório")
	}
	if _, err := e.repo.ListContributions(ctx(), e.hh, e.beto.UserID, f.ID, "", 10); err == nil {
		t.Error("lista de aportes de meta privada vazou pelo repositório")
	}
	if gs, _ := e.repo.ListGoals(ctx(), e.hh, e.beto.UserID, ListFilter{Status: "all"}); len(gs) != 0 {
		t.Errorf("lista de metas vazou pelo repositório: %d", len(gs))
	}
	if _, err := e.repo.GetGoal(ctx(), e.hh, e.beto.UserID, p.ID); err == nil {
		t.Error("projeto privado vazou pelo repositório")
	}
	// e as escritas também conferem a permissão no banco, não só no service
	if _, err := e.repo.AddContribution(ctx(), e.hh, e.beto.UserID, f.ID, Contribution{AmountCents: 1, MadeOn: "2026-11-15"}); err == nil {
		t.Error("aporte do parceiro em meta privada passou pelo repositório")
	}
	if _, err := e.repo.AddCheckin(ctx(), e.hh, e.beto.UserID, h.ID, "2026-11-15"); err == nil {
		t.Error("marcação do parceiro em hábito privado passou pelo repositório")
	}
	if _, err := e.repo.AddStep(ctx(), e.hh, e.beto.UserID, p.ID, NewStep{Title: "invasão"}); err == nil {
		t.Error("etapa do parceiro em projeto privado passou pelo repositório")
	}
	if err := e.repo.SetStatus(ctx(), e.hh, e.beto.UserID, h.ID, []string{StatusActive}, StatusDone); err == nil {
		t.Error("o parceiro concluiu meta privada pelo repositório")
	}
}
