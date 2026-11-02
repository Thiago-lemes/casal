package goal

import (
	"strings"
	"testing"
)

// A privacidade entre os parceiros vive só no código (o RLS não separa os dois do mesmo casal): este teste é a prova.
func TestPrivateGoalIsInvisibleToThePartnerEverywhere(t *testing.T) {
	e := newEnv(t)
	secret := e.habit(e.ana, "Terapia", HabitInput{})
	hh := e.shared(e.ana, "Jantar fora", SharedInput{})

	// lista, em todos os filtros
	for _, f := range []ListFilter{{}, {Scope: "mine"}, {Scope: ScopeHousehold}, {Status: "all"}, {Kind: KindHabit}} {
		if e.list(e.beto, f)[secret.ID] {
			t.Errorf("o parceiro viu a meta privada na lista %+v", f)
		}
	}
	if !e.list(e.ana, ListFilter{Scope: "mine"})[secret.ID] || !e.list(e.beto, ListFilter{})[hh.ID] {
		t.Fatal("a Ana deveria ver a própria meta e o Beto, a do casal")
	}

	// leitura por id: 404, não 403 (o parceiro não deduz que ela existe)
	_, err := e.svc.Get(ctx(), e.beto, secret.ID)
	wantErr(t, err, ErrNotFound, "ler meta privada do parceiro")

	// e nada mais funciona com ela
	_, err = e.svc.Update(ctx(), e.beto, secret.ID, UpdateInput{Version: 1, Title: "x"})
	wantErr(t, err, ErrNotFound, "editar")
	_, err = e.svc.Checkin(ctx(), e.beto, secret.ID, "")
	wantErr(t, err, ErrNotFound, "marcar")
	_, err = e.svc.History(ctx(), e.beto, secret.ID, "", "")
	wantErr(t, err, ErrNotFound, "histórico")
	_, err = e.svc.Pause(ctx(), e.beto, secret.ID)
	wantErr(t, err, ErrNotFound, "pausar")
	wantErr(t, e.svc.Delete(ctx(), e.beto, secret.ID), ErrNotFound, "apagar")

	// exportação
	ex, err := e.svc.Export(ctx(), e.beto)
	mustNoErr(t, err)
	for _, g := range ex.Goals {
		if g.Goal.ID == secret.ID {
			t.Error("a exportação do parceiro trouxe a meta privada")
		}
	}

	// contagem: o teto de metas ativas não conta as pessoais do parceiro (não vaza existência)
	n, err := e.repo.CountActive(ctx(), e.hh, e.beto.UserID)
	mustNoErr(t, err)
	if n != 1 { // só a do casal
		t.Errorf("contagem do Beto = %d, quero 1 (só a do casal)", n)
	}
	n, _ = e.repo.CountActive(ctx(), e.hh, e.ana.UserID)
	if n != 2 {
		t.Errorf("contagem da Ana = %d, quero 2", n)
	}

	// meta financeira privada: aportes também não vazam
	fin := e.saving(e.ana, ScopePersonal, "Presente surpresa", 100000)
	_, err = e.svc.AddContribution(ctx(), e.ana, fin.ID, ContributionInput{AmountCents: 5000})
	mustNoErr(t, err)
	_, err = e.svc.AddContribution(ctx(), e.beto, fin.ID, ContributionInput{AmountCents: 5000})
	wantErr(t, err, ErrNotFound, "aporte do parceiro em meta privada")
	_, err = e.svc.ListContributions(ctx(), e.beto, fin.ID, "", 10)
	wantErr(t, err, ErrNotFound, "listar aportes de meta privada")
}

func TestViewGoalShowsTitleDueAndProgressButNoDetailAndNeverEdits(t *testing.T) {
	e := newEnv(t)
	g, err := e.svc.Create(ctx(), e.ana, CreateInput{
		Kind: KindFinancial, Scope: ScopePersonal, Visibility: VisibilityView, Title: "Viagem",
		Notes: "segredo das notas", DueOn: "2027-06-01", Financial: &FinancialInput{TargetCents: 200000}})
	mustNoErr(t, err)
	_, err = e.svc.AddContribution(ctx(), e.ana, g.ID, ContributionInput{AmountCents: 50000, Note: "13º"})
	mustNoErr(t, err)

	got, err := e.svc.Get(ctx(), e.beto, g.ID)
	mustNoErr(t, err)
	if got.Title != "Viagem" || got.DueOn == nil || got.Progress.Done != 50000 || got.Progress.Percent != 25 {
		t.Fatalf("o parceiro deveria ver título, prazo e progresso: %+v", got)
	}
	if got.Access != AccessView || got.CanEdit {
		t.Errorf("acesso do parceiro: %s edit=%v", got.Access, got.CanEdit)
	}
	if got.Notes != "" || got.Financial != nil || got.Recent != nil || got.Steps != nil {
		t.Errorf("o parceiro não pode ver notas nem detalhes: %+v", got)
	}
	// aparece na lista dele
	if !e.list(e.beto, ListFilter{})[g.ID] {
		t.Error("a meta view deveria aparecer na lista do parceiro")
	}
	// mas não como meta dele nem do casal
	if e.list(e.beto, ListFilter{Scope: "mine"})[g.ID] || e.list(e.beto, ListFilter{Scope: ScopeHousehold})[g.ID] {
		t.Error("a meta view do parceiro não é dele nem do casal")
	}

	// aportes individuais: nem lista nem escrita
	_, err = e.svc.ListContributions(ctx(), e.beto, g.ID, "", 10)
	wantErr(t, err, ErrForbidden, "listar aportes de meta view")
	_, err = e.svc.AddContribution(ctx(), e.beto, g.ID, ContributionInput{AmountCents: 100})
	wantErr(t, err, ErrForbidden, "aportar em meta view")

	// nunca edita: 403 (ele a enxerga)
	_, err = e.svc.Update(ctx(), e.beto, g.ID, UpdateInput{Version: 1, Title: "invadida"})
	wantErr(t, err, ErrForbidden, "editar meta view")
	_, err = e.svc.Complete(ctx(), e.beto, g.ID)
	wantErr(t, err, ErrForbidden, "concluir meta view")
	wantErr(t, e.svc.Delete(ctx(), e.beto, g.ID), ErrForbidden, "apagar meta view")

	// a exportação do parceiro traz o resumo, sem aportes nem notas
	ex, err := e.svc.Export(ctx(), e.beto)
	mustNoErr(t, err)
	found := false
	for _, x := range ex.Goals {
		if x.Goal.ID == g.ID {
			found = true
			if len(x.Contributions) != 0 || x.Goal.Notes != "" {
				t.Errorf("exportação vazou detalhe: %+v", x)
			}
		}
	}
	if !found {
		t.Error("a exportação deveria trazer o resumo da meta view")
	}

	// só o dono muda a visibilidade; ao voltar para private, some para o parceiro
	cur, _ := e.svc.Get(ctx(), e.ana, g.ID)
	_, err = e.svc.Update(ctx(), e.ana, g.ID, UpdateInput{Version: cur.Version, Title: cur.Title, DueOn: *cur.DueOn, Visibility: VisibilityPrivate})
	mustNoErr(t, err)
	_, err = e.svc.Get(ctx(), e.beto, g.ID)
	wantErr(t, err, ErrNotFound, "depois de voltar a private")
}

func TestHouseholdGoalIsEditedByBoth(t *testing.T) {
	e := newEnv(t)
	g := e.shared(e.ana, "Jantar fora", SharedInput{})
	got, err := e.svc.Update(ctx(), e.beto, g.ID, UpdateInput{Version: g.Version, Title: "Jantar fora de casa", Shared: &struct {
		TargetCount  int  `json:"target_count"`
		RequiresBoth bool `json:"requires_both"`
	}{TargetCount: 3, RequiresBoth: true}})
	mustNoErr(t, err)
	if got.Title != "Jantar fora de casa" || got.Shared.TargetCount != 3 || !got.Shared.RequiresBoth || got.Version != g.Version+1 {
		t.Errorf("edição do parceiro: %+v", got)
	}
}

func TestStaleVersionIsAConflict(t *testing.T) {
	e := newEnv(t)
	g := e.shared(e.ana, "Cinema", SharedInput{})
	_, err := e.svc.Update(ctx(), e.ana, g.ID, UpdateInput{Version: g.Version, Title: "Cinema em casa"})
	mustNoErr(t, err)
	_, err = e.svc.Update(ctx(), e.beto, g.ID, UpdateInput{Version: g.Version, Title: "Cinema na rua"})
	wantErr(t, err, ErrConflict, "gravar com a versão velha")
	cur, _ := e.svc.Get(ctx(), e.beto, g.ID)
	if cur.Title != "Cinema em casa" {
		t.Errorf("a edição velha apagou a nova: %q", cur.Title)
	}
	_, err = e.svc.Update(ctx(), e.beto, g.ID, UpdateInput{Title: "sem versão"})
	wantErr(t, err, ErrInvalid, "sem versão")
}

func TestKindNeverChangesAndBlocksMustMatch(t *testing.T) {
	e := newEnv(t)
	h := e.habit(e.ana, "Correr", HabitInput{})
	// o JSON não tem como mudar o kind; mandar o bloco de outro tipo na edição é recusado
	_, err := e.svc.Update(ctx(), e.ana, h.ID, UpdateInput{Version: h.Version, Title: "Correr", Financial: &struct {
		TargetCents int64 `json:"target_cents"`
	}{TargetCents: 1000}})
	wantErr(t, err, ErrKindMismatch, "bloco financeiro num hábito")
	got, _ := e.svc.Get(ctx(), e.ana, h.ID)
	if got.Kind != KindHabit {
		t.Errorf("kind mudou: %s", got.Kind)
	}
}

func TestCreateValidation(t *testing.T) {
	e := newEnv(t)
	bad := func(what string, in CreateInput, want error) {
		t.Helper()
		_, err := e.svc.Create(ctx(), e.ana, in)
		wantErr(t, err, want, what)
	}
	ok := &HabitInput{TargetCount: 3, Period: PeriodWeek}
	bad("título vazio", CreateInput{Kind: KindHabit, Scope: ScopePersonal, Title: "  ", Habit: ok}, ErrInvalid)
	bad("título longo", CreateInput{Kind: KindHabit, Scope: ScopePersonal, Title: strings.Repeat("a", 121), Habit: ok}, ErrInvalid)
	bad("notas longas", CreateInput{Kind: KindHabit, Scope: ScopePersonal, Title: "x", Notes: strings.Repeat("a", 1001), Habit: ok}, ErrInvalid)
	bad("escopo", CreateInput{Kind: KindHabit, Scope: "x", Title: "x", Habit: ok}, ErrInvalid)
	bad("tipo", CreateInput{Kind: "x", Scope: ScopePersonal, Title: "x"}, ErrInvalid)
	bad("prazo no passado", CreateInput{Kind: KindHabit, Scope: ScopePersonal, Title: "x", DueOn: "2026-11-14", Habit: ok}, ErrInvalid)
	bad("prazo malformado", CreateInput{Kind: KindHabit, Scope: ScopePersonal, Title: "x", DueOn: "amanhã", Habit: ok}, ErrInvalid)
	bad("hábito do casal", CreateInput{Kind: KindHabit, Scope: ScopeHousehold, Title: "x", Habit: ok}, ErrInvalid)
	bad("hábito sem bloco", CreateInput{Kind: KindHabit, Scope: ScopePersonal, Title: "x"}, ErrInvalid)
	bad("8 vezes por semana", CreateInput{Kind: KindHabit, Scope: ScopePersonal, Title: "x", Habit: &HabitInput{TargetCount: 8, Period: PeriodWeek}}, ErrInvalid)
	bad("período errado", CreateInput{Kind: KindHabit, Scope: ScopePersonal, Title: "x", Habit: &HabitInput{TargetCount: 1, Period: "year"}}, ErrInvalid)
	bad("dias fora da máscara", CreateInput{Kind: KindHabit, Scope: ScopePersonal, Title: "x", Habit: &HabitInput{TargetCount: 1, Period: PeriodWeek, Weekdays: 128}}, ErrInvalid)
	bad("meta do casal pessoal", CreateInput{Kind: KindShared, Scope: ScopePersonal, Title: "x", Shared: &SharedInput{TargetCount: 1, Period: PeriodMonth}}, ErrInvalid)
	bad("dois blocos", CreateInput{Kind: KindHabit, Scope: ScopePersonal, Title: "x", Habit: ok, Shared: &SharedInput{TargetCount: 1, Period: PeriodMonth}}, ErrInvalid)
	bad("visibilidade do casal", CreateInput{Kind: KindShared, Scope: ScopeHousehold, Visibility: VisibilityPrivate, Title: "x", Shared: &SharedInput{TargetCount: 1, Period: PeriodMonth}}, ErrInvalid)
	bad("valor zero", CreateInput{Kind: KindFinancial, Scope: ScopePersonal, Title: "x", Financial: &FinancialInput{}}, ErrAmountInvalid)
	bad("valor acima do teto", CreateInput{Kind: KindFinancial, Scope: ScopePersonal, Title: "x", Financial: &FinancialInput{TargetCents: MaxAmountCents + 1}}, ErrAmountInvalid)
	bad("etapa de outro casal", CreateInput{Kind: KindProject, Scope: ScopeHousehold, Title: "x",
		Steps: []StepInput{{Title: "a", AssigneeID: "00000000-0000-4000-8000-000000000999"}}}, ErrInvalid)
	bad("tarefa em meta pessoal", CreateInput{Kind: KindProject, Scope: ScopePersonal, Title: "x",
		Steps: []StepInput{{Title: "a", Task: true}}}, ErrInvalid)
	bad("etapa do parceiro em meta pessoal", CreateInput{Kind: KindProject, Scope: ScopePersonal, Title: "x",
		Steps: []StepInput{{Title: "a", AssigneeID: e.beto.UserID}}}, ErrInvalid)
}

func TestStatesAndRules(t *testing.T) {
	e := newEnv(t)
	g := e.shared(e.ana, "Cinema", SharedInput{})

	paused, err := e.svc.Pause(ctx(), e.ana, g.ID)
	mustNoErr(t, err)
	if paused.Status != StatusPaused {
		t.Fatal(paused.Status)
	}
	if _, err := e.svc.Pause(ctx(), e.ana, g.ID); err != nil { // idempotente
		t.Errorf("pausar duas vezes: %v", err)
	}
	if r, err := e.svc.Resume(ctx(), e.beto, g.ID); err != nil || r.Status != StatusActive {
		t.Fatalf("retomar: %v %v", r.Status, err)
	}

	done, err := e.svc.Complete(ctx(), e.beto, g.ID)
	mustNoErr(t, err)
	if done.Status != StatusDone || done.CompletedAt == nil || done.CompletedBy == nil || *done.CompletedBy != e.beto.UserID {
		t.Fatalf("concluída: %+v", done)
	}

	// concluída não aceita novos registros
	_, err = e.svc.Checkin(ctx(), e.ana, g.ID, "")
	wantErr(t, err, ErrClosed, "marcar em meta concluída")

	// reabrir pelo /resume limpa quem concluiu
	re, err := e.svc.Resume(ctx(), e.ana, g.ID)
	mustNoErr(t, err)
	if re.Status != StatusActive || re.CompletedAt != nil || re.CompletedBy != nil {
		t.Errorf("reaberta: %+v", re)
	}

	// arquivada some da lista padrão e volta com ?status=archived; /resume não reabre arquivada
	_, err = e.svc.Archive(ctx(), e.ana, g.ID)
	mustNoErr(t, err)
	if e.list(e.ana, ListFilter{})[g.ID] {
		t.Error("arquivada não aparece na lista padrão")
	}
	if !e.list(e.ana, ListFilter{Status: StatusArchived})[g.ID] {
		t.Error("arquivada aparece com status=archived")
	}
	_, err = e.svc.Resume(ctx(), e.ana, g.ID)
	wantErr(t, err, ErrClosed, "retomar arquivada")
	_, err = e.svc.Checkin(ctx(), e.ana, g.ID, "")
	wantErr(t, err, ErrClosed, "marcar em meta arquivada")
	un, err := e.svc.Unarchive(ctx(), e.ana, g.ID)
	mustNoErr(t, err)
	if un.Status != StatusActive {
		t.Errorf("desarquivada: %s", un.Status)
	}
}

func TestActiveGoalCap(t *testing.T) {
	e := newEnv(t)
	for i := 0; i < MaxActiveGoals; i++ {
		if _, err := e.repo.CreateGoal(ctx(), NewGoal{Goal: Goal{
			HouseholdID: e.hh, CreatedBy: e.ana.UserID, Scope: ScopeHousehold, Visibility: VisibilityHousehold,
			Kind: KindProject, Title: "meta"}}); err != nil {
			t.Fatalf("meta %d: %v", i, err)
		}
	}
	_, err := e.svc.Create(ctx(), e.ana, CreateInput{Kind: KindProject, Scope: ScopeHousehold, Title: "a 101ª"})
	wantErr(t, err, ErrLimit, "passou do teto")

	// concluir libera espaço; reabrir quando está cheio recusa
	gs, _ := e.svc.List(ctx(), e.ana, ListFilter{})
	_, err = e.svc.Complete(ctx(), e.ana, gs[0].ID)
	mustNoErr(t, err)
	_, err = e.svc.Create(ctx(), e.ana, CreateInput{Kind: KindProject, Scope: ScopeHousehold, Title: "agora cabe"})
	mustNoErr(t, err)
	_, err = e.svc.Resume(ctx(), e.ana, gs[0].ID)
	wantErr(t, err, ErrLimit, "reabrir com o teto cheio")
}

func TestDeleteGoal(t *testing.T) {
	e := newEnv(t)
	g := e.shared(e.ana, "Cinema", SharedInput{})
	mustNoErr(t, e.svc.Delete(ctx(), e.beto, g.ID))
	_, err := e.svc.Get(ctx(), e.ana, g.ID)
	wantErr(t, err, ErrNotFound, "ler apagada")
	wantErr(t, e.svc.Delete(ctx(), e.ana, g.ID), ErrNotFound, "apagar duas vezes")
}
