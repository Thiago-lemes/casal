package task

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"
)

// Members é o que este pacote precisa da feature household.
type Members interface {
	AreMembers(ctx context.Context, householdID string, userIDs ...string) (bool, error)
	// OtherMember devolve a outra pessoa do casal; ok é falso se o casal só tem a própria pessoa.
	OtherMember(ctx context.Context, householdID, userID string) (id string, ok bool, err error)
}

// Notifier envia o aviso a todos os aparelhos de uma pessoa.
type Notifier interface {
	Notify(ctx context.Context, userID, title, body string) error
}

type Service struct {
	repo     Repository
	members  Members
	notifier Notifier
	log      *slog.Logger
	now      func() time.Time
	after    []func(ctx context.Context, t Task)
}

// AfterChange registra uma função chamada depois de uma tarefa ser concluída ou cancelada, já fora da transação.
// Quem registra trata as próprias falhas: o gancho não devolve erro e nunca desfaz a operação. Registre na montagem
// do app, antes de atender requisições.
func (s *Service) AfterChange(f func(ctx context.Context, t Task)) { s.after = append(s.after, f) }

func (s *Service) changed(ctx context.Context, t Task) {
	for _, f := range s.after {
		func() {
			// a tarefa já foi gravada: um gancho que quebra não pode derrubar a resposta
			defer func() {
				if r := recover(); r != nil {
					s.log.Error("gancho de tarefa falhou", "task", t.ID, "panic", r)
				}
			}()
			f(ctx, t)
		}()
	}
}

func NewService(repo Repository, members Members, notifier Notifier, log *slog.Logger) *Service {
	return &Service{repo: repo, members: members, notifier: notifier, log: log, now: time.Now}
}

// CreateInput descreve uma tarefa nova. Sem AssigneeID e sem Shared, o responsável é quem criou.
type CreateInput struct {
	Title      string
	AssigneeID string
	Shared     bool // tarefa do casal: sem responsável fixo
	DueAt      *time.Time
	Repeat     string // none (padrão), daily, weekly ou monthly: reaparece ao ser concluída
	Rotation   bool   // rodízio: a cada ciclo o responsável passa para a outra pessoa
}

func (s *Service) Create(ctx context.Context, householdID, creatorID string, in CreateInput) (Task, error) {
	title, err := cleanText(in.Title, 200, "o título")
	if err != nil {
		return Task{}, err
	}
	assignee, err := s.resolveAssignee(ctx, householdID, creatorID, in.AssigneeID, in.Shared)
	if err != nil {
		return Task{}, err
	}
	if in.Repeat == "" {
		in.Repeat = RepeatNone
	}
	switch in.Repeat {
	case RepeatNone, RepeatDaily, RepeatWeekly, RepeatMonthly:
	default:
		return Task{}, fmt.Errorf("%w: repeat deve ser none, daily, weekly ou monthly", ErrInvalidInput)
	}
	if in.Rotation && (in.Repeat == RepeatNone || assignee == nil) {
		return Task{}, fmt.Errorf("%w: o rodízio precisa de repetição e de um responsável", ErrInvalidInput)
	}
	if in.DueAt != nil {
		utc := in.DueAt.UTC()
		in.DueAt = &utc
	}
	t, err := s.repo.Create(ctx, Task{
		HouseholdID: householdID,
		CreatedBy:   creatorID,
		AssigneeID:  assignee,
		Title:       title,
		DueAt:       in.DueAt,
		Repeat:      in.Repeat,
		Rotation:    in.Rotation,
	})
	if err != nil {
		return Task{}, err
	}
	s.notifyAssigned(ctx, t, creatorID)
	return t, nil
}

func (s *Service) List(ctx context.Context, householdID, status string) ([]Task, error) {
	if status == "" {
		status = StatusOpen
	}
	switch status {
	case StatusOpen, StatusDone, StatusDeclined:
	default:
		return nil, fmt.Errorf("%w: status deve ser open, done ou declined", ErrInvalidInput)
	}
	return s.repo.List(ctx, householdID, status)
}

// Complete conclui a tarefa. Se ela se repete, a próxima ocorrência nasce junto, na mesma transação:
// no rodízio, com a outra pessoa como responsável (que é avisada).
func (s *Service) Complete(ctx context.Context, householdID, id, userID string) (Task, error) {
	cur, err := s.repo.Get(ctx, householdID, id)
	if err != nil {
		return Task{}, err
	}
	var next *Task
	if cur.Status == StatusOpen && cur.Repeat != RepeatNone {
		if next, err = s.nextOccurrence(ctx, householdID, cur); err != nil {
			return Task{}, err
		}
	}
	done, created, err := s.repo.Complete(ctx, householdID, id, userID, s.now(), next)
	if err != nil {
		return Task{}, err
	}
	if created != nil {
		s.notifyAssigned(ctx, *created, userID)
	}
	if done.Ref != nil {
		s.changed(context.WithoutCancel(ctx), done)
	}
	return done, nil
}

// nextOccurrence monta a tarefa que reaparece: mesmo título e criador; prazo avançado um ciclo;
// responsável igual, ou a outra pessoa do casal se for rodízio.
func (s *Service) nextOccurrence(ctx context.Context, householdID string, cur Task) (*Task, error) {
	next := Task{
		CreatedBy: cur.CreatedBy, AssigneeID: cur.AssigneeID, Title: cur.Title,
		Repeat: cur.Repeat, Rotation: cur.Rotation,
	}
	if cur.DueAt != nil {
		d := NextDue(*cur.DueAt, cur.Repeat, s.now())
		next.DueAt = &d
	}
	if cur.Rotation && cur.AssigneeID != nil {
		other, ok, err := s.members.OtherMember(ctx, householdID, *cur.AssigneeID)
		if err != nil {
			return nil, err
		}
		if ok { // casal de uma pessoa só: o rodízio fica com ela
			next.AssigneeID = &other
		}
	}
	return &next, nil
}

// NextDue avança o prazo um ciclo por vez, a partir do prazo anterior (e não de quando foi concluída,
// para a agenda não escorregar), até cair depois de agora. Concluir com atraso pula os ciclos perdidos.
// No mensal, o dia 31 vira o último dia do mês curto (nunca pula para o mês seguinte).
func NextDue(due time.Time, repeat string, now time.Time) time.Time {
	step := func(n int) time.Time {
		switch repeat {
		case RepeatDaily:
			return due.AddDate(0, 0, n)
		case RepeatWeekly:
			return due.AddDate(0, 0, 7*n)
		default:
			return addMonths(due, n)
		}
	}
	n := 1
	next := step(n)
	for next.Before(now) && n < 5000 {
		n++
		next = step(n)
	}
	return next
}

// addMonths soma meses sem estourar o dia: 31 de janeiro mais um mês é 28 (ou 29) de fevereiro.
func addMonths(t time.Time, months int) time.Time {
	y, m, d := t.Date()
	first := time.Date(y, m+time.Month(months), 1, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), t.Location())
	last := time.Date(first.Year(), first.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
	return time.Date(first.Year(), first.Month(), min(d, last), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), t.Location())
}

// Decline recusa a tarefa atribuída a você. A justificativa é obrigatória e vai a quem criou.
func (s *Service) Decline(ctx context.Context, householdID, id, userID, reason string) (Task, error) {
	reason, err := cleanText(reason, 500, "a justificativa")
	if err != nil {
		return Task{}, err
	}
	t, err := s.repo.Decline(ctx, householdID, id, userID, reason)
	if err != nil {
		return Task{}, err
	}
	s.notify(ctx, t.CreatedBy, "Tarefa recusada", t.Title+": "+reason)
	return t, nil
}

// Reassign troca o responsável (inclusive para quem criou, que assume a tarefa) e reabre a tarefa.
func (s *Service) Reassign(ctx context.Context, householdID, id, userID, assigneeID string, shared bool) (Task, error) {
	if !shared && assigneeID == "" {
		return Task{}, fmt.Errorf("%w: informe o responsável ou marque como tarefa do casal", ErrInvalidInput)
	}
	assignee, err := s.resolveAssignee(ctx, householdID, userID, assigneeID, shared)
	if err != nil {
		return Task{}, err
	}
	t, err := s.repo.Reassign(ctx, householdID, id, userID, assignee)
	if err != nil {
		return Task{}, err
	}
	s.notifyAssigned(ctx, t, userID)
	return t, nil
}

func (s *Service) Cancel(ctx context.Context, householdID, id, userID string) error {
	if err := s.repo.Cancel(ctx, householdID, id, userID); err != nil {
		return err
	}
	if len(s.after) > 0 {
		// a tarefa já foi cancelada: se o cliente desconectar, o gancho ainda precisa rodar
		bg := context.WithoutCancel(ctx)
		if t, err := s.repo.Get(bg, householdID, id); err == nil && t.Ref != nil {
			s.changed(bg, t)
		}
	}
	return nil
}

// RefInput descreve a tarefa que outro módulo cria para si, sem passar por uma pessoa na tela.
type RefInput struct {
	Title      string
	AssigneeID string // vazio: tarefa do casal
	DueAt      *time.Time
}

// CreateWithRef é a entrada para outros módulos (por uma interface declarada neles, ligada no app): cria a tarefa
// marcada com o ref ou devolve a que já existe com ele. Não avisa ninguém: quem chama decide se há aviso.
func (s *Service) CreateWithRef(ctx context.Context, householdID, creatorID string, in RefInput, ref string) (Task, error) {
	title, err := cleanText(in.Title, 200, "o título")
	if err != nil {
		return Task{}, err
	}
	if ref == "" || len(ref) > 100 {
		return Task{}, fmt.Errorf("%w: ref inválido", ErrInvalidInput)
	}
	assignee, err := s.resolveAssignee(ctx, householdID, creatorID, in.AssigneeID, in.AssigneeID == "")
	if err != nil {
		return Task{}, err
	}
	var due *time.Time
	if in.DueAt != nil {
		u := in.DueAt.UTC()
		due = &u
	}
	return s.repo.CreateWithRef(ctx, Task{
		HouseholdID: householdID, CreatedBy: creatorID, AssigneeID: assignee, Title: title, DueAt: due, Repeat: RepeatNone,
	}, ref)
}

// CancelByRef cancela as tarefas abertas ou recusadas com o ref e solta o vínculo das concluídas. Cancelar o que
// não existe não é erro. Não chama o gancho AfterChange: quem cancela por aqui já sabe o que fez.
func (s *Service) CancelByRef(ctx context.Context, householdID, ref string) (int64, error) {
	return s.repo.CancelByRef(ctx, householdID, ref)
}

// StatesByRef lê o estado das tarefas pelos refs; um ref sem tarefa não aparece no mapa.
func (s *Service) StatesByRef(ctx context.Context, householdID string, refs []string) (map[string]State, error) {
	return s.repo.StatesByRef(ctx, householdID, refs)
}

// resolveAssignee devolve o responsável (nil: tarefa do casal) e confere que é do casal.
func (s *Service) resolveAssignee(ctx context.Context, householdID, actorID, assigneeID string, shared bool) (*string, error) {
	if shared {
		if assigneeID != "" {
			return nil, fmt.Errorf("%w: tarefa do casal não tem responsável", ErrInvalidInput)
		}
		return nil, nil
	}
	if assigneeID == "" {
		assigneeID = actorID
	}
	ok, err := s.members.AreMembers(ctx, householdID, actorID, assigneeID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("%w: a pessoa não pertence ao casal", ErrInvalidInput)
	}
	return &assigneeID, nil
}

// notifyAssigned avisa o responsável, a menos que ele mesmo tenha feito a atribuição.
func (s *Service) notifyAssigned(ctx context.Context, t Task, actorID string) {
	if t.AssigneeID == nil || *t.AssigneeID == actorID {
		return
	}
	s.notify(ctx, *t.AssigneeID, "Nova tarefa para você", t.Title)
}

// notify nunca falha a operação: o aviso é um extra, e a tarefa já foi gravada.
// Roda fora de qualquer transação.
func (s *Service) notify(ctx context.Context, userID, title, body string) {
	if err := s.notifier.Notify(ctx, userID, title, body); err != nil {
		s.log.Warn("aviso de tarefa não enviado", "user", userID, "error", err)
	}
}

func cleanText(v string, max int, what string) (string, error) {
	v = strings.TrimSpace(v)
	if v == "" || utf8.RuneCountInString(v) > max {
		return "", fmt.Errorf("%w: %s é obrigatório (até %d caracteres)", ErrInvalidInput, what, max)
	}
	return v, nil
}

// UpdateByRef muda título e prazo das tarefas abertas ou recusadas com o ref (a etapa de uma meta foi editada).
func (s *Service) UpdateByRef(ctx context.Context, householdID, ref, title string, dueAt *time.Time) error {
	title, err := cleanText(title, 200, "o título")
	if err != nil {
		return err
	}
	if dueAt != nil {
		u := dueAt.UTC()
		dueAt = &u
	}
	return s.repo.UpdateByRef(ctx, householdID, ref, title, dueAt)
}
