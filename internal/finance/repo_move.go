package finance

import (
	"context"
	"database/sql"

	"casal/internal/db"
)

// HandleLeave roda na transação em que alguém sai do casal (veja docs/arquitetura.md).
//
//   - Se há saldo em aberto, registra um acerto "leave" (só um registro: o app não cobra ninguém) e carimba todos os
//     gastos abertos do casal, inclusive as parcelas futuras, para a dívida não crescer em silêncio depois da saída.
//   - Gastos fixos e contas da pessoa passam para quem ficou; a divisão padrão volta para igual.
//   - O que é pessoal acompanha quem saiu e o que o parceiro lia ("view") vira privado. A renda também segue a pessoa.
func (p *PostgresRepo) HandleLeave(ctx context.Context, tx *sql.Tx, h db.Handoff) error {
	if err := db.SetScope(ctx, tx, db.Scope{System: true}); err != nil {
		return err
	}
	if h.RemainingUserID != "" {
		if err := settleOnLeave(ctx, tx, h); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE recurring_expenses
			SET paid_by = $3::uuid, split = jsonb_build_object($3::uuid::text, 10000)
			WHERE household_id = $1 AND scope = 'household' AND (paid_by = $2::uuid OR split ? $2::text)`,
			h.From, h.UserID, h.RemainingUserID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE bills SET responsible_id = $3 WHERE household_id = $1 AND paid_at IS NULL AND responsible_id = $2`,
			h.From, h.UserID, h.RemainingUserID); err != nil {
			return err
		}
	}
	// a divisão combinada falava de duas pessoas; sem a outra, volta para igual e ninguém fica "concordando"
	if _, err := tx.ExecContext(ctx, `
		UPDATE finance_settings SET split_mode = 'equal', fixed_split = NULL, proportional_consent = '[]', updated_at = now()
		WHERE household_id = $1 AND (split_mode <> 'equal' OR proportional_consent <> '[]'::jsonb)`, h.From); err != nil {
		return err
	}
	for _, table := range []string{"expenses", "recurring_expenses"} {
		if _, err := tx.ExecContext(ctx, `UPDATE `+table+` SET visibility = 'private'
			WHERE household_id = $1 AND scope = 'personal' AND created_by = $2 AND visibility = 'view'`,
			h.From, h.UserID); err != nil {
			return err
		}
	}
	return nil
}

func settleOnLeave(ctx context.Context, tx *sql.Tx, h db.Handoff) error {
	open, err := queryExpenses(ctx, tx, "", `
		SELECT `+expCols+` FROM expenses e
		WHERE e.household_id = $1 AND e.scope = 'household' AND e.settlement_id IS NULL
		ORDER BY e.id FOR UPDATE`, h.From)
	if err != nil {
		return err
	}
	from, to, owed, _ := ComputeBalance(open)
	if owed == 0 {
		return nil
	}
	s, err := insertSettlement(ctx, tx, h.From, "now", from, to, owed, "leave", h.UserID)
	if err != nil {
		return err
	}
	return stampExpenses(ctx, tx, s.ID, open)
}

// HandleJoin roda quando a pessoa passa a ter um casal: o que era pessoal vem junto, sempre privado, com as
// categorias que usava (copiadas para o casal novo, se ainda não existirem lá), e a renda também.
func (p *PostgresRepo) HandleJoin(ctx context.Context, tx *sql.Tx, h db.Handoff) error {
	if err := db.SetScope(ctx, tx, db.Scope{System: true}); err != nil {
		return err
	}
	for _, table := range []string{"expenses", "recurring_expenses"} {
		// 1. leva para o casal novo as categorias que os gastos pessoais usam; o que já existe lá é ignorado
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO finance_categories (household_id, name, key)
			SELECT DISTINCT $2::uuid, c.name, c.key
			FROM `+table+` x JOIN finance_categories c ON c.id = x.category_id
			WHERE x.created_by = $1 AND x.scope = 'personal' AND x.household_id <> $2
			ON CONFLICT DO NOTHING`, h.UserID, h.To); err != nil {
			return err
		}
		// 2. muda de casal apontando para a categoria equivalente (mesmo nome, ou a mesma chave padrão)
		if _, err := tx.ExecContext(ctx, `
			UPDATE `+table+` x SET household_id = $2, visibility = 'private', category_id = m.id
			FROM finance_categories o,
			LATERAL (
				SELECT n.id FROM finance_categories n
				WHERE n.household_id = $2 AND (n.name_key = o.name_key OR (o.key IS NOT NULL AND n.key = o.key))
				ORDER BY (n.name_key = o.name_key) DESC LIMIT 1
			) m
			WHERE o.id = x.category_id AND x.created_by = $1 AND x.scope = 'personal' AND x.household_id <> $2`,
			h.UserID, h.To); err != nil {
			return err
		}
	}
	_, err := tx.ExecContext(ctx,
		`UPDATE finance_incomes SET household_id = $2 WHERE user_id = $1 AND household_id <> $2`, h.UserID, h.To)
	return err
}
