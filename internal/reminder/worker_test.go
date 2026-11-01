package reminder

import (
	"context"
	"database/sql"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	appdb "casal/internal/db"
	"casal/internal/testutil"
)

type countingNotifier struct {
	userID string
	calls  atomic.Int32
}

func (c *countingNotifier) Notify(_ context.Context, userID, _, _ string) error {
	if userID == c.userID {
		c.calls.Add(1)
	}
	return nil
}

// Dois agendadores ao mesmo tempo: o lembrete vencido é avisado uma única vez.
func TestDueReminderIsSentOnce(t *testing.T) {
	db := testutil.OpenDB(t)
	ctx := context.Background()

	user := testutil.NewUser(t, db, "worker")
	hh := testutil.NewHousehold(t, db, user)
	repo := NewPostgresRepo(db)
	created, err := repo.Create(ctx, Reminder{
		HouseholdID: hh, CreatedBy: user, RecipientID: user,
		Body: "vencido", RemindAt: time.Now().Add(-time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}

	n := &countingNotifier{userID: user}
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			NewWorker(repo, n, slog.New(slog.DiscardHandler)).tick(ctx)
		}()
	}
	wg.Wait()

	var status string
	err = appdb.InTx(ctx, db, appdb.Scope{System: true}, func(tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `SELECT status FROM reminders WHERE id = $1`, created.ID).Scan(&status)
	})
	if err != nil {
		t.Fatal(err)
	}
	if status != "sent" {
		t.Errorf("o lembrete deveria estar como sent, está %q", status)
	}
	if got := n.calls.Load(); got != 1 {
		t.Errorf("esperava 1 aviso, houve %d", got)
	}
}
