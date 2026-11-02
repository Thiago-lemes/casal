package task

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"
)

type fakeRepo struct {
	Repository
	created           []Task
	declined          Task
	current           Task  // o que Get devolve
	nextSeen          *Task // o que o service pediu para criar ao concluir
	createdByComplete *Task
}

func (f *fakeRepo) Create(_ context.Context, t Task) (Task, error) {
	t.ID = "t1"
	t.Status = StatusOpen
	f.created = append(f.created, t)
	return t, nil
}

func (f *fakeRepo) List(context.Context, string, string) ([]Task, error) { return []Task{}, nil }

func (f *fakeRepo) Decline(_ context.Context, _, _, _, reason string) (Task, error) {
	f.declined.DeclineReason = &reason
	return f.declined, nil
}

func (f *fakeRepo) Reassign(_ context.Context, _, id, _ string, assigneeID *string) (Task, error) {
	return Task{ID: id, Title: "lavar a louça", AssigneeID: assigneeID, Status: StatusOpen}, nil
}

type fakeMembers struct{ ok, alone bool }

func (f fakeMembers) AreMembers(context.Context, string, ...string) (bool, error) { return f.ok, nil }

type sentNotice struct{ user, title, body string }

type fakeNotifier struct {
	sent []sentNotice
	err  error
}

func (f *fakeNotifier) Notify(_ context.Context, userID, title, body string) error {
	f.sent = append(f.sent, sentNotice{userID, title, body})
	return f.err
}

func newTestService(membersOK bool) (*Service, *fakeRepo, *fakeNotifier) {
	repo := &fakeRepo{}
	n := &fakeNotifier{}
	svc := NewService(repo, fakeMembers{ok: membersOK}, n, slog.New(slog.NewTextHandler(io.Discard, nil)))
	return svc, repo, n
}

func TestCreateDefaultsAssigneeToCreator(t *testing.T) {
	svc, repo, n := newTestService(true)
	got, err := svc.Create(context.Background(), "h1", "u1", CreateInput{Title: "  lavar a louça "})
	if err != nil {
		t.Fatal(err)
	}
	if got.AssigneeID == nil || *got.AssigneeID != "u1" || got.Title != "lavar a louça" || len(repo.created) != 1 {
		t.Errorf("resultado inesperado: %+v", got)
	}
	if len(n.sent) != 0 {
		t.Errorf("quem cria para si mesmo não deveria ser avisado: %+v", n.sent)
	}
}

func TestCreateForPartnerNotifiesThem(t *testing.T) {
	svc, _, n := newTestService(true)
	if _, err := svc.Create(context.Background(), "h1", "u1", CreateInput{Title: "tirar o lixo", AssigneeID: "u2"}); err != nil {
		t.Fatal(err)
	}
	if len(n.sent) != 1 || n.sent[0].user != "u2" {
		t.Errorf("u2 deveria ser avisado uma vez: %+v", n.sent)
	}
}

func TestCreateSharedHasNoAssignee(t *testing.T) {
	svc, _, n := newTestService(true)
	got, err := svc.Create(context.Background(), "h1", "u1", CreateInput{Title: "lavar o banheiro", Shared: true})
	if err != nil {
		t.Fatal(err)
	}
	if got.AssigneeID != nil || len(n.sent) != 0 {
		t.Errorf("tarefa do casal não tem responsável nem aviso: %+v %+v", got, n.sent)
	}
}

func TestCreateValidation(t *testing.T) {
	cases := []struct {
		name      string
		membersOK bool
		in        CreateInput
	}{
		{"título vazio", true, CreateInput{Title: "   "}},
		{"título longo demais", true, CreateInput{Title: strings.Repeat("a", 201)}},
		{"responsável de outro casal", false, CreateInput{Title: "ok", AssigneeID: "u9"}},
		{"casal e responsável juntos", true, CreateInput{Title: "ok", Shared: true, AssigneeID: "u2"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, repo, _ := newTestService(c.membersOK)
			if _, err := svc.Create(context.Background(), "h1", "u1", c.in); !errors.Is(err, ErrInvalidInput) {
				t.Errorf("esperava ErrInvalidInput, veio %v", err)
			}
			if len(repo.created) != 0 {
				t.Error("não deveria ter gravado")
			}
		})
	}
}

func TestDeclineRequiresReasonAndNotifiesCreator(t *testing.T) {
	svc, repo, n := newTestService(true)
	repo.declined = Task{ID: "t1", CreatedBy: "u1", Title: "tirar o lixo", Status: StatusDeclined}

	for _, reason := range []string{"", "   ", strings.Repeat("a", 501)} {
		if _, err := svc.Decline(context.Background(), "h1", "t1", "u2", reason); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("justificativa %q deveria ser inválida, veio %v", reason, err)
		}
	}
	if len(n.sent) != 0 {
		t.Fatalf("recusa inválida não deveria avisar ninguém: %+v", n.sent)
	}

	if _, err := svc.Decline(context.Background(), "h1", "t1", "u2", " estou viajando "); err != nil {
		t.Fatal(err)
	}
	if len(n.sent) != 1 || n.sent[0].user != "u1" || !strings.Contains(n.sent[0].body, "estou viajando") {
		t.Errorf("u1 deveria receber a justificativa: %+v", n.sent)
	}
}

func TestNotifierFailureDoesNotFailTheOperation(t *testing.T) {
	svc, repo, n := newTestService(true)
	n.err = errors.New("push fora do ar")
	if _, err := svc.Create(context.Background(), "h1", "u1", CreateInput{Title: "ok", AssigneeID: "u2"}); err != nil {
		t.Errorf("falha no aviso não deveria derrubar a criação: %v", err)
	}
	if len(repo.created) != 1 {
		t.Error("a tarefa deveria ter sido gravada")
	}
}

func TestReassign(t *testing.T) {
	svc, _, n := newTestService(true)
	if _, err := svc.Reassign(context.Background(), "h1", "t1", "u1", "", false); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("sem responsável nem casal deveria ser inválido: %v", err)
	}
	if _, err := svc.Reassign(context.Background(), "h1", "t1", "u1", "u1", false); err != nil {
		t.Fatal(err)
	}
	if len(n.sent) != 0 {
		t.Errorf("quem assume a própria tarefa não deveria ser avisado: %+v", n.sent)
	}
	if _, err := svc.Reassign(context.Background(), "h1", "t1", "u1", "u2", false); err != nil {
		t.Fatal(err)
	}
	if len(n.sent) != 1 || n.sent[0].user != "u2" {
		t.Errorf("u2 deveria ser avisado: %+v", n.sent)
	}
}

func TestListStatus(t *testing.T) {
	svc, _, _ := newTestService(true)
	if _, err := svc.List(context.Background(), "h1", "canceled"); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("status canceled não é listável: %v", err)
	}
	if _, err := svc.List(context.Background(), "h1", ""); err != nil {
		t.Errorf("sem status, deveria listar as abertas: %v", err)
	}
}

func (f *fakeRepo) Get(context.Context, string, string) (Task, error) { return f.current, nil }

func (f *fakeRepo) Complete(_ context.Context, _, id, _ string, _ time.Time, next *Task) (Task, *Task, error) {
	f.nextSeen = next
	done := f.current
	done.ID, done.Status = id, StatusDone
	if next == nil {
		return done, nil, nil
	}
	made := *next
	made.ID, made.Status = "t2", StatusOpen
	f.createdByComplete = &made
	return done, &made, nil
}

func (f fakeMembers) OtherMember(_ context.Context, _, userID string) (string, bool, error) {
	if f.alone {
		return "", false, nil
	}
	if userID == "u1" {
		return "u2", true, nil
	}
	return "u1", true, nil
}

func TestCreateRepeatValidation(t *testing.T) {
	cases := []struct {
		name string
		in   CreateInput
		ok   bool
	}{
		{"repetição semanal", CreateInput{Title: "lixo", Repeat: RepeatWeekly}, true},
		{"rodízio semanal", CreateInput{Title: "lixo", Repeat: RepeatWeekly, Rotation: true}, true},
		{"repetição do casal", CreateInput{Title: "banheiro", Repeat: RepeatMonthly, Shared: true}, true},
		{"repetição inválida", CreateInput{Title: "lixo", Repeat: "hourly"}, false},
		{"rodízio sem repetição", CreateInput{Title: "lixo", Rotation: true}, false},
		{"rodízio numa tarefa do casal", CreateInput{Title: "lixo", Repeat: RepeatDaily, Rotation: true, Shared: true}, false},
	}
	for _, c := range cases {
		svc, repo, _ := newTestService(true)
		_, err := svc.Create(context.Background(), "h1", "u1", c.in)
		if c.ok && err != nil {
			t.Errorf("%s: deveria valer: %v", c.name, err)
		}
		if !c.ok && (!errors.Is(err, ErrInvalidInput) || len(repo.created) != 0) {
			t.Errorf("%s: esperava ErrInvalidInput sem gravar, veio %v", c.name, err)
		}
	}
	svc, repo, _ := newTestService(true)
	if _, err := svc.Create(context.Background(), "h1", "u1", CreateInput{Title: "lixo"}); err != nil || repo.created[0].Repeat != RepeatNone {
		t.Errorf("sem repeat, o padrão é none: %+v %v", repo.created, err)
	}
}

func u(s string) *string { return &s }

func TestCompleteWithoutRepeatCreatesNothing(t *testing.T) {
	svc, repo, _ := newTestService(true)
	repo.current = Task{ID: "t1", Status: StatusOpen, Repeat: RepeatNone, AssigneeID: u("u1")}
	if _, err := svc.Complete(context.Background(), "h1", "t1", "u1"); err != nil {
		t.Fatal(err)
	}
	if repo.nextSeen != nil {
		t.Errorf("tarefa sem repetição não gera a próxima: %+v", repo.nextSeen)
	}
}

func TestCompleteRepeatsKeepingAssignee(t *testing.T) {
	svc, repo, n := newTestService(true)
	svc.now = func() time.Time { return time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC) }
	due := time.Date(2026, 11, 2, 20, 0, 0, 0, time.UTC)
	repo.current = Task{ID: "t1", Title: "lixo", CreatedBy: "u1", Status: StatusOpen, Repeat: RepeatWeekly,
		AssigneeID: u("u2"), DueAt: &due}
	if _, err := svc.Complete(context.Background(), "h1", "t1", "u2"); err != nil {
		t.Fatal(err)
	}
	next := repo.nextSeen
	if next == nil || next.Title != "lixo" || next.CreatedBy != "u1" || *next.AssigneeID != "u2" || next.Repeat != RepeatWeekly {
		t.Fatalf("a próxima deveria manter título, criador, responsável e repetição: %+v", next)
	}
	if want := due.AddDate(0, 0, 7); !next.DueAt.Equal(want) {
		t.Errorf("prazo seguinte %v, queria %v", next.DueAt, want)
	}
	if len(n.sent) != 0 {
		t.Errorf("o mesmo responsável concluindo não precisa de aviso: %+v", n.sent)
	}
}

func TestCompleteRotationPassesToOtherPerson(t *testing.T) {
	svc, repo, n := newTestService(true)
	svc.now = func() time.Time { return time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC) }
	repo.current = Task{ID: "t1", Title: "lavar a louça", CreatedBy: "u1", Status: StatusOpen, Repeat: RepeatDaily,
		Rotation: true, AssigneeID: u("u2")}
	if _, err := svc.Complete(context.Background(), "h1", "t1", "u2"); err != nil {
		t.Fatal(err)
	}
	next := repo.nextSeen
	if next == nil || *next.AssigneeID != "u1" || !next.Rotation {
		t.Fatalf("o rodízio deveria passar para u1 e continuar: %+v", next)
	}
	if next.DueAt != nil {
		t.Errorf("sem prazo, a próxima também fica sem prazo: %v", next.DueAt)
	}
	if len(n.sent) != 1 || n.sent[0].user != "u1" {
		t.Errorf("quem recebe a vez deveria ser avisado: %+v", n.sent)
	}
}

func TestCompleteRotationAloneKeepsTheSamePerson(t *testing.T) {
	svc, repo, _ := newTestService(true)
	svc.members = fakeMembers{ok: true, alone: true}
	repo.current = Task{ID: "t1", Title: "x", CreatedBy: "u1", Status: StatusOpen, Repeat: RepeatDaily, Rotation: true, AssigneeID: u("u1")}
	if _, err := svc.Complete(context.Background(), "h1", "t1", "u1"); err != nil {
		t.Fatal(err)
	}
	if repo.nextSeen == nil || *repo.nextSeen.AssigneeID != "u1" {
		t.Errorf("casal de uma pessoa só: o rodízio fica com ela: %+v", repo.nextSeen)
	}
}

func TestCompleteSkipsMissedCycles(t *testing.T) {
	svc, repo, _ := newTestService(true)
	svc.now = func() time.Time { return time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC) }
	due := time.Date(2026, 11, 2, 20, 0, 0, 0, time.UTC) // semanal, 18 dias de atraso
	repo.current = Task{ID: "t1", Status: StatusOpen, Repeat: RepeatWeekly, AssigneeID: u("u1"), DueAt: &due}
	if _, err := svc.Complete(context.Background(), "h1", "t1", "u1"); err != nil {
		t.Fatal(err)
	}
	if want := time.Date(2026, 11, 23, 20, 0, 0, 0, time.UTC); !repo.nextSeen.DueAt.Equal(want) {
		t.Errorf("deveria pular os ciclos perdidos e cair em %v, caiu em %v", want, repo.nextSeen.DueAt)
	}
}

func TestNextDue(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	d := func(y, m, day int) time.Time { return time.Date(y, time.Month(m), day, 18, 30, 0, 0, time.UTC) }
	cases := []struct {
		name   string
		due    time.Time
		repeat string
		now    time.Time
		want   time.Time
	}{
		{"diário", d(2026, 3, 10), RepeatDaily, now, d(2026, 3, 11)},
		{"semanal", d(2026, 3, 10), RepeatWeekly, now, d(2026, 3, 17)},
		{"mensal", d(2026, 3, 10), RepeatMonthly, now, d(2026, 4, 10)},
		{"mensal no dia 31 vai para o fim do mês curto", d(2026, 1, 31), RepeatMonthly, now, d(2026, 2, 28)},
		{"mensal depois de fevereiro volta ao dia 31", d(2026, 1, 31), RepeatMonthly, d(2026, 3, 1), d(2026, 3, 31)},
		{"mensal em ano bissexto", d(2028, 1, 31), RepeatMonthly, time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC), d(2028, 2, 29)},
		{"virada de ano", d(2026, 12, 15), RepeatMonthly, now, d(2027, 1, 15)},
		{"atrasado pula ciclos", d(2026, 3, 1), RepeatDaily, d(2026, 3, 10), d(2026, 3, 10)},
		{"antecipado não pula", d(2026, 3, 20), RepeatWeekly, d(2026, 3, 10), d(2026, 3, 27)},
	}
	for _, c := range cases {
		if got := NextDue(c.due, c.repeat, c.now); !got.Equal(c.want) {
			t.Errorf("%s: veio %v, queria %v", c.name, got, c.want)
		}
	}
}
