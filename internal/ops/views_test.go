package ops

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"casal/internal/db"
	"casal/internal/testutil"
)

// O papel do Grafana não lê nenhuma tabela de negócio: nem as do schema public, nem as de eventos e atividade.
func TestCasalOpsNaoLeTabelasDeNegocio(t *testing.T) {
	owner := testutil.OpenDB(t)
	ops := testutil.OpenOpsDB(t)

	rows, err := owner.Query(`
		SELECT c.relname FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'public' AND c.relkind IN ('r', 'v', 'm', 'p', 'f')
		  AND has_table_privilege('casal_ops', c.oid, 'SELECT')`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		_ = rows.Scan(&name)
		t.Errorf("casal_ops consegue ler a tabela %q", name)
	}

	// e na prática: a leitura direta é negada
	for _, table := range []string{"users", "households", "reminders", "notes", "expenses", "sessions", "product_events", "user_activity"} {
		if r, err := ops.Query(`SELECT * FROM ` + table); err == nil {
			r.Close()
			t.Errorf("SELECT em %s deveria ser negado a casal_ops", table)
		}
	}
	if _, err := ops.Exec(`CREATE TABLE public.intruso (x int)`); err == nil {
		t.Error("casal_ops não deveria criar tabelas")
	}
}

// Todas as views do schema ops respondem ao papel certo.
func TestCasalOpsLeAsViewsDeAgregados(t *testing.T) {
	ops := testutil.OpenOpsDB(t)
	for _, view := range []string{"daily_activation", "weekly_active", "module_adoption", "module_usage", "retention", "push_coverage"} {
		rows, err := ops.Query(`SELECT * FROM ops.` + view)
		if err != nil {
			t.Errorf("ops.%s: %v", view, err)
			continue
		}
		rows.Close()
	}
}

// Célula com menos de 3 vira nula; com 3 ou mais mostra o número.
func TestAgregadosComMenosDeTresViramNulos(t *testing.T) {
	owner := testutil.OpenDB(t)
	ops := testutil.OpenOpsDB(t)

	var two, three sql.NullInt64
	if err := ops.QueryRow(`SELECT ops.mask(2), ops.mask(3)`).Scan(&two, &three); err != nil {
		t.Fatal(err)
	}
	if two.Valid || !three.Valid || three.Int64 != 3 {
		t.Fatalf("mask(2)=%v mask(3)=%v", two, three)
	}

	// dias reservados a este teste (nenhum outro cria gente com essas datas)
	day3, day2 := "2001-03-05", "2002-03-04"
	for i := 0; i < 3; i++ {
		id := testutil.NewUser(t, owner, "mask3")
		if _, err := owner.Exec(`UPDATE users SET created_at = $2 WHERE id = $1`, id, day3+" 12:00:00+00"); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 2; i++ {
		id := testutil.NewUser(t, owner, "mask2")
		if _, err := owner.Exec(`UPDATE users SET created_at = $2 WHERE id = $1`, id, day2+" 12:00:00+00"); err != nil {
			t.Fatal(err)
		}
	}

	signups := func(day string) sql.NullInt64 {
		var n sql.NullInt64
		if err := ops.QueryRow(`SELECT signups FROM ops.daily_activation WHERE day = $1::date`, day).Scan(&n); err != nil {
			t.Fatalf("dia %s: %v", day, err)
		}
		return n
	}
	if n := signups(day3); !n.Valid || n.Int64 != 3 {
		t.Errorf("com 3 cadastros o número deve aparecer, veio %v", n)
	}
	if n := signups(day2); n.Valid {
		t.Errorf("com 2 cadastros a célula deve ser nula, veio %v", n)
	}
}

// A tabela de eventos não tem onde guardar texto: só nome (com CHECK), ids e hora.
func TestEventosNaoTemOndeGuardarTexto(t *testing.T) {
	owner := testutil.OpenDB(t)
	rows, err := owner.Query(`
		SELECT column_name, data_type FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = 'product_events'`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var col, typ string
		_ = rows.Scan(&col, &typ)
		switch typ {
		case "uuid", "bigint", "timestamp with time zone":
		case "text":
			if col != "name" {
				t.Errorf("product_events.%s é texto livre; só name (com CHECK) pode ser", col)
			}
		default:
			t.Errorf("product_events.%s tem tipo %s (JSON e afins podem guardar conteúdo)", col, typ)
		}
	}

	// o CHECK recusa um nome fora da lista
	if _, err := owner.Exec(`INSERT INTO product_events (name) VALUES ('texto-livre')`); err == nil {
		t.Error("a tabela aceitou um nome fora da lista fechada")
	}
}

func TestPurgeApagaOQueEstaForaDaRetencao(t *testing.T) {
	owner := testutil.OpenDB(t)
	events := NewEvents(owner, discardLogger())
	userID := testutil.NewUser(t, owner, "retencao")
	old := time.Now().AddDate(-2, 0, 0)

	testutil.RunHook(t, owner, func(ctx context.Context, tx *sql.Tx, _ db.Handoff) error {
		if err := db.SetScope(ctx, tx, db.Scope{System: true}); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO product_events (name, user_id, occurred_at) VALUES ('user_registered', $1, $2)`, userID, old); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO user_activity (user_id, day) VALUES ($1, $2::date)`, userID, old)
		return err
	}, db.Handoff{})
	events.Record(context.Background(), "user_registered", userID, "") // recente

	if _, err := events.Purge(context.Background()); err != nil {
		t.Fatal(err)
	}
	var oldEvents, recent, oldDays int
	testutil.SystemScan(t, owner, `SELECT count(*) FILTER (WHERE occurred_at < now() - interval '13 months'),
		count(*) FILTER (WHERE occurred_at >= now() - interval '1 day') FROM product_events WHERE user_id = $1`,
		[]any{userID}, &oldEvents, &recent)
	testutil.SystemScan(t, owner, `SELECT count(*) FROM user_activity WHERE user_id = $1 AND day < now() - interval '13 months'`,
		[]any{userID}, &oldDays)
	if oldEvents != 0 || oldDays != 0 || recent != 1 {
		t.Errorf("eventos antigos=%d, dias antigos=%d, recentes=%d (esperado 0, 0, 1)", oldEvents, oldDays, recent)
	}
}
