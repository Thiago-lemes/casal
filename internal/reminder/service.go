package reminder

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// Members é o que este pacote precisa da feature household.
type Members interface {
	AreMembers(ctx context.Context, householdID string, userIDs ...string) (bool, error)
}

type Service struct {
	repo    Repository
	members Members
	now     func() time.Time
}

func NewService(repo Repository, members Members) *Service {
	return &Service{repo: repo, members: members, now: time.Now}
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
	return s.repo.Create(ctx, Reminder{
		HouseholdID: householdID,
		CreatedBy:   creatorID,
		RecipientID: recipientID,
		Body:        body,
		RemindAt:    remindAt.UTC(),
	})
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
