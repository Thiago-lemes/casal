package household

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"casal/internal/auth"
	"casal/internal/testutil"
)

func TestOnlyOneInviteTakesTheLastSlot(t *testing.T) {
	db := testutil.OpenDB(t)
	ctx := context.Background()

	owner := testutil.NewUser(t, db, "dono")
	b := testutil.NewUser(t, db, "b")
	c := testutil.NewUser(t, db, "c")
	hh := testutil.NewHousehold(t, db, owner)
	repo := NewPostgresRepo(db)

	// dois convites válidos para a mesma vaga
	tokens := []string{"token-um", "token-dois"}
	for _, tk := range tokens {
		if err := repo.CreateInvite(ctx, hh, owner, auth.HashToken(tk), time.Now().Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
	}

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, who := range []string{b, c} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = repo.AcceptInvite(ctx, auth.HashToken(tokens[i]), who, time.Now())
		}()
	}
	wg.Wait()

	ok, full := 0, 0
	for _, err := range errs {
		switch {
		case err == nil:
			ok++
		case errors.Is(err, ErrHouseholdFull):
			full++
		default:
			t.Errorf("erro inesperado: %v", err)
		}
	}
	if ok != 1 || full != 1 {
		t.Fatalf("esperava 1 aceito e 1 recusado, veio %d e %d", ok, full)
	}

	members, err := repo.Members(ctx, hh)
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != MaxMembers {
		t.Errorf("o casal deveria ter %d pessoas, tem %d", MaxMembers, len(members))
	}
}
