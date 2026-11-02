package finance

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"
)

func TestCategoriesSeedOnceAndArchiveBlocksNewExpenses(t *testing.T) {
	e := newEnv(t)
	cats, err := e.svc.ListCategories(ctx(), e.ana, false)
	mustNoErr(t, err)
	if len(cats) != len(defaultCategories) {
		t.Fatalf("esperava %d categorias padrão, vieram %d", len(defaultCategories), len(cats))
	}
	// semear de novo (religar o módulo) não duplica nem recria o que o casal renomeou
	groceries := e.cat(KeyGroceries)
	_, err = e.svc.RenameCategory(ctx(), e.ana, groceries, "Supermercado")
	mustNoErr(t, err)
	mustNoErr(t, e.repo.inHousehold(ctx(), e.hh, func(tx txT) error { return SeedCategories(ctx(), tx, e.hh) }))
	cats, _ = e.svc.ListCategories(ctx(), e.ana, false)
	if len(cats) != len(defaultCategories) {
		t.Errorf("religar não pode recriar categorias: %d", len(cats))
	}

	_, err = e.svc.CreateCategory(ctx(), e.ana, "  SUPERMERCADO ")
	wantErr(t, err, ErrCategoryDuplicate, "nome repetido sem diferenciar maiúsculas")

	custom, err := e.svc.CreateCategory(ctx(), e.beto, "Pets")
	mustNoErr(t, err)
	old := e.spend(e.ana, 1000, "2026-11-01")
	_, err = e.svc.ArchiveCategory(ctx(), e.ana, groceries, true)
	mustNoErr(t, err)

	_, err = e.svc.CreateExpense(ctx(), e.ana, ExpenseInput{AmountCents: 500, CategoryID: groceries})
	wantErr(t, err, ErrCategoryArchived, "gasto novo em categoria arquivada")
	if got, _ := e.svc.GetExpense(ctx(), e.ana, old.ID); got.CategoryID != groceries {
		t.Error("o gasto antigo continua na categoria arquivada")
	}
	if cats, _ := e.svc.ListCategories(ctx(), e.ana, false); len(cats) != len(defaultCategories) { // 7 padrão - 1 + Pets
		t.Errorf("a lista sem arquivadas: %d", len(cats))
	}
	if cats, _ := e.svc.ListCategories(ctx(), e.ana, true); len(cats) != len(defaultCategories)+1 {
		t.Errorf("a lista com arquivadas: %d", len(cats))
	}
	_, err = e.svc.ArchiveCategory(ctx(), e.ana, groceries, false)
	mustNoErr(t, err)
	_, err = e.svc.CreateExpense(ctx(), e.ana, ExpenseInput{AmountCents: 500, CategoryID: custom.ID})
	mustNoErr(t, err)
}

func TestConcurrentSeedsCreateCategoriesOnce(t *testing.T) {
	e := newEnv(t) // já semeou uma vez
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = e.repo.inHousehold(ctx(), e.hh, func(tx txT) error { return SeedCategories(ctx(), tx, e.hh) })
		}()
	}
	wg.Wait()
	cats, _ := e.svc.ListCategories(ctx(), e.ana, true)
	if len(cats) != len(defaultCategories) {
		t.Errorf("duas semeaduras ao mesmo tempo: %d categorias", len(cats))
	}
}

func TestExpenseAmountsAreExactCentsAndValidated(t *testing.T) {
	e := newEnv(t)
	x := e.spend(e.ana, 12345, "2026-11-10")
	if x.AmountCents != 12345 || x.PaidBy != e.ana.UserID || x.Version != 1 || x.Split[e.ana.UserID] != 5000 {
		t.Errorf("gasto inesperado: %+v", x)
	}
	for _, c := range []int64{0, -5, MaxAmountCents + 1} {
		_, err := e.svc.CreateExpense(ctx(), e.ana, ExpenseInput{AmountCents: c, CategoryID: e.cat(KeyOther)})
		wantErr(t, err, ErrAmountInvalid, "valor inválido")
	}
	_, err := e.svc.CreateExpense(ctx(), e.ana, ExpenseInput{AmountCents: MaxAmountCents, CategoryID: e.cat(KeyOther)})
	mustNoErr(t, err)
	_, err = e.svc.CreateExpense(ctx(), e.ana, ExpenseInput{AmountCents: 100, CategoryID: e.cat(KeyOther), PaidBy: "00000000-0000-0000-0000-000000000000"})
	wantErr(t, err, ErrInvalidInput, "quem pagou de fora do casal")
	_, err = e.svc.CreateExpense(ctx(), e.ana, ExpenseInput{AmountCents: 100, CategoryID: e.cat(KeyOther), Split: map[string]int{e.ana.UserID: 9000}})
	wantErr(t, err, ErrSplitInvalid, "divisão que não soma 10000")
}

func TestVersionConflictAndSettledAreRefused(t *testing.T) {
	e := newEnv(t)
	x := e.spend(e.ana, 1000, "2026-11-10")
	in := ExpenseInput{AmountCents: 2000, CategoryID: e.cat(KeyGroceries), SpentOn: "2026-11-10", Version: x.Version}

	up, err := e.svc.UpdateExpense(ctx(), e.beto, x.ID, in) // o parceiro também edita o gasto do casal
	mustNoErr(t, err)
	if up.AmountCents != 2000 || up.Version != 2 || up.UpdatedBy != e.beto.UserID {
		t.Errorf("edição mal gravada: %+v", up)
	}
	_, err = e.svc.UpdateExpense(ctx(), e.ana, x.ID, in) // ainda com a versão 1
	wantErr(t, err, ErrExpenseConflict, "versão velha")

	// acertado: não edita nem apaga
	_, err = e.svc.Settle(ctx(), e.ana, "2026-11-15", 1000) // Ana pagou 2000, divisão igual: Beto deve 1000
	mustNoErr(t, err)
	in.Version = 2
	_, err = e.svc.UpdateExpense(ctx(), e.ana, x.ID, in)
	wantErr(t, err, ErrExpenseSettled, "editar gasto acertado")
	wantErr(t, e.svc.DeleteExpense(ctx(), e.ana, x.ID), ErrExpenseSettled, "apagar gasto acertado")
}

func TestListFiltersAndPagination(t *testing.T) {
	e := newEnv(t)
	for i := 0; i < 5; i++ {
		e.spend(e.ana, int64(100+i), "2026-11-0"+string(rune('1'+i)))
	}
	e.spend(e.beto, 999, "2026-10-20")

	page, err := e.svc.ListExpenses(ctx(), e.ana, ExpenseFilter{Month: "2026-11", Limit: 2})
	mustNoErr(t, err)
	if len(page.Expenses) != 2 || page.Next == "" || page.Expenses[0].SpentOn != "2026-11-05" {
		t.Fatalf("primeira página: %+v", page)
	}
	page2, err := e.svc.ListExpenses(ctx(), e.ana, ExpenseFilter{Month: "2026-11", Limit: 2, Before: page.Next})
	mustNoErr(t, err)
	if len(page2.Expenses) != 2 || page2.Expenses[0].SpentOn != "2026-11-03" || page2.Next == "" {
		t.Fatalf("segunda página: %+v", page2)
	}
	page3, _ := e.svc.ListExpenses(ctx(), e.ana, ExpenseFilter{Month: "2026-11", Limit: 2, Before: page2.Next})
	if len(page3.Expenses) != 1 || page3.Next != "" {
		t.Errorf("última página: %+v", page3)
	}
	byPayer, _ := e.svc.ListExpenses(ctx(), e.ana, ExpenseFilter{PaidBy: e.beto.UserID})
	if len(byPayer.Expenses) != 1 || byPayer.Expenses[0].AmountCents != 999 {
		t.Errorf("filtro por quem pagou: %+v", byPayer)
	}
	_, err = e.svc.ListExpenses(ctx(), e.ana, ExpenseFilter{Month: "novembro"})
	wantErr(t, err, ErrInvalidInput, "mês inválido")
	_, err = e.svc.ListExpenses(ctx(), e.ana, ExpenseFilter{Before: "x"})
	wantErr(t, err, ErrInvalidInput, "cursor inválido")
}

func TestFromShoppingUsesGroceriesCategory(t *testing.T) {
	e := newEnv(t)
	res, err := e.svc.CreateFromShopping(ctx(), e.beto, ExpenseInput{AmountCents: 8750, Description: "feira"})
	mustNoErr(t, err)
	x := res.Expenses[0]
	if x.CategoryID != e.cat(KeyGroceries) || x.Source != SourceShopping || x.PaidBy != e.beto.UserID || x.Scope != ScopeHousehold {
		t.Errorf("gasto da lista de compras: %+v", x)
	}
}

// ---- privacidade entre os parceiros (o RLS não separa os dois: só o código) ----

func TestPersonalPrivateExpenseNeverLeaksToPartner(t *testing.T) {
	e := newEnv(t)
	secret := e.personal(e.ana, 77777, VisPrivate)
	shared := e.spend(e.ana, 1000, "2026-11-10")

	// o Beto não a vê em lugar nenhum
	_, err := e.svc.GetExpense(ctx(), e.beto, secret.ID)
	wantErr(t, err, ErrExpenseNotFound, "ler o pessoal privado do parceiro")
	_, err = e.svc.UpdateExpense(ctx(), e.beto, secret.ID, ExpenseInput{AmountCents: 1, CategoryID: e.cat(KeyLeisure), Version: 1})
	wantErr(t, err, ErrExpenseNotFound, "editar o pessoal privado do parceiro")
	wantErr(t, e.svc.DeleteExpense(ctx(), e.beto, secret.ID), ErrExpenseNotFound, "apagar o pessoal privado do parceiro")

	for _, scope := range []string{"", "household", "mine"} {
		page, err := e.svc.ListExpenses(ctx(), e.beto, ExpenseFilter{Scope: scope})
		mustNoErr(t, err)
		for _, x := range page.Expenses {
			if x.ID == secret.ID {
				t.Errorf("lista scope=%q do Beto mostrou o gasto privado da Ana", scope)
			}
		}
	}
	sumBeto, err := e.svc.Summary(ctx(), e.beto, "2026-11", "mine")
	mustNoErr(t, err)
	if sumBeto.TotalCents != 0 {
		t.Errorf("o resumo pessoal do Beto não pode somar o gasto da Ana: %+v", sumBeto)
	}
	sumHH, _ := e.svc.Summary(ctx(), e.beto, "2026-11", "household")
	if sumHH.TotalCents != 1000 {
		t.Errorf("o resumo do casal só tem o gasto do casal: %+v", sumHH)
	}
	exp, err := e.svc.Export(ctx(), e.beto)
	mustNoErr(t, err)
	raw, _ := json.Marshal(exp)
	if strings.Contains(string(raw), secret.ID) || strings.Contains(string(raw), "77777") {
		t.Error("a exportação do Beto trouxe o gasto privado da Ana")
	}
	var csvOut strings.Builder
	mustNoErr(t, exp.WriteCSV(&csvOut))
	if strings.Contains(csvOut.String(), "segredo") || strings.Contains(csvOut.String(), "77777") {
		t.Error("o CSV do Beto trouxe o gasto privado da Ana")
	}
	bal, err := e.svc.Balance(ctx(), e.beto, "")
	mustNoErr(t, err)
	if bal.AmountCents != 500 || len(bal.Expenses) != 1 || bal.Expenses[0].ID != shared.ID {
		t.Errorf("o saldo só conta o gasto do casal: %+v", bal)
	}
	// a Ana vê o dela
	if _, err := e.svc.GetExpense(ctx(), e.ana, secret.ID); err != nil {
		t.Errorf("a dona deveria ver o gasto: %v", err)
	}
}

func TestPersonalViewExpenseIsReadOnlyForPartner(t *testing.T) {
	e := newEnv(t)
	shown := e.personal(e.ana, 4200, VisView)

	got, err := e.svc.GetExpense(ctx(), e.beto, shown.ID)
	mustNoErr(t, err)
	if got.CanEdit {
		t.Error("o parceiro lê o gasto em view, mas não o edita")
	}
	_, err = e.svc.UpdateExpense(ctx(), e.beto, shown.ID, ExpenseInput{AmountCents: 1, CategoryID: e.cat(KeyLeisure), Version: 1})
	wantErr(t, err, ErrExpenseForbidden, "editar gasto em view")
	wantErr(t, e.svc.DeleteExpense(ctx(), e.beto, shown.ID), ErrExpenseForbidden, "apagar gasto em view")

	// aparece só na lista pessoal do parceiro (somente leitura), nunca na do casal nem nos totais do casal
	mine, _ := e.svc.ListExpenses(ctx(), e.beto, ExpenseFilter{Scope: "mine"})
	if len(mine.Expenses) != 1 || mine.Expenses[0].ID != shown.ID {
		t.Errorf("lista pessoal do parceiro: %+v", mine)
	}
	hh, _ := e.svc.ListExpenses(ctx(), e.beto, ExpenseFilter{})
	if len(hh.Expenses) != 0 {
		t.Errorf("o gasto pessoal não entra na lista do casal: %+v", hh)
	}
	if s, _ := e.svc.Summary(ctx(), e.beto, "", "mine"); s.TotalCents != 0 {
		t.Errorf("o resumo pessoal do parceiro não soma o gasto em view: %+v", s)
	}
	if s, _ := e.svc.Summary(ctx(), e.beto, "", "household"); s.TotalCents != 0 {
		t.Errorf("o resumo do casal não soma gasto pessoal: %+v", s)
	}

	// só o dono muda a visibilidade, edita e apaga
	up, err := e.svc.UpdateExpense(ctx(), e.ana, shown.ID, ExpenseInput{AmountCents: 4300, CategoryID: e.cat(KeyLeisure), Visibility: VisPrivate, Version: 1})
	mustNoErr(t, err)
	if up.Visibility != VisPrivate {
		t.Errorf("o dono volta o gasto para privado: %+v", up)
	}
	_, err = e.svc.GetExpense(ctx(), e.beto, shown.ID)
	wantErr(t, err, ErrExpenseNotFound, "depois de privado o parceiro não vê mais")
	mustNoErr(t, e.svc.DeleteExpense(ctx(), e.ana, shown.ID))
}

func TestPersonalExpenseRules(t *testing.T) {
	e := newEnv(t)
	_, err := e.svc.CreateExpense(ctx(), e.ana, ExpenseInput{Scope: ScopePersonal, AmountCents: 100, CategoryID: e.cat(KeyOther), PaidBy: e.beto.UserID})
	wantErr(t, err, ErrInvalidInput, "gasto pessoal pago pelo parceiro")
	_, err = e.svc.CreateExpense(ctx(), e.ana, ExpenseInput{Scope: ScopePersonal, AmountCents: 100, CategoryID: e.cat(KeyOther), Split: map[string]int{e.ana.UserID: 10000}})
	wantErr(t, err, ErrSplitInvalid, "gasto pessoal com divisão")
	_, err = e.svc.CreateExpense(ctx(), e.ana, ExpenseInput{Scope: ScopePersonal, Visibility: VisHousehold, AmountCents: 100, CategoryID: e.cat(KeyOther)})
	wantErr(t, err, ErrInvalidInput, "gasto pessoal com visibilidade do casal")
	x := e.personal(e.ana, 100, "")
	if x.Visibility != VisPrivate || x.Split != nil {
		t.Errorf("o padrão do pessoal é privado e sem divisão: %+v", x)
	}
}

// ---- parcelamento ----

func TestInstallmentsSumToTotalAndExtrasGoToFirst(t *testing.T) {
	e := newEnv(t)
	res, err := e.svc.CreateExpense(ctx(), e.ana, ExpenseInput{
		AmountCents: 100000, CategoryID: e.cat(KeyOther), SpentOn: "2026-11-15",
		Installments: &InstallmentsInput{Count: 3},
	})
	mustNoErr(t, err)
	got := res.Expenses
	if len(got) != 3 || got[0].AmountCents != 33334 || got[1].AmountCents != 33333 || got[2].AmountCents != 33333 {
		t.Fatalf("R$ 1.000,00 em 3x: %+v", got)
	}
	wantDates := []string{"2026-11-15", "2026-12-15", "2027-01-15"}
	for i, x := range got {
		if x.SpentOn != wantDates[i] || x.Installment == nil || x.Installment.No != i+1 || x.Installment.Total != 3 ||
			x.Installment.GroupID != got[0].Installment.GroupID || x.Split[e.ana.UserID] != 5000 {
			t.Errorf("parcela %d: %+v", i+1, x)
		}
	}
}

func TestInstallmentAmountGivesCountTimesValueAndValidates(t *testing.T) {
	e := newEnv(t)
	res, err := e.svc.CreateExpense(ctx(), e.ana, ExpenseInput{
		CategoryID: e.cat(KeyOther), SpentOn: "2026-11-15",
		Installments: &InstallmentsInput{Count: 10, InstallmentAmountCents: 12990},
	})
	mustNoErr(t, err)
	var total int64
	for _, x := range res.Expenses {
		total += x.AmountCents
		if x.AmountCents != 12990 {
			t.Errorf("cada parcela vale 12990: %d", x.AmountCents)
		}
	}
	if total != 129900 {
		t.Errorf("total = count × valor: %d", total)
	}
	for name, in := range map[string]ExpenseInput{
		"1 parcela":                      {AmountCents: 1000, Installments: &InstallmentsInput{Count: 1}},
		"49 parcelas":                    {AmountCents: 100000, Installments: &InstallmentsInput{Count: 49}},
		"total incoerente":               {AmountCents: 999, Installments: &InstallmentsInput{Count: 2, InstallmentAmountCents: 500}},
		"data inválida":                  {AmountCents: 1000, Installments: &InstallmentsInput{Count: 2, FirstOn: "31/12/2026"}},
		"menos de 1 centavo por parcela": {AmountCents: 3, Installments: &InstallmentsInput{Count: 4}},
	} {
		in.CategoryID = e.cat(KeyOther)
		_, err := e.svc.CreateExpense(ctx(), e.ana, in)
		wantErr(t, err, ErrInstallmentsInvalid, name)
	}
}

func TestInstallmentDay31FallsOnLastDayOfShortMonths(t *testing.T) {
	e := newEnv(t)
	res, err := e.svc.CreateExpense(ctx(), e.ana, ExpenseInput{
		AmountCents: 40000, CategoryID: e.cat(KeyOther), SpentOn: "2027-01-31",
		Installments: &InstallmentsInput{Count: 4},
	})
	mustNoErr(t, err)
	want := []string{"2027-01-31", "2027-02-28", "2027-03-31", "2027-04-30"}
	for i, x := range res.Expenses {
		if x.SpentOn != want[i] {
			t.Errorf("parcela %d em %s, esperava %s", i+1, x.SpentOn, want[i])
		}
	}
}

func TestInstallmentsInThePastBecomeOpenExpensesForNextSettlement(t *testing.T) {
	e := newEnv(t)
	res, err := e.svc.CreateExpense(ctx(), e.ana, ExpenseInput{
		AmountCents: 30000, CategoryID: e.cat(KeyOther),
		Installments: &InstallmentsInput{Count: 3, FirstOn: "2026-09-10"}, // 10/09 e 10/10 já venceram, 10/11 também
	})
	mustNoErr(t, err)
	if len(res.Expenses) != 3 {
		t.Fatalf("parcelas: %d", len(res.Expenses))
	}
	bal, err := e.svc.Balance(ctx(), e.beto, "")
	mustNoErr(t, err)
	// as 3 já venceram até hoje (15/11): Ana pagou 300,00, Beto deve 150,00
	if bal.AmountCents != 15000 || bal.FromUser != e.beto.UserID || len(bal.Expenses) != 3 {
		t.Errorf("saldo: %+v", bal)
	}
}

func TestFutureInstallmentsDoNotCountUntilTheirDate(t *testing.T) {
	e := newEnv(t)
	_, err := e.svc.CreateExpense(ctx(), e.ana, ExpenseInput{
		AmountCents: 30000, CategoryID: e.cat(KeyOther), SpentOn: "2026-11-15",
		Installments: &InstallmentsInput{Count: 3},
	})
	mustNoErr(t, err)
	bal, _ := e.svc.Balance(ctx(), e.beto, "")
	if bal.AmountCents != 5000 || len(bal.Expenses) != 1 {
		t.Errorf("hoje só a primeira parcela conta: %+v", bal)
	}
	later, _ := e.svc.Balance(ctx(), e.beto, "2027-01-15")
	if later.AmountCents != 15000 || len(later.Expenses) != 3 {
		t.Errorf("até 15/01 as três contam: %+v", later)
	}
}

func TestInstallmentsAreAllOrNothing(t *testing.T) {
	e := newEnv(t)
	group := newUUID()
	mk := func(no int) Expense {
		return Expense{CreatedBy: e.ana.UserID, PaidBy: e.ana.UserID, Scope: ScopeHousehold, Visibility: VisHousehold,
			AmountCents: 100, CategoryID: e.cat(KeyOther), SpentOn: "2026-11-15", Source: SourceManual,
			Split: map[string]int{e.ana.UserID: 10000}, Installment: &Installment{GroupID: group, No: no, Total: 3}}
	}
	// a terceira repete o número da primeira: a restrição única barra, e as duas primeiras não podem ficar
	_, err := e.repo.InsertExpenses(ctx(), e.hh, []Expense{mk(1), mk(2), mk(1)})
	if err == nil {
		t.Fatal("deveria falhar")
	}
	if _, err := e.svc.GetGroup(ctx(), e.ana, group); !errorsIs(err, ErrExpenseNotFound) {
		t.Errorf("nenhuma parcela deveria ter sido gravada: %v", err)
	}
}

func TestInstallmentsCountInTheirOwnMonthInSummary(t *testing.T) {
	e := newEnv(t)
	_, err := e.svc.CreateExpense(ctx(), e.ana, ExpenseInput{
		AmountCents: 30001, CategoryID: e.cat(KeyOther), SpentOn: "2026-11-20",
		Installments: &InstallmentsInput{Count: 3},
	})
	mustNoErr(t, err)
	nov, _ := e.svc.Summary(ctx(), e.ana, "2026-11", "household")
	dec, _ := e.svc.Summary(ctx(), e.ana, "2026-12", "household")
	jan, _ := e.svc.Summary(ctx(), e.ana, "2027-01", "household")
	if nov.TotalCents != 10001 || dec.TotalCents != 10000 || jan.TotalCents != 10000 {
		t.Errorf("cada parcela no seu mês: nov=%d dez=%d jan=%d", nov.TotalCents, dec.TotalCents, jan.TotalCents)
	}
	if dec.PreviousTotalCents != 10001 {
		t.Errorf("comparação com o mês anterior: %d", dec.PreviousTotalCents)
	}
}

func TestGroupUpdateAndDeleteOnlyTouchOpenInstallments(t *testing.T) {
	e := newEnv(t)
	res, err := e.svc.CreateExpense(ctx(), e.ana, ExpenseInput{
		AmountCents: 30000, CategoryID: e.cat(KeyOther), SpentOn: "2026-10-15", Description: "geladeira",
		Installments: &InstallmentsInput{Count: 3},
	})
	mustNoErr(t, err)
	group := res.Expenses[0].Installment.GroupID

	// acerta o que venceu até hoje: as parcelas de 15/10 e 15/11
	_, err = e.svc.Settle(ctx(), e.ana, "2026-11-15", 10000)
	mustNoErr(t, err)

	desc, newCat := "geladeira nova", e.cat(KeyHousing)
	g, err := e.svc.UpdateGroup(ctx(), e.beto, group, GroupInput{Description: &desc, CategoryID: &newCat})
	mustNoErr(t, err)
	if g.Settled != 2 {
		t.Errorf("duas parcelas acertadas: %+v", g)
	}
	for _, x := range g.Expenses {
		wantNew := !x.Settled
		if (x.Description == desc) != wantNew || (x.CategoryID == newCat) != wantNew {
			t.Errorf("parcela %d settled=%v mudou=%v", x.Installment.No, x.Settled, x.Description == desc)
		}
	}
	out, err := e.svc.DeleteGroup(ctx(), e.ana, group)
	mustNoErr(t, err)
	if out.Deleted != 1 || out.Remaining != 2 {
		t.Errorf("apaga só a aberta: %+v", out)
	}
	_, err = e.svc.DeleteGroup(ctx(), e.ana, group)
	wantErr(t, err, ErrExpenseSettled, "grupo só com parcelas acertadas")
	_, err = e.svc.GetGroup(ctx(), e.ana, newUUID())
	wantErr(t, err, ErrExpenseNotFound, "grupo que não existe")
}

func TestPersonalInstallmentsStayPrivate(t *testing.T) {
	e := newEnv(t)
	res, err := e.svc.CreateExpense(ctx(), e.ana, ExpenseInput{
		Scope: ScopePersonal, AmountCents: 20000, CategoryID: e.cat(KeyLeisure),
		Installments: &InstallmentsInput{Count: 2},
	})
	mustNoErr(t, err)
	group := res.Expenses[0].Installment.GroupID
	_, err = e.svc.GetGroup(ctx(), e.beto, group)
	wantErr(t, err, ErrExpenseNotFound, "grupo pessoal privado visto pelo parceiro")
	_, err = e.svc.DeleteGroup(ctx(), e.beto, group)
	wantErr(t, err, ErrExpenseNotFound, "apagar grupo pessoal privado do parceiro")
	if bal, _ := e.svc.Balance(ctx(), e.ana, "2027-12-31"); bal.AmountCents != 0 || len(bal.Expenses) != 0 {
		t.Errorf("parcela pessoal nunca entra no saldo: %+v", bal)
	}
}
