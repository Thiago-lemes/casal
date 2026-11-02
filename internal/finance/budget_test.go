package finance

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

func (e *env) setBudget(key string, limit int64) string {
	e.t.Helper()
	id := e.cat(key)
	_, err := e.svc.SaveBudget(ctx(), e.ana, Budget{CategoryID: id, LimitCents: limit})
	mustNoErr(e.t, err)
	return id
}

func (e *env) alerts(category string) []int {
	e.t.Helper()
	var levels []int
	mustNoErr(e.t, e.repo.inHousehold(ctx(), e.hh, func(tx txT) error {
		rows, err := tx.QueryContext(ctx(), `SELECT level FROM budget_alerts WHERE household_id = $1 AND category_id = $2 ORDER BY level`, e.hh, category)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var l int
			if err := rows.Scan(&l); err != nil {
				return err
			}
			levels = append(levels, l)
		}
		return rows.Err()
	}))
	return levels
}

func pushes(e *env, user string) int {
	n := 0
	for _, m := range e.notes.to(user) {
		if m.Title == "Orçamento do mês" {
			n++
		}
	}
	return n
}

func TestBudgetAlertsAtEightyAndHundredOncePerMonth(t *testing.T) {
	e := newEnv(t)
	cat := e.setBudget(KeyGroceries, 10000)

	e.spend(e.ana, 7000, "2026-11-10") // 70%: nada
	if pushes(e, e.beto.UserID) != 0 {
		t.Fatal("70% não avisa")
	}
	e.spend(e.ana, 1000, "2026-11-11") // 80%
	if pushes(e, e.ana.UserID) != 1 || pushes(e, e.beto.UserID) != 1 {
		t.Errorf("80%% avisa os dois uma vez: ana=%d beto=%d", pushes(e, e.ana.UserID), pushes(e, e.beto.UserID))
	}
	if body := e.notes.to(e.beto.UserID)[0].Body; !strings.Contains(body, "80%") {
		t.Errorf("texto: %q", body)
	}
	e.spend(e.beto, 500, "2026-11-12") // 85%: o 80% já foi avisado
	if pushes(e, e.ana.UserID) != 1 {
		t.Errorf("o mesmo nível não avisa de novo: %d", pushes(e, e.ana.UserID))
	}
	e.spend(e.beto, 1500, "2026-11-13") // 100%+
	if pushes(e, e.ana.UserID) != 2 {
		t.Errorf("100%% avisa uma vez: %d", pushes(e, e.ana.UserID))
	}
	e.spend(e.beto, 500, "2026-11-14")
	if pushes(e, e.ana.UserID) != 2 {
		t.Errorf("acima de 100%% não repete: %d", pushes(e, e.ana.UserID))
	}
	if got := e.alerts(cat); len(got) != 2 || got[0] != 80 || got[1] != 100 {
		t.Errorf("níveis gravados: %v", got)
	}
}

func TestBudgetJumpStraightTo100SendsOnePush(t *testing.T) {
	e := newEnv(t)
	cat := e.setBudget(KeyGroceries, 10000)
	e.spend(e.ana, 12000, "2026-11-10")
	if pushes(e, e.ana.UserID) != 1 {
		t.Errorf("um único aviso, do nível mais alto: %d", pushes(e, e.ana.UserID))
	}
	if body := e.notes.to(e.ana.UserID)[0].Body; !strings.Contains(body, "100%") {
		t.Errorf("deveria avisar 100%%: %q", body)
	}
	if got := e.alerts(cat); len(got) != 2 {
		t.Errorf("os dois níveis ficam gravados: %v", got)
	}
}

// Dois celulares lançando gastos ao mesmo tempo: cada nível sai uma vez.
func TestBudgetAlertsOnceEvenWithSimultaneousExpenses(t *testing.T) {
	e := newEnv(t)
	cat := e.setBudget(KeyGroceries, 10000)
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			p := e.ana
			if i%2 == 1 {
				p = e.beto
			}
			e.svc.CreateExpense(ctx(), p, ExpenseInput{AmountCents: 3000, CategoryID: cat, SpentOn: "2026-11-10"})
		}(i)
	}
	wg.Wait()
	if got := e.alerts(cat); len(got) != 2 {
		t.Fatalf("exatamente um aviso por nível: %v", got)
	}
	if n := pushes(e, e.ana.UserID); n < 1 || n > 2 {
		t.Errorf("no máximo um aviso por nível chegou à Ana: %d", n)
	}
}

func TestBudgetIgnoresPersonalExpensesAndOtherMonths(t *testing.T) {
	e := newEnv(t)
	cat := e.setBudget(KeyLeisure, 10000)
	e.personal(e.ana, 50000, VisView) // categoria lazer, mas é pessoal
	if pushes(e, e.ana.UserID) != 0 {
		t.Error("gasto pessoal não conta no orçamento")
	}
	e.svc.CreateExpense(ctx(), e.ana, ExpenseInput{AmountCents: 9000, CategoryID: cat, SpentOn: "2026-12-05"}) // outro mês
	if pushes(e, e.ana.UserID) != 0 || len(e.alerts(cat)) != 0 {
		t.Error("gasto de outro mês não avisa agora")
	}
	sum, _ := e.svc.Summary(ctx(), e.ana, "2026-11", "household")
	for _, c := range sum.Categories {
		if c.CategoryID == cat && (c.TotalCents != 0 || c.UsedPercent == nil || *c.UsedPercent != 0 || *c.LimitCents != 10000) {
			t.Errorf("novembro está zerado: %+v", c)
		}
	}
	dec, _ := e.svc.Summary(ctx(), e.ana, "2026-12", "household")
	for _, c := range dec.Categories {
		if c.CategoryID == cat && (c.TotalCents != 9000 || *c.UsedPercent != 90) {
			t.Errorf("dezembro tem o gasto: %+v", c)
		}
	}
	e.spend(e.ana, 1, "2026-11-15") // outra categoria, só para ter movimento
}

func TestBudgetCRUDAndSummaryComparison(t *testing.T) {
	e := newEnv(t)
	_, err := e.svc.SaveBudget(ctx(), e.ana, Budget{CategoryID: e.cat(KeyHousing), LimitCents: 0})
	wantErr(t, err, ErrAmountInvalid, "limite zero")
	_, err = e.svc.SaveBudget(ctx(), e.ana, Budget{CategoryID: newUUID(), LimitCents: 100})
	wantErr(t, err, ErrCategoryNotFound, "categoria de outro lugar")
	id := e.setBudget(KeyGroceries, 20000)
	e.setBudget(KeyGroceries, 30000) // PUT troca o limite
	bs, _ := e.svc.ListBudgets(ctx(), e.ana)
	if len(bs) != 1 || bs[0].LimitCents != 30000 {
		t.Errorf("orçamentos: %+v", bs)
	}
	e.spend(e.ana, 6000, "2026-10-10")
	e.spend(e.beto, 9000, "2026-11-10")
	sum, err := e.svc.Summary(ctx(), e.ana, "2026-11", "household")
	mustNoErr(t, err)
	if sum.TotalCents != 9000 || sum.PreviousTotalCents != 6000 || sum.PaidBy[e.beto.UserID] != 9000 || sum.PaidBy[e.ana.UserID] != 0 {
		t.Errorf("resumo: %+v", sum)
	}
	if c := sum.Categories[0]; c.CategoryID != id || c.PreviousCents != 6000 || *c.UsedPercent != 30 {
		t.Errorf("categoria: %+v", c)
	}
	mustNoErr(t, e.svc.DeleteBudget(ctx(), e.ana, id))
	wantErr(t, e.svc.DeleteBudget(ctx(), e.ana, id), ErrCategoryNotFound, "apagar de novo")
	_, err = e.svc.Summary(ctx(), e.ana, "2026-13", "")
	wantErr(t, err, ErrInvalidInput, "mês inválido")
	_, err = e.svc.Summary(ctx(), e.ana, "", "todos")
	wantErr(t, err, ErrInvalidInput, "scope inválido")
}

func TestSummaryMineOnlyCountsMyPersonalExpenses(t *testing.T) {
	e := newEnv(t)
	e.personal(e.ana, 1111, VisPrivate)
	e.personal(e.beto, 2222, VisView)
	e.spend(e.ana, 5000, "2026-11-01")
	a, _ := e.svc.Summary(ctx(), e.ana, "2026-11", "mine")
	b, _ := e.svc.Summary(ctx(), e.beto, "2026-11", "mine")
	if a.TotalCents != 1111 || b.TotalCents != 2222 {
		t.Errorf("cada um soma só o seu pessoal: ana=%d beto=%d", a.TotalCents, b.TotalCents)
	}
	if a.PaidBy != nil {
		t.Errorf("o resumo pessoal não traz quem pagou: %v", a.PaidBy)
	}
}

func TestExportOnlyWhatThePersonCanSeeAndCSVIsSafe(t *testing.T) {
	e := newEnv(t)
	e.personal(e.ana, 11, VisPrivate)
	shown := e.personal(e.ana, 22, VisView)
	_, err := e.svc.CreateExpense(ctx(), e.ana, ExpenseInput{AmountCents: 33, CategoryID: e.cat(KeyOther), Description: "=HYPERLINK(\"x\")"})
	mustNoErr(t, err)
	_, err = e.svc.CreateExpense(ctx(), e.ana, ExpenseInput{AmountCents: 44, CategoryID: e.cat(KeyOther), Installments: &InstallmentsInput{Count: 2}})
	mustNoErr(t, err)

	forBeto, err := e.svc.Export(ctx(), e.beto)
	mustNoErr(t, err)
	ids := map[string]bool{}
	for _, x := range forBeto.Expenses {
		ids[x.ID] = true
	}
	if len(forBeto.Expenses) != 4 || !ids[shown.ID] {
		t.Errorf("o Beto vê os 3 do casal (1 + 2 parcelas) e o pessoal em view: %d", len(forBeto.Expenses))
	}
	forAna, _ := e.svc.Export(ctx(), e.ana)
	if len(forAna.Expenses) != 5 {
		t.Errorf("a Ana vê os dela também: %d", len(forAna.Expenses))
	}
	var out strings.Builder
	mustNoErr(t, forAna.WriteCSV(&out))
	csv := out.String()
	if !strings.Contains(csv, "'=HYPERLINK") || strings.Contains(csv, ",=HYPERLINK") {
		t.Errorf("o CSV deveria neutralizar fórmulas:\n%s", csv)
	}
	if !strings.Contains(csv, "1/2") || !strings.Contains(csv, "Outras") {
		t.Errorf("parcela e nome da categoria no CSV:\n%s", csv)
	}
	_ = fmt.Sprint
}
