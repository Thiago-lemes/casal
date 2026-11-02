package note

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// fakeRepo guarda uma nota só, o bastante para exercitar as regras do service.
type fakeRepo struct {
	Repository
	note    Note
	updated int
}

func (f *fakeRepo) Create(_ context.Context, n Note) (Note, error) {
	n.ID, n.Version = "n1", 1
	f.note = n
	return n, nil
}

func (f *fakeRepo) Get(_ context.Context, _, _, userID string) (Note, error) {
	if f.note.ID == "" || (f.note.OwnerID != userID && f.note.Visibility == VisibilityPrivate) {
		return Note{}, ErrNotFound
	}
	return f.note, nil
}

func (f *fakeRepo) Update(_ context.Context, _, _ string, n Note, version int) (Note, error) {
	if version != f.note.Version {
		return Note{}, ErrNotFound
	}
	n.Version = version + 1
	f.note = n
	f.updated++
	return n, nil
}

func (f *fakeRepo) Delete(context.Context, string, string, string) error {
	f.note = Note{}
	return nil
}

func newService(n Note) (*Service, *fakeRepo) {
	repo := &fakeRepo{note: n}
	return NewService(repo), repo
}

func TestCreateDefaultsToPrivateAndCleansContent(t *testing.T) {
	svc, _ := newService(Note{})
	got, err := svc.Create(context.Background(), "h1", "u1", Content{
		Title: "  Mercado ", Kind: KindChecklist,
		Items: []Item{{Text: " leite "}, {Text: "pão", Done: true}},
		Tags:  []string{" Casa ", "casa", "COMPRAS"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Visibility != VisibilityPrivate || got.Title != "Mercado" || got.OwnerID != "u1" {
		t.Errorf("resultado inesperado: %+v", got)
	}
	if got.Items[0].Text != "leite" || len(got.Tags) != 2 || got.Tags[0] != "casa" {
		t.Errorf("itens ou etiquetas mal limpos: %+v %+v", got.Items, got.Tags)
	}
}

func TestCreateValidation(t *testing.T) {
	cases := []struct {
		name string
		c    Content
	}{
		{"sem título", Content{Title: "  ", Kind: KindText}},
		{"título longo", Content{Title: strings.Repeat("a", 201), Kind: KindText}},
		{"tipo inválido", Content{Title: "x", Kind: "imagem"}},
		{"sem tipo", Content{Title: "x"}},
		{"visibilidade inválida", Content{Title: "x", Kind: KindText, Visibility: "todos"}},
		{"texto com itens", Content{Title: "x", Kind: KindText, Items: []Item{{Text: "a"}}}},
		{"checklist com texto livre", Content{Title: "x", Kind: KindChecklist, Body: "solto"}},
		{"item vazio", Content{Title: "x", Kind: KindChecklist, Items: []Item{{Text: " "}}}},
		{"itens demais", Content{Title: "x", Kind: KindChecklist, Items: make([]Item, 201)}},
		{"etiqueta vazia", Content{Title: "x", Kind: KindText, Tags: []string{" "}}},
		{"etiquetas demais", Content{Title: "x", Kind: KindText, Tags: []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11"}}},
		{"texto longo", Content{Title: "x", Kind: KindText, Body: strings.Repeat("a", 20001)}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, repo := newService(Note{})
			if _, err := svc.Create(context.Background(), "h1", "u1", c.c); !errors.Is(err, ErrInvalidInput) {
				t.Errorf("esperava ErrInvalidInput, veio %v", err)
			}
			if repo.note.ID != "" {
				t.Error("não deveria ter gravado")
			}
		})
	}
}

func TestCanEdit(t *testing.T) {
	cases := []struct {
		visibility string
		user       string
		want       bool
	}{
		{VisibilityPrivate, "dono", true},
		{VisibilityView, "dono", true},
		{VisibilityHousehold, "dono", true},
		{VisibilityPrivate, "parceiro", false},
		{VisibilityView, "parceiro", false},
		{VisibilityHousehold, "parceiro", true},
	}
	for _, c := range cases {
		if got := CanEdit(Note{OwnerID: "dono", Visibility: c.visibility}, c.user); got != c.want {
			t.Errorf("CanEdit(%s, %s) = %v, queria %v", c.visibility, c.user, got, c.want)
		}
	}
}

func TestUpdatePermissions(t *testing.T) {
	base := Note{ID: "n1", OwnerID: "dono", Title: "t", Kind: KindText, Version: 1}
	edit := Content{Title: "novo", Body: "texto"}
	ctx := context.Background()

	t.Run("nota privada: o parceiro nem sabe que existe", func(t *testing.T) {
		n := base
		n.Visibility = VisibilityPrivate
		svc, _ := newService(n)
		if _, err := svc.Update(ctx, "h1", "n1", "parceiro", edit, 1); !errors.Is(err, ErrNotFound) {
			t.Errorf("esperava ErrNotFound, veio %v", err)
		}
	})
	t.Run("nota só de leitura: o parceiro vê mas não edita", func(t *testing.T) {
		n := base
		n.Visibility = VisibilityView
		svc, repo := newService(n)
		if _, err := svc.Update(ctx, "h1", "n1", "parceiro", edit, 1); !errors.Is(err, ErrForbidden) {
			t.Errorf("esperava ErrForbidden, veio %v", err)
		}
		if repo.updated != 0 {
			t.Error("não deveria ter gravado")
		}
	})
	t.Run("nota do casal: o parceiro edita mas não muda a visibilidade", func(t *testing.T) {
		n := base
		n.Visibility = VisibilityHousehold
		svc, _ := newService(n)
		if _, err := svc.Update(ctx, "h1", "n1", "parceiro", edit, 1); err != nil {
			t.Errorf("o parceiro deveria editar: %v", err)
		}
		hide := edit
		hide.Visibility = VisibilityPrivate
		if _, err := svc.Update(ctx, "h1", "n1", "parceiro", hide, 2); !errors.Is(err, ErrForbidden) {
			t.Errorf("o parceiro não pode esconder a nota: %v", err)
		}
	})
	t.Run("o dono muda a visibilidade", func(t *testing.T) {
		n := base
		n.Visibility = VisibilityPrivate
		svc, repo := newService(n)
		share := edit
		share.Visibility = VisibilityHousehold
		got, err := svc.Update(ctx, "h1", "n1", "dono", share, 1)
		if err != nil || got.Visibility != VisibilityHousehold || repo.note.Visibility != VisibilityHousehold {
			t.Errorf("visibilidade não mudou: %+v %v", got, err)
		}
	})
	t.Run("visibilidade vazia mantém a atual", func(t *testing.T) {
		n := base
		n.Visibility = VisibilityView
		svc, _ := newService(n)
		got, err := svc.Update(ctx, "h1", "n1", "dono", edit, 1)
		if err != nil || got.Visibility != VisibilityView {
			t.Errorf("deveria manter view: %+v %v", got, err)
		}
	})
}

func TestUpdateKeepsKindAndDetectsStaleVersion(t *testing.T) {
	n := Note{ID: "n1", OwnerID: "dono", Title: "t", Kind: KindChecklist, Visibility: VisibilityHousehold, Version: 3}
	svc, repo := newService(n)
	ctx := context.Background()

	// quem editou com a versão 2 não viu a 3: conflito, e nada é gravado
	if _, err := svc.Update(ctx, "h1", "n1", "dono", Content{Title: "x", Items: []Item{{Text: "a"}}}, 2); !errors.Is(err, ErrConflict) {
		t.Errorf("esperava ErrConflict, veio %v", err)
	}
	if repo.updated != 0 {
		t.Error("versão velha não pode gravar")
	}

	// o tipo não muda na edição: um texto livre numa checklist é recusado
	if _, err := svc.Update(ctx, "h1", "n1", "dono", Content{Title: "x", Body: "solto"}, 3); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("checklist não aceita texto livre: %v", err)
	}
	got, err := svc.Update(ctx, "h1", "n1", "dono", Content{Title: "x", Items: []Item{{Text: "a", Done: true}}}, 3)
	if err != nil || got.Version != 4 || got.Kind != KindChecklist {
		t.Errorf("edição válida falhou: %+v %v", got, err)
	}
}

func TestDeleteOnlyOwner(t *testing.T) {
	n := Note{ID: "n1", OwnerID: "dono", Title: "t", Kind: KindText, Visibility: VisibilityHousehold, Version: 1}
	svc, repo := newService(n)
	if err := svc.Delete(context.Background(), "h1", "n1", "parceiro"); !errors.Is(err, ErrForbidden) {
		t.Errorf("o parceiro não apaga, nem nota do casal: %v", err)
	}
	if repo.note.ID == "" {
		t.Fatal("a nota não deveria ter sido apagada")
	}
	if err := svc.Delete(context.Background(), "h1", "n1", "dono"); err != nil {
		t.Errorf("o dono deveria apagar: %v", err)
	}
}

func TestListRejectsHugeQuery(t *testing.T) {
	svc, _ := newService(Note{})
	if _, err := svc.List(context.Background(), "h1", "u1", strings.Repeat("a", 101), ""); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("busca longa demais deveria ser inválida: %v", err)
	}
}
