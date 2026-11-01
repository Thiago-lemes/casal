package app

import (
	"context"
	"testing"
	"time"

	"casal/internal/auth"
	"casal/internal/household"
	"casal/internal/testutil"
)

// As limpezas apagam só o que venceu e deixam o resto intacto.
func TestPurgesRemoveOnlyStaleRows(t *testing.T) {
	conn := testutil.OpenDB(t)
	ctx := context.Background()

	user := testutil.NewUser(t, conn, "purge")
	hh := testutil.NewHousehold(t, conn, user)

	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := conn.ExecContext(ctx, q, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`INSERT INTO sessions (user_id, token_hash, expires_at) VALUES ($1, 'p-velha', now() - interval '1 hour')`, user)
	exec(`INSERT INTO sessions (user_id, token_hash, expires_at) VALUES ($1, 'p-viva', now() + interval '1 hour')`, user)
	exec(`INSERT INTO household_invites (household_id, token_hash, created_by, expires_at) VALUES ($1, 'i-velho', $2, now() - interval '40 days')`, hh, user)
	exec(`INSERT INTO household_invites (household_id, token_hash, created_by, expires_at) VALUES ($1, 'i-recente', $2, now() - interval '1 day')`, hh, user)
	exec(`INSERT INTO household_invites (household_id, token_hash, created_by, expires_at) VALUES ($1, 'i-valido', $2, now() + interval '3 days')`, hh, user)

	authSvc, err := auth.NewService(auth.NewPostgresRepo(conn), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := authSvc.PurgeExpired(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := household.NewService(household.NewPostgresRepo(conn)).PurgeStaleInvites(ctx); err != nil {
		t.Fatal(err)
	}

	exists := func(table, hash string) bool {
		var n int
		if err := conn.QueryRowContext(ctx, `SELECT count(*) FROM `+table+` WHERE token_hash = $1`, hash).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n > 0
	}
	for table, want := range map[string]map[string]bool{
		"sessions":          {"p-velha": false, "p-viva": true},
		"household_invites": {"i-velho": false, "i-recente": true, "i-valido": true},
	} {
		for hash, keep := range want {
			if exists(table, hash) != keep {
				t.Errorf("%s/%s: esperava presente=%v", table, hash, keep)
			}
		}
	}
}
