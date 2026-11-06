package finance

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

const expCols = `e.id, e.household_id, e.created_by, e.updated_by, e.paid_by, e.scope, e.visibility, e.amount_cents,
	e.category_id, to_char(e.spent_on, 'YYYY-MM-DD'), e.description, e.split, e.source, e.recurring_id, e.bill_id,
	e.installment_group_id, e.installment_no, e.installments_total, e.settlement_id, e.version, e.created_at, e.updated_at,
	e.card_id`

// visibleTo é o filtro de privacidade que TODA leitura de gasto usa: o do casal é dos dois; o pessoal é de quem o
// criou, e do parceiro só se estiver em "view". O RLS não separa os parceiros, então só este filtro faz isso.
// n é a posição do parâmetro com o id de quem pergunta.
func visibleTo(n int) string {
	return fmt.Sprintf(`(e.scope = 'household' OR e.created_by = $%d OR e.visibility = 'view')`, n)
}

func scanExpense(s scanner, viewer string) (Expense, error) {
	var e Expense
	var updatedBy, recurring, bill, group, settlement, card sql.NullString
	var no, total sql.NullInt32
	var split []byte
	if err := s.Scan(&e.ID, &e.HouseholdID, &e.CreatedBy, &updatedBy, &e.PaidBy, &e.Scope, &e.Visibility,
		&e.AmountCents, &e.CategoryID, &e.SpentOn, &e.Description, &split, &e.Source, &recurring, &bill,
		&group, &no, &total, &settlement, &e.Version, &e.CreatedAt, &e.UpdatedAt, &card); err != nil {
		return Expense{}, err
	}
	e.UpdatedBy, e.RecurringID, e.BillID, e.SettlementID = updatedBy.String, recurring.String, bill.String, settlement.String
	e.Settled = settlement.Valid
	e.CardID = card.String
	if viewer != "" && viewer == e.PaidBy { // o cartão é de quem pagou: o parceiro não o vê
		e.MyCardID = e.CardID
	}
	e.Split = parseSplit(split)
	if group.Valid {
		e.Installment = &Installment{GroupID: group.String, No: int(no.Int32), Total: int(total.Int32)}
	}
	e.CanEdit = !e.Settled && (e.Scope == ScopeHousehold || e.CreatedBy == viewer)
	return e, nil
}

func queryExpenses(ctx context.Context, tx *sql.Tx, viewer, query string, args ...any) ([]Expense, error) {
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Expense{}
	for rows.Next() {
		e, err := scanExpense(rows, viewer)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func insertExpense(ctx context.Context, tx *sql.Tx, e Expense) (Expense, error) {
	var group, no, total any
	if e.Installment != nil {
		group, no, total = e.Installment.GroupID, e.Installment.No, e.Installment.Total
	}
	return scanExpense(tx.QueryRowContext(ctx, `
		WITH ins AS (
			INSERT INTO expenses (household_id, created_by, paid_by, scope, visibility, amount_cents, category_id, spent_on,
				description, split, source, recurring_id, bill_id, installment_group_id, installment_no, installments_total, card_id)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8::date, $9, $10::jsonb, $11, $12::uuid, $13::uuid, $14::uuid, $15, $16, $17::uuid)
			RETURNING *
		)
		SELECT `+expCols+` FROM ins e`,
		e.HouseholdID, e.CreatedBy, e.PaidBy, e.Scope, e.Visibility, e.AmountCents, e.CategoryID, e.SpentOn,
		e.Description, jsonArg(e.Split), e.Source, nullable(e.RecurringID), nullable(e.BillID), group, no, total, nullable(e.CardID)), e.CreatedBy)
}

// InsertExpenses grava todos os gastos numa transação (as parcelas de uma compra: ou todas, ou nenhuma).
func (p *PostgresRepo) InsertExpenses(ctx context.Context, hh string, es []Expense) ([]Expense, error) {
	out := make([]Expense, 0, len(es))
	err := p.inHousehold(ctx, hh, func(tx *sql.Tx) error {
		for _, e := range es {
			e.HouseholdID = hh
			got, err := insertExpense(ctx, tx, e)
			if err != nil {
				return err
			}
			out = append(out, got)
		}
		return nil
	})
	return out, err
}

func (p *PostgresRepo) GetExpense(ctx context.Context, hh, id, viewer string) (Expense, error) {
	var out Expense
	err := p.inHousehold(ctx, hh, func(tx *sql.Tx) error {
		var err error
		out, err = scanExpense(tx.QueryRowContext(ctx, `
			SELECT `+expCols+` FROM expenses e
			WHERE e.id = $1 AND e.household_id = $2 AND `+visibleTo(3), id, hh, viewer), viewer)
		return err
	})
	if isNoRows(err) {
		return Expense{}, ErrExpenseNotFound
	}
	return out, err
}

func (p *PostgresRepo) ListExpenses(ctx context.Context, hh, viewer string, f ExpenseFilter) ([]Expense, error) {
	args := []any{hh}
	where := []string{"e.household_id = $1"}
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, strings.ReplaceAll(cond, "?", fmt.Sprintf("$%d", len(args))))
	}
	if f.Scope == "mine" {
		// os meus pessoais e, de leitura, os que o parceiro deixou em "view"
		add(`e.scope = 'personal' AND (e.created_by = ?::uuid OR e.visibility = 'view')`, viewer)
	} else {
		where = append(where, `e.scope = 'household'`)
	}
	if f.Month != "" {
		add(`e.spent_on >= (?||'-01')::date`, f.Month)
		where = append(where, fmt.Sprintf(`e.spent_on < ($%d||'-01')::date + interval '1 month'`, len(args)))
	}
	if f.CategoryID != "" {
		add(`e.category_id = ?::uuid`, f.CategoryID)
	}
	if f.PaidBy != "" {
		add(`e.paid_by = ?::uuid`, f.PaidBy)
	}
	if f.CardID != "" { // o cartão é pessoal: só acha o que EU paguei com ele
		add(`e.card_id = ?::uuid`, f.CardID)
		add(`e.paid_by = ?::uuid`, viewer)
	}
	if f.Before != "" {
		date, id, _ := strings.Cut(f.Before, ",")
		args = append(args, date, id)
		where = append(where, fmt.Sprintf(`(e.spent_on, e.id) < ($%d::date, $%d::uuid)`, len(args)-1, len(args)))
	}
	args = append(args, f.Limit)
	var out []Expense
	err := p.inHousehold(ctx, hh, func(tx *sql.Tx) error {
		var err error
		out, err = queryExpenses(ctx, tx, viewer, `
			SELECT `+expCols+` FROM expenses e
			WHERE `+strings.Join(where, " AND ")+`
			ORDER BY e.spent_on DESC, e.id DESC
			LIMIT $`+fmt.Sprint(len(args)), args...)
		return err
	})
	return out, err
}

func (p *PostgresRepo) UpdateExpense(ctx context.Context, hh string, e Expense, version int, viewer string) (Expense, error) {
	var out Expense
	err := p.inHousehold(ctx, hh, func(tx *sql.Tx) error {
		var err error
		out, err = scanExpense(tx.QueryRowContext(ctx, `
			WITH upd AS (
				UPDATE expenses SET paid_by = $4, visibility = $5, amount_cents = $6, category_id = $7, spent_on = $8::date,
					description = $9, split = $10::jsonb, updated_by = $11, card_id = $12::uuid, version = version + 1, updated_at = now()
				WHERE id = $1 AND household_id = $2 AND version = $3 AND settlement_id IS NULL
				  AND (scope = 'household' OR created_by = $11)
				RETURNING *
			)
			SELECT `+expCols+` FROM upd e`,
			e.ID, hh, version, e.PaidBy, e.Visibility, e.AmountCents, e.CategoryID, e.SpentOn, e.Description,
			jsonArg(e.Split), viewer, nullable(e.CardID)), viewer)
		return err
	})
	return out, err
}

func (p *PostgresRepo) DeleteExpense(ctx context.Context, hh, id string) (bool, error) {
	var n int64
	err := p.inHousehold(ctx, hh, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx,
			`DELETE FROM expenses WHERE id = $1 AND household_id = $2 AND settlement_id IS NULL`, id, hh)
		if err != nil {
			return err
		}
		n, _ = res.RowsAffected()
		return nil
	})
	return n > 0, err
}

func (p *PostgresRepo) GetGroup(ctx context.Context, hh, groupID, viewer string) ([]Expense, error) {
	var out []Expense
	err := p.inHousehold(ctx, hh, func(tx *sql.Tx) error {
		var err error
		out, err = queryExpenses(ctx, tx, viewer, `
			SELECT `+expCols+` FROM expenses e
			WHERE e.installment_group_id = $1 AND e.household_id = $2 AND `+visibleTo(3)+`
			ORDER BY e.installment_no`, groupID, hh, viewer)
		return err
	})
	return out, err
}

// UpdateGroup muda só as parcelas ainda abertas; as acertadas ficam como estão. Em grupo pessoal, só o dono.
func (p *PostgresRepo) UpdateGroup(ctx context.Context, hh, groupID, viewer string, patch GroupPatch) (int64, error) {
	var n int64
	err := p.inHousehold(ctx, hh, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `
			UPDATE expenses SET description = coalesce($4, description), category_id = coalesce($5::uuid, category_id),
				paid_by = coalesce($6::uuid, paid_by), split = coalesce($7::jsonb, split),
				card_id = CASE WHEN $8 THEN $9::uuid
				               WHEN $6::uuid IS NOT NULL AND $6::uuid <> paid_by THEN NULL -- outro pagador: o cartão não vale
				               ELSE card_id END,
				updated_by = $3, version = version + 1, updated_at = now()
			WHERE installment_group_id = $1 AND household_id = $2 AND settlement_id IS NULL
			  AND (scope = 'household' OR created_by = $3)
			  AND (NOT $8 OR paid_by = $3 OR $6::uuid = $3)`, // trocar o cartão só vale nas parcelas que eu paguei
			groupID, hh, viewer, patch.Description, patch.CategoryID, patch.PaidBy, jsonArg(patch.Split),
			patch.CardSet, nullable(patch.CardID))
		if err != nil {
			return err
		}
		n, _ = res.RowsAffected()
		return nil
	})
	return n, err
}

func (p *PostgresRepo) DeleteGroup(ctx context.Context, hh, groupID, viewer string) (int64, int64, error) {
	var deleted, remaining int64
	err := p.inHousehold(ctx, hh, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `
			DELETE FROM expenses
			WHERE installment_group_id = $1 AND household_id = $2 AND settlement_id IS NULL
			  AND (scope = 'household' OR created_by = $3)`, groupID, hh, viewer)
		if err != nil {
			return err
		}
		deleted, _ = res.RowsAffected()
		return tx.QueryRowContext(ctx,
			`SELECT count(*) FROM expenses WHERE installment_group_id = $1 AND household_id = $2`, groupID, hh).Scan(&remaining)
	})
	return deleted, remaining, err
}

// VisibleExpenses são todos os gastos que a pessoa pode ver (para a exportação), do mais antigo ao mais novo.
func (p *PostgresRepo) VisibleExpenses(ctx context.Context, hh, viewer string, limit int) ([]Expense, error) {
	var out []Expense
	err := p.inHousehold(ctx, hh, func(tx *sql.Tx) error {
		var err error
		out, err = queryExpenses(ctx, tx, viewer, `
			SELECT `+expCols+` FROM expenses e
			WHERE e.household_id = $1 AND `+visibleTo(2)+`
			ORDER BY e.spent_on, e.id LIMIT $3`, hh, viewer, limit)
		return err
	})
	return out, err
}

// ---- acerto ----

const openLimit = 5000

// OpenExpenses são os gastos do casal ainda abertos até a data. Gasto pessoal nunca entra.
func (p *PostgresRepo) OpenExpenses(ctx context.Context, hh, through string) ([]Expense, error) {
	var out []Expense
	err := p.inHousehold(ctx, hh, func(tx *sql.Tx) error {
		var err error
		out, err = queryExpenses(ctx, tx, "", `
			SELECT `+expCols+` FROM expenses e
			WHERE e.household_id = $1 AND e.scope = 'household' AND e.settlement_id IS NULL AND e.spent_on <= $2::date
			ORDER BY e.spent_on, e.id LIMIT $3`, hh, through, openLimit)
		return err
	})
	return out, err
}

// CreateSettlement trava os gastos abertos, recalcula o saldo e só grava se bater com o valor enviado. Dois acertos
// ao mesmo tempo se enfileiram nos gastos travados: o segundo já não encontra nada em aberto e dá settlement_stale.
func (p *PostgresRepo) CreateSettlement(ctx context.Context, hh, createdBy, through string, amount int64) (Settlement, error) {
	var out Settlement
	err := p.inHousehold(ctx, hh, func(tx *sql.Tx) error {
		open, err := queryExpenses(ctx, tx, "", `
			SELECT `+expCols+` FROM expenses e
			WHERE e.household_id = $1 AND e.scope = 'household' AND e.settlement_id IS NULL AND e.spent_on <= $2::date
			ORDER BY e.id FOR UPDATE`, hh, through)
		if err != nil {
			return err
		}
		from, to, owed, _ := ComputeBalance(open)
		if owed == 0 || owed != amount {
			return ErrSettlementStale
		}
		out, err = insertSettlement(ctx, tx, hh, through, from, to, owed, "manual", createdBy)
		if err != nil {
			return err
		}
		return stampExpenses(ctx, tx, out.ID, open)
	})
	return out, err
}

func insertSettlement(ctx context.Context, tx *sql.Tx, hh, through, from, to string, amount int64, kind, createdBy string) (Settlement, error) {
	var s Settlement
	err := tx.QueryRowContext(ctx, `
		INSERT INTO settlements (household_id, through_on, from_user, to_user, amount_cents, kind, created_by)
		VALUES ($1, $2::date, $3, $4, $5, $6, $7)
		RETURNING id, to_char(through_on, 'YYYY-MM-DD'), from_user, to_user, amount_cents, kind, created_by, created_at`,
		hh, through, from, to, amount, kind, createdBy).
		Scan(&s.ID, &s.ThroughOn, &s.FromUser, &s.ToUser, &s.AmountCents, &s.Kind, &s.CreatedBy, &s.CreatedAt)
	return s, err
}

func stampExpenses(ctx context.Context, tx *sql.Tx, settlementID string, es []Expense) error {
	ids := make([]string, len(es))
	for i, e := range es {
		ids[i] = e.ID
	}
	_, err := tx.ExecContext(ctx,
		`UPDATE expenses SET settlement_id = $1 WHERE id = ANY($2::uuid[])`, settlementID, uuidArray(ids))
	return err
}

func (p *PostgresRepo) ListSettlements(ctx context.Context, hh string) ([]Settlement, error) {
	out := []Settlement{}
	err := p.inHousehold(ctx, hh, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
			SELECT id, to_char(through_on, 'YYYY-MM-DD'), from_user, to_user, amount_cents, kind, created_by, created_at
			FROM settlements WHERE household_id = $1 ORDER BY created_at DESC, id LIMIT 200`, hh)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var s Settlement
			if err := rows.Scan(&s.ID, &s.ThroughOn, &s.FromUser, &s.ToUser, &s.AmountCents, &s.Kind, &s.CreatedBy, &s.CreatedAt); err != nil {
				return err
			}
			out = append(out, s)
		}
		return rows.Err()
	})
	return out, err
}

// ComputeBalance soma, por pessoa, o que pagou menos a parte que lhe cabe em cada gasto. Quem tem o saldo mais
// negativo deve ao de saldo mais positivo. paid é o total que cada um adiantou pelo casal.
func ComputeBalance(es []Expense) (from, to string, owed int64, paid map[string]int64) {
	net := map[string]int64{}
	paid = map[string]int64{}
	for _, e := range es {
		net[e.PaidBy] += e.AmountCents
		paid[e.PaidBy] += e.AmountCents
		for id, share := range Shares(e.AmountCents, e.Split) {
			net[id] -= share
		}
	}
	var minID, maxID string
	for id, v := range net {
		if minID == "" || v < net[minID] || (v == net[minID] && id < minID) {
			minID = id
		}
		if maxID == "" || v > net[maxID] || (v == net[maxID] && id < maxID) {
			maxID = id
		}
	}
	if minID == "" || net[minID] >= 0 {
		return "", "", 0, paid
	}
	return minID, maxID, -net[minID], paid
}
