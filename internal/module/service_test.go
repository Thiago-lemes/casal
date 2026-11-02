package module

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"
)

type fakeRepo struct {
	on    map[string]time.Time
	hooks int
}

func (f *fakeRepo) Enabled(context.Context, string) (map[string]time.Time, error) { return f.on, nil }
func (f *fakeRepo) IsEnabled(_ context.Context, _, name string) (bool, error) {
	_, ok := f.on[name]
	return ok, nil
}
func (f *fakeRepo) Enable(ctx context.Context, hh, name string, hooks ...Hook) (bool, error) {
	if _, ok := f.on[name]; ok {
		return false, nil
	}
	for _, h := range hooks {
		if err := h(ctx, nil, hh); err != nil {
			return false, err
		}
		f.hooks++
	}
	f.on[name] = time.Now()
	return true, nil
}
func (f *fakeRepo) Disable(_ context.Context, _, name string) error {
	delete(f.on, name)
	return nil
}

func TestListShowsKnownModulesOffByDefault(t *testing.T) {
	svc := NewService(&fakeRepo{on: map[string]time.Time{}})
	got, err := svc.List(context.Background(), "h1")
	if err != nil || len(got) != 2 || got[0].Name != Finance || got[1].Name != Goals {
		t.Fatalf("módulos conhecidos: %+v %v", got, err)
	}
	for _, m := range got {
		if m.Enabled || m.EnabledAt != nil {
			t.Errorf("sem linha, o módulo é desligado: %+v", m)
		}
	}
}

func TestSetEnablesRunsHookOnceAndDisables(t *testing.T) {
	repo := &fakeRepo{on: map[string]time.Time{}}
	svc := NewService(repo)
	svc.OnEnable(Finance, func(context.Context, *sql.Tx, string) error { return nil })
	ctx := context.Background()

	st, err := svc.Set(ctx, "h1", Finance, true)
	if err != nil || !st.Enabled || st.EnabledAt == nil {
		t.Fatalf("ligar: %+v %v", st, err)
	}
	if _, err := svc.Set(ctx, "h1", Finance, true); err != nil {
		t.Fatal(err)
	}
	if repo.hooks != 1 {
		t.Errorf("o gancho deveria rodar só na passagem de desligado a ligado, rodou %d", repo.hooks)
	}
	st, err = svc.Set(ctx, "h1", Finance, false)
	if err != nil || st.Enabled {
		t.Errorf("desligar: %+v %v", st, err)
	}
	// religar roda o gancho de novo: ele precisa ser idempotente
	if _, err := svc.Set(ctx, "h1", Finance, true); err != nil || repo.hooks != 2 {
		t.Errorf("religar: hooks=%d %v", repo.hooks, err)
	}
}

func TestSetRejectsUnknownModule(t *testing.T) {
	svc := NewService(&fakeRepo{on: map[string]time.Time{}})
	if _, err := svc.Set(context.Background(), "h1", "marmita", true); !errors.Is(err, ErrNotFound) {
		t.Errorf("módulo que não existe: %v", err)
	}
}

func TestHookFailureKeepsModuleOff(t *testing.T) {
	repo := &fakeRepo{on: map[string]time.Time{}}
	svc := NewService(repo)
	boom := errors.New("falhou")
	svc.OnEnable(Finance, func(context.Context, *sql.Tx, string) error { return boom })
	if _, err := svc.Set(context.Background(), "h1", Finance, true); !errors.Is(err, boom) {
		t.Fatalf("o erro do gancho deveria subir: %v", err)
	}
	if on, _ := repo.IsEnabled(context.Background(), "h1", Finance); on {
		t.Error("com o gancho falhando, o módulo não pode ficar ligado")
	}
}
