package app

import (
	"context"
	"errors"
	"time"

	"casal/internal/admin"
	"casal/internal/module"
	"casal/internal/ops"
	"casal/internal/signup"
)

// Os adaptadores abaixo ligam o painel de suporte às features sem que ele as importe: o admin declara o que precisa e
// só o app conhece os dois lados.

// adminDirectory entrega ao painel os metadados do ops.Inspector (contagens, nunca conteúdo).
type adminDirectory struct{ in *ops.Inspector }

func (d adminDirectory) SearchHouseholds(ctx context.Context, q string) ([]admin.HouseholdSummary, error) {
	found, err := d.in.SearchHouseholds(ctx, q)
	if err != nil {
		return nil, err
	}
	out := make([]admin.HouseholdSummary, 0, len(found))
	for _, h := range found {
		out = append(out, summary(h))
	}
	return out, nil
}

func summary(h ops.HouseholdSummary) admin.HouseholdSummary {
	s := admin.HouseholdSummary{ID: h.ID, CreatedAt: h.CreatedAt, Members: make([]admin.Person, 0, len(h.Members))}
	for _, m := range h.Members {
		s.Members = append(s.Members, admin.Person(m))
	}
	return s
}

func (d adminDirectory) Household(ctx context.Context, id string) (admin.HouseholdDetail, error) {
	h, err := d.in.Household(ctx, id)
	if err != nil {
		return admin.HouseholdDetail{}, adminErr(err)
	}
	return admin.HouseholdDetail{
		HouseholdSummary: summary(h.HouseholdSummary),
		Modules:          h.Modules, LastUsedAt: h.LastUsedAt, PushDevices: h.PushDevices, Counts: h.Counts,
	}, nil
}

func (d adminDirectory) User(ctx context.Context, id string) (admin.UserDetail, error) {
	u, err := d.in.User(ctx, id)
	if err != nil {
		return admin.UserDetail{}, adminErr(err)
	}
	return admin.UserDetail{
		Person: admin.Person(u.Person), CreatedAt: u.CreatedAt, BlockedAt: u.BlockedAt,
		HouseholdID: u.HouseholdID, LastUsedAt: u.LastUsedAt, PushDevices: u.PushDevices,
	}, nil
}

func adminErr(err error) error {
	if errors.Is(err, ops.ErrNotFound) {
		return admin.ErrNotFound
	}
	return err
}

// adminModules liga e desliga módulos pelo module.Service: os mesmos ganchos e regras de quando o casal faz.
type adminModules struct{ modules *module.Service }

func (m adminModules) List(ctx context.Context, householdID string) ([]admin.ModuleState, error) {
	list, err := m.modules.List(ctx, householdID)
	if err != nil {
		return nil, err
	}
	out := make([]admin.ModuleState, 0, len(list))
	for _, s := range list {
		out = append(out, admin.ModuleState{Name: s.Name, Enabled: s.Enabled})
	}
	return out, nil
}

func (m adminModules) Set(ctx context.Context, householdID, name string, enabled bool) error {
	_, err := m.modules.Set(ctx, householdID, name, enabled)
	return err
}

// adminCodes administra os códigos de cadastro do beta.
type adminCodes struct{ codes *signup.Service }

func (c adminCodes) Create(ctx context.Context, note string, maxUses int, ttl time.Duration, createdBy string) (admin.SignupCode, string, error) {
	code, raw, err := c.codes.Create(ctx, note, maxUses, ttl, createdBy)
	return admin.SignupCode(code), raw, err
}

func (c adminCodes) List(ctx context.Context) ([]admin.SignupCode, error) {
	list, err := c.codes.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]admin.SignupCode, 0, len(list))
	for _, code := range list {
		out = append(out, admin.SignupCode(code))
	}
	return out, nil
}

func (c adminCodes) Revoke(ctx context.Context, id string) error { return c.codes.Revoke(ctx, id) }
