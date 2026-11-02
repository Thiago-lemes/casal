package note

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"casal/internal/apperr"
	"casal/internal/db"
)

var (
	ErrInvalidInput = apperr.New(apperr.Invalid, "dados inválidos")
	ErrNotFound     = apperr.NewCoded(apperr.NotFound, "note_not_found", "anotação não encontrada")
	ErrForbidden    = apperr.NewCoded(apperr.Forbidden, "note_forbidden", "você não pode alterar esta anotação")
	ErrConflict     = apperr.NewCoded(apperr.Conflict, "note_conflict", "a anotação foi alterada por outra pessoa; recarregue")
)

const (
	KindText      = "text"
	KindChecklist = "checklist"

	VisibilityPrivate   = "private"   // só o dono
	VisibilityView      = "view"      // o parceiro lê
	VisibilityHousehold = "household" // os dois leem e editam
)

type Item struct {
	Text string `json:"text"`
	Done bool   `json:"done"`
}

type Note struct {
	ID          string    `json:"id"`
	HouseholdID string    `json:"-"`
	OwnerID     string    `json:"owner_id"`
	OwnerName   string    `json:"owner_name"`
	Title       string    `json:"title"`
	Kind        string    `json:"kind"`
	Body        string    `json:"body"`
	Items       []Item    `json:"items"`
	Tags        []string  `json:"tags"`
	Visibility  string    `json:"visibility"`
	Pinned      bool      `json:"pinned"`
	Version     int       `json:"version"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Repository interface {
	Create(ctx context.Context, n Note) (Note, error)
	List(ctx context.Context, householdID, userID, query, tag string) ([]Note, error)
	Get(ctx context.Context, householdID, id, userID string) (Note, error)
	// Update só grava se a versão ainda for a informada e se a pessoa puder editar.
	Update(ctx context.Context, householdID, userID string, n Note, version int) (Note, error)
	Delete(ctx context.Context, householdID, id, userID string) error
}

type PostgresRepo struct{ db *sql.DB }

func NewPostgresRepo(db *sql.DB) *PostgresRepo { return &PostgresRepo{db: db} }

// Colunas lidas em todas as consultas. O JOIN em users é só leitura, para mostrar o nome do dono.
const cols = `n.id, n.household_id, n.owner_id, u.name, n.title, n.kind, n.body, n.items, n.tags,
	n.visibility, n.pinned, n.version, n.created_at, n.updated_at`

// Quem pode ler: o dono, ou o parceiro quando a nota foi compartilhada.
// param é o placeholder do userID na consulta que usa a condição ("$2", "$3"...).
func visibleTo(param string) string {
	return `(n.owner_id = ` + param + ` OR n.visibility IN ('view', 'household'))`
}

type scanner interface{ Scan(dest ...any) error }

func scan(s scanner) (Note, error) {
	var n Note
	var items, tags []byte
	err := s.Scan(&n.ID, &n.HouseholdID, &n.OwnerID, &n.OwnerName, &n.Title, &n.Kind, &n.Body,
		&items, &tags, &n.Visibility, &n.Pinned, &n.Version, &n.CreatedAt, &n.UpdatedAt)
	if err != nil {
		return Note{}, err
	}
	if err := json.Unmarshal(items, &n.Items); err != nil {
		return Note{}, err
	}
	if err := json.Unmarshal(tags, &n.Tags); err != nil {
		return Note{}, err
	}
	if n.Items == nil {
		n.Items = []Item{}
	}
	if n.Tags == nil {
		n.Tags = []string{}
	}
	return n, nil
}

// jsonArgs devolve items e tags já serializados; nunca nulos, para o CHECK de array passar.
func jsonArgs(n Note) (items, tags string, err error) {
	if n.Items == nil {
		n.Items = []Item{}
	}
	if n.Tags == nil {
		n.Tags = []string{}
	}
	i, err := json.Marshal(n.Items)
	if err != nil {
		return "", "", err
	}
	t, err := json.Marshal(n.Tags)
	if err != nil {
		return "", "", err
	}
	return string(i), string(t), nil
}

func (p *PostgresRepo) Create(ctx context.Context, in Note) (Note, error) {
	items, tags, err := jsonArgs(in)
	if err != nil {
		return Note{}, err
	}
	var out Note
	err = db.InTx(ctx, p.db, db.Scope{HouseholdID: in.HouseholdID}, func(tx *sql.Tx) error {
		var err error
		out, err = scan(tx.QueryRowContext(ctx, `
			WITH ins AS (
				INSERT INTO notes (household_id, owner_id, title, kind, body, items, tags, visibility, pinned)
				VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7::jsonb, $8, $9)
				RETURNING *
			)
			SELECT `+cols+` FROM ins n JOIN users u ON u.id = n.owner_id`,
			in.HouseholdID, in.OwnerID, in.Title, in.Kind, in.Body, items, tags, in.Visibility, in.Pinned))
		return err
	})
	return out, err
}

// List devolve as notas que a pessoa pode ler, fixadas primeiro e depois as mais recentes.
// query busca no título, no texto e nos itens; tag filtra por uma etiqueta exata.
func (p *PostgresRepo) List(ctx context.Context, householdID, userID, query, tag string) ([]Note, error) {
	out := []Note{}
	err := db.InTx(ctx, p.db, db.Scope{HouseholdID: householdID}, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
			SELECT `+cols+`
			FROM notes n JOIN users u ON u.id = n.owner_id
			WHERE n.household_id = $1 AND `+visibleTo("$2")+`
			  AND ($3 = '' OR n.title ILIKE $3 OR n.body ILIKE $3 OR n.items::text ILIKE $3)
			  AND ($4 = '' OR n.tags @> to_jsonb($4::text))
			ORDER BY n.pinned DESC, n.updated_at DESC
			LIMIT 200`, householdID, userID, likePattern(query), tag)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			n, err := scan(rows)
			if err != nil {
				return err
			}
			out = append(out, n)
		}
		return rows.Err()
	})
	return out, err
}

// likePattern prepara o termo para ILIKE, tratando %, _ e \ como texto comum. Vazio fica vazio.
func likePattern(q string) string {
	if q == "" {
		return ""
	}
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + r.Replace(q) + "%"
}

func (p *PostgresRepo) Get(ctx context.Context, householdID, id, userID string) (Note, error) {
	var out Note
	err := db.InTx(ctx, p.db, db.Scope{HouseholdID: householdID}, func(tx *sql.Tx) error {
		var err error
		out, err = scan(tx.QueryRowContext(ctx, `
			SELECT `+cols+`
			FROM notes n JOIN users u ON u.id = n.owner_id
			WHERE n.id = $1 AND n.household_id = $2 AND `+visibleTo("$3"),
			id, householdID, userID))
		return err
	})
	if errors.Is(err, sql.ErrNoRows) {
		return Note{}, ErrNotFound
	}
	return out, err
}

// Quem pode editar: o dono, ou os dois quando a nota é do casal. Só o dono muda a visibilidade
// (o CASE protege isso mesmo que o chamador se esqueça).
func (p *PostgresRepo) Update(ctx context.Context, householdID, userID string, in Note, version int) (Note, error) {
	items, tags, err := jsonArgs(in)
	if err != nil {
		return Note{}, err
	}
	var out Note
	err = db.InTx(ctx, p.db, db.Scope{HouseholdID: householdID}, func(tx *sql.Tx) error {
		var err error
		out, err = scan(tx.QueryRowContext(ctx, `
			WITH upd AS (
				UPDATE notes n SET title = $4, body = $5, items = $6::jsonb, tags = $7::jsonb,
					visibility = CASE WHEN n.owner_id = $3 THEN $8 ELSE n.visibility END,
					pinned = $9, version = n.version + 1, updated_at = now()
				WHERE n.id = $1 AND n.household_id = $2 AND n.version = $10
				  AND (n.owner_id = $3 OR n.visibility = 'household')
				RETURNING *
			)
			SELECT `+cols+` FROM upd n JOIN users u ON u.id = n.owner_id`,
			in.ID, householdID, userID, in.Title, in.Body, items, tags, in.Visibility, in.Pinned, version))
		return err
	})
	if errors.Is(err, sql.ErrNoRows) {
		return Note{}, ErrNotFound
	}
	return out, err
}

// Só o dono apaga, mesmo numa nota do casal.
func (p *PostgresRepo) Delete(ctx context.Context, householdID, id, userID string) error {
	return db.InTx(ctx, p.db, db.Scope{HouseholdID: householdID}, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `
			DELETE FROM notes WHERE id = $1 AND household_id = $2 AND owner_id = $3`,
			id, householdID, userID)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrNotFound
		}
		return nil
	})
}

// HandleLeave roda quando alguém sai do casal. As notas do casal passam para quem ficou; as que o parceiro
// só lia deixam de ser compartilhadas (viram privadas e acompanham quem saiu). Roda na transação da saída.
func (p *PostgresRepo) HandleLeave(ctx context.Context, tx *sql.Tx, h db.Handoff) error {
	if err := db.SetScope(ctx, tx, db.Scope{System: true}); err != nil {
		return err
	}
	if h.RemainingUserID != "" {
		if _, err := tx.ExecContext(ctx, `
			UPDATE notes SET owner_id = $3, version = version + 1
			WHERE household_id = $1 AND owner_id = $2 AND visibility = 'household'`,
			h.From, h.UserID, h.RemainingUserID); err != nil {
			return err
		}
	}
	_, err := tx.ExecContext(ctx, `
		UPDATE notes SET visibility = 'private', version = version + 1
		WHERE household_id = $1 AND owner_id = $2 AND visibility = 'view'`, h.From, h.UserID)
	return err
}

// HandleJoin roda quando a pessoa passa a ter um casal: as notas dela, deixadas para trás ao sair,
// vêm junto, sempre privadas (nada vira visível ao novo parceiro sem a pessoa escolher).
func (p *PostgresRepo) HandleJoin(ctx context.Context, tx *sql.Tx, h db.Handoff) error {
	if err := db.SetScope(ctx, tx, db.Scope{System: true}); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `
		UPDATE notes SET household_id = $2, visibility = 'private', version = version + 1
		WHERE owner_id = $1 AND household_id <> $2`, h.UserID, h.To)
	return err
}
