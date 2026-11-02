package testutil

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"casal/internal/db"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// OpenDB conecta no banco de teste ou pula o teste se TEST_DATABASE_URL não estiver definida.
func OpenDB(t *testing.T) *sql.DB {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("defina TEST_DATABASE_URL para rodar os testes de integração")
	}
	db, err := sql.Open("pgx", url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Ping(); err != nil {
		t.Fatalf("banco de teste indisponível: %v", err)
	}
	return db
}

// NewUser cria uma pessoa com e-mail único e a apaga no fim do teste.
func NewUser(t *testing.T, db *sql.DB, tag string) string {
	t.Helper()
	var id string
	// o sufixo aleatório vem do banco: pacotes de teste rodam em paralelo no mesmo banco
	if err := db.QueryRow(
		`INSERT INTO users (email, name, password_hash)
		 VALUES ($1 || '-' || gen_random_uuid() || '@teste.local', $2, 'x') RETURNING id`,
		tag, "Teste "+tag).Scan(&id); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Exec(`DELETE FROM users WHERE id = $1`, id) })
	return id
}

// NewHousehold cria um casal com os membros informados e o apaga no fim do teste.
func NewHousehold(t *testing.T, db *sql.DB, memberIDs ...string) string {
	t.Helper()
	var id string
	if err := db.QueryRow(`INSERT INTO households DEFAULT VALUES RETURNING id`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Exec(`DELETE FROM households WHERE id = $1`, id) })
	for _, m := range memberIDs {
		if _, err := db.Exec(
			`INSERT INTO household_members (household_id, user_id) VALUES ($1, $2)`, id, m); err != nil {
			t.Fatal(err)
		}
	}
	return id
}

// RunHook roda um gancho de mudança de casal (db.Hook) numa transação própria e a confirma,
// como o household faz ao sair, criar casal ou aceitar convite.
func RunHook(t *testing.T, conn *sql.DB, hook db.Hook, h db.Handoff) {
	t.Helper()
	ctx := context.Background()
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if err := hook(ctx, tx, h); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
}

// SystemScan roda uma consulta de uma linha no escopo de sistema (que enxerga todos os casais) e lê o resultado
// em dest. Serve para o teste conferir o que ficou no banco mesmo com o RLS ligado, onde um SELECT sem escopo
// não enxerga nada.
func SystemScan(t *testing.T, conn *sql.DB, query string, args []any, dest ...any) {
	t.Helper()
	ctx := context.Background()
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if err := db.SetScope(ctx, tx, db.Scope{System: true}); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRowContext(ctx, query, args...).Scan(dest...); err != nil {
		t.Fatalf("consulta de conferência falhou: %v\n%s", err, query)
	}
}

// SystemStrings devolve a primeira coluna de todas as linhas, no escopo de sistema (veja SystemScan).
func SystemStrings(t *testing.T, conn *sql.DB, query string, args ...any) []string {
	t.Helper()
	ctx := context.Background()
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if err := db.SetScope(ctx, tx, db.Scope{System: true}); err != nil {
		t.Fatal(err)
	}
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}
