package finance

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"

	"casal/internal/identity"
	"casal/internal/testutil"
)

func TestBalanceIsSymmetricBetweenPartners(t *testing.T) {
	e := newEnv(t)
	e.spend(e.ana, 10000, "2026-11-01")
	e.spend(e.beto, 4000, "2026-11-02")

	forAna, err := e.svc.Balance(ctx(), e.ana, "")
	mustNoErr(t, err)
	forBeto, err := e.svc.Balance(ctx(), e.beto, "")
	mustNoErr(t, err)
	// Ana pagou 10000 e Beto 4000; a parte de cada um é 7000: Beto deve 3000 à Ana
	for _, b := range []BalanceView{forAna, forBeto} {
		if b.AmountCents != 3000 || b.FromUser != e.beto.UserID || b.ToUser != e.ana.UserID {
			t.Errorf("saldo: %+v", b)
		}
		if b.PaidCents[e.ana.UserID] != 10000 || b.PaidCents[e.beto.UserID] != 4000 {
			t.Errorf("quanto cada um pagou: %v", b.PaidCents)
		}
	}
}

func TestSettleZeroesBalanceAndLocksExpenses(t *testing.T) {
	e := newEnv(t)
	x := e.spend(e.ana, 10000, "2026-11-01")
	st, err := e.svc.Settle(ctx(), e.beto, "", 5000)
	mustNoErr(t, err)
	if st.FromUser != e.beto.UserID || st.ToUser != e.ana.UserID || st.AmountCents != 5000 || st.Kind != "manual" {
		t.Errorf("acerto: %+v", st)
	}
	bal, _ := e.svc.Balance(ctx(), e.ana, "")
	if bal.AmountCents != 0 || len(bal.Expenses) != 0 {
		t.Errorf("o saldo deveria voltar a zero: %+v", bal)
	}
	got, _ := e.svc.GetExpense(ctx(), e.ana, x.ID)
	if !got.Settled || got.SettlementID != st.ID || got.CanEdit {
		t.Errorf("o gasto deveria estar travado: %+v", got)
	}
	_, err = e.svc.Settle(ctx(), e.ana, "", 5000)
	wantErr(t, err, ErrNothingToSettle, "acertar sem saldo")
	if list, _ := e.svc.ListSettlements(ctx(), e.ana); len(list) != 1 {
		t.Errorf("histórico: %+v", list)
	}
	if msgs := e.notes.to(e.ana.UserID); len(msgs) != 1 || msgs[0].Title != "Acerto de contas registrado" {
		t.Errorf("a Ana deveria ser avisada do acerto: %+v", msgs)
	}
}

func TestSettleWithWrongAmountWritesNothing(t *testing.T) {
	e := newEnv(t)
	x := e.spend(e.ana, 10000, "2026-11-01")
	for _, wrong := range []int64{4999, 5001, 10000} {
		_, err := e.svc.Settle(ctx(), e.beto, "", wrong)
		wantErr(t, err, ErrSettlementStale, fmt.Sprintf("acerto de %d", wrong))
	}
	_, err := e.svc.Settle(ctx(), e.beto, "", 0)
	wantErr(t, err, ErrAmountInvalid, "acerto zerado")
	if list, _ := e.svc.ListSettlements(ctx(), e.ana); len(list) != 0 {
		t.Errorf("nenhum acerto deveria ter sido gravado: %+v", list)
	}
	if got, _ := e.svc.GetExpense(ctx(), e.ana, x.ID); got.Settled {
		t.Error("o gasto não pode ter sido travado")
	}
}

// Dois acertos ao mesmo tempo: um passa, o outro vê o saldo mudado.
func TestConcurrentSettlementsOnePassesOneIsStale(t *testing.T) {
	e := newEnv(t)
	e.spend(e.ana, 10000, "2026-11-01")

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = e.repo.CreateSettlement(ctx(), e.hh, e.beto.UserID, "2026-11-15", 5000)
		}()
	}
	wg.Wait()
	ok, stale := 0, 0
	for _, err := range errs {
		switch {
		case err == nil:
			ok++
		case errorsIs(err, ErrSettlementStale):
			stale++
		default:
			t.Errorf("erro inesperado: %v", err)
		}
	}
	if ok != 1 || stale != 1 {
		t.Errorf("esperava 1 acerto e 1 settlement_stale, veio %d e %d", ok, stale)
	}
	if list, _ := e.svc.ListSettlements(ctx(), e.ana); len(list) != 1 {
		t.Errorf("só um acerto gravado: %d", len(list))
	}
}

func TestExpenseAddedBeforeSettleMakesItStale(t *testing.T) {
	e := newEnv(t)
	e.spend(e.ana, 10000, "2026-11-01")
	bal, _ := e.svc.Balance(ctx(), e.beto, "")
	e.spend(e.ana, 2000, "2026-11-02") // alguém lança outro gasto depois de o Beto ver o saldo
	_, err := e.svc.Settle(ctx(), e.beto, "", bal.AmountCents)
	wantErr(t, err, ErrSettlementStale, "acerto com o saldo velho")
}

func TestPersonalAndSingleMemberNeverSettle(t *testing.T) {
	e := newEnv(t)
	e.personal(e.ana, 99999, VisView)
	_, err := e.svc.Settle(ctx(), e.ana, "", 1)
	wantErr(t, err, ErrNothingToSettle, "só gasto pessoal")

	solo := testutil.NewUser(t, e.db, "solo")
	hh := testutil.NewHousehold(t, e.db, solo)
	p := identity.Principal{UserID: solo, HouseholdID: hh, Timezone: "America/Sao_Paulo"}
	e.people.mu.Lock()
	e.people.ps = []Person{{ID: solo, Name: "Solo", Timezone: "America/Sao_Paulo"}}
	e.people.mu.Unlock()
	_, err = e.svc.Balance(ctx(), p, "")
	wantErr(t, err, ErrSingleMember, "saldo com uma pessoa")
	_, err = e.svc.Settle(ctx(), p, "", 100)
	wantErr(t, err, ErrSingleMember, "acerto com uma pessoa")
}

func TestSplitIsStoredPerExpenseAndNotRewrittenWhenDefaultChanges(t *testing.T) {
	e := newEnv(t)
	before := e.spend(e.ana, 10000, "2026-11-01")

	_, err := e.svc.SaveSettings(ctx(), e.ana, ModeFixed, map[string]int{e.ana.UserID: 7000, e.beto.UserID: 3000})
	mustNoErr(t, err)
	after := e.spend(e.ana, 10000, "2026-11-02")
	// um presente para a outra pessoa: 0/100 só neste gasto
	res, err := e.svc.CreateExpense(ctx(), e.ana, ExpenseInput{AmountCents: 10000, CategoryID: e.cat(KeyLeisure), Split: map[string]int{e.beto.UserID: 10000}})
	mustNoErr(t, err)

	got, _ := e.svc.GetExpense(ctx(), e.ana, before.ID)
	if got.Split[e.ana.UserID] != 5000 {
		t.Errorf("o gasto antigo guarda a divisão da época: %v", got.Split)
	}
	if after.Split[e.ana.UserID] != 7000 || after.Split[e.beto.UserID] != 3000 {
		t.Errorf("o gasto novo usa a divisão combinada: %v", after.Split)
	}
	gift := res.Expenses[0]
	if gift.Split[e.beto.UserID] != 10000 || gift.Split[e.ana.UserID] != 0 {
		t.Errorf("a divisão enviada no gasto vale só para ele: %v", gift.Split)
	}
	// saldo: Ana pagou 30000; partes da Ana: 5000 + 7000 + 0 = 12000, então Beto deve 18000
	bal, _ := e.svc.Balance(ctx(), e.ana, "")
	if bal.AmountCents != 18000 || bal.FromUser != e.beto.UserID {
		t.Errorf("saldo com divisões diferentes: %+v", bal)
	}
}

// ---- renda e divisão proporcional ----

const (
	anaIncome  = 1_234_567 // R$ 12.345,67
	betoIncome = 7_654_321 // R$ 76.543,21
)

// leaks diz se o texto contém o valor da renda, sozinho ou formatado.
func leaks(raw []byte, cents int64) bool {
	s := string(raw)
	return strings.Contains(s, fmt.Sprint(cents)) || strings.Contains(s, fmt.Sprintf("%d,%02d", cents/100, cents%100)) ||
		strings.Contains(s, fmt.Sprintf("%d.%02d", cents/100, cents%100))
}

func (e *env) proportional() {
	e.t.Helper()
	_, err := e.svc.SetIncome(ctx(), e.ana, anaIncome)
	mustNoErr(e.t, err)
	_, err = e.svc.SetIncome(ctx(), e.beto, betoIncome)
	mustNoErr(e.t, err)
	_, err = e.svc.Consent(ctx(), e.ana)
	mustNoErr(e.t, err)
	_, err = e.svc.Consent(ctx(), e.beto)
	mustNoErr(e.t, err)
	_, err = e.svc.SaveSettings(ctx(), e.ana, ModeProportional, nil)
	mustNoErr(e.t, err)
}

func TestProportionalNeedsBothIncomesAndBothConsents(t *testing.T) {
	e := newEnv(t)
	_, err := e.svc.SaveSettings(ctx(), e.ana, ModeProportional, nil)
	wantErr(t, err, ErrProportionalOff, "sem renda nem concordância")

	_, err = e.svc.SetIncome(ctx(), e.ana, anaIncome)
	mustNoErr(t, err)
	_, err = e.svc.SetIncome(ctx(), e.beto, betoIncome)
	mustNoErr(t, err)
	_, err = e.svc.SaveSettings(ctx(), e.ana, ModeProportional, nil)
	wantErr(t, err, ErrProportionalOff, "rendas sem concordância")

	_, err = e.svc.Consent(ctx(), e.ana)
	mustNoErr(t, err)
	_, err = e.svc.SaveSettings(ctx(), e.ana, ModeProportional, nil)
	wantErr(t, err, ErrProportionalOff, "só uma concordância")

	_, err = e.svc.Consent(ctx(), e.beto)
	mustNoErr(t, err)
	view, err := e.svc.SaveSettings(ctx(), e.ana, ModeProportional, nil)
	mustNoErr(t, err)
	if !view.Proportional.Available || view.Mode != ModeProportional {
		t.Fatalf("com tudo, a proporcional vale: %+v", view)
	}
	a, b := view.Proportional.Split[e.ana.UserID], view.Proportional.Split[e.beto.UserID]
	if a+b != PointsTotal || a != 1389 || b != 8611 {
		t.Errorf("12.345,67 ÷ 88.888,88 = 13,89%%: %d/%d", a, b)
	}
	// o novo gasto sai com essa divisão
	x := e.spend(e.ana, 10000, "2026-11-03")
	if x.Split[e.ana.UserID] != 1389 || x.Split[e.beto.UserID] != 8611 {
		t.Errorf("gasto novo: %v", x.Split)
	}
	// o parceiro avisa-se de que a divisão mudou, sem valores
	for _, m := range e.notes.to(e.beto.UserID) {
		if m.Title == "A divisão padrão mudou" && (leaks([]byte(m.Body), anaIncome) || strings.Contains(m.Body, "%")) {
			t.Errorf("o aviso não pode trazer valores: %+v", m)
		}
	}
}

func TestProportionalFallsBackToEqualWhenUnavailable(t *testing.T) {
	e := newEnv(t)
	e.proportional()
	// a renda some por fora do fluxo normal (que já volta o casal para igual): o gasto cai em igual com o aviso
	mustNoErr(t, e.repo.inHousehold(ctx(), e.hh, func(tx txT) error {
		_, err := tx.ExecContext(ctx(), `DELETE FROM finance_incomes WHERE user_id = $1`, e.beto.UserID)
		return err
	}))
	res, err := e.svc.CreateExpense(ctx(), e.ana, ExpenseInput{AmountCents: 10000, CategoryID: e.cat(KeyOther)})
	mustNoErr(t, err)
	if !res.SplitFallback || res.Expenses[0].Split[e.ana.UserID] != 5000 {
		t.Errorf("esperava split_fallback com divisão igual: %+v", res)
	}
}

func TestWithdrawingConsentOrDeletingIncomeGoesBackToEqual(t *testing.T) {
	e := newEnv(t)
	e.proportional()
	view, err := e.svc.WithdrawConsent(ctx(), e.beto)
	mustNoErr(t, err)
	if view.Mode != ModeEqual || view.Proportional.Available {
		t.Errorf("retirar a concordância volta para igual: %+v", view)
	}
	e.proportional()
	e.notes.mu.Lock()
	e.notes.sent = nil
	e.notes.mu.Unlock()
	mustNoErr(t, e.svc.DeleteIncome(ctx(), e.ana))
	view, _ = e.svc.GetSettings(ctx(), e.ana)
	if view.Mode != ModeEqual || view.Proportional.Available || view.Proportional.IncomeSet[e.ana.UserID] {
		t.Errorf("apagar a renda volta para igual: %+v", view)
	}
	if msgs := e.notes.to(e.beto.UserID); len(msgs) != 1 || msgs[0].Title != "A divisão padrão mudou" {
		t.Errorf("o Beto deveria ser avisado: %+v", msgs)
	}
}

func TestIncomeChangeNotifiesPartnerWithoutValuesAndKeepsOldExpenses(t *testing.T) {
	e := newEnv(t)
	e.proportional()
	old := e.spend(e.ana, 10000, "2026-11-03")
	e.notes.mu.Lock()
	e.notes.sent = nil
	e.notes.mu.Unlock()

	_, err := e.svc.SetIncome(ctx(), e.beto, 9_999_999)
	mustNoErr(t, err)
	msgs := e.notes.to(e.ana.UserID)
	if len(msgs) != 1 || msgs[0].Title != "A divisão padrão mudou" {
		t.Fatalf("a Ana deveria ser avisada: %+v", msgs)
	}
	raw, _ := json.Marshal(msgs)
	if leaks(raw, 9_999_999) || leaks(raw, betoIncome) {
		t.Errorf("o aviso trouxe a renda: %s", raw)
	}
	if got, _ := e.svc.GetExpense(ctx(), e.ana, old.ID); got.Split[e.ana.UserID] != 1389 {
		t.Errorf("mudar a renda não reescreve gastos antigos: %v", got.Split)
	}
	if x := e.spend(e.ana, 10000, "2026-11-04"); x.Split[e.ana.UserID] == 1389 {
		t.Errorf("o gasto novo já usa a renda nova: %v", x.Split)
	}
	_, err = e.svc.SetIncome(ctx(), e.beto, 0)
	wantErr(t, err, ErrIncomeInvalid, "renda zero")
	_, err = e.svc.SetIncome(ctx(), e.beto, MaxAmountCents+1)
	wantErr(t, err, ErrIncomeInvalid, "renda acima do teto")
}

// A renda do parceiro nunca aparece em nenhuma resposta, com os dois no mesmo casal.
func TestPartnerIncomeNeverAppearsInAnyResponse(t *testing.T) {
	e := newEnv(t)
	e.proportional()
	e.spend(e.beto, 20000, "2026-11-01")
	e.personal(e.beto, 500, VisView)
	b0, _ := e.svc.Balance(ctx(), e.ana, "")
	_, err := e.svc.Settle(ctx(), e.ana, "", b0.AmountCents)
	mustNoErr(t, err)

	// tudo o que a Ana consegue ler
	own, _ := e.svc.GetIncome(ctx(), e.ana)
	if own == nil || *own != anaIncome {
		t.Fatalf("a Ana lê a própria renda: %v", own)
	}
	settings, _ := e.svc.GetSettings(ctx(), e.ana)
	bal, _ := e.svc.Balance(ctx(), e.ana, "")
	sum, _ := e.svc.Summary(ctx(), e.ana, "2026-11", "household")
	sumMine, _ := e.svc.Summary(ctx(), e.ana, "2026-11", "mine")
	list, _ := e.svc.ListExpenses(ctx(), e.ana, ExpenseFilter{})
	mine, _ := e.svc.ListExpenses(ctx(), e.ana, ExpenseFilter{Scope: "mine"})
	exp, _ := e.svc.Export(ctx(), e.ana)
	sets, _ := e.svc.ListSettlements(ctx(), e.ana)
	cats, _ := e.svc.ListCategories(ctx(), e.ana, true)
	var csv strings.Builder
	_ = exp.WriteCSV(&csv)

	for name, v := range map[string]any{"settings": settings, "balance": bal, "summary": sum, "summary mine": sumMine,
		"expenses": list, "mine": mine, "export": exp, "settlements": sets, "categories": cats, "csv": csv.String(),
		"notificações": e.notes.to(e.ana.UserID)} {
		raw, _ := json.Marshal(v)
		if leaks(raw, betoIncome) {
			t.Errorf("a renda do Beto apareceu em %s: %s", name, raw)
		}
	}
	// e a simétrica: a do Beto não traz a da Ana
	bsettings, _ := e.svc.GetSettings(ctx(), e.beto)
	bexp, _ := e.svc.Export(ctx(), e.beto)
	for name, v := range map[string]any{"settings": bsettings, "export": bexp} {
		raw, _ := json.Marshal(v)
		if leaks(raw, anaIncome) {
			t.Errorf("a renda da Ana apareceu para o Beto em %s: %s", name, raw)
		}
	}
	// a exportação do dono traz a dele
	if bexp.MyIncomeCents == nil || *bexp.MyIncomeCents != betoIncome {
		t.Errorf("a exportação do dono traz a própria renda: %v", bexp.MyIncomeCents)
	}
	if exp.MyIncomeCents == nil || *exp.MyIncomeCents != anaIncome {
		t.Errorf("a exportação da Ana traz só a dela: %v", exp.MyIncomeCents)
	}
	// o parceiro sem renda informada não lê nada de ninguém
	mustNoErr(t, e.svc.DeleteIncome(ctx(), e.beto))
	if c, _ := e.svc.GetIncome(ctx(), e.beto); c != nil {
		t.Error("renda apagada")
	}
}

func TestIncomeMovesOnlyWithItsOwnerRowsAreScopedByUser(t *testing.T) {
	e := newEnv(t)
	_, err := e.svc.SetIncome(ctx(), e.ana, anaIncome)
	mustNoErr(t, err)
	// o parceiro não lê a renda da Ana nem pedindo direto ao repositório pelo id dela e o casal dele
	if _, ok, _ := e.repo.GetIncome(ctx(), e.hh, e.beto.UserID); ok {
		t.Error("o Beto não tem renda")
	}
	if c, _ := e.svc.GetIncome(ctx(), e.beto); c != nil {
		t.Errorf("GET /income do Beto devolveu uma renda: %v", *c)
	}
}
