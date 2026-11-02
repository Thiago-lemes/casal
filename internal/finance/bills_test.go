package finance

import (
	"sort"
	"strings"
	"testing"
	"time"
)

func (e *env) bill(in BillInput) Bill {
	e.t.Helper()
	b, err := e.svc.CreateBill(ctx(), e.ana, in)
	mustNoErr(e.t, err)
	return b
}

func utcHour(t time.Time) int { return t.UTC().Hour() }

func refsOf(m map[string]*fakeRem) []string {
	out := make([]string, 0, len(m))
	for r := range m {
		out = append(out, r)
	}
	sort.Strings(out)
	return out
}

func TestBillCreatesRemindersThreeDaysBeforeAndOnTheDayInPayersTimezone(t *testing.T) {
	e := newEnv(t)
	// o Beto paga e está em Manaus (UTC-4): 09:00 locais são 13:00 UTC; em São Paulo (UTC-3) seriam 12:00 UTC
	e.people.mu.Lock()
	e.people.ps[1].Timezone = "America/Manaus"
	e.people.mu.Unlock()
	b := e.bill(BillInput{Title: "Luz", DueOn: "2026-11-20", ResponsibleID: e.beto.UserID})
	if b.SyncPending || b.CategoryID != e.cat(KeyBills) || b.Repeat != "none" || b.AmountCents != nil {
		t.Fatalf("conta: %+v", b)
	}
	act := e.rem.active()
	stored, err := e.svc.GetBill(ctx(), e.ana, b.ID)
	mustNoErr(t, err)
	if len(act) != 2 || len(stored.ReminderRefs) != 2 {
		t.Fatalf("esperava 2 lembretes (3 dias antes e no dia): %v / %v", refsOf(act), stored.ReminderRefs)
	}
	for ref, r := range act {
		if r.Recipient != e.beto.UserID || utcHour(r.At) != 13 {
			t.Errorf("%s: %+v (09:00 em Manaus = 13:00 UTC)", ref, r)
		}
		wantDay := 20
		if strings.HasSuffix(ref, ":d3") {
			wantDay = 17
		}
		if r.At.UTC().Day() != wantDay {
			t.Errorf("%s no dia %d, esperava %d", ref, r.At.UTC().Day(), wantDay)
		}
		if !strings.Contains(r.Body, "Luz") {
			t.Errorf("texto: %q", r.Body)
		}
	}
}

func TestBillNotifyBothAndPastRemindersAreSkipped(t *testing.T) {
	e := newEnv(t)
	// vence amanhã (16/11): o de "3 dias antes" já passou e não é criado; o do dia, sim, para os dois
	e.bill(BillInput{Title: "Internet", DueOn: "2026-11-16", NotifyBoth: true})
	act := e.rem.active()
	if len(act) != 2 {
		t.Fatalf("esperava só o lembrete do dia, para os dois: %v", refsOf(act))
	}
	got := map[string]bool{}
	for ref, r := range act {
		got[r.Recipient] = true
		if !strings.HasSuffix(ref, ":d0") {
			t.Errorf("só o lembrete do dia deveria existir: %s", ref)
		}
	}
	if !got[e.ana.UserID] || !got[e.beto.UserID] {
		t.Errorf("os dois deveriam receber: %v", got)
	}
}

func TestChangingDueDateRedoesReminders(t *testing.T) {
	e := newEnv(t)
	b := e.bill(BillInput{Title: "Luz", DueOn: "2026-11-20"})
	oldRefs := refsOf(e.rem.active())

	up, err := e.svc.UpdateBill(ctx(), e.ana, b.ID, BillInput{Title: "Luz", DueOn: "2026-11-25"})
	mustNoErr(t, err)
	if up.DueOn != "2026-11-25" || up.SyncPending {
		t.Fatalf("conta: %+v", up)
	}
	now := refsOf(e.rem.active())
	if len(now) != 2 {
		t.Fatalf("depois de mudar: %v", now)
	}
	for _, r := range oldRefs {
		for _, n := range now {
			if r == n {
				t.Errorf("o lembrete velho %s deveria ter sido cancelado", r)
			}
		}
		if !e.rem.byRef[r].Canceled {
			t.Errorf("%s deveria estar cancelado", r)
		}
	}
	for _, n := range now {
		if !strings.Contains(n, "2026-11-25") {
			t.Errorf("o novo ref carrega o novo vencimento: %s", n)
		}
	}
	// trocar o responsável também refaz
	up, err = e.svc.UpdateBill(ctx(), e.ana, b.ID, BillInput{Title: "Luz", DueOn: "2026-11-25", ResponsibleID: e.beto.UserID})
	mustNoErr(t, err)
	for _, r := range e.rem.active() {
		if r.Recipient != e.beto.UserID {
			t.Errorf("o lembrete agora é do Beto: %+v", r)
		}
	}
}

func TestPayBillCreatesExpenseAndCancelsReminders(t *testing.T) {
	e := newEnv(t)
	b := e.bill(BillInput{Title: "Luz", DueOn: "2026-11-20"})
	res, err := e.svc.PayBill(ctx(), e.beto, b.ID, PayInput{AmountCents: 21090})
	mustNoErr(t, err)
	if res.Bill.PaidAt == nil || res.Bill.ExpenseID != res.Expense.ID || res.Bill.PaidBy != e.beto.UserID {
		t.Fatalf("conta paga: %+v", res.Bill)
	}
	x := res.Expense
	if x.Source != SourceBill || x.BillID != b.ID || x.AmountCents != 21090 || x.PaidBy != e.beto.UserID ||
		x.CategoryID != b.CategoryID || x.Description != "Luz" || x.Split[e.ana.UserID] != 5000 || x.Scope != ScopeHousehold {
		t.Errorf("gasto da conta: %+v", x)
	}
	if len(e.rem.active()) != 0 {
		t.Errorf("pagar cancela os lembretes pendentes: %v", refsOf(e.rem.active()))
	}
	_, err = e.svc.PayBill(ctx(), e.ana, b.ID, PayInput{AmountCents: 21090})
	wantErr(t, err, ErrBillAlreadyPaid, "pagar duas vezes")
	_, err = e.svc.UpdateBill(ctx(), e.ana, b.ID, BillInput{Title: "x", DueOn: "2026-12-01"})
	wantErr(t, err, ErrBillAlreadyPaid, "editar conta paga")
	if open, _ := e.svc.ListBills(ctx(), e.ana, "open", "", false); len(open) != 0 {
		t.Errorf("sem abertas: %+v", open)
	}
	if paid, _ := e.svc.ListBills(ctx(), e.ana, "paid", "", false); len(paid) != 1 {
		t.Errorf("uma paga: %+v", paid)
	}
}

func TestPayBillNeedsAnAmountWhenTheBillHasNone(t *testing.T) {
	e := newEnv(t)
	b := e.bill(BillInput{Title: "Luz", DueOn: "2026-11-20"})
	_, err := e.svc.PayBill(ctx(), e.ana, b.ID, PayInput{})
	wantErr(t, err, ErrAmountInvalid, "conta sem valor e sem valor no pagamento")
	fixed := int64(5000)
	b2 := e.bill(BillInput{Title: "Aluguel", DueOn: "2026-11-20", AmountCents: &fixed})
	res, err := e.svc.PayBill(ctx(), e.ana, b2.ID, PayInput{})
	mustNoErr(t, err)
	if res.Expense.AmountCents != 5000 {
		t.Errorf("usa o valor da conta: %+v", res.Expense)
	}
}

// Se o gasto falhar, a conta continua aberta: pagar e criar o gasto são uma transação só.
func TestPayBillIsAtomic(t *testing.T) {
	e := newEnv(t)
	b := e.bill(BillInput{Title: "Luz", DueOn: "2026-11-20"})
	_, _, _, err := e.repo.PayBill(ctx(), e.hh, b.ID, Expense{
		CreatedBy: e.ana.UserID, PaidBy: e.ana.UserID, Scope: ScopeHousehold, Visibility: VisHousehold, AmountCents: 100,
		CategoryID: newUUID(), // categoria que não existe: o gasto falha
		SpentOn:    "2026-11-15", Split: map[string]int{e.ana.UserID: 10000}, Source: SourceBill,
	})
	if err == nil {
		t.Fatal("deveria falhar")
	}
	got, err := e.svc.GetBill(ctx(), e.ana, b.ID)
	mustNoErr(t, err)
	if got.PaidAt != nil || got.ExpenseID != "" {
		t.Errorf("a conta deveria continuar aberta: %+v", got)
	}
	if page, _ := e.svc.ListExpenses(ctx(), e.ana, ExpenseFilter{}); len(page.Expenses) != 0 {
		t.Errorf("nenhum gasto deveria ter sido criado: %+v", page.Expenses)
	}
}

func TestMonthlyBillCreatesNextWithEndOfMonthRule(t *testing.T) {
	e := newEnv(t)
	amount := int64(10000)
	b := e.bill(BillInput{Title: "Condomínio", DueOn: "2027-01-31", Repeat: "monthly", AmountCents: &amount, NotifyBoth: true})
	res, err := e.svc.PayBill(ctx(), e.ana, b.ID, PayInput{PaidOn: "2027-01-30"})
	mustNoErr(t, err)
	n := res.NextBill
	if n == nil || n.DueOn != "2027-02-28" || n.Title != "Condomínio" || n.Repeat != "monthly" || n.PaidAt != nil ||
		n.AmountCents == nil || *n.AmountCents != 10000 || !n.NotifyBoth {
		t.Fatalf("próxima conta: %+v", n)
	}
	res2, err := e.svc.PayBill(ctx(), e.ana, n.ID, PayInput{})
	mustNoErr(t, err)
	if res2.NextBill == nil || res2.NextBill.DueOn != "2027-03-31" {
		t.Errorf("dia 31 volta em março: %+v", res2.NextBill)
	}
	if res.Expense.SpentOn != "2027-01-30" {
		t.Errorf("paid_on vira a data do gasto: %s", res.Expense.SpentOn)
	}
}

func TestBillListOrderingAndDueToday(t *testing.T) {
	e := newEnv(t)
	e.bill(BillInput{Title: "Futura", DueOn: "2026-12-10"})
	e.bill(BillInput{Title: "Vencida", DueOn: "2026-11-01"})
	e.bill(BillInput{Title: "Hoje", DueOn: "2026-11-15"})

	open, err := e.svc.ListBills(ctx(), e.ana, "", "", false)
	mustNoErr(t, err)
	if len(open) != 3 || open[0].Title != "Vencida" || open[1].Title != "Hoje" || open[2].Title != "Futura" {
		t.Errorf("as vencidas vêm primeiro: %+v", open)
	}
	today, _ := e.svc.ListBills(ctx(), e.ana, "", "", true)
	if len(today) != 2 || today[0].Title != "Vencida" || today[1].Title != "Hoje" {
		t.Errorf("due=today traz as de hoje e as vencidas: %+v", today)
	}
	before, _ := e.svc.ListBills(ctx(), e.ana, "open", "2026-11-10", false)
	if len(before) != 1 || before[0].Title != "Vencida" {
		t.Errorf("due_before: %+v", before)
	}
	_, err = e.svc.ListBills(ctx(), e.ana, "x", "", false)
	wantErr(t, err, ErrInvalidInput, "status inválido")
}

func TestReminderFailureReturnsSyncPendingAndJobReconciles(t *testing.T) {
	e := newEnv(t)
	e.enableModule()
	e.rem.failCreate = true
	b := e.bill(BillInput{Title: "Luz", DueOn: "2026-11-20"})
	if !b.SyncPending {
		t.Fatal("a conta deveria avisar sync_pending")
	}
	if got, _ := e.svc.GetBill(ctx(), e.ana, b.ID); got.PaidAt != nil {
		t.Errorf("a conta foi gravada mesmo assim: %+v", got)
	}
	if len(e.rem.active()) != 0 {
		t.Fatal("nenhum lembrete ainda")
	}
	e.rem.failCreate = false
	n, err := e.svc.ReconcileReminders(ctx())
	mustNoErr(t, err)
	if n < 1 || len(e.rem.active()) != 2 {
		t.Errorf("o job deveria refazer os lembretes: n=%d %v", n, refsOf(e.rem.active()))
	}
	// rodar de novo não muda nada
	before := refsOf(e.rem.active())
	_, _ = e.svc.ReconcileReminders(ctx())
	if after := refsOf(e.rem.active()); strings.Join(before, ",") != strings.Join(after, ",") {
		t.Errorf("a reconciliação é idempotente: %v -> %v", before, after)
	}
}

func TestDisablingModuleCancelsRemindersAndEnablingRedoesThem(t *testing.T) {
	e := newEnv(t)
	e.enableModule()
	b := e.bill(BillInput{Title: "Luz", DueOn: "2026-11-20"})
	if len(e.rem.active()) != 2 {
		t.Fatal("lembretes criados")
	}
	e.svc.ModuleChanged(ctx(), e.hh, false)
	if len(e.rem.active()) != 0 {
		t.Errorf("sem o módulo, nenhum lembrete de Finanças existe: %v", refsOf(e.rem.active()))
	}
	if got, err := e.svc.GetBill(ctx(), e.ana, b.ID); err != nil || len(got.ReminderRefs) != 0 {
		t.Errorf("a conta continua, sem refs: %+v %v", got, err)
	}
	e.svc.ModuleChanged(ctx(), e.hh, true)
	if len(e.rem.active()) != 2 {
		t.Errorf("religar traz tudo de volta: %v", refsOf(e.rem.active()))
	}
}

func TestDeleteBillCancelsReminders(t *testing.T) {
	e := newEnv(t)
	b := e.bill(BillInput{Title: "Luz", DueOn: "2026-11-20"})
	mustNoErr(t, e.svc.DeleteBill(ctx(), e.ana, b.ID))
	if len(e.rem.active()) != 0 {
		t.Errorf("apagar a conta cancela os lembretes: %v", refsOf(e.rem.active()))
	}
	_, err := e.svc.GetBill(ctx(), e.ana, b.ID)
	wantErr(t, err, ErrBillNotFound, "conta apagada")
}

func TestBillValidation(t *testing.T) {
	e := newEnv(t)
	neg := int64(-1)
	for name, in := range map[string]BillInput{
		"sem título":          {DueOn: "2026-11-20"},
		"sem data":            {Title: "x"},
		"data inválida":       {Title: "x", DueOn: "20/11/2026"},
		"valor negativo":      {Title: "x", DueOn: "2026-11-20", AmountCents: &neg},
		"repeat inválido":     {Title: "x", DueOn: "2026-11-20", Repeat: "weekly"},
		"responsável de fora": {Title: "x", DueOn: "2026-11-20", ResponsibleID: newUUID()},
	} {
		if _, err := e.svc.CreateBill(ctx(), e.ana, in); err == nil {
			t.Errorf("%s deveria falhar", name)
		}
	}
}
