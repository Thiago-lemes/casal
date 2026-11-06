package feedback

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"casal/internal/apperr"
	"casal/internal/db"
	"casal/internal/testutil"
)

type fixture struct {
	svc        *Service
	conn       *sql.DB
	ana, bia   string // parceiras do casal A
	cris       string // pessoa do casal B
	houseA, hB string
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	conn := testutil.OpenDB(t)
	f := fixture{svc: NewService(conn), conn: conn}
	f.ana = testutil.NewUser(t, conn, "fb-ana")
	f.bia = testutil.NewUser(t, conn, "fb-bia")
	f.cris = testutil.NewUser(t, conn, "fb-cris")
	f.houseA = testutil.NewHousehold(t, conn, f.ana, f.bia)
	f.hB = testutil.NewHousehold(t, conn, f.cris)
	return f
}

func TestEnviaEValida(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	if err := f.svc.Send(ctx, f.houseA, f.ana, "  o botão de salvar some no celular  ", "tasks"); err != nil {
		t.Fatal(err)
	}
	for name, call := range map[string]func() error{
		"vazia":         func() error { return f.svc.Send(ctx, f.houseA, f.ana, "   ", "") },
		"longa demais":  func() error { return f.svc.Send(ctx, f.houseA, f.ana, strings.Repeat("a", 2001), "") },
		"tela inválida": func() error { return f.svc.Send(ctx, f.houseA, f.ana, "oi", "../../etc") },
	} {
		if err := call(); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%s: esperava ErrInvalidInput, veio %v", name, err)
		}
	}
	// 2000 caracteres (contados como letras, não bytes) cabem
	if err := f.svc.Send(ctx, f.houseA, f.bia, strings.Repeat("ã", 2000), ""); err != nil {
		t.Errorf("2000 letras deveriam caber: %v", err)
	}

	items, err := f.svc.List(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	var mine *Item
	for i := range items {
		if items[i].HouseholdID == f.houseA && items[i].Screen == "tasks" {
			mine = &items[i]
		}
	}
	if mine == nil || mine.Message != "o botão de salvar some no celular" || mine.Author != "Teste fb-ana" {
		t.Fatalf("mensagem não encontrada ou diferente: %+v", mine)
	}
}

func TestLimiteDiarioPorPessoa(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	for i := 0; i < dailyLimit; i++ {
		if err := f.svc.Send(ctx, f.houseA, f.ana, "mensagem", ""); err != nil {
			t.Fatalf("envio %d: %v", i+1, err)
		}
	}
	err := f.svc.Send(ctx, f.houseA, f.ana, "mais uma", "")
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != "feedback_rate_limited" {
		t.Fatalf("o 11º envio deveria dar feedback_rate_limited, veio %v", err)
	}
	// o limite é por pessoa: a parceira e o outro casal não são afetados
	if err := f.svc.Send(ctx, f.houseA, f.bia, "oi", ""); err != nil {
		t.Errorf("outra pessoa do mesmo casal: %v", err)
	}
	if err := f.svc.Send(ctx, f.hB, f.cris, "oi", ""); err != nil {
		t.Errorf("outro casal: %v", err)
	}
	// 24 horas depois a janela andou e a pessoa volta a poder escrever
	f.svc.now = func() time.Time { return time.Now().Add(25 * time.Hour) }
	if err := f.svc.Send(ctx, f.houseA, f.ana, "de novo", ""); err != nil {
		t.Errorf("depois de 24 h deveria liberar: %v", err)
	}
}

// Dois envios ao mesmo tempo não furam o limite.
func TestLimiteDiarioResisteAEnviosSimultaneos(t *testing.T) {
	f := newFixture(t)
	var ok, limited atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			switch err := f.svc.Send(context.Background(), f.houseA, f.ana, "simultâneo", ""); {
			case err == nil:
				ok.Add(1)
			case errors.Is(err, ErrRateLimited):
				limited.Add(1)
			default:
				t.Errorf("erro inesperado: %v", err)
			}
		}()
	}
	wg.Wait()
	if ok.Load() != dailyLimit || limited.Load() != 20-dailyLimit {
		t.Errorf("ok=%d limitados=%d, esperado %d e %d", ok.Load(), limited.Load(), dailyLimit, 20-dailyLimit)
	}
}

// Isolamento entre casais: com o escopo do casal B, a mensagem do casal A não aparece. Quem trava isso é o RLS (o
// casal nem tem rota de leitura); o teste roda como casal_app no make test-integration.
func TestCasalNaoEnxergaOFeedbackDeOutro(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	var rls bool
	if err := f.conn.QueryRow(`SELECT NOT rolbypassrls AND NOT rolsuper FROM pg_roles WHERE rolname = current_user`).Scan(&rls); err != nil || !rls {
		t.Skip("TEST_DATABASE_URL precisa usar o papel casal_app para testar o RLS")
	}
	if err := f.svc.Send(ctx, f.houseA, f.ana, "texto do casal A", ""); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Send(ctx, f.hB, f.cris, "texto do casal B", ""); err != nil {
		t.Fatal(err)
	}

	visible := func(scope db.Scope) (own, others int) {
		t.Helper()
		err := db.InTx(ctx, f.conn, scope, func(tx *sql.Tx) error {
			return tx.QueryRowContext(ctx, `SELECT
				count(*) FILTER (WHERE household_id = $1), count(*) FILTER (WHERE household_id <> $1)
				FROM feedback WHERE user_id IN ($2, $3, $4)`, scope.HouseholdID, f.ana, f.bia, f.cris).Scan(&own, &others)
		})
		if err != nil {
			t.Fatal(err)
		}
		return own, others
	}
	if own, others := visible(db.Scope{HouseholdID: f.hB}); own != 1 || others != 0 {
		t.Errorf("casal B vê %d próprias e %d de outro casal (esperado 1 e 0)", own, others)
	}
	if own, others := visible(db.Scope{HouseholdID: f.houseA}); own != 1 || others != 0 {
		t.Errorf("casal A vê %d próprias e %d de outro casal (esperado 1 e 0)", own, others)
	}
	// o painel (escopo de sistema) vê os dois
	if own, others := visible(db.Scope{HouseholdID: f.houseA, System: true}); own != 1 || others != 1 {
		t.Errorf("escopo de sistema: %d e %d (esperado 1 e 1)", own, others)
	}
	// e escrever em nome de outro casal é recusado pelo RLS (WITH CHECK)
	err := db.InTx(ctx, f.conn, db.Scope{HouseholdID: f.hB}, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO feedback (household_id, user_id, message) VALUES ($1, $2, 'forjada')`, f.houseA, f.cris)
		return err
	})
	if err == nil {
		t.Error("o casal B não deveria conseguir gravar feedback no casal A")
	}
}

func TestPainelListaEMarcaComoLida(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if err := f.svc.Send(ctx, f.houseA, f.ana, "primeira", "house"); err != nil {
		t.Fatal(err)
	}
	items, _ := f.svc.List(ctx, true)
	var id string
	for _, it := range items {
		if it.HouseholdID == f.houseA && it.Message == "primeira" {
			id = it.ID
		}
	}
	if id == "" {
		t.Fatal("a mensagem nova deveria estar entre as não lidas")
	}
	if err := f.svc.MarkRead(ctx, id); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.MarkRead(ctx, id); err != nil {
		t.Errorf("marcar de novo deveria ser inofensivo: %v", err)
	}
	unread, _ := f.svc.List(ctx, true)
	for _, it := range unread {
		if it.ID == id {
			t.Error("depois de lida, a mensagem não pode aparecer nas não lidas")
		}
	}
	all, _ := f.svc.List(ctx, false)
	found := false
	for _, it := range all {
		if it.ID == id && it.ReadAt != nil {
			found = true
		}
	}
	if !found {
		t.Error("a mensagem lida deveria continuar na lista completa, com read_at")
	}
	if err := f.svc.MarkRead(ctx, "00000000-0000-0000-0000-000000000000"); !errors.Is(err, ErrNotFound) {
		t.Errorf("id inexistente: %v", err)
	}
}
