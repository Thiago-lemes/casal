package finance

import (
	"testing"

	"casal/internal/db"
	"casal/internal/testutil"
)

func (e *env) leave(user, remaining string) {
	e.t.Helper()
	// o household tira a pessoa do casal e depois roda os ganchos, na mesma transação
	if _, err := e.db.Exec(`DELETE FROM household_members WHERE user_id = $1`, user); err != nil {
		e.t.Fatal(err)
	}
	testutil.RunHook(e.t, e.db, e.repo.HandleLeave, db.Handoff{UserID: user, From: e.hh, RemainingUserID: remaining})
}

func (e *env) join(user, to string) {
	e.t.Helper()
	testutil.RunHook(e.t, e.db, e.repo.HandleJoin, db.Handoff{UserID: user, To: to})
}

func TestLeavingWithBalanceCreatesLeaveSettlementAndLocksFutureInstallments(t *testing.T) {
	e := newEnv(t)
	e.spend(e.ana, 10000, "2026-11-01")
	res, err := e.svc.CreateExpense(ctx(), e.ana, ExpenseInput{
		AmountCents: 30000, CategoryID: e.cat(KeyOther), SpentOn: "2026-11-10", Installments: &InstallmentsInput{Count: 3},
	})
	mustNoErr(t, err)
	group := res.Expenses[0].Installment.GroupID

	e.leave(e.beto.UserID, e.ana.UserID)

	list, _ := e.svc.ListSettlements(ctx(), e.ana)
	// Ana pagou 10000 + 30000 (todas as parcelas, inclusive as futuras): Beto deixa de dever 20000
	if len(list) != 1 || list[0].Kind != "leave" || list[0].FromUser != e.beto.UserID || list[0].ToUser != e.ana.UserID || list[0].AmountCents != 20000 {
		t.Fatalf("acerto de saída: %+v", list)
	}
	g, _ := e.svc.GetGroup(ctx(), e.ana, group)
	if g.Settled != 3 {
		t.Errorf("o acerto de saída trava também as parcelas futuras: %+v", g)
	}
	if open, _ := e.repo.OpenExpenses(ctx(), e.hh, "2099-12-31"); len(open) != 0 {
		t.Errorf("nada em aberto depois da saída: %d", len(open))
	}
}

func TestLeavingWithoutBalanceCreatesNoSettlement(t *testing.T) {
	e := newEnv(t)
	e.spend(e.ana, 10000, "2026-11-01")
	e.spend(e.beto, 10000, "2026-11-02")
	e.leave(e.beto.UserID, e.ana.UserID)
	if list, _ := e.svc.ListSettlements(ctx(), e.ana); len(list) != 0 {
		t.Errorf("saldo zero: sem acerto de saída: %+v", list)
	}
}

func TestLeavingHandsResponsibilitiesToWhoStaysAndResetsSplit(t *testing.T) {
	e := newEnv(t)
	e.enableModule()
	e.proportional()
	r, err := e.svc.CreateRecurring(ctx(), e.ana, RecurringInput{AmountCents: 5000, CategoryID: e.cat(KeyBills), DayOfMonth: 5, StartOn: "2026-11-01", PaidBy: e.beto.UserID})
	mustNoErr(t, err)
	b := e.bill(BillInput{Title: "Luz", DueOn: "2026-11-20", ResponsibleID: e.beto.UserID})

	e.leave(e.beto.UserID, e.ana.UserID)

	rec, _ := e.repo.GetRecurring(ctx(), e.hh, r.ID, e.ana.UserID)
	if rec.PaidBy != e.ana.UserID || rec.Split[e.ana.UserID] != 10000 || len(rec.Split) != 1 {
		t.Errorf("o fixo passa para quem ficou, só com ela na divisão: %+v", rec)
	}
	bill, _ := e.svc.GetBill(ctx(), e.ana, b.ID)
	if bill.ResponsibleID != e.ana.UserID {
		t.Errorf("a conta passa para quem ficou: %+v", bill)
	}
	// o job refaz o lembrete: o do Beto some (ele não é mais membro) e nasce o da Ana
	e.people.mu.Lock()
	e.people.ps = e.people.ps[:1]
	e.people.mu.Unlock()
	_, err = e.svc.ReconcileReminders(ctx())
	mustNoErr(t, err)
	for ref, rem := range e.rem.active() {
		if rem.Recipient != e.ana.UserID {
			t.Errorf("lembrete %s ficou com quem saiu", ref)
		}
	}
	if len(e.rem.active()) != 2 {
		t.Errorf("a Ana recebe os dois lembretes: %v", refsOf(e.rem.active()))
	}
	st, _ := e.repo.GetSettings(ctx(), e.hh)
	if st.Mode != ModeEqual || len(st.Consent) != 0 {
		t.Errorf("a divisão volta para igual: %+v", st)
	}
}

func TestPersonalDataFollowsThePersonAndArrivesPrivate(t *testing.T) {
	e := newEnv(t)
	e.enableModule()
	mine := e.personal(e.beto, 1234, VisView)
	res, err := e.svc.CreateExpense(ctx(), e.beto, ExpenseInput{
		Scope: ScopePersonal, AmountCents: 6000, CategoryID: e.cat(KeyLeisure), Installments: &InstallmentsInput{Count: 2},
	})
	mustNoErr(t, err)
	fixo, err := e.svc.CreateRecurring(ctx(), e.beto, RecurringInput{Scope: ScopePersonal, Visibility: VisView, AmountCents: 2990, CategoryID: e.cat(KeyLeisure), DayOfMonth: 20, StartOn: "2026-11-01"})
	mustNoErr(t, err)
	custom, _ := e.svc.CreateCategory(ctx(), e.beto, "Pets")
	pet, err := e.svc.CreateExpense(ctx(), e.beto, ExpenseInput{Scope: ScopePersonal, AmountCents: 800, CategoryID: custom.ID})
	mustNoErr(t, err)
	_, err = e.svc.SetIncome(ctx(), e.beto, betoIncome)
	mustNoErr(t, err)
	shared := e.spend(e.ana, 1000, "2026-11-02")

	e.leave(e.beto.UserID, e.ana.UserID)

	// quem ficou não vê mais o "view" do Beto: virou privado
	if _, err := e.svc.GetExpense(ctx(), e.ana, mine.ID); !errorsIs(err, ErrExpenseNotFound) {
		t.Errorf("o que a Ana lia vira privado: %v", err)
	}
	if _, err := e.svc.GetExpense(ctx(), e.ana, shared.ID); err != nil {
		t.Errorf("o gasto do casal fica: %v", err)
	}
	if _, err := e.repo.GetRecurring(ctx(), e.hh, fixo.ID, e.ana.UserID); !errorsIs(err, ErrRecurringNotFound) {
		t.Errorf("o fixo pessoal virou privado: %v", err)
	}

	// o Beto cria outro casal: o que era dele vem junto, privado, com as categorias que usava
	hh2 := testutil.NewHousehold(t, e.db, e.beto.UserID)
	e.join(e.beto.UserID, hh2)
	p2 := e.beto
	p2.HouseholdID = hh2

	page, err := e.svc.ListExpenses(ctx(), p2, ExpenseFilter{Scope: "mine"})
	mustNoErr(t, err)
	if len(page.Expenses) != 4 { // 1 avulso + 2 parcelas + 1 de Pets
		t.Fatalf("os pessoais acompanham a pessoa: %d", len(page.Expenses))
	}
	cats, _ := e.svc.ListCategories(ctx(), p2, true)
	valid := map[string]string{}
	for _, c := range cats {
		valid[c.ID] = c.Name
	}
	for _, x := range page.Expenses {
		if x.Visibility != VisPrivate || x.HouseholdID != hh2 {
			t.Errorf("chega privado e no casal novo: %+v", x)
		}
		if _, ok := valid[x.CategoryID]; !ok {
			t.Errorf("a categoria precisa ser do casal novo: %+v", x)
		}
	}
	_ = pet
	for _, x := range page.Expenses {
		if x.AmountCents == 800 && valid[x.CategoryID] != "Pets" {
			t.Errorf("a categoria própria (Pets) deveria ter sido levada: %q", valid[x.CategoryID])
		}
	}
	if g, err := e.svc.GetGroup(ctx(), p2, res.Expenses[0].Installment.GroupID); err != nil || len(g.Expenses) != 2 {
		t.Errorf("as parcelas pessoais vêm juntas: %v", err)
	}
	if rs, _ := e.svc.ListRecurring(ctx(), p2); len(rs) != 1 || rs[0].Visibility != VisPrivate {
		t.Errorf("o fixo pessoal vem privado: %+v", rs)
	}
	// a renda acompanha a pessoa
	if c, _ := e.svc.GetIncome(ctx(), p2); c == nil || *c != betoIncome {
		t.Errorf("a renda acompanha quem sai: %v", c)
	}
	if c, _ := e.svc.GetIncome(ctx(), e.ana); c != nil {
		t.Errorf("a Ana não herda renda nenhuma: %v", *c)
	}
	// e nada dele ficou no casal antigo
	old, _ := e.svc.ListExpenses(ctx(), e.ana, ExpenseFilter{Scope: "mine"})
	if len(old.Expenses) != 0 {
		t.Errorf("o casal antigo não guarda o pessoal de quem saiu: %+v", old.Expenses)
	}
}
