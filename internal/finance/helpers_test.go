package finance

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

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
}

func (f *fakeReminders) CreateWithRef(_ context.Context, _, _, recipient, body string, at time.Time, ref string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failCreate {
		return errors.New("reminder fora do ar")
	}
	if f.byRef == nil {
		f.byRef = map[string]*fakeRem{}
	}
	if cur := f.byRef[ref]; cur != nil && !cur.Canceled {
		return nil
	}
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

// active devolve os lembretes pendentes (não cancelados).
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

// env é um casal de teste: Ana e Beto no mesmo casal, com as categorias padrão e um relógio fixo.
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
	notes  *fakeNotifier
}

// hoje fixo dos testes: 15/11/2026 (domingo), meio-dia em São Paulo.
var testNow = time.Date(2026, 11, 15, 15, 0, 0, 0, time.UTC)

func newEnv(t *testing.T) *env {
	t.Helper()
	db := testutil.OpenDB(t)
	ana := testutil.NewUser(t, db, "ana")
	beto := testutil.NewUser(t, db, "beto")
	hh := testutil.NewHousehold(t, db, ana, beto)

	e := &env{t: t, db: db, repo: NewPostgresRepo(db), hh: hh,
		people: &fakePeople{ps: []Person{
			{ID: ana, Name: "Ana", Timezone: "America/Sao_Paulo"},
			{ID: beto, Name: "Beto", Timezone: "America/Sao_Paulo"},
		}},
		rem: &fakeReminders{}, notes: &fakeNotifier{},
		ana:  identity.Principal{UserID: ana, HouseholdID: hh, Timezone: "America/Sao_Paulo"},
		beto: identity.Principal{UserID: beto, HouseholdID: hh, Timezone: "America/Sao_Paulo"},
	}
	e.svc = NewService(e.repo, e.people, e.rem, e.notes, slog.New(slog.NewTextHandler(io.Discard, nil)))
	e.svc.now = func() time.Time { return testNow }

	err := e.repo.inHousehold(context.Background(), hh, func(tx *sql.Tx) error {
		return SeedCategories(context.Background(), tx, hh)
	})
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func (e *env) cat(key string) string {
	e.t.Helper()
	c, err := e.repo.CategoryByKey(context.Background(), e.hh, key)
	if err != nil {
		e.t.Fatal(err)
	}
	return c.ID
}

// spend registra um gasto do casal (divisão do casal) e devolve o primeiro gasto criado.
func (e *env) spend(p identity.Principal, cents int64, spentOn string) Expense {
	e.t.Helper()
	res, err := e.svc.CreateExpense(context.Background(), p, ExpenseInput{
		AmountCents: cents, CategoryID: e.cat(KeyGroceries), SpentOn: spentOn, Description: "teste",
	})
	if err != nil {
		e.t.Fatal(err)
	}
	return res.Expenses[0]
}

// personal registra um gasto pessoal.
func (e *env) personal(p identity.Principal, cents int64, vis string) Expense {
	e.t.Helper()
	res, err := e.svc.CreateExpense(context.Background(), p, ExpenseInput{
		Scope: ScopePersonal, Visibility: vis, AmountCents: cents, CategoryID: e.cat(KeyLeisure), Description: "segredo",
	})
	if err != nil {
		e.t.Fatal(err)
	}
	return res.Expenses[0]
}

const KeyLeisure = "leisure"

func ctx() context.Context { return context.Background() }

func mustNoErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func wantErr(t *testing.T, err error, want error, what string) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Errorf("%s: esperava %v, veio %v", what, want, err)
	}
}

// rlsRole diz se a conexão de teste é o papel sem BYPASSRLS (casal_app).
func rlsRole(t *testing.T, db *sql.DB) bool {
	t.Helper()
	var rls bool
	if err := db.QueryRow(`SELECT NOT rolbypassrls AND NOT rolsuper FROM pg_roles WHERE rolname = current_user`).Scan(&rls); err != nil {
		t.Fatal(err)
	}
	return rls
}

type txT = *sql.Tx

var errorsIs = errors.Is

const KeyHousing = "housing"
