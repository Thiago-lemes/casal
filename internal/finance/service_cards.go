package finance

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"casal/internal/identity"
)

// Cartões são pessoais: toda consulta leva o dono (quem pergunta). O parceiro não lista, não lê e não usa o cartão
// de outra pessoa; o gasto do casal pago com o cartão de alguém aparece ao parceiro como qualquer outro.

type CardInput struct {
	Name       string
	Brand      string
	Last4      string
	ClosingDay *int
	DueDay     *int
}

func (in CardInput) card(p identity.Principal) (Card, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" || utf8.RuneCountInString(name) > maxCardName {
		return Card{}, fmt.Errorf("%w: o nome do cartão tem de 1 a %d caracteres", ErrInvalidInput, maxCardName)
	}
	brand, err := checkText(in.Brand, maxCardBrand, "a bandeira")
	if err != nil {
		return Card{}, err
	}
	last4 := strings.TrimSpace(in.Last4)
	if last4 != "" && (len(last4) != 4 || strings.Trim(last4, "0123456789") != "") {
		return Card{}, fmt.Errorf("%w: informe os 4 últimos dígitos ou deixe em branco", ErrInvalidInput)
	}
	for _, d := range []*int{in.ClosingDay, in.DueDay} {
		if d != nil && (*d < 1 || *d > 31) {
			return Card{}, fmt.Errorf("%w: o dia vai de 1 a 31", ErrInvalidInput)
		}
	}
	return Card{HouseholdID: p.HouseholdID, OwnerID: p.UserID, Name: name, Brand: brand, Last4: last4,
		ClosingDay: in.ClosingDay, DueDay: in.DueDay}, nil
}

func (s *Service) ListCards(ctx context.Context, p identity.Principal, archived bool) ([]Card, error) {
	return s.repo.ListCards(ctx, p.HouseholdID, p.UserID, archived)
}

func (s *Service) CreateCard(ctx context.Context, p identity.Principal, in CardInput) (Card, error) {
	c, err := in.card(p)
	if err != nil {
		return Card{}, err
	}
	return s.repo.CreateCard(ctx, c)
}

func (s *Service) UpdateCard(ctx context.Context, p identity.Principal, id string, in CardInput) (Card, error) {
	if !uuidRe.MatchString(id) {
		return Card{}, ErrCardNotFound
	}
	c, err := in.card(p)
	if err != nil {
		return Card{}, err
	}
	c.ID = id
	return s.repo.UpdateCard(ctx, c)
}

// ArchiveCard tira o cartão da lista de escolha sem mexer nos gastos antigos (cartão não se apaga).
func (s *Service) ArchiveCard(ctx context.Context, p identity.Principal, id string, archived bool) (Card, error) {
	if !uuidRe.MatchString(id) {
		return Card{}, ErrCardNotFound
	}
	return s.repo.SetCardArchived(ctx, p.HouseholdID, p.UserID, id, archived)
}

// usableCard confere que o cartão é de quem registra E que quem pagou é essa mesma pessoa (cartão é pessoal: o gasto
// que o parceiro pagou não pode usar o meu). Cartão arquivado só vale se já era o do gasto (keep).
func (s *Service) usableCard(ctx context.Context, p identity.Principal, paidBy, cardID, keep string) error {
	if cardID == "" {
		return nil
	}
	if !uuidRe.MatchString(cardID) {
		return ErrCardNotFound
	}
	c, err := s.repo.GetCard(ctx, p.HouseholdID, p.UserID, cardID) // cartão de outra pessoa: 404, como se não existisse
	if err != nil {
		return err
	}
	if paidBy != p.UserID {
		return ErrCardInvalid
	}
	if c.Archived && cardID != keep {
		return ErrCardArchived
	}
	return nil
}

// cardOnUpdate decide o cartão do gasto editado. Quem pagou é dono do cartão: se ele edita, o campo substitui (vazio
// tira o cartão). Se o parceiro edita o gasto, o cartão fica como está, a menos que o pagador mude (o cartão não
// acompanha outra pessoa).
func (s *Service) cardOnUpdate(ctx context.Context, p identity.Principal, cur Expense, nextPaidBy, wanted string) (string, error) {
	switch {
	case wanted != "":
		return wanted, s.usableCard(ctx, p, nextPaidBy, wanted, cur.CardID)
	case p.UserID == cur.PaidBy:
		return "", nil
	case nextPaidBy == cur.PaidBy:
		return cur.CardID, nil
	}
	return "", nil
}

// CardSummary é o total do mês num cartão de quem pergunta.
type CardSummary struct {
	CardID     string `json:"card_id"`
	TotalCents int64  `json:"total_cents"`
}
