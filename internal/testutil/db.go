package testutil

import (
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

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
	email := fmt.Sprintf("%s-%d@teste.local", tag, time.Now().UnixNano())
	if err := db.QueryRow(
		`INSERT INTO users (email, name, password_hash) VALUES ($1, $2, 'x') RETURNING id`,
		email, "Teste "+tag).Scan(&id); err != nil {
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
