package note

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	maxTitle    = 200
	maxBody     = 20000
	maxItems    = 200
	maxItemText = 300
	maxTags     = 10
	maxTagLen   = 30
	maxQuery    = 100
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service { return &Service{repo: repo} }

// Content é o que a pessoa escreve. Visibility vazia significa "não mudar" (na criação, privada).
type Content struct {
	Title      string
	Kind       string // só na criação; depois a nota não muda de tipo
	Body       string
	Items      []Item
	Tags       []string
	Visibility string
	Pinned     bool
}

func (s *Service) Create(ctx context.Context, householdID, userID string, c Content) (Note, error) {
	if c.Visibility == "" {
		c.Visibility = VisibilityPrivate
	}
	c, err := normalize(c, c.Kind)
	if err != nil {
		return Note{}, err
	}
	return s.repo.Create(ctx, Note{
		HouseholdID: householdID, OwnerID: userID,
		Title: c.Title, Kind: c.Kind, Body: c.Body, Items: c.Items, Tags: c.Tags,
		Visibility: c.Visibility, Pinned: c.Pinned,
	})
}

func (s *Service) List(ctx context.Context, householdID, userID, query, tag string) ([]Note, error) {
	query = strings.TrimSpace(query)
	tag = strings.ToLower(strings.TrimSpace(tag))
	if utf8.RuneCountInString(query) > maxQuery || utf8.RuneCountInString(tag) > maxTagLen {
		return nil, fmt.Errorf("%w: a busca é longa demais", ErrInvalidInput)
	}
	return s.repo.List(ctx, householdID, userID, query, tag)
}

func (s *Service) Get(ctx context.Context, householdID, id, userID string) (Note, error) {
	return s.repo.Get(ctx, householdID, id, userID)
}

// Update substitui o conteúdo da nota. version é a que a pessoa viu; se outra pessoa gravou
// antes, volta ErrConflict e nada é perdido.
func (s *Service) Update(ctx context.Context, householdID, id, userID string, c Content, version int) (Note, error) {
	cur, err := s.repo.Get(ctx, householdID, id, userID)
	if err != nil {
		return Note{}, err // quem não pode ler a nota nem sabe que ela existe
	}
	if !CanEdit(cur, userID) {
		return Note{}, ErrForbidden
	}
	if c.Visibility == "" {
		c.Visibility = cur.Visibility
	}
	if c.Visibility != cur.Visibility && cur.OwnerID != userID {
		return Note{}, ErrForbidden // só o dono muda quem vê
	}
	c, err = normalize(c, cur.Kind)
	if err != nil {
		return Note{}, err
	}
	if version != cur.Version {
		return Note{}, ErrConflict
	}
	cur.Title, cur.Body, cur.Items, cur.Tags = c.Title, c.Body, c.Items, c.Tags
	cur.Visibility, cur.Pinned = c.Visibility, c.Pinned
	out, err := s.repo.Update(ctx, householdID, userID, cur, version)
	if errors.Is(err, ErrNotFound) {
		return Note{}, ErrConflict // existia e era editável um instante atrás: alguém gravou no meio
	}
	return out, err
}

// Delete: só o dono apaga. Quem pode ler mas não é o dono recebe ErrForbidden.
func (s *Service) Delete(ctx context.Context, householdID, id, userID string) error {
	cur, err := s.repo.Get(ctx, householdID, id, userID)
	if err != nil {
		return err
	}
	if cur.OwnerID != userID {
		return ErrForbidden
	}
	return s.repo.Delete(ctx, householdID, id, userID)
}

// CanEdit: o dono edita sempre; o parceiro só nas notas do casal.
func CanEdit(n Note, userID string) bool {
	return n.OwnerID == userID || n.Visibility == VisibilityHousehold
}

// normalize valida e limpa o conteúdo conforme o tipo da nota.
func normalize(c Content, kind string) (Content, error) {
	c.Title = strings.TrimSpace(c.Title)
	if c.Title == "" || utf8.RuneCountInString(c.Title) > maxTitle {
		return c, fmt.Errorf("%w: o título é obrigatório (até %d caracteres)", ErrInvalidInput, maxTitle)
	}
	switch c.Visibility {
	case VisibilityPrivate, VisibilityView, VisibilityHousehold:
	default:
		return c, fmt.Errorf("%w: visibilidade deve ser private, view ou household", ErrInvalidInput)
	}

	switch kind {
	case KindText:
		if len(c.Items) > 0 {
			return c, fmt.Errorf("%w: nota de texto não tem itens", ErrInvalidInput)
		}
		if utf8.RuneCountInString(c.Body) > maxBody {
			return c, fmt.Errorf("%w: o texto passa de %d caracteres", ErrInvalidInput, maxBody)
		}
		c.Items = []Item{}
	case KindChecklist:
		if strings.TrimSpace(c.Body) != "" {
			return c, fmt.Errorf("%w: checklist não tem texto livre", ErrInvalidInput)
		}
		if len(c.Items) > maxItems {
			return c, fmt.Errorf("%w: no máximo %d itens", ErrInvalidInput, maxItems)
		}
		items := make([]Item, 0, len(c.Items))
		for _, it := range c.Items {
			it.Text = strings.TrimSpace(it.Text)
			if it.Text == "" || utf8.RuneCountInString(it.Text) > maxItemText {
				return c, fmt.Errorf("%w: cada item precisa de texto (até %d caracteres)", ErrInvalidInput, maxItemText)
			}
			items = append(items, it)
		}
		c.Items, c.Body = items, ""
	default:
		return c, fmt.Errorf("%w: o tipo deve ser text ou checklist", ErrInvalidInput)
	}
	c.Kind = kind

	tags, err := cleanTags(c.Tags)
	if err != nil {
		return c, err
	}
	c.Tags = tags
	return c, nil
}

// cleanTags põe em minúsculas, tira espaços e repetições e confere os limites.
func cleanTags(in []string) ([]string, error) {
	out := make([]string, 0, len(in))
	seen := map[string]bool{}
	for _, t := range in {
		t = strings.ToLower(strings.TrimSpace(t))
		if t == "" || utf8.RuneCountInString(t) > maxTagLen {
			return nil, fmt.Errorf("%w: cada etiqueta tem de 1 a %d caracteres", ErrInvalidInput, maxTagLen)
		}
		if !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	if len(out) > maxTags {
		return nil, fmt.Errorf("%w: no máximo %d etiquetas", ErrInvalidInput, maxTags)
	}
	return out, nil
}
