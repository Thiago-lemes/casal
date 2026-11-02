package task

import (
	"context"
	"errors"
	"testing"
	"time"

	"casal/internal/testutil"
)

func TestHouseholdsAreIsolated(t *testing.T) {
	db := testutil.OpenDB(t)
	ctx := context.Background()

	userA := testutil.NewUser(t, db, "a")
	userB := testutil.NewUser(t, db, "b")
	hhA := testutil.NewHousehold(t, db, userA)
	hhB := testutil.NewHousehold(t, db, userB)
	repo := NewPostgresRepo(db)

	created, err := repo.Create(ctx, Task{HouseholdID: hhA, CreatedBy: userA, AssigneeID: &userA, Title: "segredo do casal A"})
	if err != nil {
		t.Fatal(err)
	}

	// tarefa do casal (sem responsável): qualquer membro conclui, então só o casal isola
	shared, err := repo.Create(ctx, Task{HouseholdID: hhA, CreatedBy: userA, Title: "tarefa do casal A"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := repo.Complete(ctx, hhB, shared.ID, userB, time.Now(), nil); !errors.Is(err, ErrNotFound) {
		t.Errorf("B concluiu uma tarefa do casal A: %v", err)
	}

	list, err := repo.List(ctx, hhB, StatusOpen)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Errorf("o casal B não deveria ver tarefas do A, viu %d", len(list))
	}
	if _, _, err := repo.Complete(ctx, hhB, created.ID, userB, time.Now(), nil); !errors.Is(err, ErrNotFound) {
		t.Errorf("B concluiu uma tarefa de A: %v", err)
	}
	if _, err := repo.Reassign(ctx, hhB, created.ID, userB, &userB); !errors.Is(err, ErrNotFound) {
		t.Errorf("B reatribuiu uma tarefa de A: %v", err)
	}
	if err := repo.Cancel(ctx, hhB, created.ID, userB); !errors.Is(err, ErrNotFound) {
		t.Errorf("B cancelou uma tarefa de A: %v", err)
	}
}

// Dentro do mesmo casal, cada ação tem seu dono: só quem pode consegue.
func TestPermissionsInsideHousehold(t *testing.T) {
	db := testutil.OpenDB(t)
	ctx := context.Background()

	ana := testutil.NewUser(t, db, "ana")
	beto := testutil.NewUser(t, db, "beto")
	hh := testutil.NewHousehold(t, db, ana, beto)
	repo := NewPostgresRepo(db)

	create := func(assignee *string) Task {
		t.Helper()
		got, err := repo.Create(ctx, Task{HouseholdID: hh, CreatedBy: ana, AssigneeID: assignee, Title: "tarefa"})
		if err != nil {
			t.Fatal(err)
		}
		return got
	}

	// tarefa da Ana para ela mesma: o Beto não mexe
	own := create(&ana)
	if _, _, err := repo.Complete(ctx, hh, own.ID, beto, time.Now(), nil); !errors.Is(err, ErrNotFound) {
		t.Errorf("Beto concluiu a tarefa da Ana: %v", err)
	}
	if _, err := repo.Decline(ctx, hh, own.ID, beto, "não"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Beto recusou a tarefa da Ana: %v", err)
	}
	if _, err := repo.Decline(ctx, hh, own.ID, ana, "não"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Ana recusou a própria tarefa: %v", err)
	}

	// tarefa da Ana para o Beto: ele recusa; só ela reatribui ou cancela
	forBeto := create(&beto)
	if _, err := repo.Reassign(ctx, hh, forBeto.ID, beto, &beto); !errors.Is(err, ErrNotFound) {
		t.Errorf("o responsável reatribuiu sem ser o criador: %v", err)
	}
	if err := repo.Cancel(ctx, hh, forBeto.ID, beto); !errors.Is(err, ErrNotFound) {
		t.Errorf("o responsável cancelou sem ser o criador: %v", err)
	}
	declined, err := repo.Decline(ctx, hh, forBeto.ID, beto, "estou viajando")
	if err != nil {
		t.Fatalf("Beto deveria conseguir recusar: %v", err)
	}
	if declined.Status != StatusDeclined || declined.DeclineReason == nil || *declined.DeclineReason != "estou viajando" {
		t.Errorf("recusa mal gravada: %+v", declined)
	}
	if _, _, err := repo.Complete(ctx, hh, forBeto.ID, beto, time.Now(), nil); !errors.Is(err, ErrNotFound) {
		t.Errorf("tarefa recusada não pode ser concluída: %v", err)
	}

	// a Ana assume: a tarefa reabre e a justificativa some
	reopened, err := repo.Reassign(ctx, hh, forBeto.ID, ana, &ana)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.Status != StatusOpen || reopened.DeclineReason != nil || *reopened.AssigneeID != ana {
		t.Errorf("reatribuição mal feita: %+v", reopened)
	}

	// tarefa do casal: qualquer um conclui
	shared := create(nil)
	done, _, err := repo.Complete(ctx, hh, shared.ID, beto, time.Now(), nil)
	if err != nil {
		t.Fatalf("qualquer um deveria concluir a tarefa do casal: %v", err)
	}
	if done.Status != StatusDone || done.CompletedBy == nil || *done.CompletedBy != beto {
		t.Errorf("conclusão mal gravada: %+v", done)
	}

	// o criador confirma o que foi atribuído ao parceiro
	confirm := create(&beto)
	if _, _, err := repo.Complete(ctx, hh, confirm.ID, ana, time.Now(), nil); err != nil {
		t.Errorf("o criador deveria poder concluir: %v", err)
	}
}

// Com o papel casal_app (RLS ligado), o banco barra o casal errado mesmo sem o WHERE household_id.
func TestRLSBlocksOtherHouseholdWithoutWhere(t *testing.T) {
	db := testutil.OpenDB(t)
	ctx := context.Background()

	var rls bool
	if err := db.QueryRow(`SELECT NOT rolbypassrls AND NOT rolsuper FROM pg_roles WHERE rolname = current_user`).Scan(&rls); err != nil || !rls {
		t.Skip("TEST_DATABASE_URL precisa usar o papel casal_app para testar o RLS")
	}

	userA := testutil.NewUser(t, db, "rls-a")
	hhA := testutil.NewHousehold(t, db, userA)
	hhB := testutil.NewHousehold(t, db)
	repo := NewPostgresRepo(db)
	if _, err := repo.Create(ctx, Task{HouseholdID: hhA, CreatedBy: userA, AssigneeID: &userA, Title: "do A"}); err != nil {
		t.Fatal(err)
	}

	// escopo do casal B, consulta SEM filtro de casal: não pode enxergar a linha do A
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.household_id', $1, true)`, hhB); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM tasks WHERE created_by = $1`, userA).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("o RLS deveria esconder a tarefa do casal A, mas %d linha(s) apareceram", n)
	}
}

// A próxima ocorrência nasce na mesma transação da conclusão, e só uma vez.
func TestCompleteCreatesNextOccurrenceOnce(t *testing.T) {
	db := testutil.OpenDB(t)
	ctx := context.Background()

	ana := testutil.NewUser(t, db, "ana")
	beto := testutil.NewUser(t, db, "beto")
	hh := testutil.NewHousehold(t, db, ana, beto)
	repo := NewPostgresRepo(db)

	due := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	rec, err := repo.Create(ctx, Task{
		HouseholdID: hh, CreatedBy: ana, AssigneeID: &beto, Title: "tirar o lixo",
		DueAt: &due, Repeat: RepeatWeekly, Rotation: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if rec.Repeat != RepeatWeekly || !rec.Rotation {
		t.Fatalf("repetição não voltou do banco: %+v", rec)
	}

	nextDue := due.AddDate(0, 0, 7)
	next := &Task{CreatedBy: ana, AssigneeID: &ana, Title: rec.Title, DueAt: &nextDue, Repeat: RepeatWeekly, Rotation: true}
	done, created, err := repo.Complete(ctx, hh, rec.ID, beto, time.Now(), next)
	if err != nil {
		t.Fatal(err)
	}
	if done.Status != StatusDone || created == nil || created.Status != StatusOpen || created.ID == done.ID ||
		*created.AssigneeID != ana || !created.DueAt.Equal(nextDue) || !created.Rotation {
		t.Fatalf("conclusão ou próxima mal feitas: %+v %+v", done, created)
	}

	// concluir de novo falha e não cria outra próxima
	if _, c2, err := repo.Complete(ctx, hh, rec.ID, beto, time.Now(), next); !errors.Is(err, ErrNotFound) || c2 != nil {
		t.Errorf("concluir duas vezes deveria dar ErrNotFound sem criar nada: %v %v", c2, err)
	}
	open, err := repo.List(ctx, hh, StatusOpen)
	if err != nil || len(open) != 1 || open[0].ID != created.ID {
		t.Errorf("deveria haver só a próxima ocorrência aberta: %+v %v", open, err)
	}

	// quem não pode concluir não gera a próxima
	if _, c3, err := repo.Complete(ctx, hh, created.ID, beto, time.Now(), next); !errors.Is(err, ErrNotFound) || c3 != nil {
		t.Errorf("o Beto não é o responsável nem o criador: %v %v", c3, err)
	}
	open, _ = repo.List(ctx, hh, StatusOpen)
	if len(open) != 1 {
		t.Errorf("uma conclusão negada não pode criar tarefas: %d abertas", len(open))
	}
}

// Recusar e cancelar não fazem a tarefa reaparecer; reatribuir para o casal desliga o rodízio.
func TestDeclineCancelAndShareDoNotRepeat(t *testing.T) {
	db := testutil.OpenDB(t)
	ctx := context.Background()

	ana := testutil.NewUser(t, db, "ana")
	beto := testutil.NewUser(t, db, "beto")
	hh := testutil.NewHousehold(t, db, ana, beto)
	repo := NewPostgresRepo(db)

	mk := func() Task {
		t.Helper()
		got, err := repo.Create(ctx, Task{HouseholdID: hh, CreatedBy: ana, AssigneeID: &beto, Title: "lixo", Repeat: RepeatDaily, Rotation: true})
		if err != nil {
			t.Fatal(err)
		}
		return got
	}

	declined := mk()
	if _, err := repo.Decline(ctx, hh, declined.ID, beto, "viajando"); err != nil {
		t.Fatal(err)
	}
	canceled := mk()
	if err := repo.Cancel(ctx, hh, canceled.ID, ana); err != nil {
		t.Fatal(err)
	}
	open, _ := repo.List(ctx, hh, StatusOpen)
	if len(open) != 0 {
		t.Errorf("recusar e cancelar não criam a próxima: %+v", open)
	}

	// passar para o casal: sem responsável não há rodízio (o banco recusaria a combinação)
	shared, err := repo.Reassign(ctx, hh, declined.ID, ana, nil)
	if err != nil {
		t.Fatalf("reatribuir uma tarefa com rodízio ao casal não pode quebrar: %v", err)
	}
	if shared.AssigneeID != nil || shared.Rotation || shared.Repeat != RepeatDaily {
		t.Errorf("deveria virar do casal, sem rodízio, mas ainda repetir: %+v", shared)
	}
	// passar para outra pessoa mantém o rodízio
	other := mk()
	kept, err := repo.Reassign(ctx, hh, other.ID, ana, &ana)
	if err != nil || !kept.Rotation || *kept.AssigneeID != ana {
		t.Errorf("com responsável, o rodízio continua: %+v %v", kept, err)
	}
}
