package goal

import (
	"context"
	"database/sql"

	"casal/internal/db"
)

// HandleLeave roda na transação em que alguém sai do casal (veja docs/arquitetura.md).
//
//   - As metas do casal ficam com quem permanece, com os aportes e as marcações dos dois. A que exigia os dois passa a
//     exigir só quem sobrou, para não travar. As etapas da pessoa passam para quem ficou.
//   - As metas pessoais acompanham a pessoa (ficam com ela até entrar em outro casal, onde HandleJoin as leva), e as
//     que o parceiro lia ("view") viram privadas.
func (p *PostgresRepo) HandleLeave(ctx context.Context, tx *sql.Tx, h db.Handoff) error {
	if err := db.SetScope(ctx, tx, db.Scope{System: true}); err != nil {
		return err
	}
	if h.RemainingUserID != "" {
		if _, err := tx.ExecContext(ctx, `
			UPDATE goal_shared s SET requires_both = false
			FROM goals g
			WHERE s.goal_id = g.id AND g.household_id = $1 AND g.scope = 'household' AND s.requires_both`, h.From); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE goal_steps s SET assignee_id = $3
			FROM goals g
			WHERE s.goal_id = g.id AND g.household_id = $1 AND g.scope = 'household' AND s.assignee_id = $2`,
			h.From, h.UserID, h.RemainingUserID); err != nil {
			return err
		}
	}
	_, err := tx.ExecContext(ctx, `
		UPDATE goals SET visibility = 'private', updated_at = now()
		WHERE household_id = $1 AND scope = 'personal' AND owner_id = $2 AND visibility = 'view'`, h.From, h.UserID)
	return err
}

// HandleJoin roda quando a pessoa passa a ter um casal: as metas pessoais deixadas para trás vêm junto, sempre
// privadas, com tudo o que é delas (parâmetros, aportes, marcações e etapas).
func (p *PostgresRepo) HandleJoin(ctx context.Context, tx *sql.Tx, h db.Handoff) error {
	if err := db.SetScope(ctx, tx, db.Scope{System: true}); err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, `
		UPDATE goals SET household_id = $2, visibility = 'private', updated_at = now()
		WHERE owner_id = $1 AND scope = 'personal' AND household_id <> $2 RETURNING id`, h.UserID, h.To)
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	if len(ids) == 0 {
		return nil
	}
	for _, table := range []string{"goal_financial", "goal_habit", "goal_shared", "goal_contributions", "goal_checkins", "goal_steps"} {
		if _, err := tx.ExecContext(ctx, `UPDATE `+table+` SET household_id = $2 WHERE goal_id = ANY($1)`, ids, h.To); err != nil {
			return err
		}
	}
	return nil
}
