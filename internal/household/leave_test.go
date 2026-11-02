package household

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"casal/internal/db"
	"casal/internal/testutil"
)

// recorder guarda os Handoff que os ganchos receberam.
type recorder struct{ got []db.Handoff }

func (r *recorder) hook(_ context.Context, _ *sql.Tx, h db.Handoff) error {
	r.got = append(r.got, h)
	return nil
}

func memberCount(t *testing.T, conn *sql.DB, hh string) int {
	t.Helper()
	var n int
	if err := conn.QueryRow(`SELECT count(*) FROM household_members WHERE household_id = $1`, hh).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func inviteCount(t *testing.T, conn *sql.DB, hh string) int {
	t.Helper()
	var n int
	if err := conn.QueryRow(`SELECT count(*) FROM household_invites WHERE household_id = $1`, hh).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestLeaveRemovesMemberAndTellsTheHooks(t *testing.T) {
	conn := testutil.OpenDB(t)
	ctx := context.Background()
	ana := testutil.NewUser(t, conn, "ana")
	beto := testutil.NewUser(t, conn, "beto")
	hh := testutil.NewHousehold(t, conn, ana, beto)

	svc := NewService(NewPostgresRepo(conn))
	rec := &recorder{}
	svc.OnLeave(rec.hook)

	if err := svc.Leave(ctx, ana); err != nil {
		t.Fatal(err)
	}
	if memberCount(t, conn, hh) != 1 {
		t.Error("a Ana deveria ter saído e o Beto ficado")
	}
	if _, err := svc.HouseholdOf(ctx, ana); !errors.Is(err, ErrNoHousehold) {
		t.Errorf("depois de sair, a Ana não tem casal: %v", err)
	}
	if id, err := svc.HouseholdOf(ctx, beto); err != nil || id != hh {
		t.Errorf("o Beto continua no casal: %v %v", id, err)
	}
	want := db.Handoff{UserID: ana, From: hh, RemainingUserID: beto}
	if len(rec.got) != 1 || rec.got[0] != want {
		t.Errorf("o gancho deveria receber %+v, recebeu %+v", want, rec.got)
	}
}

func TestLeaveWithoutHouseholdFails(t *testing.T) {
	conn := testutil.OpenDB(t)
	ana := testutil.NewUser(t, conn, "ana")
	svc := NewService(NewPostgresRepo(conn))
	if err := svc.Leave(context.Background(), ana); !errors.Is(err, ErrNoHousehold) {
		t.Errorf("sem casal, esperava ErrNoHousehold: %v", err)
	}
}

func TestLeaveAloneEmptiesTheHouseholdAndRevokesItsInvites(t *testing.T) {
	conn := testutil.OpenDB(t)
	ctx := context.Background()
	ana := testutil.NewUser(t, conn, "ana")
	hh := testutil.NewHousehold(t, conn, ana)
	svc := NewService(NewPostgresRepo(conn))
	rec := &recorder{}
	svc.OnLeave(rec.hook)

	if _, _, err := svc.Invite(ctx, hh, ana); err != nil {
		t.Fatal(err)
	}
	if err := svc.Leave(ctx, ana); err != nil {
		t.Fatal(err)
	}
	if memberCount(t, conn, hh) != 0 {
		t.Error("o casal deveria ficar vazio")
	}
	if inviteCount(t, conn, hh) != 0 {
		t.Error("casal vazio não pode manter convite aberto: alguém entraria num casal sem dono")
	}
	if len(rec.got) != 1 || rec.got[0].RemainingUserID != "" {
		t.Errorf("sem ninguém que fique, RemainingUserID é vazio: %+v", rec.got)
	}
}

func TestLeaveRevokesOnlyTheLeaversPendingInvites(t *testing.T) {
	conn := testutil.OpenDB(t)
	ctx := context.Background()
	ana := testutil.NewUser(t, conn, "ana")
	beto := testutil.NewUser(t, conn, "beto")
	hh := testutil.NewHousehold(t, conn, ana, beto)
	svc := NewService(NewPostgresRepo(conn))

	// casal cheio não gera convite pela API; insere direto, como se fossem de antes
	insert := func(by string, hash string) {
		t.Helper()
		if _, err := conn.Exec(`INSERT INTO household_invites (household_id, token_hash, created_by, expires_at)
			VALUES ($1, $2, $3, now() + interval '1 day')`, hh, hash+"-"+ana, by); err != nil {
			t.Fatal(err)
		}
	}
	insert(ana, "da-ana")
	insert(beto, "do-beto")

	if err := svc.Leave(ctx, ana); err != nil {
		t.Fatal(err)
	}
	var left []string
	rows, err := conn.Query(`SELECT created_by::text FROM household_invites WHERE household_id = $1`, hh)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var by string
		if err := rows.Scan(&by); err != nil {
			t.Fatal(err)
		}
		left = append(left, by)
	}
	if len(left) != 1 || left[0] != beto {
		t.Errorf("só o convite do Beto deveria sobrar: %v", left)
	}
}

func TestFailingHookRollsBackTheWholeLeave(t *testing.T) {
	conn := testutil.OpenDB(t)
	ctx := context.Background()
	ana := testutil.NewUser(t, conn, "ana")
	beto := testutil.NewUser(t, conn, "beto")
	hh := testutil.NewHousehold(t, conn, ana, beto)
	svc := NewService(NewPostgresRepo(conn))
	boom := errors.New("gancho quebrou")
	svc.OnLeave(func(context.Context, *sql.Tx, db.Handoff) error { return boom })

	if err := svc.Leave(ctx, ana); !errors.Is(err, boom) {
		t.Fatalf("o erro do gancho deveria voltar: %v", err)
	}
	if memberCount(t, conn, hh) != 2 {
		t.Error("com o gancho falhando, a saída inteira deveria ser desfeita")
	}
}

func TestJoinHooksRunOnCreateAndAccept(t *testing.T) {
	conn := testutil.OpenDB(t)
	ctx := context.Background()
	ana := testutil.NewUser(t, conn, "ana")
	beto := testutil.NewUser(t, conn, "beto")
	svc := NewService(NewPostgresRepo(conn))
	rec := &recorder{}
	svc.OnJoin(rec.hook)

	hh, err := svc.Create(ctx, ana)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Exec(`DELETE FROM households WHERE id = $1`, hh) })
	token, _, err := svc.Invite(ctx, hh, ana)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Accept(ctx, beto, token); err != nil {
		t.Fatal(err)
	}
	want := []db.Handoff{{UserID: ana, To: hh}, {UserID: beto, To: hh}}
	if len(rec.got) != 2 || rec.got[0] != want[0] || rec.got[1] != want[1] {
		t.Errorf("os ganchos de entrada deveriam receber %+v, receberam %+v", want, rec.got)
	}
}

func TestFailingJoinHookUndoesTheMembership(t *testing.T) {
	conn := testutil.OpenDB(t)
	ctx := context.Background()
	ana := testutil.NewUser(t, conn, "ana")
	svc := NewService(NewPostgresRepo(conn))
	svc.OnJoin(func(context.Context, *sql.Tx, db.Handoff) error { return errors.New("gancho quebrou") })

	if _, err := svc.Create(ctx, ana); err == nil {
		t.Fatal("deveria falhar")
	}
	if _, err := svc.HouseholdOf(ctx, ana); !errors.Is(err, ErrNoHousehold) {
		t.Errorf("a pessoa não pode ficar com um casal criado pela metade: %v", err)
	}
}

// Quem saiu pode aceitar outro convite (e quem criou um casal sozinho, sair e entrar no do parceiro).
func TestAcceptInviteAfterLeaving(t *testing.T) {
	conn := testutil.OpenDB(t)
	ctx := context.Background()
	ana := testutil.NewUser(t, conn, "ana")
	beto := testutil.NewUser(t, conn, "beto")
	hhBeto := testutil.NewHousehold(t, conn, beto)
	svc := NewService(NewPostgresRepo(conn))

	hhAna, err := svc.Create(ctx, ana)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Exec(`DELETE FROM households WHERE id = $1`, hhAna) })
	token, _, err := svc.Invite(ctx, hhBeto, beto)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Accept(ctx, ana, token); !errors.Is(err, ErrAlreadyInHousehold) {
		t.Fatalf("quem já tem casal não aceita convite: %v", err)
	}
	if err := svc.Leave(ctx, ana); err != nil {
		t.Fatal(err)
	}
	if id, err := svc.Accept(ctx, ana, token); err != nil || id != hhBeto {
		t.Errorf("depois de sair, a Ana deveria aceitar o convite do Beto: %v %v", id, err)
	}
}
