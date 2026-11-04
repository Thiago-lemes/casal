package reminder

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
}

// createNotifyTimeout limita o aviso de "lembrete novo", que roda na requisição de quem criou.
const createNotifyTimeout = 5 * time.Second

type Service struct {
	repo     Repository
	members  Members
	notifier Notifier
	log      *slog.Logger
	now      func() time.Time
}

func NewService(repo Repository, members Members, notifier Notifier, log *slog.Logger) *Service {
	return &Service{repo: repo, members: members, notifier: notifier, log: log, now: time.Now}
}

func (s *Service) Create(ctx context.Context, householdID, creatorID, recipientID, body string, remindAt time.Time) (Reminder, error) {
	body = strings.TrimSpace(body)
	if body == "" || utf8.RuneCountInString(body) > 500 {
		return Reminder{}, fmt.Errorf("%w: o texto é obrigatório (até 500 caracteres)", ErrInvalidInput)
	}
	if remindAt.IsZero() {
		return Reminder{}, fmt.Errorf("%w: informe a data e a hora", ErrInvalidInput)
	}
	if remindAt.Before(s.now().Add(-time.Minute)) {
		return Reminder{}, fmt.Errorf("%w: o horário já passou", ErrInvalidInput)
	}
	if recipientID == "" {
		recipientID = creatorID
	}
	ok, err := s.members.AreMembers(ctx, householdID, creatorID, recipientID)
	if err != nil {
		return Reminder{}, err
	}
	if !ok {
		return Reminder{}, fmt.Errorf("%w: a pessoa não pertence ao casal", ErrInvalidInput)
	}
	r, err := s.repo.Create(ctx, Reminder{
		HouseholdID: householdID,
		CreatedBy:   creatorID,
		RecipientID: recipientID,
		Body:        body,
		RemindAt:    remindAt.UTC(),
	})
	if err != nil {
		return Reminder{}, err
	}
	s.notifyCreated(ctx, r)
	return r, nil
}

// notifyCreated avisa o parceiro na hora que alguém cria um lembrete para ele. Roda depois da transação
// (rede nunca dentro dela) e o erro só vai para o log: o lembrete já existe e o aviso da hora marcada ainda sai.
// Quem cria um lembrete para si mesmo não recebe nada. Só o Create (feito por uma pessoa) avisa, não o CreateWithRef.
func (s *Service) notifyCreated(ctx context.Context, r Reminder) {
	if r.RecipientID == r.CreatedBy {
		return
	}
	// o contexto da requisição pode terminar logo depois da resposta, então o aviso tem prazo próprio
	nctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), createNotifyTimeout)
	defer cancel()
	title := r.CreatedByName + " criou um lembrete para você"
	if err := s.notifier.Notify(nctx, r.RecipientID, title, r.Body); err != nil {
		s.log.Warn("aviso de lembrete novo não enviado", "reminder", r.ID, "err", err)
	}
}

// CreateWithRef é a entrada para outros módulos (por meio de uma interface declarada neles e ligada no app):
// cria o lembrete sem checar se o horário já passou (quem chama decide) e marca com um ref, para cancelá-lo
// depois com CancelByRef. Criar de novo com um ref ainda ativo devolve o lembrete que já existe.
func (s *Service) CreateWithRef(ctx context.Context, householdID, creatorID, recipientID, body string, remindAt time.Time, ref string) (Reminder, error) {
	body = strings.TrimSpace(body)
	if body == "" || utf8.RuneCountInString(body) > 500 {
		return Reminder{}, fmt.Errorf("%w: o texto é obrigatório (até 500 caracteres)", ErrInvalidInput)
	}
	if remindAt.IsZero() {
		return Reminder{}, fmt.Errorf("%w: informe a data e a hora", ErrInvalidInput)
	}
	if ref == "" || len(ref) > 100 {
		return Reminder{}, fmt.Errorf("%w: ref inválido", ErrInvalidInput)
	}
	if recipientID == "" {
		recipientID = creatorID
	}
	ok, err := s.members.AreMembers(ctx, householdID, creatorID, recipientID)
	if err != nil {
		return Reminder{}, err
	}
	if !ok {
		return Reminder{}, fmt.Errorf("%w: a pessoa não pertence ao casal", ErrInvalidInput)
	}
	return s.repo.CreateWithRef(ctx, Reminder{
		HouseholdID: householdID,
		CreatedBy:   creatorID,
		RecipientID: recipientID,
		Body:        body,
		RemindAt:    remindAt.UTC(),
	}, ref)
}

// CancelByRef cancela os lembretes pendentes criados com esse ref e devolve quantos eram. Cancelar o que não existe
// não é erro.
func (s *Service) CancelByRef(ctx context.Context, householdID, ref string) (int64, error) {
	return s.repo.CancelByRef(ctx, householdID, ref)
}

func (s *Service) List(ctx context.Context, householdID, userID string) ([]Reminder, error) {
	return s.repo.List(ctx, householdID, userID)
}

func (s *Service) Resolve(ctx context.Context, householdID, id, userID string) (Reminder, error) {
	return s.repo.Resolve(ctx, householdID, id, userID, s.now())
}

func (s *Service) Snooze(ctx context.Context, householdID, id, userID string, minutes int) (Reminder, error) {
	if minutes < 1 || minutes > 24*60 {
		return Reminder{}, fmt.Errorf("%w: adie de 1 a 1440 minutos", ErrInvalidInput)
	}
	return s.repo.Snooze(ctx, householdID, id, userID, s.now().Add(time.Duration(minutes)*time.Minute))
}

func (s *Service) Cancel(ctx context.Context, householdID, id, userID string) error {
	return s.repo.Cancel(ctx, householdID, id, userID)
}
