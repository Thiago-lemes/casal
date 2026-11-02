package finance

import (
	"testing"
)

// populate cria dados do casal em todas as tabelas de Finanças e devolve os ids.
type populated struct {
	expense, group, recurring, bill, category, settlement string
}

func populate(t *testing.T, e *env) populated {
	t.Helper()
	e.enableModule()
	var p populated
	x := e.spend(e.ana, 10000, "2026-11-01")
	p.expense = x.ID
	res, err := e.svc.CreateExpense(ctx(), e.ana, ExpenseInput{AmountCents: 20000, CategoryID: e.cat(KeyOther), SpentOn: "2026-11-15", Installments: &InstallmentsInput{Count: 2}})
	mustNoErr(t, err)
	p.group = res.Expenses[0].Installment.GroupID
	r, err := e.svc.CreateRecurring(ctx(), e.ana, rentInput(e, 5))
	mustNoErr(t, err)
	p.recurring = r.ID
	p.bill = e.bill(BillInput{Title: "Luz", DueOn: "2026-11-20"}).ID
	c, err := e.svc.CreateCategory(ctx(), e.ana, "Pets")
	mustNoErr(t, err)
	p.category = c.ID
	e.setBudget(KeyGroceries, 10000) // 100% com o gasto de 10000 já lançado? não: o alerta nasce no próximo gasto
	e.spend(e.ana, 9000, "2026-11-02")
	_, err = e.svc.SetIncome(ctx(), e.ana, anaIncome)
	mustNoErr(t, err)
	_, err = e.svc.SaveSettings(ctx(), e.ana, ModeFixed, map[string]int{e.ana.UserID: 6000, e.beto.UserID: 4000})
	mustNoErr(t, err)
	st, err := e.svc.Settle(ctx(), e.beto, "2026-11-02", 9500)
	mustNoErr(t, err)
	p.settlement = st.ID
	return p
}

func TestHouseholdsAreIsolated(t *testing.T) {
	a, b := newEnv(t), newEnv(t)
	pa := populate(t, a)

	// o casal B usa os ids do casal A e não acha nada: 404 em tudo
	_, err := b.svc.GetExpense(ctx(), b.ana, pa.expense)
	wantErr(t, err, ErrExpenseNotFound, "ler gasto de outro casal")
	_, err = b.svc.UpdateExpense(ctx(), b.ana, pa.expense, ExpenseInput{AmountCents: 1, CategoryID: b.cat(KeyOther), Version: 1})
	wantErr(t, err, ErrExpenseNotFound, "editar gasto de outro casal")
	wantErr(t, b.svc.DeleteExpense(ctx(), b.ana, pa.expense), ErrExpenseNotFound, "apagar gasto de outro casal")
	_, err = b.svc.GetGroup(ctx(), b.ana, pa.group)
	wantErr(t, err, ErrExpenseNotFound, "ler parcelas de outro casal")
	desc := "invadido"
	_, err = b.svc.UpdateGroup(ctx(), b.ana, pa.group, GroupInput{Description: &desc})
	wantErr(t, err, ErrExpenseNotFound, "editar parcelas de outro casal")
	_, err = b.svc.DeleteGroup(ctx(), b.ana, pa.group)
	wantErr(t, err, ErrExpenseNotFound, "apagar parcelas de outro casal")

	_, err = b.svc.UpdateRecurring(ctx(), b.ana, pa.recurring, rentInput(b, 5))
	wantErr(t, err, ErrRecurringNotFound, "editar fixo de outro casal")
	wantErr(t, b.svc.DeleteRecurring(ctx(), b.ana, pa.recurring), ErrRecurringNotFound, "apagar fixo de outro casal")
	_, err = b.svc.SetRecurringActive(ctx(), b.ana, pa.recurring, false)
	wantErr(t, err, ErrRecurringNotFound, "pausar fixo de outro casal")

	_, err = b.svc.GetBill(ctx(), b.ana, pa.bill)
	wantErr(t, err, ErrBillNotFound, "ler conta de outro casal")
	_, err = b.svc.UpdateBill(ctx(), b.ana, pa.bill, BillInput{Title: "x", DueOn: "2026-12-01"})
	wantErr(t, err, ErrBillNotFound, "editar conta de outro casal")
	_, err = b.svc.PayBill(ctx(), b.ana, pa.bill, PayInput{AmountCents: 100})
	wantErr(t, err, ErrBillNotFound, "pagar conta de outro casal")
	wantErr(t, b.svc.DeleteBill(ctx(), b.ana, pa.bill), ErrBillNotFound, "apagar conta de outro casal")

	_, err = b.svc.RenameCategory(ctx(), b.ana, pa.category, "invadida")
	wantErr(t, err, ErrCategoryNotFound, "renomear categoria de outro casal")
	_, err = b.svc.ArchiveCategory(ctx(), b.ana, pa.category, true)
	wantErr(t, err, ErrCategoryNotFound, "arquivar categoria de outro casal")
	_, err = b.svc.CreateExpense(ctx(), b.ana, ExpenseInput{AmountCents: 100, CategoryID: pa.category})
	wantErr(t, err, ErrCategoryNotFound, "gasto na categoria de outro casal")
	_, err = b.svc.SaveBudget(ctx(), b.ana, Budget{CategoryID: pa.category, LimitCents: 100})
	wantErr(t, err, ErrCategoryNotFound, "orçamento na categoria de outro casal")
	_, err = b.svc.CreateBill(ctx(), b.ana, BillInput{Title: "x", DueOn: "2026-12-01", CategoryID: pa.category})
	wantErr(t, err, ErrCategoryNotFound, "conta na categoria de outro casal")
	wantErr(t, b.svc.DeleteBudget(ctx(), b.ana, pa.category), ErrCategoryNotFound, "apagar orçamento de outro casal")
	// quem pagou de outro casal
	_, err = b.svc.CreateExpense(ctx(), b.ana, ExpenseInput{AmountCents: 100, CategoryID: b.cat(KeyOther), PaidBy: a.ana.UserID})
	wantErr(t, err, ErrInvalidInput, "pagador de outro casal")

	// as listas do B não trazem nada do A
	if page, _ := b.svc.ListExpenses(ctx(), b.ana, ExpenseFilter{}); len(page.Expenses) != 0 {
		t.Errorf("lista de gastos do B: %+v", page.Expenses)
	}
	if page, _ := b.svc.ListExpenses(ctx(), b.ana, ExpenseFilter{Scope: "mine"}); len(page.Expenses) != 0 {
		t.Errorf("lista pessoal do B: %+v", page.Expenses)
	}
	if bal, _ := b.svc.Balance(ctx(), b.ana, "2030-01-01"); bal.AmountCents != 0 || len(bal.Expenses) != 0 {
		t.Errorf("saldo do B: %+v", bal)
	}
	_, err = b.svc.Settle(ctx(), b.ana, "2030-01-01", 9500)
	wantErr(t, err, ErrNothingToSettle, "acertar o B não toca o A")
	if l, _ := b.svc.ListSettlements(ctx(), b.ana); len(l) != 0 {
		t.Errorf("acertos do B: %+v", l)
	}
	if l, _ := b.svc.ListRecurring(ctx(), b.ana); len(l) != 0 {
		t.Errorf("fixos do B: %+v", l)
	}
	if l, _ := b.svc.ListBills(ctx(), b.ana, "", "", false); len(l) != 0 {
		t.Errorf("contas do B: %+v", l)
	}
	if l, _ := b.svc.ListBudgets(ctx(), b.ana); len(l) != 0 {
		t.Errorf("orçamentos do B: %+v", l)
	}
	if c, _ := b.svc.GetIncome(ctx(), b.ana); c != nil {
		t.Errorf("renda do B: %v", *c)
	}
	exp, _ := b.svc.Export(ctx(), b.ana)
	if len(exp.Expenses) != 0 || len(exp.Settlements) != 0 {
		t.Errorf("exportação do B: %+v", exp)
	}
	sum, _ := b.svc.Summary(ctx(), b.ana, "2026-11", "household")
	if sum.TotalCents != 0 {
		t.Errorf("resumo do B: %+v", sum)
	}
	// os dados do A seguem intactos
	if got, err := a.svc.GetExpense(ctx(), a.ana, pa.expense); err != nil || got.AmountCents != 10000 {
		t.Errorf("o gasto do A mudou: %+v %v", got, err)
	}
	if _, err := a.svc.GetBill(ctx(), a.ana, pa.bill); err != nil {
		t.Errorf("a conta do A sumiu: %v", err)
	}
}

// Com o papel casal_app (RLS ligado), o banco barra o casal errado mesmo sem o WHERE household_id, em todas as tabelas.
func TestRLSBlocksOtherHouseholdInEveryTable(t *testing.T) {
	a := newEnv(t)
	if !rlsRole(t, a.db) {
		t.Skip("TEST_DATABASE_URL precisa usar o papel casal_app para testar o RLS")
	}
	populate(t, a)
	other := newEnv(t)

	tables := []string{"finance_categories", "finance_settings", "finance_incomes", "settlements", "recurring_expenses",
		"bills", "expenses", "budgets", "budget_alerts", "household_modules"}
	for _, table := range tables {
		// no casal do A há linhas (conferido no escopo dele) e, no escopo do outro casal, o banco esconde todas
		var own, foreign int
		mustNoErr(t, a.repo.inHousehold(ctx(), a.hh, func(tx txT) error {
			return tx.QueryRowContext(ctx(), `SELECT count(*) FROM `+table+` WHERE household_id = $1`, a.hh).Scan(&own)
		}))
		mustNoErr(t, other.repo.inHousehold(ctx(), other.hh, func(tx txT) error {
			return tx.QueryRowContext(ctx(), `SELECT count(*) FROM `+table).Scan(&foreign) // sem WHERE de propósito
		}))
		if own == 0 {
			t.Errorf("%s: o teste precisa de linhas no casal A", table)
		}
		// o outro casal só enxerga as próprias linhas (que são poucas); nunca as do A
		var leaked int
		mustNoErr(t, other.repo.inHousehold(ctx(), other.hh, func(tx txT) error {
			return tx.QueryRowContext(ctx(), `SELECT count(*) FROM `+table+` WHERE household_id = $1`, a.hh).Scan(&leaked)
		}))
		if leaked != 0 {
			t.Errorf("%s: o RLS deveria esconder as %d linhas do casal A, mas %d apareceram", table, own, leaked)
		}
		_ = foreign
	}
}

func TestSystemScopeIsRequiredToCrossHouseholds(t *testing.T) {
	a := newEnv(t)
	if !rlsRole(t, a.db) {
		t.Skip("TEST_DATABASE_URL precisa usar o papel casal_app")
	}
	populate(t, a)
	// sem escopo nenhum, o papel do app não enxerga linha alguma
	var n int
	mustNoErr(t, a.db.QueryRow(`SELECT count(*) FROM expenses`).Scan(&n))
	if n != 0 {
		t.Errorf("sem escopo, o RLS esconde tudo: %d linhas", n)
	}
}
