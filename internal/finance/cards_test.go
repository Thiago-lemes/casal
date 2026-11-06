package finance

import (
	"testing"

	"casal/internal/identity"
	"casal/internal/testutil"
)

func (e *env) card(p identity.Principal, name string) Card {
	e.t.Helper()
	c, err := e.svc.CreateCard(ctx(), p, CardInput{Name: name})
	mustNoErr(e.t, err)
	return c
}

// spendOn registra um gasto do casal pago por p, com o cartão.
func (e *env) spendOn(p identity.Principal, cents int64, cardID string) Expense {
	e.t.Helper()
	res, err := e.svc.CreateExpense(ctx(), p, ExpenseInput{
		AmountCents: cents, CategoryID: e.cat(KeyGroceries), SpentOn: "2026-11-03", CardID: cardID,
	})
	mustNoErr(e.t, err)
	return res.Expenses[0]
}

func TestCardValidationAndPerOwnerNames(t *testing.T) {
	e := newEnv(t)
	d := func(v int) *int { return &v }
	for _, bad := range []CardInput{
		{Name: ""}, {Name: "   "}, {Name: "x", Last4: "12a4"}, {Name: "x", Last4: "123"},
		{Name: "x", ClosingDay: d(0)}, {Name: "x", DueDay: d(32)}, {Name: "x", Brand: "uma bandeira com nome enorme demais"},
	} {
		if _, err := e.svc.CreateCard(ctx(), e.ana, bad); !errorsIs(err, ErrInvalidInput) {
			t.Errorf("%+v: esperava dados inválidos, veio %v", bad, err)
		}
	}
	c, err := e.svc.CreateCard(ctx(), e.ana, CardInput{Name: "Nubank", Brand: "Mastercard", Last4: "4321", ClosingDay: d(25), DueDay: d(5)})
	mustNoErr(t, err)
	if c.Name != "Nubank" || c.Last4 != "4321" || *c.ClosingDay != 25 || *c.DueDay != 5 || c.Archived {
		t.Fatalf("cartão: %+v", c)
	}
	_, err = e.svc.CreateCard(ctx(), e.ana, CardInput{Name: " nubank "})
	wantErr(t, err, ErrCardDuplicate, "mesmo nome (sem diferenciar maiúscula)")
	// o parceiro pode ter um cartão com o mesmo nome: os nomes são por pessoa
	e.card(e.beto, "Nubank")

	up, err := e.svc.UpdateCard(ctx(), e.ana, c.ID, CardInput{Name: "Nu roxinho"})
	mustNoErr(t, err)
	if up.Name != "Nu roxinho" || up.Last4 != "" || up.ClosingDay != nil {
		t.Errorf("editar substitui o cartão: %+v", up)
	}
	_, err = e.svc.UpdateCard(ctx(), e.ana, "nao-e-uuid", CardInput{Name: "x"})
	wantErr(t, err, ErrCardNotFound, "id inválido")
}

func TestCardsArePrivateBetweenPartners(t *testing.T) {
	e := newEnv(t)
	anas := e.card(e.ana, "Itaú")

	if l, _ := e.svc.ListCards(ctx(), e.beto, true); len(l) != 0 {
		t.Errorf("o parceiro não lista o cartão da Ana: %+v", l)
	}
	if l, _ := e.svc.ListCards(ctx(), e.ana, false); len(l) != 1 {
		t.Errorf("a Ana lista o dela: %+v", l)
	}
	_, err := e.svc.UpdateCard(ctx(), e.beto, anas.ID, CardInput{Name: "invadido"})
	wantErr(t, err, ErrCardNotFound, "editar cartão do parceiro")
	_, err = e.svc.ArchiveCard(ctx(), e.beto, anas.ID, true)
	wantErr(t, err, ErrCardNotFound, "arquivar cartão do parceiro")
	// usar o cartão do parceiro num gasto: 404, como se não existisse
	_, err = e.svc.CreateExpense(ctx(), e.beto, ExpenseInput{AmountCents: 100, CategoryID: e.cat(KeyOther), CardID: anas.ID})
	wantErr(t, err, ErrCardNotFound, "gasto no cartão do parceiro")
	if l, _ := e.svc.ListCards(ctx(), e.ana, false); len(l) != 1 || l[0].Name != "Itaú" || l[0].Archived {
		t.Errorf("o cartão da Ana segue intacto: %+v", l)
	}

	// o gasto do casal pago no cartão da Ana: a Ana vê o cartão, o Beto vê o gasto sem ele
	x := e.spendOn(e.ana, 5000, anas.ID)
	if x.MyCardID != anas.ID {
		t.Errorf("a Ana vê o cartão do gasto: %+v", x)
	}
	got, err := e.svc.GetExpense(ctx(), e.beto, x.ID)
	mustNoErr(t, err)
	if got.MyCardID != "" {
		t.Errorf("o Beto não vê o cartão da Ana: %+v", got)
	}
	page, _ := e.svc.ListExpenses(ctx(), e.beto, ExpenseFilter{Month: "2026-11"})
	for _, ex := range page.Expenses {
		if ex.MyCardID != "" {
			t.Errorf("lista do Beto vazou o cartão: %+v", ex)
		}
	}
	// filtrar pelo cartão da Ana: o Beto não acha nada
	page, _ = e.svc.ListExpenses(ctx(), e.beto, ExpenseFilter{CardID: anas.ID})
	if len(page.Expenses) != 0 {
		t.Errorf("o Beto filtrou pelo cartão da Ana: %+v", page.Expenses)
	}
	page, _ = e.svc.ListExpenses(ctx(), e.ana, ExpenseFilter{CardID: anas.ID})
	if len(page.Expenses) != 1 || page.Expenses[0].ID != x.ID {
		t.Errorf("a Ana filtra pelo cartão: %+v", page.Expenses)
	}
}

func TestExpenseCardRules(t *testing.T) {
	e := newEnv(t)
	c := e.card(e.ana, "Visa")

	// cartão é de quem pagou: a Ana não lança um gasto pago pelo Beto no cartão dela
	_, err := e.svc.CreateExpense(ctx(), e.ana, ExpenseInput{AmountCents: 100, CategoryID: e.cat(KeyOther), PaidBy: e.beto.UserID, CardID: c.ID})
	wantErr(t, err, ErrCardInvalid, "gasto pago pelo parceiro no cartão da Ana")

	// o cartão é opcional
	if x := e.spendOn(e.ana, 100, ""); x.MyCardID != "" {
		t.Errorf("sem cartão: %+v", x)
	}

	// arquivado: não serve para gasto novo, mas o gasto antigo mantém
	x := e.spendOn(e.ana, 7000, c.ID)
	_, err = e.svc.ArchiveCard(ctx(), e.ana, c.ID, true)
	mustNoErr(t, err)
	_, err = e.svc.CreateExpense(ctx(), e.ana, ExpenseInput{AmountCents: 100, CategoryID: e.cat(KeyOther), CardID: c.ID})
	wantErr(t, err, ErrCardArchived, "gasto novo em cartão arquivado")
	up, err := e.svc.UpdateExpense(ctx(), e.ana, x.ID, ExpenseInput{AmountCents: 7500, CategoryID: x.CategoryID, CardID: c.ID, Version: x.Version})
	mustNoErr(t, err)
	if up.MyCardID != c.ID || up.AmountCents != 7500 {
		t.Errorf("o gasto antigo mantém o cartão arquivado: %+v", up)
	}
	if l, _ := e.svc.ListCards(ctx(), e.ana, false); len(l) != 0 {
		t.Errorf("arquivado sai da lista de escolha: %+v", l)
	}
	if l, _ := e.svc.ListCards(ctx(), e.ana, true); len(l) != 1 || !l[0].Archived {
		t.Errorf("arquivado aparece quando se pede: %+v", l)
	}
	_, err = e.svc.ArchiveCard(ctx(), e.ana, c.ID, false)
	mustNoErr(t, err)

	// editar: o dono troca e tira o cartão; o campo vazio tira
	other := e.card(e.ana, "Elo")
	up, err = e.svc.UpdateExpense(ctx(), e.ana, x.ID, ExpenseInput{AmountCents: 7500, CategoryID: x.CategoryID, CardID: other.ID, Version: up.Version})
	mustNoErr(t, err)
	if up.MyCardID != other.ID {
		t.Errorf("trocou de cartão: %+v", up)
	}
	up, err = e.svc.UpdateExpense(ctx(), e.ana, x.ID, ExpenseInput{AmountCents: 7500, CategoryID: x.CategoryID, Version: up.Version})
	mustNoErr(t, err)
	if up.MyCardID != "" {
		t.Errorf("tirou o cartão: %+v", up)
	}

	// o parceiro edita o gasto da Ana: o cartão fica como está
	up, err = e.svc.UpdateExpense(ctx(), e.ana, x.ID, ExpenseInput{AmountCents: 7500, CategoryID: x.CategoryID, CardID: c.ID, Version: up.Version})
	mustNoErr(t, err)
	byBeto, err := e.svc.UpdateExpense(ctx(), e.beto, x.ID, ExpenseInput{AmountCents: 8000, CategoryID: x.CategoryID, PaidBy: e.ana.UserID, Version: up.Version})
	mustNoErr(t, err)
	if raw, _ := e.repo.GetExpense(ctx(), e.hh, x.ID, e.ana.UserID); raw.MyCardID != c.ID || byBeto.AmountCents != 8000 {
		t.Errorf("o cartão sobrevive à edição do parceiro: %+v", raw)
	}
	// mas se o Beto passa o pagamento para ele, o cartão da Ana não o acompanha
	moved, err := e.svc.UpdateExpense(ctx(), e.beto, x.ID, ExpenseInput{AmountCents: 8000, CategoryID: x.CategoryID, PaidBy: e.beto.UserID, Version: byBeto.Version})
	mustNoErr(t, err)
	if raw, _ := e.repo.GetExpense(ctx(), e.hh, x.ID, e.beto.UserID); raw.CardID != "" || moved.PaidBy != e.beto.UserID {
		t.Errorf("outro pagador, sem o cartão da Ana: %+v", raw)
	}
	// e o Beto não consegue pôr o cartão da Ana de volta
	_, err = e.svc.UpdateExpense(ctx(), e.beto, x.ID, ExpenseInput{AmountCents: 8000, CategoryID: x.CategoryID, CardID: c.ID, Version: moved.Version})
	wantErr(t, err, ErrCardNotFound, "parceiro põe o cartão da Ana")
}

func TestInstallmentsInheritTheCard(t *testing.T) {
	e := newEnv(t)
	c := e.card(e.ana, "Nubank")
	res, err := e.svc.CreateExpense(ctx(), e.ana, ExpenseInput{
		AmountCents: 30000, CategoryID: e.cat(KeyOther), SpentOn: "2026-11-10", CardID: c.ID, Installments: &InstallmentsInput{Count: 3},
	})
	mustNoErr(t, err)
	for _, x := range res.Expenses {
		if x.MyCardID != c.ID {
			t.Errorf("parcela %d sem o cartão: %+v", x.Installment.No, x)
		}
	}
	group := res.Expenses[0].Installment.GroupID

	// trocar o cartão da compra vale para as parcelas todas
	other := e.card(e.ana, "Itaú")
	g, err := e.svc.UpdateGroup(ctx(), e.ana, group, GroupInput{CardID: &other.ID})
	mustNoErr(t, err)
	for _, x := range g.Expenses {
		if x.MyCardID != other.ID {
			t.Errorf("parcela %d não trocou: %+v", x.Installment.No, x)
		}
	}
	// o parceiro não troca nem tira o cartão da Ana
	empty := ""
	_, err = e.svc.UpdateGroup(ctx(), e.beto, group, GroupInput{CardID: &empty})
	wantErr(t, err, ErrCardInvalid, "parceiro tira o cartão da compra")
	_, err = e.svc.UpdateGroup(ctx(), e.beto, group, GroupInput{CardID: &other.ID})
	wantErr(t, err, ErrCardInvalid, "parceiro põe cartão na compra da Ana")
	// editar outra coisa não mexe no cartão
	desc := "notebook"
	g, err = e.svc.UpdateGroup(ctx(), e.beto, group, GroupInput{Description: &desc})
	mustNoErr(t, err)
	if raw, _ := e.repo.GetGroup(ctx(), e.hh, group, e.ana.UserID); raw[0].MyCardID != other.ID {
		t.Errorf("a descrição não mexe no cartão: %+v", raw[0])
	}
	// tirar o cartão
	g, err = e.svc.UpdateGroup(ctx(), e.ana, group, GroupInput{CardID: &empty})
	mustNoErr(t, err)
	if g.Expenses[0].MyCardID != "" || g.Expenses[2].MyCardID != "" {
		t.Errorf("tirou o cartão: %+v", g.Expenses)
	}
}

func TestSummaryByCard(t *testing.T) {
	e := newEnv(t)
	nu, itau := e.card(e.ana, "Nubank"), e.card(e.ana, "Itaú")
	bc := e.card(e.beto, "Inter")
	e.spendOn(e.ana, 10000, nu.ID)
	e.spendOn(e.ana, 2500, nu.ID)
	e.spendOn(e.ana, 4000, itau.ID)
	e.spendOn(e.ana, 600, "")
	e.spendOn(e.beto, 9900, bc.ID)
	e.spendOn(e.beto, 300, "")
	// pessoal da Ana no Nubank: só entra no "mine"
	_, err := e.svc.CreateExpense(ctx(), e.ana, ExpenseInput{Scope: ScopePersonal, AmountCents: 777, CategoryID: e.cat(KeyLeisure), SpentOn: "2026-11-04", CardID: nu.ID})
	mustNoErr(t, err)

	sum, err := e.svc.Summary(ctx(), e.ana, "2026-11", "household")
	mustNoErr(t, err)
	want := map[string]int64{nu.ID: 12500, itau.ID: 4000}
	if len(sum.Cards) != 2 || sum.Cards[0].CardID != nu.ID || sum.Cards[0].TotalCents != 12500 || sum.Cards[1].TotalCents != 4000 {
		t.Fatalf("cartões da Ana (maior primeiro): %+v, esperava %v", sum.Cards, want)
	}
	if sum.WithoutCardCents != 900 || sum.TotalCents != 10000+2500+4000+600+9900+300 {
		t.Errorf("sem cartão = 900 e o total inclui o cartão do Beto: %+v", sum)
	}

	// o Beto só vê o cartão dele: nada do Nubank ou do Itaú da Ana
	sum, err = e.svc.Summary(ctx(), e.beto, "2026-11", "household")
	mustNoErr(t, err)
	if len(sum.Cards) != 1 || sum.Cards[0].CardID != bc.ID || sum.Cards[0].TotalCents != 9900 || sum.WithoutCardCents != 900 {
		t.Errorf("cartões do Beto: %+v", sum)
	}

	sum, err = e.svc.Summary(ctx(), e.ana, "2026-11", "mine")
	mustNoErr(t, err)
	if len(sum.Cards) != 1 || sum.Cards[0].CardID != nu.ID || sum.Cards[0].TotalCents != 777 || sum.WithoutCardCents != 0 {
		t.Errorf("pessoal da Ana: %+v", sum)
	}
	sum, _ = e.svc.Summary(ctx(), e.beto, "2026-11", "mine")
	if len(sum.Cards) != 0 || sum.WithoutCardCents != 0 {
		t.Errorf("o pessoal da Ana nunca entra no resumo do Beto: %+v", sum)
	}
}

func TestExportShowsOnlyMyCards(t *testing.T) {
	e := newEnv(t)
	nu := e.card(e.ana, "Nubank")
	e.spendOn(e.ana, 1000, nu.ID)
	d, err := e.svc.Export(ctx(), e.beto)
	mustNoErr(t, err)
	if len(d.Cards) != 0 {
		t.Errorf("o parceiro não exporta cartões da Ana: %+v", d.Cards)
	}
	for _, x := range d.Expenses {
		if x.MyCardID != "" {
			t.Errorf("exportação do Beto vazou o cartão: %+v", x)
		}
	}
	d, _ = e.svc.Export(ctx(), e.ana)
	if len(d.Cards) != 1 {
		t.Errorf("a Ana exporta o dela: %+v", d.Cards)
	}
}

func TestCardsAreIsolatedBetweenHouseholds(t *testing.T) {
	a, b := newEnv(t), newEnv(t)
	c := a.card(a.ana, "Nubank")
	_, err := b.svc.CreateExpense(ctx(), b.ana, ExpenseInput{AmountCents: 100, CategoryID: b.cat(KeyOther), CardID: c.ID})
	wantErr(t, err, ErrCardNotFound, "cartão de outro casal")
	_, err = b.svc.UpdateCard(ctx(), b.ana, c.ID, CardInput{Name: "x"})
	wantErr(t, err, ErrCardNotFound, "editar cartão de outro casal")
	if l, _ := b.svc.ListCards(ctx(), b.ana, true); len(l) != 0 {
		t.Errorf("cartões do B: %+v", l)
	}
}

func TestCardFollowsThePersonWhenLeavingAndJoining(t *testing.T) {
	e := newEnv(t)
	e.enableModule()
	c := e.card(e.beto, "Inter")
	shared := e.spendOn(e.beto, 5000, c.ID)
	mine, err := e.svc.CreateExpense(ctx(), e.beto, ExpenseInput{Scope: ScopePersonal, AmountCents: 900, CategoryID: e.cat(KeyLeisure), CardID: c.ID})
	mustNoErr(t, err)

	e.leave(e.beto.UserID, e.ana.UserID)

	// o gasto do casal que ficou para trás perde a ligação com o cartão que foi embora com o Beto
	if raw, _ := e.repo.GetExpense(ctx(), e.hh, shared.ID, e.ana.UserID); raw.CardID != "" {
		t.Errorf("gasto do casal ainda aponta para o cartão de quem saiu: %+v", raw)
	}

	novo := testutil.NewHousehold(t, e.db, e.beto.UserID)
	e.join(e.beto.UserID, novo)

	cards, err := e.repo.ListCards(ctx(), novo, e.beto.UserID, true)
	mustNoErr(t, err)
	if len(cards) != 1 || cards[0].ID != c.ID {
		t.Fatalf("o cartão vai com a pessoa para o casal novo: %+v", cards)
	}
	if raw, err := e.repo.GetExpense(ctx(), novo, mine.Expenses[0].ID, e.beto.UserID); err != nil || raw.MyCardID != c.ID {
		t.Errorf("o gasto pessoal mantém o cartão: %+v %v", raw, err)
	}
	if l, _ := e.repo.ListCards(ctx(), e.hh, e.beto.UserID, true); len(l) != 0 {
		t.Errorf("o cartão não fica no casal antigo: %+v", l)
	}
}
