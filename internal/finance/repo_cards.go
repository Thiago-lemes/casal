package finance

import (
	"context"
	"database/sql"

	"casal/internal/db"
)

const cardCols = `c.id, c.household_id, c.owner_id, c.name, c.brand, c.last4, c.closing_day, c.due_day, c.archived_at IS NOT NULL`

func scanCard(s scanner) (Card, error) {
	var c Card
	var closing, due sql.NullInt32
	if err := s.Scan(&c.ID, &c.HouseholdID, &c.OwnerID, &c.Name, &c.Brand, &c.Last4, &closing, &due, &c.Archived); err != nil {
		return Card{}, err
	}
	if closing.Valid {
		v := int(closing.Int32)
		c.ClosingDay = &v
	}
	if due.Valid {
		v := int(due.Int32)
		c.DueDay = &v
	}
	return c, nil
}

func (p *PostgresRepo) oneCard(ctx context.Context, hh, query string, args ...any) (Card, error) {
	var out Card
	err := p.inHousehold(ctx, hh, func(tx *sql.Tx) error {
		var err error
		out, err = scanCard(tx.QueryRowContext(ctx, query, args...))
		return err
	})
	if isNoRows(err) {
		return Card{}, ErrCardNotFound
	}
	if db.IsUniqueViolation(err) {
		return Card{}, ErrCardDuplicate
	}
	return out, err
}

// ListCards devolve só os cartões do dono: o parceiro nunca lê os de outra pessoa.
func (p *PostgresRepo) ListCards(ctx context.Context, hh, owner string, archived bool) ([]Card, error) {
	out := []Card{}
	err := p.inHousehold(ctx, hh, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
			SELECT `+cardCols+` FROM finance_cards c
			WHERE c.household_id = $1 AND c.owner_id = $2 AND ($3 OR c.archived_at IS NULL)
			ORDER BY c.archived_at IS NOT NULL, c.created_at, c.name_key`, hh, owner, archived)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			c, err := scanCard(rows)
			if err != nil {
				return err
			}
			out = append(out, c)
		}
		return rows.Err()
	})
	return out, err
}

func (p *PostgresRepo) GetCard(ctx context.Context, hh, owner, id string) (Card, error) {
	return p.oneCard(ctx, hh, `
		SELECT `+cardCols+` FROM finance_cards c WHERE c.id = $1 AND c.household_id = $2 AND c.owner_id = $3`, id, hh, owner)
}

func (p *PostgresRepo) CreateCard(ctx context.Context, c Card) (Card, error) {
	return p.oneCard(ctx, c.HouseholdID, `
		WITH ins AS (
			INSERT INTO finance_cards (household_id, owner_id, name, brand, last4, closing_day, due_day)
			VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING *)
		SELECT `+cardCols+` FROM ins c`,
		c.HouseholdID, c.OwnerID, c.Name, c.Brand, c.Last4, intPtr(c.ClosingDay), intPtr(c.DueDay))
}

func (p *PostgresRepo) UpdateCard(ctx context.Context, c Card) (Card, error) {
	return p.oneCard(ctx, c.HouseholdID, `
		WITH upd AS (
			UPDATE finance_cards SET name = $4, brand = $5, last4 = $6, closing_day = $7, due_day = $8
			WHERE id = $1 AND household_id = $2 AND owner_id = $3 RETURNING *)
		SELECT `+cardCols+` FROM upd c`,
		c.ID, c.HouseholdID, c.OwnerID, c.Name, c.Brand, c.Last4, intPtr(c.ClosingDay), intPtr(c.DueDay))
}

func (p *PostgresRepo) SetCardArchived(ctx context.Context, hh, owner, id string, archived bool) (Card, error) {
	return p.oneCard(ctx, hh, `
		WITH upd AS (
			UPDATE finance_cards SET archived_at = CASE WHEN $4 THEN coalesce(archived_at, now()) ELSE NULL END
			WHERE id = $1 AND household_id = $2 AND owner_id = $3 RETURNING *)
		SELECT `+cardCols+` FROM upd c`, id, hh, owner, archived)
}

func intPtr(v *int) any {
	if v == nil {
		return nil
	}
	return *v
}

// CardTotals: em "mine" soma só os gastos pessoais de quem pergunta; no resto, os do casal. Só entram os gastos sem
// cartão e os pagos por quem pergunta com um cartão dele; o cartão do parceiro não aparece em lugar nenhum.
func (p *PostgresRepo) CardTotals(ctx context.Context, hh, viewer, scope, month string) (map[string]int64, int64, error) {
	byCard := map[string]int64{}
	var without int64
	err := p.inHousehold(ctx, hh, func(tx *sql.Tx) error {
		cond := `e.scope = 'household'`
		if scope == "mine" {
			cond = `e.scope = 'personal' AND e.created_by = $3`
		}
		rows, err := tx.QueryContext(ctx, `
			SELECT e.card_id, sum(e.amount_cents) FROM expenses e
			WHERE e.household_id = $1 AND `+cond+`
			  AND (e.card_id IS NULL OR e.paid_by = $3)
			  AND e.spent_on >= ($2||'-01')::date AND e.spent_on < ($2||'-01')::date + interval '1 month'
			GROUP BY e.card_id`, hh, month, viewer)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id sql.NullString
			var total int64
			if err := rows.Scan(&id, &total); err != nil {
				return err
			}
			if id.Valid {
				byCard[id.String] = total
			} else {
				without = total
			}
		}
		return rows.Err()
	})
	return byCard, without, err
}
