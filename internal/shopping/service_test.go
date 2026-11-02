package shopping

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type fakeRepo struct {
	Repository
	created   []Item
	boughtCut time.Time
	purgeCut  time.Time
}

func (f *fakeRepo) Create(_ context.Context, it Item) (Item, bool, error) {
	it.ID = "i1"
	f.created = append(f.created, it)
	return it, false, nil
}

func (f *fakeRepo) List(_ context.Context, _ string, since time.Time) ([]Item, error) {
	f.boughtCut = since
	return []Item{}, nil
}

func (f *fakeRepo) PurgeBought(_ context.Context, cutoff time.Time) (int64, error) {
	f.purgeCut = cutoff
	return 0, nil
}

var fixedNow = time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)

func newService() (*Service, *fakeRepo) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	svc.now = func() time.Time { return fixedNow }
	return svc, repo
}

func qty(v float64) *float64 { return &v }

func TestCreateCleansAndDefaults(t *testing.T) {
	svc, repo := newService()
	if _, _, err := svc.Create(context.Background(), "h1", "u1", Input{Name: "  Leite  ", Quantity: qty(2), Unit: " L "}); err != nil {
		t.Fatal(err)
	}
	got := repo.created[0]
	if got.Name != "Leite" || got.Unit != "L" || got.Category != "other" || got.Source != SourceManual ||
		got.AddedBy != "u1" || got.HouseholdID != "h1" || *got.Quantity != 2 {
		t.Errorf("resultado inesperado: %+v", got)
	}
	// sem quantidade nem unidade também vale ("pão")
	if _, _, err := svc.Create(context.Background(), "h1", "u1", Input{Name: "pão", Category: "grocery"}); err != nil {
		t.Errorf("item sem quantidade deveria valer: %v", err)
	}
}

func TestCreateValidation(t *testing.T) {
	cases := []struct {
		name string
		in   Input
	}{
		{"sem nome", Input{Name: "  "}},
		{"nome longo", Input{Name: strings.Repeat("a", 121)}},
		{"quantidade zero", Input{Name: "x", Quantity: qty(0)}},
		{"quantidade negativa", Input{Name: "x", Quantity: qty(-1)}},
		{"quantidade enorme", Input{Name: "x", Quantity: qty(100001)}},
		{"unidade longa", Input{Name: "x", Quantity: qty(1), Unit: "quilogramas"}},
		{"unidade sem quantidade", Input{Name: "x", Unit: "kg"}},
		{"categoria inválida", Input{Name: "x", Category: "bebidas"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, repo := newService()
			if _, _, err := svc.Create(context.Background(), "h1", "u1", c.in); !errors.Is(err, ErrInvalidInput) {
				t.Errorf("esperava ErrInvalidInput, veio %v", err)
			}
			if len(repo.created) != 0 {
				t.Error("não deveria ter gravado")
			}
		})
	}
}

func TestListShowsBoughtForOneDay(t *testing.T) {
	svc, repo := newService()
	if _, err := svc.List(context.Background(), "h1"); err != nil {
		t.Fatal(err)
	}
	if want := fixedNow.Add(-24 * time.Hour); !repo.boughtCut.Equal(want) {
		t.Errorf("comprados valem desde %v, queria %v", repo.boughtCut, want)
	}
}

func TestPurgeKeepsThirtyDays(t *testing.T) {
	svc, repo := newService()
	if _, err := svc.PurgeBought(context.Background()); err != nil {
		t.Fatal(err)
	}
	if want := fixedNow.Add(-30 * 24 * time.Hour); !repo.purgeCut.Equal(want) {
		t.Errorf("limpeza corta em %v, queria %v", repo.purgeCut, want)
	}
}
