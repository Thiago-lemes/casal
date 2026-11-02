package app

import (
	"context"

	"casal/internal/household"
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
