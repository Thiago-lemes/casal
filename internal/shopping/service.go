package shopping

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	maxName  = 120
	maxUnit  = 10
	maxQty   = 100000
	boughtOn = 24 * time.Hour      // quanto tempo um item comprado continua visível na lista
	keepFor  = 30 * 24 * time.Hour // depois disso, o comprado é apagado (menos os recorrentes)
)

var categories = map[string]bool{
	"produce": true, "grocery": true, "cleaning": true, "butcher": true, "other": true,
}

type Service struct {
	repo Repository
	now  func() time.Time
}

func NewService(repo Repository) *Service { return &Service{repo: repo, now: time.Now} }

// Input é o que a pessoa escreve. Categoria vazia vira "other".
type Input struct {
	Name      string
	Quantity  *float64
	Unit      string
	Category  string
	Recurring bool
}

func (s *Service) Create(ctx context.Context, householdID, userID string, in Input) (Item, bool, error) {
	it, err := clean(in)
	if err != nil {
		return Item{}, false, err
	}
	it.HouseholdID, it.AddedBy, it.Source = householdID, userID, SourceManual
	return s.repo.Create(ctx, it)
}

// List devolve os itens abertos e os comprados nas últimas 24 horas.
func (s *Service) List(ctx context.Context, householdID string) ([]Item, error) {
	return s.repo.List(ctx, householdID, s.now().Add(-boughtOn))
}

func (s *Service) Update(ctx context.Context, householdID, id string, in Input) (Item, error) {
	it, err := clean(in)
	if err != nil {
		return Item{}, err
	}
	it.ID = id
	return s.repo.Update(ctx, householdID, it)
}

func (s *Service) MarkBought(ctx context.Context, householdID, id, userID string) (Item, error) {
	return s.repo.SetBought(ctx, householdID, id, userID, true, s.now())
}

// Unmark desfaz a compra (marcou sem querer). Se já há um igual aberto, volta ErrDuplicate.
func (s *Service) Unmark(ctx context.Context, householdID, id, userID string) (Item, error) {
	return s.repo.SetBought(ctx, householdID, id, userID, false, s.now())
}

func (s *Service) Delete(ctx context.Context, householdID, id string) error {
	return s.repo.Delete(ctx, householdID, id)
}

// Restock traz de volta os itens recorrentes que já foram comprados e diz quantos voltaram.
func (s *Service) Restock(ctx context.Context, householdID string) (int64, error) {
	return s.repo.Restock(ctx, householdID)
}

// PurgeBought apaga os itens comprados há mais de 30 dias, exceto os recorrentes. Rode periodicamente.
func (s *Service) PurgeBought(ctx context.Context) (int64, error) {
	return s.repo.PurgeBought(ctx, s.now().Add(-keepFor))
}

func clean(in Input) (Item, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" || utf8.RuneCountInString(name) > maxName {
		return Item{}, fmt.Errorf("%w: o nome é obrigatório (até %d caracteres)", ErrInvalidInput, maxName)
	}
	unit := strings.TrimSpace(in.Unit)
	if utf8.RuneCountInString(unit) > maxUnit {
		return Item{}, fmt.Errorf("%w: a unidade passa de %d caracteres", ErrInvalidInput, maxUnit)
	}
	if in.Quantity != nil && (*in.Quantity <= 0 || *in.Quantity > maxQty) {
		return Item{}, fmt.Errorf("%w: a quantidade deve ser maior que zero e até %d", ErrInvalidInput, maxQty)
	}
	if unit != "" && in.Quantity == nil {
		return Item{}, fmt.Errorf("%w: informe a quantidade junto da unidade", ErrInvalidInput)
	}
	category := in.Category
	if category == "" {
		category = "other"
	}
	if !categories[category] {
		return Item{}, fmt.Errorf("%w: categoria deve ser produce, grocery, cleaning, butcher ou other", ErrInvalidInput)
	}
	return Item{Name: name, Quantity: in.Quantity, Unit: unit, Category: category, Recurring: in.Recurring}, nil
}
