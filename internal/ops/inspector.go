package ops

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"casal/internal/apperr"
	"casal/internal/db"
)

// ErrNotFound é devolvido quando o casal ou a pessoa não existe.
var ErrNotFound = apperr.New(apperr.NotFound, "não encontrado")

// Person é uma pessoa, com o que o suporte precisa para localizar a conta.
type Person struct{ ID, Email, Name string }

// HouseholdSummary é um casal nos resultados da busca.
type HouseholdSummary struct {
	ID        string
	CreatedAt time.Time
	Members   []Person
}

// HouseholdDetail é a ficha do casal: metadados e CONTAGENS. Nenhuma coluna de conteúdo é lida aqui (título, texto,
// valor, descrição); só count(*). A contagem é o total do módulo, sem distinguir item pessoal de item do casal.
type HouseholdDetail struct {
	HouseholdSummary
	Modules     []string
	LastUsedAt  *time.Time
	PushDevices int
	Counts      map[string]int
}

// UserDetail é a ficha de uma pessoa.
type UserDetail struct {
	Person
	CreatedAt   time.Time
	BlockedAt   *time.Time
	HouseholdID *string
	LastUsedAt  *time.Time
	PushDevices int
}

// Inspector lê os metadados que o painel de suporte mostra. Roda no escopo de sistema (atravessa casais) e por isso é
// o único lugar que faz essas leituras: o painel só enxerga o que está nas estruturas acima.
type Inspector struct{ db *sql.DB }

func NewInspector(conn *sql.DB) *Inspector { return &Inspector{db: conn} }

const maxSearchResults = 20

// escapeLike protege os curingas do LIKE: quem busca por "50%" procura o texto, não "50 e qualquer coisa".
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

func (i *Inspector) SearchHouseholds(ctx context.Context, q string) ([]HouseholdSummary, error) {
	pattern := "%" + escapeLike(strings.TrimSpace(q)) + "%"
	var out []HouseholdSummary
	err := db.InTx(ctx, i.db, db.Scope{System: true}, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
			SELECT h.id, h.created_at, u.id, u.email, u.name
			FROM households h
			JOIN household_members m ON m.household_id = h.id
			JOIN users u ON u.id = m.user_id
			WHERE h.id IN (
				SELECT m2.household_id FROM household_members m2 JOIN users u2 ON u2.id = m2.user_id
				WHERE u2.email ILIKE $1 ESCAPE '\' OR u2.name ILIKE $1 ESCAPE '\'
				LIMIT $2)
			ORDER BY h.created_at DESC, h.id, u.name`, pattern, maxSearchResults)
		if err != nil {
			return err
		}
		defer rows.Close()
		index := map[string]int{}
		for rows.Next() {
			var hid string
			var created time.Time
			var p Person
			if err := rows.Scan(&hid, &created, &p.ID, &p.Email, &p.Name); err != nil {
				return err
			}
			at, ok := index[hid]
			if !ok {
				out = append(out, HouseholdSummary{ID: hid, CreatedAt: created})
				at = len(out) - 1
				index[hid] = at
			}
			out[at].Members = append(out[at].Members, p)
		}
		return rows.Err()
	})
	return out, err
}

// Household devolve a ficha do casal (ErrNotFound se não existe).
func (i *Inspector) Household(ctx context.Context, id string) (HouseholdDetail, error) {
	var d HouseholdDetail
	d.ID = id
	d.Counts = map[string]int{}
	err := db.InTx(ctx, i.db, db.Scope{System: true}, func(tx *sql.Tx) error {
		var counts [7]int
		var last sql.NullTime
		err := tx.QueryRowContext(ctx, `
			SELECT h.created_at,
			  (SELECT count(*) FROM push_subscriptions p JOIN household_members m ON m.user_id = p.user_id WHERE m.household_id = h.id),
			  (SELECT max(coalesce(s.last_used_at, a.day::timestamptz))
			     FROM household_members m
			     LEFT JOIN sessions s ON s.user_id = m.user_id
			     LEFT JOIN user_activity a ON a.user_id = m.user_id
			    WHERE m.household_id = h.id),
			  (SELECT count(*) FROM reminders WHERE household_id = h.id),
			  (SELECT count(*) FROM tasks WHERE household_id = h.id),
			  (SELECT count(*) FROM notes WHERE household_id = h.id),
			  (SELECT count(*) FROM shopping_items WHERE household_id = h.id),
			  (SELECT count(*) FROM routine_blocks WHERE household_id = h.id),
			  (SELECT count(*) FROM expenses WHERE household_id = h.id),
			  (SELECT count(*) FROM goals WHERE household_id = h.id)
			FROM households h WHERE h.id = $1`, id).
			Scan(&d.CreatedAt, &d.PushDevices, &last,
				&counts[0], &counts[1], &counts[2], &counts[3], &counts[4], &counts[5], &counts[6])
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if last.Valid {
			d.LastUsedAt = &last.Time
		}
		for k, name := range []string{"reminders", "tasks", "notes", "shopping", "routine", "finance", "goals"} {
			d.Counts[name] = counts[k]
		}

		rows, err := tx.QueryContext(ctx, `
			SELECT u.id, u.email, u.name FROM household_members m JOIN users u ON u.id = m.user_id
			WHERE m.household_id = $1 ORDER BY m.joined_at`, id)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var p Person
			if err := rows.Scan(&p.ID, &p.Email, &p.Name); err != nil {
				return err
			}
			d.Members = append(d.Members, p)
		}
		if err := rows.Err(); err != nil {
			return err
		}

		mods, err := tx.QueryContext(ctx, `SELECT module FROM household_modules WHERE household_id = $1 ORDER BY module`, id)
		if err != nil {
			return err
		}
		defer mods.Close()
		d.Modules = []string{}
		for mods.Next() {
			var m string
			if err := mods.Scan(&m); err != nil {
				return err
			}
			d.Modules = append(d.Modules, m)
		}
		return mods.Err()
	})
	return d, err
}

// User devolve a ficha da pessoa (ErrNotFound se não existe).
func (i *Inspector) User(ctx context.Context, id string) (UserDetail, error) {
	var u UserDetail
	err := db.InTx(ctx, i.db, db.Scope{System: true}, func(tx *sql.Tx) error {
		var last sql.NullTime
		err := tx.QueryRowContext(ctx, `
			SELECT u.id, u.email, u.name, u.created_at, u.blocked_at,
			  (SELECT m.household_id FROM household_members m WHERE m.user_id = u.id),
			  (SELECT max(coalesce(s.last_used_at, a.day::timestamptz))
			     FROM (SELECT 1) x
			     LEFT JOIN sessions s ON s.user_id = u.id
			     LEFT JOIN user_activity a ON a.user_id = u.id),
			  (SELECT count(*) FROM push_subscriptions p WHERE p.user_id = u.id)
			FROM users u WHERE u.id = $1`, id).
			Scan(&u.ID, &u.Email, &u.Name, &u.CreatedAt, &u.BlockedAt, &u.HouseholdID, &last, &u.PushDevices)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if last.Valid {
			u.LastUsedAt = &last.Time
		}
		return err
	})
	return u, err
}
