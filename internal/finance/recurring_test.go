package finance

import (
	"context"
	"testing"
	"time"
)

// enableModule liga o módulo do casal (os jobs só olham casais com Finanças ligado).
func (e *env) enableModule() {
	e.t.Helper()
	mustNoErr(e.t, e.repo.inHousehold(ctx(), e.hh, func(tx txT) error {
		_, err := tx.ExecContext(ctx(), `INSERT INTO household_modules (household_id, module) VALUES ($1, 'finance') ON CONFLICT DO NOTHING`, e.hh)
		return err
	}))
}

func (e *env) setNow(t time.Time) { e.svc.now = func() time.Time { return t } }

func (e *env) recurring(p int, in RecurringInput) Recurring {
	e.t.Helper()
	_ = p
	r, err := e.svc.CreateRecurring(ctx(), e.ana, in)
	mustNoErr(e.t, err)
	return r
}

func (e *env) expensesOf(r Recurring) []Expense {
	e.t.Helper()
	var out []Expense
	mustNoErr(e.t, e.repo.inHousehold(ctx(), e.hh, func(tx txT) error {
		var err error
		out, err = queryExpenses(ctx(), tx, e.ana.UserID, `SELECT `+expCols+` FROM expenses e
			WHERE e.household_id = $1 AND e.recurring_id = $2 ORDER BY e.spent_on`, e.hh, r.ID)
		return err
	}))
	return out
}

func rentInput(e *env, day int) RecurringInput {
	return RecurringInput{AmountCents: 150000, CategoryID: e.cat("housing"), Description: "aluguel", DayOfMonth: day, StartOn: "2026-11-01"}
}

func TestRecurringGeneratesOnceAndIsIdempotent(t *testing.T) {
	e := newEnv(t)
	e.enableModule()
	r := e.recurring(0, rentInput(e, 5)) // dia 5 já passou em 15/11 e o início é 01/11: aparece na criação
	got := e.expensesOf(r)
	if len(got) != 1 || got[0].SpentOn != "2026-11-05" || got[0].Source != SourceRecurring || got[0].AmountCents != 150000 ||
		got[0].PaidBy != e.ana.UserID || got[0].Split[e.beto.UserID] != 5000 {
		t.Fatalf("gasto gerado na criação: %+v", got)
	}
	for i := 0; i < 3; i++ { // o job roda de novo e de novo: nada muda
		_, err := e.svc.GenerateRecurring(ctx())
		mustNoErr(t, err)
	}
	if got := e.expensesOf(r); len(got) != 1 {
		t.Errorf("gerar duas vezes não duplica: %d gastos", len(got))
	}

	// no mês seguinte, ao chegar o dia, nasce o novo
	e.setNow(time.Date(2026, 12, 4, 15, 0, 0, 0, time.UTC))
	_, _ = e.svc.GenerateRecurring(ctx())
	if got := e.expensesOf(r); len(got) != 1 {
		t.Errorf("dia 4 de dezembro ainda não chegou ao dia 5: %d", len(got))
	}
	e.setNow(time.Date(2026, 12, 5, 15, 0, 0, 0, time.UTC))
	n, err := e.svc.GenerateRecurring(ctx())
	mustNoErr(t, err)
	if got := e.expensesOf(r); len(got) != 2 || got[1].SpentOn != "2026-12-05" || n < 1 {
		t.Errorf("em 05/12 nasce o de dezembro: %+v (criados %d)", got, n)
	}
}

func TestRecurringDay31InFebruaryFallsOnLastDay(t *testing.T) {
	e := newEnv(t)
	e.enableModule()
	e.setNow(time.Date(2027, 2, 10, 15, 0, 0, 0, time.UTC))
	in := rentInput(e, 31)
	in.StartOn = "2027-02-01"
	r := e.recurring(0, in)
	if got := e.expensesOf(r); len(got) != 0 {
		t.Fatalf("dia 31 ainda não chegou em 10/02: %+v", got)
	}
	e.setNow(time.Date(2027, 2, 28, 15, 0, 0, 0, time.UTC))
	_, _ = e.svc.GenerateRecurring(ctx())
	if got := e.expensesOf(r); len(got) != 1 || got[0].SpentOn != "2027-02-28" {
		t.Errorf("dia 31 em fevereiro cai no dia 28: %+v", got)
	}
}

func TestRecurringPauseStopsGenerationAndResumeContinues(t *testing.T) {
	e := newEnv(t)
	e.enableModule()
	in := rentInput(e, 20)
	r := e.recurring(0, in)
	_, err := e.svc.SetRecurringActive(ctx(), e.ana, r.ID, false)
	mustNoErr(t, err)
	e.setNow(time.Date(2026, 11, 20, 15, 0, 0, 0, time.UTC))
	_, _ = e.svc.GenerateRecurring(ctx())
	if got := e.expensesOf(r); len(got) != 0 {
		t.Fatalf("pausado não gera: %+v", got)
	}
	_, err = e.svc.SetRecurringActive(ctx(), e.beto, r.ID, true)
	mustNoErr(t, err)
	_, _ = e.svc.GenerateRecurring(ctx())
	if got := e.expensesOf(r); len(got) != 1 || got[0].SpentOn != "2026-11-20" {
		t.Errorf("retomado gera: %+v", got)
	}
}

func TestEditingGeneratedExpenseDoesNotChangeRecurringAndDeletedOneStaysDeleted(t *testing.T) {
	e := newEnv(t)
	e.enableModule()
	r := e.recurring(0, rentInput(e, 5))
	gen := e.expensesOf(r)[0]

	up, err := e.svc.UpdateExpense(ctx(), e.beto, gen.ID, ExpenseInput{AmountCents: 160000, CategoryID: e.cat("housing"), SpentOn: gen.SpentOn, Version: gen.Version})
	mustNoErr(t, err)
	if up.AmountCents != 160000 {
		t.Fatalf("edição: %+v", up)
	}
	cur, err := e.svc.repo.GetRecurring(ctx(), e.hh, r.ID, e.ana.UserID)
	mustNoErr(t, err)
	if cur.AmountCents != 150000 {
		t.Errorf("o fixo não muda quando se edita um gasto gerado: %d", cur.AmountCents)
	}
	mustNoErr(t, e.svc.DeleteExpense(ctx(), e.ana, gen.ID))
	_, _ = e.svc.GenerateRecurring(ctx())
	if got := e.expensesOf(r); len(got) != 0 {
		t.Errorf("o gasto apagado não volta na próxima rodada: %+v", got)
	}
}

func TestRecurringUsesPayersTimezoneForTheDay(t *testing.T) {
	e := newEnv(t)
	e.enableModule()
	// 15/11 15:00 UTC já é 16/11 04:00 na Nova Zelândia e ainda 15/11 em São Paulo
	_, err := e.db.Exec(`UPDATE users SET timezone = 'Pacific/Auckland' WHERE id = $1`, e.beto.UserID)
	mustNoErr(t, err)
	e.people.mu.Lock()
	e.people.ps[1].Timezone = "Pacific/Auckland"
	e.people.mu.Unlock()

	in := rentInput(e, 16)
	pay, err := e.svc.CreateRecurring(ctx(), e.ana, RecurringInput{AmountCents: 1000, CategoryID: e.cat("housing"), DayOfMonth: 16, StartOn: "2026-11-01", PaidBy: e.beto.UserID})
	mustNoErr(t, err)
	byAna, err := e.svc.CreateRecurring(ctx(), e.ana, in)
	mustNoErr(t, err)
	if got := e.expensesOf(pay); len(got) != 1 || got[0].SpentOn != "2026-11-16" {
		t.Errorf("para quem paga na Nova Zelândia o dia 16 já chegou: %+v", got)
	}
	if got := e.expensesOf(byAna); len(got) != 0 {
		t.Errorf("para quem paga em São Paulo o dia 16 ainda não chegou: %+v", got)
	}
}

func TestRecurringRules(t *testing.T) {
	e := newEnv(t)
	bad := rentInput(e, 5)
	bad.StartOn = "2026-10-31" // antes do mês corrente
	_, err := e.svc.CreateRecurring(ctx(), e.ana, bad)
	wantErr(t, err, ErrInvalidInput, "início antes do mês atual")
	bad = rentInput(e, 32)
	_, err = e.svc.CreateRecurring(ctx(), e.ana, bad)
	wantErr(t, err, ErrInvalidInput, "dia 32")
	bad = rentInput(e, 5)
	bad.EndOn = "2026-10-01"
	_, err = e.svc.CreateRecurring(ctx(), e.ana, bad)
	wantErr(t, err, ErrInvalidInput, "fim antes do início")
	bad = rentInput(e, 5)
	bad.AmountCents = 0
	_, err = e.svc.CreateRecurring(ctx(), e.ana, bad)
	wantErr(t, err, ErrAmountInvalid, "valor zero")

	// fixo pessoal é do dono: o parceiro não vê (privado) nem mexe
	p, err := e.svc.CreateRecurring(ctx(), e.ana, RecurringInput{Scope: ScopePersonal, AmountCents: 3990, CategoryID: e.cat("leisure"), DayOfMonth: 10, StartOn: "2026-11-01"})
	mustNoErr(t, err)
	_, err = e.svc.repo.GetRecurring(ctx(), e.hh, p.ID, e.beto.UserID)
	wantErr(t, err, ErrRecurringNotFound, "fixo pessoal privado visto pelo parceiro")
	list, _ := e.svc.ListRecurring(ctx(), e.beto)
	for _, r := range list {
		if r.ID == p.ID {
			t.Error("a lista do parceiro mostrou o fixo pessoal privado")
		}
	}
	wantErr(t, e.svc.DeleteRecurring(ctx(), e.beto, p.ID), ErrRecurringNotFound, "apagar fixo pessoal privado do parceiro")
	_ = context.Background
}
