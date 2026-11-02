package app

import (
	"context"
	"time"

	"casal/internal/finance"
	"casal/internal/goal"
	"casal/internal/household"
	"casal/internal/reminder"
	"casal/internal/routine"
	"casal/internal/task"
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

// goalPeople dá a Metas os membros do casal com o fuso de cada um.
type goalPeople struct{ households *household.Service }

func (p goalPeople) Members(ctx context.Context, householdID string) ([]goal.Person, error) {
	members, err := p.households.Members(ctx, householdID)
	if err != nil {
		return nil, err
	}
	out := make([]goal.Person, 0, len(members))
	for _, m := range members {
		out = append(out, goal.Person{ID: m.ID, Name: m.Name, Timezone: m.Timezone})
	}
	return out, nil
}

// goalReminders deixa Metas criar e cancelar lembretes (por ref) sem importar o pacote reminder.
type goalReminders struct{ reminders *reminder.Service }

func (r goalReminders) CreateWithRef(ctx context.Context, householdID, creatorID, recipientID, body string, remindAt time.Time, ref string) error {
	_, err := r.reminders.CreateWithRef(ctx, householdID, creatorID, recipientID, body, remindAt, ref)
	return err
}

func (r goalReminders) CancelByRef(ctx context.Context, householdID, ref string) error {
	_, err := r.reminders.CancelByRef(ctx, householdID, ref)
	return err
}

// goalTasks deixa Metas criar, cancelar e ler o estado das tarefas das etapas sem importar o pacote task.
type goalTasks struct{ tasks *task.Service }

func (t goalTasks) Create(ctx context.Context, householdID, creatorID, assigneeID, title string, dueAt *time.Time, ref string) (string, error) {
	made, err := t.tasks.CreateWithRef(ctx, householdID, creatorID, task.RefInput{Title: title, AssigneeID: assigneeID, DueAt: dueAt}, ref)
	if err != nil {
		return "", err
	}
	return made.ID, nil
}

func (t goalTasks) CancelByRef(ctx context.Context, householdID, ref string) error {
	_, err := t.tasks.CancelByRef(ctx, householdID, ref)
	return err
}

func (t goalTasks) UpdateByRef(ctx context.Context, householdID, ref, title string, dueAt *time.Time) error {
	return t.tasks.UpdateByRef(ctx, householdID, ref, title, dueAt)
}

func (t goalTasks) StatesByRef(ctx context.Context, householdID string, refs []string) (map[string]goal.TaskState, error) {
	states, err := t.tasks.StatesByRef(ctx, householdID, refs)
	if err != nil {
		return nil, err
	}
	out := make(map[string]goal.TaskState, len(states))
	for ref, st := range states {
		out[ref] = goal.TaskState{Status: st.Status, AssigneeID: st.AssigneeID, CompletedAt: st.CompletedAt}
	}
	return out, nil
}

// goalSpending deixa Metas ler o orçamento e o gasto do casal em Finanças sem importar o pacote finance.
type goalSpending struct{ finance *finance.Service }

func (s goalSpending) MonthlySpent(ctx context.Context, householdID, categoryID string, from, to time.Time) (int64, bool, map[string]int64, error) {
	return s.finance.CategoryMonthlySpend(ctx, householdID, categoryID, from, to)
}
