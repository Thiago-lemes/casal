package finance

import (
	"context"
	"database/sql"
	"encoding/json"

	"casal/internal/db"
)

// ---- gastos fixos ----

const recCols = `r.id, r.household_id, r.created_by, r.paid_by, r.amount_cents, r.category_id, r.description, r.scope,
	r.visibility, r.split, r.day_of_month, to_char(r.start_on, 'YYYY-MM-DD'), to_char(r.end_on, 'YYYY-MM-DD'), r.active`

func scanRecurring(s scanner, viewer string, extra ...any) (Recurring, error) {
	var r Recurring
	var split []byte
	var end sql.NullString
	dest := append([]any{&r.ID, &r.HouseholdID, &r.CreatedBy, &r.PaidBy, &r.AmountCents, &r.CategoryID, &r.Description,
		&r.Scope, &r.Visibility, &split, &r.DayOfMonth, &r.StartOn, &end, &r.Active}, extra...)
	if err := s.Scan(dest...); err != nil {
		return Recurring{}, err
	}
	r.Split, r.EndOn = parseSplit(split), end.String
	r.CanEdit = r.Scope == ScopeHousehold || r.CreatedBy == viewer
	return r, nil
}

func (p *PostgresRepo) oneRecurring(ctx context.Context, hh, viewer, query string, args ...any) (Recurring, error) {
	var out Recurring
	err := p.inHousehold(ctx, hh, func(tx *sql.Tx) error {
		var err error
		out, err = scanRecurring(tx.QueryRowContext(ctx, query, args...), viewer)
		return err
	})
	if isNoRows(err) {
		return Recurring{}, ErrRecurringNotFound
	}
	return out, err
}

func (p *PostgresRepo) InsertRecurring(ctx context.Context, r Recurring) (Recurring, error) {
	return p.oneRecurring(ctx, r.HouseholdID, r.CreatedBy, `
		WITH ins AS (
			INSERT INTO recurring_expenses (household_id, created_by, paid_by, amount_cents, category_id, description, scope,
				visibility, split, day_of_month, start_on, end_on)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb, $10, $11::date, $12::date)
			RETURNING *
		)
		SELECT `+recCols+` FROM ins r`,
		r.HouseholdID, r.CreatedBy, r.PaidBy, r.AmountCents, r.CategoryID, r.Description, r.Scope, r.Visibility,
		jsonArg(r.Split), r.DayOfMonth, r.StartOn, nullable(r.EndOn))
}

func (p *PostgresRepo) ListRecurring(ctx context.Context, hh, viewer string) ([]Recurring, error) {
	out := []Recurring{}
	err := p.inHousehold(ctx, hh, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
			SELECT `+recCols+` FROM recurring_expenses r
			WHERE r.household_id = $1 AND (r.scope = 'household' OR r.created_by = $2 OR r.visibility = 'view')
			ORDER BY r.day_of_month, r.id LIMIT 200`, hh, viewer)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			r, err := scanRecurring(rows, viewer)
			if err != nil {
				return err
			}
			out = append(out, r)
		}
		return rows.Err()
	})
	return out, err
}

func (p *PostgresRepo) GetRecurring(ctx context.Context, hh, id, viewer string) (Recurring, error) {
	return p.oneRecurring(ctx, hh, viewer, `
		SELECT `+recCols+` FROM recurring_expenses r
		WHERE r.id = $1 AND r.household_id = $2 AND (r.scope = 'household' OR r.created_by = $3 OR r.visibility = 'view')`,
		id, hh, viewer)
}

func (p *PostgresRepo) UpdateRecurring(ctx context.Context, r Recurring, viewer string) (Recurring, error) {
	return p.oneRecurring(ctx, r.HouseholdID, viewer, `
		WITH upd AS (
			UPDATE recurring_expenses SET paid_by = $4, amount_cents = $5, category_id = $6, description = $7,
				visibility = $8, split = $9::jsonb, day_of_month = $10, end_on = $11::date
			WHERE id = $1 AND household_id = $2 AND (scope = 'household' OR created_by = $3)
			RETURNING *
		)
		SELECT `+recCols+` FROM upd r`,
		r.ID, r.HouseholdID, viewer, r.PaidBy, r.AmountCents, r.CategoryID, r.Description, r.Visibility,
		jsonArg(r.Split), r.DayOfMonth, nullable(r.EndOn))
}

func (p *PostgresRepo) DeleteRecurring(ctx context.Context, hh, id, viewer string) (bool, error) {
	var n int64
	err := p.inHousehold(ctx, hh, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `
			DELETE FROM recurring_expenses WHERE id = $1 AND household_id = $2 AND (scope = 'household' OR created_by = $3)`,
			id, hh, viewer)
		if err != nil {
			return err
		}
		n, _ = res.RowsAffected()
		return nil
	})
	return n > 0, err
}

func (p *PostgresRepo) SetRecurringActive(ctx context.Context, hh, id, viewer string, active bool) (Recurring, error) {
	return p.oneRecurring(ctx, hh, viewer, `
		WITH upd AS (
			UPDATE recurring_expenses SET active = $4
			WHERE id = $1 AND household_id = $2 AND (scope = 'household' OR created_by = $3)
			RETURNING *
		)
		SELECT `+recCols+` FROM upd r`, id, hh, viewer, active)
}

// DueRecurring lista os gastos fixos ativos dos casais com o módulo ligado, com o fuso de quem paga. Atravessa todos
// os casais, por isso roda no escopo de sistema. O fixo pessoal só vale enquanto o dono ainda é do casal.
func (p *PostgresRepo) DueRecurring(ctx context.Context) ([]DueRecurring, error) {
	var out []DueRecurring
	err := db.InTx(ctx, p.db, db.Scope{System: true}, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
			SELECT `+recCols+`, u.timezone, coalesce(to_char(r.last_generated_on, 'YYYY-MM-DD'), '')
			FROM recurring_expenses r
			JOIN users u ON u.id = r.paid_by
			JOIN household_modules m ON m.household_id = r.household_id AND m.module = 'finance'
			WHERE r.active
			  AND (r.scope = 'household' OR EXISTS (
			        SELECT 1 FROM household_members hm WHERE hm.household_id = r.household_id AND hm.user_id = r.created_by))
			LIMIT 5000`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var d DueRecurring
			r, err := scanRecurring(rows, "", &d.Timezone, &d.LastGenerated)
			if err != nil {
				return err
			}
			d.Recurring = r
			out = append(out, d)
		}
		return rows.Err()
	})
	return out, err
}

func (p *PostgresRepo) GenerateRecurring(ctx context.Context, r Recurring, spentOn string) (Expense, bool, error) {
	var out Expense
	inserted := true
	err := p.inHousehold(ctx, r.HouseholdID, func(tx *sql.Tx) error {
		var err error
		out, err = scanExpense(tx.QueryRowContext(ctx, `
			WITH ins AS (
				INSERT INTO expenses (household_id, created_by, paid_by, scope, visibility, amount_cents, category_id, spent_on,
					description, split, source, recurring_id)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8::date, $9, $10::jsonb, 'recurring', $11)
				ON CONFLICT (recurring_id, spent_on) WHERE recurring_id IS NOT NULL DO NOTHING
				RETURNING *
			)
			SELECT `+expCols+` FROM ins e`,
			r.HouseholdID, r.CreatedBy, r.PaidBy, r.Scope, r.Visibility, r.AmountCents, r.CategoryID, spentOn,
			r.Description, jsonArg(r.Split), r.ID), r.CreatedBy)
		if isNoRows(err) {
			inserted, err = false, nil
		}
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `UPDATE recurring_expenses SET last_generated_on = $2::date WHERE id = $1`, r.ID, spentOn)
		return err
	})
	return out, inserted, err
}

// ---- contas a pagar ----

const billCols = `b.id, b.household_id, b.created_by, b.title, b.amount_cents, to_char(b.due_on, 'YYYY-MM-DD'), b.due_day,
	b.responsible_id, b.category_id, b.notify_both, b.repeat, b.paid_at, b.paid_by, b.expense_id, b.reminder_refs`

func scanBill(s scanner) (Bill, error) {
	var b Bill
	var amount sql.NullInt64
	var paidAt sql.NullTime
	var paidBy, expense sql.NullString
	var refs []byte
	if err := s.Scan(&b.ID, &b.HouseholdID, &b.CreatedBy, &b.Title, &amount, &b.DueOn, &b.DueDay, &b.ResponsibleID,
		&b.CategoryID, &b.NotifyBoth, &b.Repeat, &paidAt, &paidBy, &expense, &refs); err != nil {
		return Bill{}, err
	}
	if amount.Valid {
		b.AmountCents = &amount.Int64
	}
	if paidAt.Valid {
		b.PaidAt = &paidAt.Time
	}
	b.PaidBy, b.ExpenseID = paidBy.String, expense.String
	_ = json.Unmarshal(refs, &b.ReminderRefs)
	return b, nil
}

func (p *PostgresRepo) oneBill(ctx context.Context, hh, query string, args ...any) (Bill, error) {
	var out Bill
	err := p.inHousehold(ctx, hh, func(tx *sql.Tx) error {
		var err error
		out, err = scanBill(tx.QueryRowContext(ctx, query, args...))
		return err
	})
	if isNoRows(err) {
		return Bill{}, ErrBillNotFound
	}
	return out, err
}

func insertBill(ctx context.Context, tx *sql.Tx, b Bill) (Bill, error) {
	return scanBill(tx.QueryRowContext(ctx, `
		WITH ins AS (
			INSERT INTO bills (household_id, created_by, title, amount_cents, due_on, due_day, responsible_id, category_id,
				notify_both, repeat)
			VALUES ($1, $2, $3, $4, $5::date, $6, $7, $8, $9, $10)
			RETURNING *
		)
		SELECT `+billCols+` FROM ins b`,
		b.HouseholdID, b.CreatedBy, b.Title, b.AmountCents, b.DueOn, b.DueDay, b.ResponsibleID, b.CategoryID,
		b.NotifyBoth, b.Repeat))
}

func (p *PostgresRepo) InsertBill(ctx context.Context, b Bill) (Bill, error) {
	var out Bill
	err := p.inHousehold(ctx, b.HouseholdID, func(tx *sql.Tx) error {
		var err error
		out, err = insertBill(ctx, tx, b)
		return err
	})
	return out, err
}

func (p *PostgresRepo) GetBill(ctx context.Context, hh, id string) (Bill, error) {
	return p.oneBill(ctx, hh, `SELECT `+billCols+` FROM bills b WHERE b.id = $1 AND b.household_id = $2`, id, hh)
}

func (p *PostgresRepo) ListBills(ctx context.Context, hh string, f BillFilter) ([]Bill, error) {
	out := []Bill{}
	err := p.inHousehold(ctx, hh, func(tx *sql.Tx) error {
		q := `SELECT ` + billCols + ` FROM bills b WHERE b.household_id = $1`
		args := []any{hh}
		if f.Status == "paid" {
			q += ` AND b.paid_at IS NOT NULL`
		} else {
			q += ` AND b.paid_at IS NULL`
		}
		if f.DueBefore != "" {
			q += ` AND b.due_on <= $2::date`
			args = append(args, f.DueBefore)
		}
		if f.Status == "paid" {
			q += ` ORDER BY b.paid_at DESC, b.id LIMIT 200`
		} else {
			q += ` ORDER BY b.due_on, b.id LIMIT 200` // vencidas primeiro
		}
		rows, err := tx.QueryContext(ctx, q, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			b, err := scanBill(rows)
			if err != nil {
				return err
			}
			out = append(out, b)
		}
		return rows.Err()
	})
	return out, err
}

// UpdateBill só muda conta aberta; sem linha, o service descobre se não existe ou se já foi paga.
func (p *PostgresRepo) UpdateBill(ctx context.Context, b Bill) (Bill, error) {
	var out Bill
	err := p.inHousehold(ctx, b.HouseholdID, func(tx *sql.Tx) error {
		var err error
		out, err = scanBill(tx.QueryRowContext(ctx, `
			WITH upd AS (
				UPDATE bills SET title = $3, amount_cents = $4, due_on = $5::date, due_day = $6, responsible_id = $7,
					category_id = $8, notify_both = $9, repeat = $10
				WHERE id = $1 AND household_id = $2 AND paid_at IS NULL
				RETURNING *
			)
			SELECT `+billCols+` FROM upd b`,
			b.ID, b.HouseholdID, b.Title, b.AmountCents, b.DueOn, b.DueDay, b.ResponsibleID, b.CategoryID, b.NotifyBoth, b.Repeat))
		return err
	})
	return out, err
}

func (p *PostgresRepo) DeleteBill(ctx context.Context, hh, id string) (Bill, error) {
	return p.oneBill(ctx, hh, `
		WITH del AS (DELETE FROM bills WHERE id = $1 AND household_id = $2 RETURNING *)
		SELECT `+billCols+` FROM del b`, id, hh)
}

// PayBill marca a conta como paga e cria o gasto na mesma transação: se o gasto falhar, a conta continua aberta.
// Conta mensal cria a próxima, com o vencimento no mês seguinte.
func (p *PostgresRepo) PayBill(ctx context.Context, hh, id string, e Expense) (Bill, Expense, *Bill, error) {
	var paid Bill
	var exp Expense
	var next *Bill
	err := p.inHousehold(ctx, hh, func(tx *sql.Tx) error {
		cur, err := scanBill(tx.QueryRowContext(ctx,
			`SELECT `+billCols+` FROM bills b WHERE b.id = $1 AND b.household_id = $2 FOR UPDATE`, id, hh))
		if isNoRows(err) {
			return ErrBillNotFound
		}
		if err != nil {
			return err
		}
		if cur.PaidAt != nil {
			return ErrBillAlreadyPaid
		}
		e.HouseholdID, e.BillID = hh, id
		if exp, err = insertExpense(ctx, tx, e); err != nil {
			return err
		}
		paid, err = scanBill(tx.QueryRowContext(ctx, `
			WITH upd AS (
				UPDATE bills SET paid_at = now(), paid_by = $3, expense_id = $4 WHERE id = $1 AND household_id = $2 RETURNING *
			)
			SELECT `+billCols+` FROM upd b`, id, hh, e.PaidBy, exp.ID))
		if err != nil {
			return err
		}
		if cur.Repeat == "monthly" {
			due, _ := parseDate(cur.DueOn)
			n := cur
			n.DueOn = fmtDate(AddMonths(due, 1, cur.DueDay))
			nb, err := insertBill(ctx, tx, n)
			if err != nil {
				return err
			}
			next = &nb
		}
		return nil
	})
	return paid, exp, next, err
}

func (p *PostgresRepo) SetBillRefs(ctx context.Context, hh, id string, refs []string) error {
	b, _ := json.Marshal(append([]string{}, refs...))
	return p.inHousehold(ctx, hh, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			`UPDATE bills SET reminder_refs = $3::jsonb WHERE id = $1 AND household_id = $2`, id, hh, string(b))
		return err
	})
}

func (p *PostgresRepo) OpenBillsForSync(ctx context.Context, limit int) ([]Bill, error) {
	var out []Bill
	err := db.InTx(ctx, p.db, db.Scope{System: true}, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
			SELECT `+billCols+` FROM bills b
			JOIN household_modules m ON m.household_id = b.household_id AND m.module = 'finance'
			WHERE b.paid_at IS NULL ORDER BY b.due_on, b.id LIMIT $1`, limit)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			b, err := scanBill(rows)
			if err != nil {
				return err
			}
			out = append(out, b)
		}
		return rows.Err()
	})
	return out, err
}

func (p *PostgresRepo) BillsWithRefs(ctx context.Context, hh string) ([]Bill, error) {
	out := []Bill{}
	err := p.inHousehold(ctx, hh, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
			SELECT `+billCols+` FROM bills b
			WHERE b.household_id = $1 AND b.paid_at IS NULL AND b.reminder_refs <> '[]'::jsonb`, hh)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			b, err := scanBill(rows)
			if err != nil {
				return err
			}
			out = append(out, b)
		}
		return rows.Err()
	})
	return out, err
}

// ---- orçamento ----

func (p *PostgresRepo) ListBudgets(ctx context.Context, hh string) ([]Budget, error) {
	out := []Budget{}
	err := p.inHousehold(ctx, hh, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx,
			`SELECT category_id, limit_cents FROM budgets WHERE household_id = $1 ORDER BY category_id`, hh)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var b Budget
			if err := rows.Scan(&b.CategoryID, &b.LimitCents); err != nil {
				return err
			}
			out = append(out, b)
		}
		return rows.Err()
	})
	return out, err
}

func (p *PostgresRepo) SaveBudget(ctx context.Context, hh string, b Budget) error {
	return p.inHousehold(ctx, hh, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO budgets (household_id, category_id, limit_cents) VALUES ($1, $2, $3)
			ON CONFLICT (household_id, category_id) DO UPDATE SET limit_cents = EXCLUDED.limit_cents`,
			hh, b.CategoryID, b.LimitCents)
		return err
	})
}

func (p *PostgresRepo) DeleteBudget(ctx context.Context, hh, categoryID string) (bool, error) {
	var n int64
	err := p.inHousehold(ctx, hh, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx,
			`DELETE FROM budgets WHERE household_id = $1 AND category_id = $2`, hh, categoryID)
		if err != nil {
			return err
		}
		n, _ = res.RowsAffected()
		return nil
	})
	return n > 0, err
}

func (p *PostgresRepo) BudgetLimit(ctx context.Context, hh, categoryID string) (int64, bool, error) {
	var limit int64
	err := p.inHousehold(ctx, hh, func(tx *sql.Tx) error {
		return tx.QueryRowContext(ctx,
			`SELECT limit_cents FROM budgets WHERE household_id = $1 AND category_id = $2`, hh, categoryID).Scan(&limit)
	})
	if isNoRows(err) {
		return 0, false, nil
	}
	return limit, err == nil, err
}

// MonthUsage é o que o casal gastou na categoria no mês (month = AAAA-MM). Gasto pessoal não conta.
func (p *PostgresRepo) MonthUsage(ctx context.Context, hh, categoryID, month string) (int64, error) {
	var used int64
	err := p.inHousehold(ctx, hh, func(tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `
			SELECT coalesce(sum(amount_cents), 0) FROM expenses
			WHERE household_id = $1 AND scope = 'household' AND category_id = $2
			  AND spent_on >= ($3||'-01')::date AND spent_on < ($3||'-01')::date + interval '1 month'`,
			hh, categoryID, month).Scan(&used)
	})
	return used, err
}

func (p *PostgresRepo) TryAlert(ctx context.Context, hh, categoryID, month string, level int) (bool, error) {
	var n int64
	err := p.inHousehold(ctx, hh, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `
			INSERT INTO budget_alerts (household_id, category_id, month, level) VALUES ($1, $2, ($3||'-01')::date, $4)
			ON CONFLICT DO NOTHING`, hh, categoryID, month, level)
		if err != nil {
			return err
		}
		n, _ = res.RowsAffected()
		return nil
	})
	return n > 0, err
}

// ---- resumo ----

func (p *PostgresRepo) CategoryTotals(ctx context.Context, hh, viewer, scope, month string) (map[string]int64, error) {
	out := map[string]int64{}
	err := p.inHousehold(ctx, hh, func(tx *sql.Tx) error {
		// "mine" soma só os pessoais de quem pergunta (nunca os "view" do parceiro); o resto, só os do casal
		cond := `e.scope = 'household'`
		args := []any{hh, month}
		if scope == "mine" {
			cond = `e.scope = 'personal' AND e.created_by = $3`
			args = append(args, viewer)
		}
		rows, err := tx.QueryContext(ctx, `
			SELECT e.category_id, sum(e.amount_cents) FROM expenses e
			WHERE e.household_id = $1 AND `+cond+`
			  AND e.spent_on >= ($2||'-01')::date AND e.spent_on < ($2||'-01')::date + interval '1 month'
			GROUP BY e.category_id`, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			var total int64
			if err := rows.Scan(&id, &total); err != nil {
				return err
			}
			out[id] = total
		}
		return rows.Err()
	})
	return out, err
}

func (p *PostgresRepo) PaidTotals(ctx context.Context, hh, month string) (map[string]int64, error) {
	out := map[string]int64{}
	err := p.inHousehold(ctx, hh, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
			SELECT e.paid_by, sum(e.amount_cents) FROM expenses e
			WHERE e.household_id = $1 AND e.scope = 'household'
			  AND e.spent_on >= ($2||'-01')::date AND e.spent_on < ($2||'-01')::date + interval '1 month'
			GROUP BY e.paid_by`, hh, month)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			var total int64
			if err := rows.Scan(&id, &total); err != nil {
				return err
			}
			out[id] = total
		}
		return rows.Err()
	})
	return out, err
}
