package goal

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"casal/internal/db"
	"casal/internal/identity"
	"casal/internal/testutil"
)

type fakePeople struct {
	mu sync.Mutex
	ps []Person
}

func (f *fakePeople) Members(context.Context, string) ([]Person, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]Person{}, f.ps...), nil
}

type fakeRem struct {
	Recipient string
	Body      string
	At        time.Time
	Canceled  bool
}

type fakeReminders struct {
	mu         sync.Mutex
	byRef      map[string]*fakeRem
	failCreate bool
	creates    int
}

func (f *fakeReminders) CreateWithRef(_ context.Context, _, _, recipient, body string, at time.Time, ref string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failCreate {
		return errors.New("lembretes fora do ar")
	}
	if f.byRef == nil {
		f.byRef = map[string]*fakeRem{}
	}
	if cur := f.byRef[ref]; cur != nil && !cur.Canceled {
		return nil // idempotente
	}
	f.creates++
	f.byRef[ref] = &fakeRem{Recipient: recipient, Body: body, At: at}
	return nil
}

func (f *fakeReminders) CancelByRef(_ context.Context, _, ref string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r := f.byRef[ref]; r != nil {
		r.Canceled = true
	}
	return nil
}

func (f *fakeReminders) active() map[string]*fakeRem {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := map[string]*fakeRem{}
	for k, v := range f.byRef {
		if !v.Canceled {
			out[k] = v
		}
	}
	return out
}

type fakeTask struct {
	ID       string
	Title    string
	Assignee string
	Due      *time.Time
	Status   string
	Done     *time.Time
}

type fakeTasks struct {
	mu     sync.Mutex
	byRef  map[string]*fakeTask
	seq    int
	failed bool
}

func (f *fakeTasks) Create(_ context.Context, _, _, assignee, title string, due *time.Time, ref string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failed {
		return "", errors.New("tarefas fora do ar")
	}
	if f.byRef == nil {
		f.byRef = map[string]*fakeTask{}
	}
	if cur := f.byRef[ref]; cur != nil && cur.Status != "canceled" {
		return cur.ID, nil
	}
	f.seq++
	t := &fakeTask{ID: newUUIDForTest(f.seq), Title: title, Assignee: assignee, Due: due, Status: "open"}
	f.byRef[ref] = t
	return t.ID, nil
}

func (f *fakeTasks) CancelByRef(_ context.Context, _, ref string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if t := f.byRef[ref]; t != nil && (t.Status == "open" || t.Status == "declined") {
		t.Status = "canceled"
	}
	return nil
}

func (f *fakeTasks) UpdateByRef(_ context.Context, _, ref, title string, due *time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if t := f.byRef[ref]; t != nil && (t.Status == "open" || t.Status == "declined") {
		t.Title, t.Due = title, due
	}
	return nil
}

func (f *fakeTasks) StatesByRef(_ context.Context, _ string, refs []string) (map[string]TaskState, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := map[string]TaskState{}
	for _, r := range refs {
		if t := f.byRef[r]; t != nil {
			st := TaskState{Status: t.Status, CompletedAt: t.Done}
			if t.Assignee != "" {
				a := t.Assignee
				st.AssigneeID = &a
			}
			out[r] = st
		}
	}
	return out, nil
}

// fora do código de Metas: o que a tela de Tarefas faz.
func (f *fakeTasks) set(ref, status string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	t := f.byRef[ref]
	t.Status = status
	if status == "done" {
		now := time.Now()
		t.Done = &now
	}
}

func (f *fakeTasks) reassign(ref, user string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byRef[ref].Assignee = user
}

func (f *fakeTasks) get(ref string) *fakeTask {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.byRef[ref]
}

func newUUIDForTest(n int) string {
	return "00000000-0000-4000-8000-" + padHex(n)
}

func padHex(n int) string {
	const digits = "0123456789abcdef"
	out := []byte("000000000000")
	for i := 11; i >= 0 && n > 0; i-- {
		out[i] = digits[n%16]
		n /= 16
	}
	return string(out)
}

type sentMsg struct{ User, Title, Body string }

type fakeNotifier struct {
	mu   sync.Mutex
	sent []sentMsg
}

func (f *fakeNotifier) Notify(_ context.Context, user, title, body string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, sentMsg{user, title, body})
	return nil
}

func (f *fakeNotifier) to(user string) []sentMsg {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []sentMsg
	for _, m := range f.sent {
		if m.User == user {
			out = append(out, m)
		}
	}
	return out
}

func (f *fakeNotifier) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.sent)
}

// fakeSpending: um orçamento por categoria e o gasto do casal por mês.
type fakeSpending struct {
	mu    sync.Mutex
	limit map[string]int64
	spent map[string]map[string]int64 // categoria -> mês -> centavos
	calls int
	froms []time.Time
}

func (f *fakeSpending) MonthlySpent(_ context.Context, _, cat string, from, _ time.Time) (int64, bool, map[string]int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.froms = append(f.froms, from)
	l, ok := f.limit[cat]
	if !ok {
		return 0, false, nil, nil
	}
	out := map[string]int64{}
	for m, v := range f.spent[cat] {
		out[m] = v
	}
	return l, true, out, nil
}

type fakeModules struct {
	mu sync.Mutex
	on map[string]bool
}

func (f *fakeModules) IsEnabled(_ context.Context, _, name string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.on[name], nil
}

func (f *fakeModules) set(name string, on bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.on[name] = on
}

// hoje fixo dos testes: domingo 15/11/2026, meio-dia em São Paulo.
var testNow = time.Date(2026, 11, 15, 15, 0, 0, 0, time.UTC)

// env é um casal de teste: Ana e Beto no mesmo casal (dois parceiros, como manda o CLAUDE.md), com relógio fixo.
type env struct {
	t      *testing.T
	db     *sql.DB
	repo   *PostgresRepo
	svc    *Service
	hh     string
	ana    identity.Principal
	beto   identity.Principal
	people *fakePeople
	rem    *fakeReminders
	tasks  *fakeTasks
	notes  *fakeNotifier
	spend  *fakeSpending
	mods   *fakeModules
	now    time.Time
}

func newEnv(t *testing.T) *env {
	t.Helper()
	conn := testutil.OpenDB(t)
	ana := testutil.NewUser(t, conn, "ana")
	beto := testutil.NewUser(t, conn, "beto")
	hh := testutil.NewHousehold(t, conn, ana, beto)
	e := &env{t: t, db: conn, repo: NewPostgresRepo(conn), hh: hh, now: testNow,
		people: &fakePeople{ps: []Person{
			{ID: ana, Name: "Ana", Timezone: "America/Sao_Paulo"},
			{ID: beto, Name: "Beto", Timezone: "America/Sao_Paulo"},
		}},
		rem: &fakeReminders{}, tasks: &fakeTasks{}, notes: &fakeNotifier{},
		spend: &fakeSpending{limit: map[string]int64{}, spent: map[string]map[string]int64{}},
		mods:  &fakeModules{on: map[string]bool{"finance": true, "goals": true}},
		ana:   identity.Principal{UserID: ana, HouseholdID: hh, Timezone: "America/Sao_Paulo"},
		beto:  identity.Principal{UserID: beto, HouseholdID: hh, Timezone: "America/Sao_Paulo"},
	}
	e.svc = NewService(e.repo, e.tasks, e.rem, e.people, e.notes, e.spend, e.mods,
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	e.svc.now = func() time.Time { return e.now }
	return e
}

func ctx() context.Context { return context.Background() }

func mustNoErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
}

func wantErr(t *testing.T, err, want error, what string) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Fatalf("%s: esperava %v, veio %v", what, want, err)
	}
}

// habit cria um hábito pessoal da Ana (3 vezes por semana).
func (e *env) habit(p identity.Principal, title string, in HabitInput) Goal {
	e.t.Helper()
	if in.Period == "" {
		in.Period = PeriodWeek
	}
	if in.TargetCount == 0 {
		in.TargetCount = 3
	}
	g, err := e.svc.Create(ctx(), p, CreateInput{Kind: KindHabit, Scope: ScopePersonal, Title: title, Habit: &in})
	mustNoErr(e.t, err)
	return g
}

func (e *env) shared(p identity.Principal, title string, in SharedInput) Goal {
	e.t.Helper()
	if in.Period == "" {
		in.Period = PeriodMonth
	}
	if in.TargetCount == 0 {
		in.TargetCount = 2
	}
	g, err := e.svc.Create(ctx(), p, CreateInput{Kind: KindShared, Scope: ScopeHousehold, Title: title, Shared: &in})
	mustNoErr(e.t, err)
	return g
}

func (e *env) saving(p identity.Principal, scope, title string, target int64) Goal {
	e.t.Helper()
	g, err := e.svc.Create(ctx(), p, CreateInput{Kind: KindFinancial, Scope: scope, Title: title,
		Financial: &FinancialInput{TargetCents: target}})
	mustNoErr(e.t, err)
	return g
}

func (e *env) project(p identity.Principal, scope, title string, steps ...StepInput) Goal {
	e.t.Helper()
	g, err := e.svc.Create(ctx(), p, CreateInput{Kind: KindProject, Scope: scope, Title: title, Steps: steps})
	mustNoErr(e.t, err)
	return g
}

func ids(gs []Goal) map[string]bool {
	out := map[string]bool{}
	for _, g := range gs {
		out[g.ID] = true
	}
	return out
}

func (e *env) list(p identity.Principal, f ListFilter) map[string]bool {
	e.t.Helper()
	gs, err := e.svc.List(ctx(), p, f)
	mustNoErr(e.t, err)
	return ids(gs)
}

// category cria uma categoria de Finanças de verdade (a meta de economia do orçamento tem FK para ela).
func (e *env) category(name string) string {
	e.t.Helper()
	var id string
	err := db.InTx(ctx(), e.db, db.Scope{HouseholdID: e.hh}, func(tx *sql.Tx) error {
		return tx.QueryRowContext(ctx(),
			`INSERT INTO finance_categories (household_id, name) VALUES ($1, $2) RETURNING id`, e.hh, name).Scan(&id)
	})
	mustNoErr(e.t, err)
	return id
}

// enableModule liga o módulo Metas no banco: os jobs só olham casais com ele ligado.
func (e *env) enableModule() {
	e.t.Helper()
	err := db.InTx(ctx(), e.db, db.Scope{HouseholdID: e.hh}, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx(),
			`INSERT INTO household_modules (household_id, module) VALUES ($1, 'goals') ON CONFLICT DO NOTHING`, e.hh)
		return err
	})
	mustNoErr(e.t, err)
}
