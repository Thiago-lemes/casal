package app

import (
	"context"
	"time"

	"casal/internal/finance"
	"casal/internal/household"
	"casal/internal/reminder"
	"casal/internal/routine"
)

// routinePeople deixa a rotina listar os membros do casal sem importar o pacote household:
// só o app conhece os dois.
type routinePeople struct{ households *household.Service }

func (p routinePeople) Members(ctx context.Context, householdID string) ([]routine.Person, error) {
	members, err := p.households.Members(ctx, householdID)
	if err != nil {
		return nil, err
	}
	out := make([]routine.Person, 0, len(members))
	for _, m := range members {
		out = append(out, routine.Person{ID: m.ID, Name: m.Name})
	}
	return out, nil
}

// financePeople dá a Finanças os membros do casal com o fuso de cada um.
type financePeople struct{ households *household.Service }

func (p financePeople) Members(ctx context.Context, householdID string) ([]finance.Person, error) {
	members, err := p.households.Members(ctx, householdID)
	if err != nil {
		return nil, err
	}
	out := make([]finance.Person, 0, len(members))
	for _, m := range members {
		out = append(out, finance.Person{ID: m.ID, Name: m.Name, Timezone: m.Timezone})
	}
	return out, nil
}

// financeReminders deixa Finanças criar e cancelar lembretes (por ref) sem importar o pacote reminder.
type financeReminders struct{ reminders *reminder.Service }

func (r financeReminders) CreateWithRef(ctx context.Context, householdID, creatorID, recipientID, body string, remindAt time.Time, ref string) error {
	_, err := r.reminders.CreateWithRef(ctx, householdID, creatorID, recipientID, body, remindAt, ref)
	return err
}

func (r financeReminders) CancelByRef(ctx context.Context, householdID, ref string) error {
	_, err := r.reminders.CancelByRef(ctx, householdID, ref)
	return err
}
