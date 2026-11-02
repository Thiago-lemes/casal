package goal

import (
	"testing"
)

func (e *env) pay(goalID string, cents int64) ContributionResult {
	e.t.Helper()
	r, err := e.svc.AddContribution(ctx(), e.ana, goalID, ContributionInput{AmountCents: cents})
	mustNoErr(e.t, err)
	return r
}

func TestContributionsSumExactlyInCentsAndRefuseOverdraw(t *testing.T) {
	e := newEnv(t)
	g := e.saving(e.ana, ScopePersonal, "Reserva", 100000)

	e.pay(g.ID, 3333)
	e.pay(g.ID, 3333)
	r := e.pay(g.ID, 3334)
	if r.Goal.Progress.Done != 10000 || r.Goal.Progress.Percent != 10 || r.Goal.Progress.Total != 100000 {
		t.Fatalf("progresso: %+v", r.Goal.Progress)
	}

	// retirada maior que o guardado: recusada, e nada muda
	_, err := e.svc.AddContribution(ctx(), e.ana, g.ID, ContributionInput{AmountCents: -10001})
	wantErr(t, err, ErrContributionNegative, "retirada acima do guardado")
	// retirada exata zera, sem ficar negativa
	r, err = e.svc.AddContribution(ctx(), e.ana, g.ID, ContributionInput{AmountCents: -10000, Note: "conserto do carro"})
	mustNoErr(t, err)
	if r.Goal.Progress.Done != 0 || r.Contribution.AmountCents != -10000 {
		t.Errorf("depois da retirada: %+v", r.Goal.Progress)
	}

	_, err = e.svc.AddContribution(ctx(), e.ana, g.ID, ContributionInput{AmountCents: 0})
	wantErr(t, err, ErrAmountInvalid, "valor zero")
	_, err = e.svc.AddContribution(ctx(), e.ana, g.ID, ContributionInput{AmountCents: MaxAmountCents + 1})
	wantErr(t, err, ErrAmountInvalid, "valor acima do teto")
	_, err = e.svc.AddContribution(ctx(), e.ana, g.ID, ContributionInput{AmountCents: 100, MadeOn: "2026-11-16"})
	wantErr(t, err, ErrInvalid, "aporte no futuro")
}

func TestDeletingAContributionRecalculatesAndKeepsTheBalanceNonNegative(t *testing.T) {
	e := newEnv(t)
	g := e.saving(e.ana, ScopePersonal, "Reserva", 100000)
	a := e.pay(g.ID, 8000)
	e.pay(g.ID, 2000)
	w, err := e.svc.AddContribution(ctx(), e.ana, g.ID, ContributionInput{AmountCents: -7000})
	mustNoErr(t, err)

	// apagar o aporte de 8.000 deixaria a retirada de 7.000 sem saldo (2.000 − 7.000)
	_, err = e.svc.DeleteContribution(ctx(), e.ana, g.ID, a.Contribution.ID)
	wantErr(t, err, ErrContributionNegative, "apagar aporte que sustenta uma retirada")

	got, err := e.svc.DeleteContribution(ctx(), e.ana, g.ID, w.Contribution.ID)
	mustNoErr(t, err)
	if got.Progress.Done != 10000 {
		t.Errorf("apagar a retirada devolve o saldo: %d", got.Progress.Done)
	}
	got, err = e.svc.DeleteContribution(ctx(), e.ana, g.ID, a.Contribution.ID)
	mustNoErr(t, err)
	if got.Progress.Done != 2000 {
		t.Errorf("depois de apagar o aporte: %d", got.Progress.Done)
	}
	_, err = e.svc.DeleteContribution(ctx(), e.ana, g.ID, a.Contribution.ID)
	wantErr(t, err, ErrContributionNotFound, "apagar duas vezes")
}

func TestOnlyTheAuthorOrOwnerDeletesAContribution(t *testing.T) {
	e := newEnv(t)
	g := e.saving(e.ana, ScopeHousehold, "Casa nova", 500000)
	c, err := e.svc.AddContribution(ctx(), e.ana, g.ID, ContributionInput{AmountCents: 10000})
	mustNoErr(t, err)
	// numa meta do casal, o parceiro vê (e aporta), mas não apaga o aporte de quem o registrou
	_, err = e.svc.AddContribution(ctx(), e.beto, g.ID, ContributionInput{AmountCents: 500})
	mustNoErr(t, err)
	_, err = e.svc.DeleteContribution(ctx(), e.beto, g.ID, c.Contribution.ID)
	wantErr(t, err, ErrForbidden, "apagar aporte do parceiro")
	_, err = e.svc.DeleteContribution(ctx(), e.ana, g.ID, c.Contribution.ID)
	mustNoErr(t, err)

	page, err := e.svc.ListContributions(ctx(), e.ana, g.ID, "", 10)
	mustNoErr(t, err)
	if len(page.Items) != 1 || page.Items[0].CreatedBy != e.beto.UserID || page.Items[0].CreatedByName == "" {
		t.Errorf("a lista mostra quem registrou: %+v", page.Items)
	}
}

func TestContributionsPaginateNewestFirst(t *testing.T) {
	e := newEnv(t)
	g := e.saving(e.ana, ScopePersonal, "Reserva", 100000)
	for i := int64(1); i <= 5; i++ {
		_, err := e.svc.AddContribution(ctx(), e.ana, g.ID, ContributionInput{AmountCents: i * 100, MadeOn: "2026-11-0" + string(rune('0'+i))})
		mustNoErr(t, err)
	}
	p1, err := e.svc.ListContributions(ctx(), e.ana, g.ID, "", 2)
	mustNoErr(t, err)
	if len(p1.Items) != 2 || p1.Items[0].AmountCents != 500 || p1.Items[1].AmountCents != 400 || p1.NextBefore == "" {
		t.Fatalf("página 1: %+v", p1)
	}
	p2, err := e.svc.ListContributions(ctx(), e.ana, g.ID, p1.NextBefore, 2)
	mustNoErr(t, err)
	if len(p2.Items) != 2 || p2.Items[0].AmountCents != 300 || p2.Items[1].AmountCents != 200 {
		t.Fatalf("página 2: %+v", p2)
	}
	p3, _ := e.svc.ListContributions(ctx(), e.ana, g.ID, p2.NextBefore, 2)
	if len(p3.Items) != 1 || p3.NextBefore != "" {
		t.Errorf("página 3: %+v", p3)
	}
}

func TestContributionRulesByKindAndState(t *testing.T) {
	e := newEnv(t)
	h := e.habit(e.ana, "Correr", HabitInput{})
	_, err := e.svc.AddContribution(ctx(), e.ana, h.ID, ContributionInput{AmountCents: 100})
	wantErr(t, err, ErrKindMismatch, "aporte em hábito")

	g := e.saving(e.ana, ScopePersonal, "Reserva", 1000)
	e.pay(g.ID, 500)
	_, err = e.svc.Complete(ctx(), e.ana, g.ID)
	mustNoErr(t, err)
	_, err = e.svc.AddContribution(ctx(), e.ana, g.ID, ContributionInput{AmountCents: 100})
	wantErr(t, err, ErrClosed, "aporte em meta concluída")
}

func TestPaceRequiredPerMonthAndSuggestComplete(t *testing.T) {
	e := newEnv(t)
	// hoje 15/11/2026; prazo 15/02/2027 = 3 meses inteiros
	g, err := e.svc.Create(ctx(), e.ana, CreateInput{Kind: KindFinancial, Scope: ScopePersonal, Title: "Viagem", DueOn: "2027-02-15",
		Financial: &FinancialInput{TargetCents: 100000}})
	mustNoErr(t, err)
	if g.Progress.RequiredPerMonthCents == nil || *g.Progress.RequiredPerMonthCents != 33334 {
		t.Fatalf("por mês: %+v", g.Progress)
	}
	if g.Progress.OnTrack == nil || *g.Progress.OnTrack != true { // 0 guardado no dia 0 do prazo: no ritmo
		t.Errorf("no primeiro dia está no ritmo: %+v", g.Progress.OnTrack)
	}
	// um mês depois, sem guardar nada, está atrasado
	e.now = e.now.AddDate(0, 1, 0)
	g, _ = e.svc.Get(ctx(), e.ana, g.ID)
	if g.Progress.OnTrack == nil || *g.Progress.OnTrack {
		t.Errorf("sem guardar nada em um terço do prazo: %+v", g.Progress.OnTrack)
	}
	if g.Progress.RequiredPerMonthCents == nil || *g.Progress.RequiredPerMonthCents != 50000 {
		t.Errorf("faltam 2 meses: %+v", g.Progress.RequiredPerMonthCents)
	}

	// bateu a meta: o app sugere concluir, não conclui sozinho
	r := e.pay(g.ID, 100000)
	if !r.Goal.Progress.SuggestComplete || r.Goal.Status != StatusActive || r.Goal.Progress.Percent != 100 {
		t.Errorf("sugestão: %+v status=%s", r.Goal.Progress, r.Goal.Status)
	}
	if r.Goal.Progress.RequiredPerMonthCents != nil {
		t.Error("meta batida não tem quanto guardar por mês")
	}
	// passou da meta: a barra fica em 100
	r = e.pay(g.ID, 5000)
	if r.Goal.Progress.Percent != 100 || r.Goal.Progress.Done != 105000 {
		t.Errorf("acima da meta: %+v", r.Goal.Progress)
	}
	// sem prazo, não há ritmo
	n := e.saving(e.ana, ScopePersonal, "Sem prazo", 1000)
	if n.Progress.OnTrack != nil || n.Progress.RequiredPerMonthCents != nil {
		t.Errorf("sem prazo: %+v", n.Progress)
	}
}

func (e *env) budgetGoal(cat string, baseline string) (Goal, error) {
	return e.svc.Create(ctx(), e.ana, CreateInput{Kind: KindFinancial, Scope: ScopeHousehold, Title: "Economizar no mercado",
		Financial: &FinancialInput{TargetCents: 200000, Source: SourceBudgetSavings, CategoryID: cat, BaselineOn: baseline}})
}

func TestBudgetSavingsReadsOnlyHouseholdSpendingAndMarksTheCurrentMonthProvisional(t *testing.T) {
	e := newEnv(t)
	cat := e.category("Mercado")
	e.spend.limit[cat] = 50000
	e.spend.spent[cat] = map[string]int64{"2026-09": 30000, "2026-10": 80000, "2026-11": 20000}

	g, err := e.budgetGoal(cat, "2026-09-01")
	mustNoErr(t, err)
	// set sobra 20.000, out estourou (0, nunca negativo), nov sobra 30.000 até agora
	if g.Progress.Done != 50000 || !g.Progress.Provisional || g.Progress.Unavailable || g.Progress.Percent != 25 {
		t.Fatalf("economia: %+v", g.Progress)
	}
	if g.Financial.Source != SourceBudgetSavings || g.Financial.CategoryID == nil {
		t.Errorf("parâmetros: %+v", g.Financial)
	}

	// gasto novo no mês corrente derruba a economia (por isso é provisório)
	e.spend.spent[cat]["2026-11"] = 45000
	g, _ = e.svc.Get(ctx(), e.beto, g.ID)
	if g.Progress.Done != 25000 {
		t.Errorf("economia depois do gasto: %d", g.Progress.Done)
	}

	// aporte não existe nesta forma de medir
	_, err = e.svc.AddContribution(ctx(), e.ana, g.ID, ContributionInput{AmountCents: 100})
	wantErr(t, err, ErrKindMismatch, "aporte em meta de economia do orçamento")

	// Finanças desligado: a meta fica, o progresso fica indisponível e volta quando religa
	e.mods.set("finance", false)
	g, err = e.svc.Get(ctx(), e.ana, g.ID)
	mustNoErr(t, err)
	if !g.Progress.Unavailable || g.Progress.Done != 0 {
		t.Errorf("com Finanças desligado: %+v", g.Progress)
	}
	e.mods.set("finance", true)
	g, _ = e.svc.Get(ctx(), e.ana, g.ID)
	if g.Progress.Unavailable || g.Progress.Done != 25000 {
		t.Errorf("religado: %+v", g.Progress)
	}
}

func TestBudgetSavingsPreconditions(t *testing.T) {
	e := newEnv(t)
	cat := e.category("Lazer")

	// categoria sem limite
	_, err := e.budgetGoal(cat, "")
	wantErr(t, err, ErrInvalid, "categoria sem limite de orçamento")

	e.spend.limit[cat] = 10000
	// meta pessoal não lê o gasto do casal
	_, err = e.svc.Create(ctx(), e.ana, CreateInput{Kind: KindFinancial, Scope: ScopePersonal, Title: "x",
		Financial: &FinancialInput{TargetCents: 1000, Source: SourceBudgetSavings, CategoryID: cat}})
	wantErr(t, err, ErrInvalid, "economia do orçamento numa meta pessoal")

	// Finanças desligado
	e.mods.set("finance", false)
	_, err = e.budgetGoal(cat, "")
	wantErr(t, err, ErrModuleRequired, "Finanças desligado")
	e.mods.set("finance", true)

	// categoria só com aportes
	_, err = e.svc.Create(ctx(), e.ana, CreateInput{Kind: KindFinancial, Scope: ScopeHousehold, Title: "x",
		Financial: &FinancialInput{TargetCents: 1000, CategoryID: cat}})
	wantErr(t, err, ErrInvalid, "category_id em meta de aportes")

	// funciona
	_, err = e.budgetGoal(cat, "")
	mustNoErr(t, err)
}
