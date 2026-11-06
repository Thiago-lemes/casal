package auth

import (
	"context"
	"testing"
	"time"

	"casal/internal/testutil"
)

// A atividade é registrada no máximo uma vez por hora por sessão, e o dia de atividade vira uma linha por pessoa.
func TestUltimoUsoDaSessaoUmaVezPorHora(t *testing.T) {
	conn := testutil.OpenDB(t)
	repo := NewPostgresRepo(conn)
	ctx := context.Background()
	userID := testutil.NewUser(t, conn, "atividade")
	hash := "hash-atividade-" + userID
	if err := repo.CreateSession(ctx, userID, hash, time.Now().Add(24*time.Hour)); err != nil {
		t.Fatal(err)
	}

	lastUsed := func() (time.Time, int) {
		var at time.Time
		var days int
		if err := conn.QueryRow(`SELECT coalesce(max(last_used_at), 'epoch') FROM sessions WHERE token_hash = $1`, hash).Scan(&at); err != nil {
			t.Fatal(err)
		}
		if err := conn.QueryRow(`SELECT count(*) FROM user_activity WHERE user_id = $1`, userID).Scan(&days); err != nil {
			t.Fatal(err)
		}
		return at, days
	}

	t0 := time.Now().UTC().Truncate(time.Second)
	if _, err := repo.UserBySession(ctx, hash, t0); err != nil {
		t.Fatal(err)
	}
	first, days := lastUsed()
	if !first.Equal(t0) || days != 1 {
		t.Fatalf("primeiro uso: last_used_at=%v (esperado %v), dias=%d (esperado 1)", first, t0, days)
	}

	// 30 minutos depois: nada muda (não escreve a cada requisição)
	if _, err := repo.UserBySession(ctx, hash, t0.Add(30*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if again, _ := lastUsed(); !again.Equal(first) {
		t.Errorf("dentro da mesma hora o last_used_at não pode mudar: %v -> %v", first, again)
	}

	// 2 horas depois: atualiza
	later := t0.Add(2 * time.Hour)
	if _, err := repo.UserBySession(ctx, hash, later); err != nil {
		t.Fatal(err)
	}
	if got, _ := lastUsed(); !got.Equal(later) {
		t.Errorf("após 2h: last_used_at=%v (esperado %v)", got, later)
	}

	// sessão vencida não entra e não registra atividade
	expired := "hash-vencida-" + userID
	if err := repo.CreateSession(ctx, userID, expired, time.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.UserBySession(ctx, expired, time.Now()); err == nil {
		t.Error("sessão vencida não deveria autenticar")
	}
}
