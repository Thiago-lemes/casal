package goal

import (
	"strings"
	"testing"

	"casal/internal/db"
	"casal/internal/identity"
	"casal/internal/testutil"
)

type populatedMove struct {
	habit, view, privProject, sharedBoth, houseProject, houseFinancial Goal
}

func populateMove(t *testing.T, e *env) populatedMove {
	t.Helper()
	var m populatedMove
	m.habit = e.habit(e.ana, "Correr", HabitInput{TargetCount: 3})
	e.mark(e.ana, m.habit.ID, "2026-11-10")
	var err error
	m.view, err = e.svc.Create(ctx(), e.ana, CreateInput{Kind: KindFinancial, Scope: ScopePersonal, Visibility: VisibilityView,
		Title: "Viagem", Financial: &FinancialInput{TargetCents: 100000}})
	mustNoErr(t, err)
	e.pay(m.view.ID, 25000)
	m.privProject = e.project(e.ana, ScopePersonal, "Tese", StepInput{Title: "Capítulo 1"}, StepInput{Title: "Capítulo 2"})

	m.sharedBoth = e.shared(e.ana, "Jantar fora", SharedInput{TargetCount: 2, RequiresBoth: true})
	e.mark(e.ana, m.sharedBoth.ID, "2026-11-10")
	e.mark(e.beto, m.sharedBoth.ID, "2026-11-10")
	e.mark(e.ana, m.sharedBoth.ID, "2026-11-12") // só ela: não conta enquanto exigir os dois

	m.houseProject = e.project(e.ana, ScopeHousehold, "Reforma",
		StepInput{Title: "da Ana", AssigneeID: e.ana.UserID}, StepInput{Title: "do Beto", AssigneeID: e.beto.UserID})
	m.houseFinancial = e.saving(e.ana, ScopeHousehold, "Casa nova", 500000)
	e.pay(m.houseFinancial.ID, 10000)
	return m
}

func TestLeavingTheHouseholdKeepsHouseholdGoalsAndTakesPersonalOnes(t *testing.T) {
	e := newEnv(t)
	m := populateMove(t, e)
	if g, _ := e.svc.Get(ctx(), e.ana, m.sharedBoth.ID); g.Progress.Done != 1 {
		t.Fatalf("antes de sair, exigindo os dois, só 1 dia conta: %+v", g.Progress)
	}

	testutil.RunHook(t, e.db, e.repo.HandleLeave, db.Handoff{UserID: e.ana.UserID, From: e.hh, RemainingUserID: e.beto.UserID})
	e.people.ps = e.people.ps[1:] // o casal agora só tem o Beto

	// a meta do casal continua com o Beto, com as marcações dos dois, e deixa de exigir os dois
	g, err := e.svc.Get(ctx(), e.beto, m.sharedBoth.ID)
	mustNoErr(t, err)
	if g.Shared.RequiresBoth {
		t.Error("requires_both deveria virar false para a meta não travar")
	}
	// o dia 12 (só da Ana, que saiu) não conta para o casal de uma pessoa; o dia 10 sim, e o Beto marcou
	if g.Progress.Done != 1 {
		t.Errorf("progresso depois de sair: %+v", g.Progress)
	}
	h, err := e.svc.History(ctx(), e.beto, m.sharedBoth.ID, "2026-11-01", "2026-11-30")
	mustNoErr(t, err)
	if len(h.Days) != 2 {
		t.Errorf("as marcações de quem saiu ficam no histórico: %+v", h.Days)
	}

	// etapas da Ana passam para o Beto
	proj, _ := e.svc.Get(ctx(), e.beto, m.houseProject.ID)
	for _, s := range proj.Steps {
		if s.AssigneeID == nil || *s.AssigneeID != e.beto.UserID {
			t.Errorf("etapa %q deveria ser do Beto: %v", s.Title, s.AssigneeID)
		}
	}
	// aportes do casal ficam
	fin, _ := e.svc.Get(ctx(), e.beto, m.houseFinancial.ID)
	if fin.Progress.Done != 10000 {
		t.Errorf("aportes do casal: %+v", fin.Progress)
	}

	// as pessoais da Ana somem para o Beto, inclusive a que ele lia ("view" virou privada)
	for _, id := range []string{m.habit.ID, m.view.ID, m.privProject.ID} {
		_, err := e.svc.Get(ctx(), e.beto, id)
		wantErr(t, err, ErrNotFound, "meta pessoal de quem saiu")
	}
	var vis string
	testutil.SystemScan(t, e.db, `SELECT visibility FROM goals WHERE id = $1`, []any{m.view.ID}, &vis)
	if vis != VisibilityPrivate {
		t.Errorf("a meta view deveria virar private: %s", vis)
	}
	// e continuam do dono, no casal que ele deixou, até ele entrar em outro
	var owner string
	testutil.SystemScan(t, e.db, `SELECT owner_id FROM goals WHERE id = $1`, []any{m.habit.ID}, &owner)
	if owner != e.ana.UserID {
		t.Errorf("dono: %s", owner)
	}
}

func TestLeavingAloneStillMakesViewGoalsPrivate(t *testing.T) {
	e := newEnv(t)
	m := populateMove(t, e)
	testutil.RunHook(t, e.db, e.repo.HandleLeave, db.Handoff{UserID: e.ana.UserID, From: e.hh})
	var vis string
	testutil.SystemScan(t, e.db, `SELECT visibility FROM goals WHERE id = $1`, []any{m.view.ID}, &vis)
	if vis != VisibilityPrivate {
		t.Errorf("visibilidade: %s", vis)
	}
}

func TestJoiningAnotherHouseholdBringsPersonalGoalsAlwaysPrivate(t *testing.T) {
	e := newEnv(t)
	m := populateMove(t, e)
	testutil.RunHook(t, e.db, e.repo.HandleLeave, db.Handoff{UserID: e.ana.UserID, From: e.hh, RemainingUserID: e.beto.UserID})

	carla := testutil.NewUser(t, e.db, "carla")
	hh2 := testutil.NewHousehold(t, e.db, carla)
	testutil.RunHook(t, e.db, e.repo.HandleJoin, db.Handoff{UserID: e.ana.UserID, To: hh2})

	ana2 := identity.Principal{UserID: e.ana.UserID, HouseholdID: hh2, Timezone: "America/Sao_Paulo"}
	got := e.list(ana2, ListFilter{Status: "all"})
	for _, id := range []string{m.habit.ID, m.view.ID, m.privProject.ID} {
		if !got[id] {
			t.Errorf("a meta pessoal %s deveria vir junto", id)
		}
	}
	for _, id := range []string{m.sharedBoth.ID, m.houseProject.ID, m.houseFinancial.ID} {
		if got[id] {
			t.Errorf("a meta do casal %s não acompanha a pessoa", id)
		}
	}
	// tudo o que é dela veio junto: parâmetros, aportes, marcações e etapas
	fin, err := e.svc.Get(ctx(), ana2, m.view.ID)
	mustNoErr(t, err)
	if fin.Progress.Done != 25000 || fin.Visibility != VisibilityPrivate {
		t.Errorf("meta financeira no casal novo: %+v vis=%s", fin.Progress, fin.Visibility)
	}
	proj, _ := e.svc.Get(ctx(), ana2, m.privProject.ID)
	if len(proj.Steps) != 2 {
		t.Errorf("etapas: %+v", proj.Steps)
	}
	habit, _ := e.svc.Get(ctx(), ana2, m.habit.ID)
	if habit.Progress.Done != 1 {
		t.Errorf("marcações: %+v", habit.Progress)
	}
	for _, table := range []string{"goal_habit", "goal_financial", "goal_contributions", "goal_checkins", "goal_steps"} {
		var left int
		testutil.SystemScan(t, e.db, `SELECT count(*) FROM `+table+` WHERE household_id = $1 AND goal_id = ANY($2)`,
			[]any{e.hh, []string{m.habit.ID, m.view.ID, m.privProject.ID}}, &left)
		if left != 0 {
			t.Errorf("%s ficou %d linha(s) no casal antigo", table, left)
		}
	}
	// o parceiro do casal antigo não vê nada disso
	for _, id := range []string{m.habit.ID, m.view.ID, m.privProject.ID} {
		_, err := e.svc.Get(ctx(), e.beto, id)
		wantErr(t, err, ErrNotFound, "o parceiro antigo")
	}
	// entrar de novo (sem nada para trazer) não quebra
	testutil.RunHook(t, e.db, e.repo.HandleJoin, db.Handoff{UserID: e.ana.UserID, To: hh2})
}

func TestExportOnlyBringsWhatThePersonCanSee(t *testing.T) {
	e := newEnv(t)
	m := populateMove(t, e)
	// o Beto tem uma privada que a Ana não pode ver
	secret := e.habit(e.beto, "Segredo do Beto", HabitInput{})
	e.mark(e.beto, secret.ID, "2026-11-10")

	anaEx, err := e.svc.Export(ctx(), e.ana)
	mustNoErr(t, err)
	got := map[string]ExportGoal{}
	for _, g := range anaEx.Goals {
		got[g.Goal.ID] = g
	}
	if _, leaked := got[secret.ID]; leaked {
		t.Fatal("a exportação da Ana trouxe a meta privada do Beto")
	}
	for _, id := range []string{m.habit.ID, m.view.ID, m.privProject.ID, m.sharedBoth.ID, m.houseProject.ID, m.houseFinancial.ID} {
		if _, ok := got[id]; !ok {
			t.Errorf("faltou a meta %s na exportação da Ana", id)
		}
	}
	if len(got[m.view.ID].Contributions) != 1 || len(got[m.habit.ID].Checkins) != 1 || len(got[m.sharedBoth.ID].Checkins) != 3 {
		t.Errorf("aportes e marcações: %+v %+v", got[m.view.ID], got[m.sharedBoth.ID])
	}
	if len(got[m.privProject.ID].Goal.Steps) != 2 {
		t.Errorf("etapas: %+v", got[m.privProject.ID].Goal.Steps)
	}

	betoEx, _ := e.svc.Export(ctx(), e.beto)
	for _, g := range betoEx.Goals {
		switch g.Goal.ID {
		case m.habit.ID, m.privProject.ID:
			t.Errorf("a exportação do Beto trouxe a privada da Ana: %s", g.Goal.Title)
		case m.view.ID:
			if len(g.Contributions) != 0 || g.Goal.Notes != "" || g.Goal.Financial != nil {
				t.Errorf("meta view deveria vir só como resumo: %+v", g)
			}
		}
	}
}

func TestExportCSVNeutralizesFormulas(t *testing.T) {
	e := newEnv(t)
	g := e.saving(e.ana, ScopePersonal, `=HYPERLINK("http://x")`, 1000)
	e.pay(g.ID, 500)
	ex, err := e.svc.Export(ctx(), e.ana)
	mustNoErr(t, err)
	var sb strings.Builder
	mustNoErr(t, ex.WriteCSV(&sb))
	out := sb.String()
	if !strings.HasPrefix(out, "record,goal_id,kind,scope,status,title,date,user_id,amount_cents,text\n") {
		t.Errorf("cabeçalho: %q", strings.SplitN(out, "\n", 2)[0])
	}
	if strings.Contains(out, ",=HYPERLINK") || !strings.Contains(out, "'=HYPERLINK") {
		t.Errorf("fórmula não neutralizada:\n%s", out)
	}
	if !strings.Contains(out, "contribution,"+g.ID) || !strings.Contains(out, ",500,") {
		t.Errorf("aporte ausente:\n%s", out)
	}
}
