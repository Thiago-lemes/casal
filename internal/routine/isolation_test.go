package routine

import (
	"context"
	"errors"
	"testing"
	"time"

	"casal/internal/testutil"
)

func newBlock(owner, hh, title, visibility string) Block {
	return Block{
		HouseholdID: hh, OwnerID: owner, Title: title, Category: "work", Visibility: visibility,
		Recurrence: RecurrenceWeekly, Weekdays: 0b0011111, IntervalWeeks: 1,
		StartDate: day("2026-11-02"), StartMin: 9 * 60, EndMin: 18 * 60,
	}
}

func titles(bs []Block) []string {
	out := []string{}
	for _, b := range bs {
		out = append(out, b.Title)
	}
	return out
}

func TestHouseholdsAreIsolated(t *testing.T) {
	db := testutil.OpenDB(t)
	ctx := context.Background()

	userA := testutil.NewUser(t, db, "a")
	userB := testutil.NewUser(t, db, "b")
	hhA := testutil.NewHousehold(t, db, userA)
	hhB := testutil.NewHousehold(t, db, userB)
	repo := NewPostgresRepo(db)

	// bloco do casal: o mais exposto, pois qualquer membro do casal A o vê e edita
	created, err := repo.Create(ctx, newBlock(userA, hhA, "bloco do casal A", VisibilityHousehold))
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.SetException(ctx, hhA, userA, Exception{BlockID: created.ID, Date: day("2026-11-03"), Skipped: true}); err != nil {
		t.Fatal(err)
	}

	list, err := repo.ListVisible(ctx, hhB, userB)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Errorf("o casal B não deveria ver blocos do A, viu %v", titles(list))
	}
	exc, err := repo.ListExceptions(ctx, hhB, userB, day("2026-11-01"), day("2026-11-30"))
	if err != nil || len(exc) != 0 {
		t.Errorf("o casal B não deveria ver exceções do A: %v %v", exc, err)
	}
	if _, err := repo.Get(ctx, hhB, created.ID, userB); !errors.Is(err, ErrNotFound) {
		t.Errorf("B leu um bloco de A: %v", err)
	}
	if _, err := repo.Update(ctx, hhB, userB, created, created.Version); !errors.Is(err, ErrNotFound) {
		t.Errorf("B editou um bloco do casal A: %v", err)
	}
	if err := repo.SetException(ctx, hhB, userB, Exception{BlockID: created.ID, Date: day("2026-11-04"), Skipped: true}); !errors.Is(err, ErrNotFound) {
		t.Errorf("B abriu exceção num bloco do casal A: %v", err)
	}
	if err := repo.DeleteException(ctx, hhB, userB, created.ID, day("2026-11-03")); !errors.Is(err, ErrNotFound) {
		t.Errorf("B apagou uma exceção do casal A: %v", err)
	}
	if err := repo.Delete(ctx, hhB, created.ID, userB); !errors.Is(err, ErrNotFound) {
		t.Errorf("B apagou um bloco de A: %v", err)
	}
}

// Dentro do mesmo casal, a visibilidade decide o que cada pessoa vê e altera.
// Esta é a trava que só existe no código (o RLS isola casais, não parceiros).
func TestVisibilityInsideHousehold(t *testing.T) {
	db := testutil.OpenDB(t)
	ctx := context.Background()

	ana := testutil.NewUser(t, db, "ana")
	beto := testutil.NewUser(t, db, "beto")
	hh := testutil.NewHousehold(t, db, ana, beto)
	repo := NewPostgresRepo(db)

	mk := func(title, visibility string) Block {
		t.Helper()
		b, err := repo.Create(ctx, newBlock(ana, hh, title, visibility))
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	private := mk("privado", VisibilityPrivate)
	busy := mk("ocupado", VisibilityBusy)
	view := mk("detalhes", VisibilityView)
	shared := mk("do casal", VisibilityHousehold)
	for _, b := range []Block{private, busy, view, shared} {
		if err := repo.SetException(ctx, hh, ana, Exception{BlockID: b.ID, Date: day("2026-11-03"), Skipped: true}); err != nil {
			t.Fatal(err)
		}
	}

	// leitura: o Beto não vê o privado
	if _, err := repo.Get(ctx, hh, private.ID, beto); !errors.Is(err, ErrNotFound) {
		t.Errorf("Beto leu o bloco privado da Ana: %v", err)
	}
	list, err := repo.ListVisible(ctx, hh, beto)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 {
		t.Errorf("Beto deveria ver 3 blocos (busy, view e household), viu %v", titles(list))
	}
	for _, b := range list {
		if b.ID == private.ID {
			t.Error("o bloco privado apareceu na lista do Beto")
		}
	}
	all, err := repo.ListVisible(ctx, hh, ana)
	if err != nil || len(all) != 4 {
		t.Errorf("Ana deveria ver os 4: %v %v", titles(all), err)
	}
	exc, err := repo.ListExceptions(ctx, hh, beto, day("2026-11-01"), day("2026-11-30"))
	if err != nil || len(exc) != 3 {
		t.Errorf("Beto deveria ver as exceções de 3 blocos, não a do privado: %d %v", len(exc), err)
	}
	for _, e := range exc {
		if e.BlockID == private.ID {
			t.Error("a exceção do bloco privado vazou para o Beto")
		}
	}

	// escrita: só o dono edita o que não é do casal
	for _, b := range []Block{private, busy, view} {
		if _, err := repo.Update(ctx, hh, beto, b, b.Version); !errors.Is(err, ErrNotFound) {
			t.Errorf("Beto editou %q: %v", b.Title, err)
		}
		if err := repo.SetException(ctx, hh, beto, Exception{BlockID: b.ID, Date: day("2026-11-04"), Skipped: true}); !errors.Is(err, ErrNotFound) {
			t.Errorf("Beto abriu exceção em %q: %v", b.Title, err)
		}
		if err := repo.DeleteException(ctx, hh, beto, b.ID, day("2026-11-03")); !errors.Is(err, ErrNotFound) {
			t.Errorf("Beto apagou a exceção de %q: %v", b.Title, err)
		}
	}
	edited := shared
	edited.Title = "o Beto mexeu"
	got, err := repo.Update(ctx, hh, beto, edited, shared.Version)
	if err != nil {
		t.Fatalf("Beto deveria editar o bloco do casal: %v", err)
	}
	if got.Title != "o Beto mexeu" || got.Version != shared.Version+1 {
		t.Errorf("edição mal gravada: %+v", got)
	}
	if err := repo.SetException(ctx, hh, beto, Exception{BlockID: shared.ID, Date: day("2026-11-04"), Skipped: true}); err != nil {
		t.Errorf("Beto deveria abrir exceção no bloco do casal: %v", err)
	}

	// o Beto não consegue esconder o bloco do casal, mesmo mandando outra visibilidade
	edited = got
	edited.Visibility = VisibilityPrivate
	got, err = repo.Update(ctx, hh, beto, edited, got.Version)
	if err != nil {
		t.Fatal(err)
	}
	if got.Visibility != VisibilityHousehold {
		t.Errorf("o parceiro mudou a visibilidade para %q", got.Visibility)
	}

	// só o dono apaga
	for _, b := range []Block{busy, view, shared} {
		if err := repo.Delete(ctx, hh, b.ID, beto); !errors.Is(err, ErrNotFound) {
			t.Errorf("Beto apagou %q: %v", b.Title, err)
		}
	}
	if err := repo.Delete(ctx, hh, private.ID, ana); err != nil {
		t.Errorf("a Ana deveria apagar o próprio bloco: %v", err)
	}
}

func TestUpdateWithStaleVersionWritesNothing(t *testing.T) {
	db := testutil.OpenDB(t)
	ctx := context.Background()

	ana := testutil.NewUser(t, db, "ana")
	hh := testutil.NewHousehold(t, db, ana)
	repo := NewPostgresRepo(db)

	b, err := repo.Create(ctx, newBlock(ana, hh, "original", VisibilityPrivate))
	if err != nil {
		t.Fatal(err)
	}
	b.Title = "primeira edição"
	if _, err := repo.Update(ctx, hh, ana, b, b.Version); err != nil {
		t.Fatal(err)
	}
	b.Title = "edição com versão velha"
	if _, err := repo.Update(ctx, hh, ana, b, b.Version); !errors.Is(err, ErrNotFound) {
		t.Errorf("versão velha deveria falhar: %v", err)
	}
	cur, err := repo.Get(ctx, hh, b.ID, ana)
	if err != nil {
		t.Fatal(err)
	}
	if cur.Title != "primeira edição" || cur.Version != 2 {
		t.Errorf("a edição velha alterou o bloco: %+v", cur)
	}
}

// O que vai e volta do banco: máscara de dias, datas, minutos e o fuso do dono.
func TestBlockRoundTripAndExceptionUpsert(t *testing.T) {
	db := testutil.OpenDB(t)
	ctx := context.Background()

	ana := testutil.NewUser(t, db, "ana")
	hh := testutil.NewHousehold(t, db, ana)
	if _, err := db.Exec(`UPDATE users SET timezone = 'America/Manaus', name = 'Ana' WHERE id = $1`, ana); err != nil {
		t.Fatal(err)
	}
	repo := NewPostgresRepo(db)

	end := day("2026-12-18")
	in := newBlock(ana, hh, "Plantão", VisibilityView)
	in.Weekdays, in.IntervalWeeks, in.EndDate, in.Location, in.EndMin = 0b1000101, 2, &end, "hospital", 1440
	created, err := repo.Create(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	got, err := repo.Get(ctx, hh, created.ID, ana)
	if err != nil {
		t.Fatal(err)
	}
	if got.Weekdays != 0b1000101 || got.IntervalWeeks != 2 || got.EndMin != 1440 || got.Location != "hospital" ||
		!got.StartDate.Equal(day("2026-11-02")) || got.EndDate == nil || !got.EndDate.Equal(end) ||
		got.OwnerTZ != "America/Manaus" || got.OwnerName != "Ana" {
		t.Errorf("o bloco não voltou como entrou: %+v", got)
	}

	// a mesma data duas vezes troca a exceção, não duplica
	d := day("2026-11-04")
	if err := repo.SetException(ctx, hh, ana, Exception{BlockID: created.ID, Date: d, Skipped: true}); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetException(ctx, hh, ana, Exception{BlockID: created.ID, Date: d, StartMin: 600, EndMin: 700}); err != nil {
		t.Fatal(err)
	}
	exc, err := repo.ListExceptions(ctx, hh, ana, day("2026-11-01"), day("2026-11-30"))
	if err != nil || len(exc) != 1 || exc[0].Skipped || exc[0].StartMin != 600 || exc[0].EndMin != 700 || !exc[0].Date.Equal(d) {
		t.Errorf("a exceção deveria ter sido trocada: %+v %v", exc, err)
	}
	// fora do período pedido, não vem
	if exc, _ := repo.ListExceptions(ctx, hh, ana, day("2026-12-01"), day("2026-12-31")); len(exc) != 0 {
		t.Errorf("exceção fora do período: %+v", exc)
	}
	if err := repo.DeleteException(ctx, hh, ana, created.ID, d); err != nil {
		t.Fatal(err)
	}
	if err := repo.DeleteException(ctx, hh, ana, created.ID, d); !errors.Is(err, ErrNotFound) {
		t.Errorf("apagar de novo deveria dar ErrNotFound: %v", err)
	}

	// apagar o bloco leva as exceções junto
	if err := repo.SetException(ctx, hh, ana, Exception{BlockID: created.ID, Date: d, Skipped: true}); err != nil {
		t.Fatal(err)
	}
	if err := repo.Delete(ctx, hh, created.ID, ana); err != nil {
		t.Fatal(err)
	}
	var n int
	testutil.SystemScan(t, db, `SELECT count(*) FROM routine_exceptions WHERE block_id = $1`, []any{created.ID}, &n)
	if n != 0 {
		t.Errorf("as exceções deveriam sumir com o bloco: %d", n)
	}
}

// O caminho todo, do banco ao horário livre: a visibilidade vale também para os cálculos.
func TestServiceEndToEndWithRealRepo(t *testing.T) {
	db := testutil.OpenDB(t)
	ctx := context.Background()

	ana := testutil.NewUser(t, db, "ana")
	beto := testutil.NewUser(t, db, "beto")
	hh := testutil.NewHousehold(t, db, ana, beto)
	svc := NewService(NewPostgresRepo(db), fakePeople{})
	svc.now = func() time.Time { return fixedNow }

	in := valid() // seg a sex, 9h às 18h
	in.Visibility = VisibilityPrivate
	saved, err := svc.Create(ctx, hh, ana, in)
	if err != nil {
		t.Fatal(err)
	}
	free := FreeInput{Date: "2026-11-02", Timezone: "America/Sao_Paulo"}

	// privado: o Beto tem o dia livre; a Ana, não
	w, err := svc.FreeWindows(ctx, hh, beto, free)
	if err != nil || len(w) != 1 {
		t.Errorf("com o bloco privado, o Beto vê o dia livre: %v %v", w, err)
	}
	w, err = svc.FreeWindows(ctx, hh, ana, free)
	if err != nil || len(w) != 2 {
		t.Errorf("para a Ana, o expediente ocupa o dia: %v %v", w, err)
	}

	// a Ana compartilha só o horário: o Beto passa a ver "ocupado", sem detalhes
	in.Visibility = VisibilityBusy
	if _, err := svc.Update(ctx, hh, saved.Block.ID, ana, in, saved.Block.Version); err != nil {
		t.Fatal(err)
	}
	occ, err := svc.Occurrences(ctx, hh, beto, "2026-11-02", "2026-11-02")
	if err != nil || len(occ) != 1 || !occ[0].Busy || occ[0].Title != "" || occ[0].OwnerID != ana {
		t.Errorf("o Beto deveria ver só 'ocupado': %+v %v", occ, err)
	}
	w, _ = svc.FreeWindows(ctx, hh, beto, free)
	if len(w) != 2 {
		t.Errorf("agora o expediente da Ana ocupa o horário também para o Beto: %v", w)
	}

	// uma folga libera o dia, para os dois
	if err := svc.SetException(ctx, hh, saved.Block.ID, ana, "2026-11-02", true, "", ""); err != nil {
		t.Fatal(err)
	}
	w, _ = svc.FreeWindows(ctx, hh, beto, free)
	if len(w) != 1 {
		t.Errorf("com a folga, o dia fica livre: %v", w)
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
	b, err := repo.Create(ctx, newBlock(userA, hhA, "do A", VisibilityHousehold))
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.SetException(ctx, hhA, userA, Exception{BlockID: b.ID, Date: day("2026-11-03"), Skipped: true}); err != nil {
		t.Fatal(err)
	}

	// escopo do casal B, consulta SEM filtro de casal: não pode enxergar as linhas do A
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.household_id', $1, true)`, hhB); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"routine_blocks", "routine_exceptions"} {
		var n int
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM `+table+` WHERE household_id = $1`, hhA).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			t.Errorf("o RLS deveria esconder %s do casal A, mas %d linha(s) apareceram", table, n)
		}
	}
}
