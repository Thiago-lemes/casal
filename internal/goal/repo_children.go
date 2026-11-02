package goal

import (
	"context"
	"database/sql"
	"time"
)

// Toda leitura e escrita nas tabelas filhas passa por goals: a privacidade é da meta (visibleTo / writableBy),
// e a tabela filha nunca é consultada sozinha.

// access diz se a pessoa enxerga a meta e com que nível. ErrNotFound: não vê; AccessView: só o resumo.
func access(ctx context.Context, tx *sql.Tx, hh, viewer, id string) (kind, level string, err error) {
	var full bool
	err = tx.QueryRowContext(ctx, `
		SELECT g.kind, `+fullTo("$3")+` FROM goals g
		WHERE g.id = $1 AND g.household_id = $2 AND `+visibleTo("$3"), id, hh, viewer).Scan(&kind, &full)
	if isNoRows(err) {
		return "", "", ErrNotFound
	}
	level = AccessView
	if full {
		level = AccessFull
	}
	return kind, level, err
}

// ---- aportes ----

func sumContributions(ctx context.Context, tx *sql.Tx, goalID string) (int64, error) {
	var sum int64
	err := tx.QueryRowContext(ctx, `SELECT coalesce(sum(amount_cents), 0) FROM goal_contributions WHERE goal_id = $1`, goalID).Scan(&sum)
	return sum, err
}

const contribCols = `c.id, c.amount_cents, c.made_on, c.note, c.created_by, u.name, c.created_at`

func scanContribution(r scanner) (Contribution, error) {
	var c Contribution
	var madeOn time.Time
	if err := r.Scan(&c.ID, &c.AmountCents, &madeOn, &c.Note, &c.CreatedBy, &c.CreatedByName, &c.CreatedAt); err != nil {
		return Contribution{}, err
	}
	c.MadeOn = fmtDay(madeOn)
	return c, nil
}

func (p *PostgresRepo) AddContribution(ctx context.Context, hh, actor, goalID string, c Contribution) (Contribution, error) {
	var out Contribution
	err := p.in(ctx, hh, func(tx *sql.Tx) error {
		l, err := lockGoal(ctx, tx, hh, actor, goalID)
		if err != nil {
			return err
		}
		if l.Kind != KindFinancial || l.Source != SourceContributions {
			return ErrKindMismatch
		}
		if l.closed() {
			return ErrClosed
		}
		sum, err := sumContributions(ctx, tx, goalID)
		if err != nil {
			return err
		}
		if sum+c.AmountCents < 0 {
			return ErrContributionNegative
		}
		out, err = scanContribution(tx.QueryRowContext(ctx, `
			WITH ins AS (
				INSERT INTO goal_contributions (goal_id, household_id, created_by, amount_cents, made_on, note)
				VALUES ($1, $2, $3, $4, $5, $6) RETURNING *
			)
			SELECT `+contribCols+` FROM ins c JOIN users u ON u.id = c.created_by`,
			goalID, hh, actor, c.AmountCents, c.MadeOn, c.Note))
		return err
	})
	return out, err
}

func (p *PostgresRepo) ListContributions(ctx context.Context, hh, viewer, goalID, before string, limit int) ([]Contribution, error) {
	out := []Contribution{}
	err := p.in(ctx, hh, func(tx *sql.Tx) error {
		_, level, err := access(ctx, tx, hh, viewer, goalID)
		if err != nil {
			return err
		}
		if level != AccessFull { // quem só enxerga vê o total, nunca os aportes
			return ErrForbidden
		}
		q := `SELECT ` + contribCols + ` FROM goal_contributions c
			JOIN goals g ON g.id = c.goal_id JOIN users u ON u.id = c.created_by
			WHERE c.goal_id = $1 AND g.household_id = $2 AND ` + visibleTo("$3")
		args := []any{goalID, hh, viewer}
		if before != "" {
			args = append(args, before)
			q += ` AND (c.made_on, c.created_at, c.id) < (
				SELECT b.made_on, b.created_at, b.id FROM goal_contributions b WHERE b.id = $4 AND b.goal_id = $1)`
		}
		args = append(args, limit)
		q += ` ORDER BY c.made_on DESC, c.created_at DESC, c.id DESC LIMIT $` + itoa(len(args))
		rows, err := tx.QueryContext(ctx, q, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			c, err := scanContribution(rows)
			if err != nil {
				return err
			}
			out = append(out, c)
		}
		return rows.Err()
	})
	return out, err
}

func (p *PostgresRepo) DeleteContribution(ctx context.Context, hh, actor, goalID, id string) error {
	return p.in(ctx, hh, func(tx *sql.Tx) error {
		l, err := lockGoal(ctx, tx, hh, actor, goalID)
		if err != nil {
			return err
		}
		if l.Kind != KindFinancial {
			return ErrKindMismatch
		}
		if l.closed() {
			return ErrClosed
		}
		var createdBy string
		var amount int64
		err = tx.QueryRowContext(ctx, `
			SELECT c.created_by, c.amount_cents FROM goal_contributions c
			JOIN goals g ON g.id = c.goal_id
			WHERE c.id = $1 AND c.goal_id = $2 AND g.household_id = $3 AND `+visibleTo("$4"),
			id, goalID, hh, actor).Scan(&createdBy, &amount)
		if isNoRows(err) {
			return ErrContributionNotFound
		}
		if err != nil {
			return err
		}
		// só quem registrou, ou o dono da meta (numa meta do casal não há dono: fica quem registrou)
		if createdBy != actor && (l.OwnerID == nil || *l.OwnerID != actor) {
			return ErrForbidden
		}
		sum, err := sumContributions(ctx, tx, goalID)
		if err != nil {
			return err
		}
		if sum-amount < 0 { // apagar um aporte não pode deixar uma retirada sem saldo
			return ErrContributionNegative
		}
		_, err = tx.ExecContext(ctx, `DELETE FROM goal_contributions WHERE id = $1 AND goal_id = $2`, id, goalID)
		return err
	})
}

func (p *PostgresRepo) SumContributions(ctx context.Context, hh, viewer string, goalIDs []string) (map[string]int64, error) {
	out := map[string]int64{}
	if len(goalIDs) == 0 {
		return out, nil
	}
	err := p.in(ctx, hh, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
			SELECT c.goal_id, sum(c.amount_cents) FROM goal_contributions c
			JOIN goals g ON g.id = c.goal_id
			WHERE c.goal_id = ANY($1) AND g.household_id = $2 AND `+visibleTo("$3")+`
			GROUP BY c.goal_id`, goalIDs, hh, viewer)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			var sum int64
			if err := rows.Scan(&id, &sum); err != nil {
				return err
			}
			out[id] = sum
		}
		return rows.Err()
	})
	return out, err
}

// AllContributions devolve os aportes das metas em que a pessoa tem o detalhe (dona, ou meta do casal).
func (p *PostgresRepo) AllContributions(ctx context.Context, hh, viewer string, goalIDs []string) (map[string][]Contribution, error) {
	out := map[string][]Contribution{}
	if len(goalIDs) == 0 {
		return out, nil
	}
	err := p.in(ctx, hh, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
			SELECT c.goal_id, `+contribCols+` FROM goal_contributions c
			JOIN goals g ON g.id = c.goal_id JOIN users u ON u.id = c.created_by
			WHERE c.goal_id = ANY($1) AND g.household_id = $2 AND `+fullTo("$3")+`
			ORDER BY c.goal_id, c.made_on, c.created_at, c.id`, goalIDs, hh, viewer)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var goalID string
			var c Contribution
			var madeOn time.Time
			if err := rows.Scan(&goalID, &c.ID, &c.AmountCents, &madeOn, &c.Note, &c.CreatedBy, &c.CreatedByName, &c.CreatedAt); err != nil {
				return err
			}
			c.MadeOn = fmtDay(madeOn)
			out[goalID] = append(out[goalID], c)
		}
		return rows.Err()
	})
	return out, err
}

// ---- marcações ----

func (p *PostgresRepo) AddCheckin(ctx context.Context, hh, actor, goalID, day string) (bool, error) {
	var created bool
	err := p.in(ctx, hh, func(tx *sql.Tx) error {
		l, err := lockGoal(ctx, tx, hh, actor, goalID)
		if err != nil {
			return err
		}
		if l.Kind != KindHabit && l.Kind != KindShared {
			return ErrKindMismatch
		}
		if l.closed() {
			return ErrClosed
		}
		res, err := tx.ExecContext(ctx, `
			INSERT INTO goal_checkins (goal_id, household_id, user_id, day) VALUES ($1, $2, $3, $4)
			ON CONFLICT (goal_id, user_id, day) DO NOTHING`, goalID, hh, actor, day)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		created = n > 0
		return nil
	})
	return created, err
}

func (p *PostgresRepo) RemoveCheckin(ctx context.Context, hh, actor, goalID, day string) error {
	return p.in(ctx, hh, func(tx *sql.Tx) error {
		l, err := lockGoal(ctx, tx, hh, actor, goalID)
		if err != nil {
			return err
		}
		if l.Kind != KindHabit && l.Kind != KindShared {
			return ErrKindMismatch
		}
		if l.closed() {
			return ErrClosed
		}
		// só a própria marcação; desfazer o que não existe não é erro
		_, err = tx.ExecContext(ctx,
			`DELETE FROM goal_checkins WHERE goal_id = $1 AND user_id = $2 AND day = $3`, goalID, actor, day)
		return err
	})
}

// Checkins devolve as marcações das metas visíveis no intervalo [from, to). É a base do progresso, que qualquer
// pessoa que enxerga a meta pode ver; o detalhe dia a dia só sai para quem tem AccessFull (o service confere).
func (p *PostgresRepo) Checkins(ctx context.Context, hh, viewer string, goalIDs []string, from, to time.Time) (map[string][]Checkin, error) {
	out := map[string][]Checkin{}
	if len(goalIDs) == 0 {
		return out, nil
	}
	err := p.in(ctx, hh, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
			SELECT c.goal_id, c.user_id, c.day FROM goal_checkins c
			JOIN goals g ON g.id = c.goal_id
			WHERE c.goal_id = ANY($1) AND g.household_id = $2 AND `+visibleTo("$3")+`
			  AND c.day >= $4 AND c.day < $5
			ORDER BY c.day, c.user_id`, goalIDs, hh, viewer, from, to)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id, user string
			var day time.Time
			if err := rows.Scan(&id, &user, &day); err != nil {
				return err
			}
			out[id] = append(out[id], Checkin{UserID: user, Day: fmtDay(day)})
		}
		return rows.Err()
	})
	return out, err
}

// ---- etapas ----

const stepCols = `id, goal_id, position, title, assignee_id, due_on, done_at, want_task, task_id`

func (p *PostgresRepo) AddStep(ctx context.Context, hh, actor, goalID string, s NewStep) (Step, error) {
	var out Step
	err := p.in(ctx, hh, func(tx *sql.Tx) error {
		l, err := lockGoal(ctx, tx, hh, actor, goalID)
		if err != nil {
			return err
		}
		if l.Kind != KindProject {
			return ErrKindMismatch
		}
		if l.closed() {
			return ErrClosed
		}
		var n, maxPos int
		if err := tx.QueryRowContext(ctx, `
			SELECT count(*), coalesce(max(position), 0) FROM goal_steps WHERE goal_id = $1 AND canceled_at IS NULL`,
			goalID).Scan(&n, &maxPos); err != nil {
			return err
		}
		if n >= MaxSteps {
			return ErrLimit
		}
		out, _, err = scanStep(tx.QueryRowContext(ctx, `
			INSERT INTO goal_steps (goal_id, household_id, position, title, assignee_id, due_on, want_task)
			VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING `+stepCols,
			goalID, hh, maxPos+1, s.Title, s.AssigneeID, s.DueOn, s.WantTask))
		return err
	})
	return out, err
}

func (p *PostgresRepo) GetStep(ctx context.Context, hh, goalID, stepID string) (Step, error) {
	var out Step
	err := p.in(ctx, hh, func(tx *sql.Tx) error {
		var err error
		out, _, err = scanStep(tx.QueryRowContext(ctx, `
			SELECT `+stepCols+` FROM goal_steps
			WHERE id = $1 AND goal_id = $2 AND household_id = $3 AND canceled_at IS NULL`, stepID, goalID, hh))
		return err
	})
	if isNoRows(err) {
		return Step{}, ErrStepNotFound
	}
	return out, err
}

// lockStep trava a meta (conferindo permissão e estado) e devolve a etapa ainda ativa.
func lockStep(ctx context.Context, tx *sql.Tx, hh, actor, goalID, stepID string) (Step, error) {
	l, err := lockGoal(ctx, tx, hh, actor, goalID)
	if err != nil {
		return Step{}, err
	}
	if l.Kind != KindProject {
		return Step{}, ErrKindMismatch
	}
	if l.closed() {
		return Step{}, ErrClosed
	}
	s, _, err := scanStep(tx.QueryRowContext(ctx, `
		SELECT `+stepCols+` FROM goal_steps
		WHERE id = $1 AND goal_id = $2 AND household_id = $3 AND canceled_at IS NULL FOR UPDATE`, stepID, goalID, hh))
	if isNoRows(err) {
		return Step{}, ErrStepNotFound
	}
	return s, err
}

func (p *PostgresRepo) UpdateStep(ctx context.Context, hh, actor, goalID, stepID string, pa StepPatch) error {
	return p.in(ctx, hh, func(tx *sql.Tx) error {
		if _, err := lockStep(ctx, tx, hh, actor, goalID, stepID); err != nil {
			return err
		}
		if pa.Title != nil {
			if _, err := tx.ExecContext(ctx, `UPDATE goal_steps SET title = $2 WHERE id = $1`, stepID, *pa.Title); err != nil {
				return err
			}
		}
		if pa.SetAssignee {
			if _, err := tx.ExecContext(ctx, `UPDATE goal_steps SET assignee_id = $2 WHERE id = $1`, stepID, pa.AssigneeID); err != nil {
				return err
			}
		}
		if pa.SetDue {
			if _, err := tx.ExecContext(ctx, `UPDATE goal_steps SET due_on = $2 WHERE id = $1`, stepID, pa.DueOn); err != nil {
				return err
			}
		}
		if pa.Position == nil {
			return nil
		}
		// reordena: tira a etapa da fila, encaixa na posição pedida e renumera todas de 1 em diante
		rows, err := tx.QueryContext(ctx, `
			SELECT id FROM goal_steps WHERE goal_id = $1 AND canceled_at IS NULL ORDER BY position, created_at`, goalID)
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
			if id != stepID {
				ids = append(ids, id)
			}
		}
		rows.Close()
		pos := *pa.Position - 1
		if pos < 0 {
			pos = 0
		}
		if pos > len(ids) {
			pos = len(ids)
		}
		ids = append(ids[:pos], append([]string{stepID}, ids[pos:]...)...)
		for i, id := range ids {
			if _, err := tx.ExecContext(ctx, `UPDATE goal_steps SET position = $2 WHERE id = $1`, id, i+1); err != nil {
				return err
			}
		}
		return nil
	})
}

func (p *PostgresRepo) CancelStep(ctx context.Context, hh, actor, goalID, stepID string) error {
	return p.in(ctx, hh, func(tx *sql.Tx) error {
		if _, err := lockStep(ctx, tx, hh, actor, goalID, stepID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `
			UPDATE goal_steps SET canceled_at = now(), want_task = false, task_id = NULL WHERE id = $1`, stepID)
		return err
	})
}

func (p *PostgresRepo) SetStepDone(ctx context.Context, hh, actor, goalID, stepID string, done bool) error {
	return p.in(ctx, hh, func(tx *sql.Tx) error {
		s, err := lockStep(ctx, tx, hh, actor, goalID, stepID)
		if err != nil {
			return err
		}
		if s.wantTask { // o estado de uma etapa com tarefa é o da tarefa
			return ErrStepHasTask
		}
		_, err = tx.ExecContext(ctx, `
			UPDATE goal_steps SET done_at = CASE WHEN $2 THEN coalesce(done_at, now()) END WHERE id = $1`, stepID, done)
		return err
	})
}

func (p *PostgresRepo) SetStepTask(ctx context.Context, hh, actor, goalID, stepID string, want bool, taskID *string, markDone bool) error {
	return p.in(ctx, hh, func(tx *sql.Tx) error {
		if _, err := lockStep(ctx, tx, hh, actor, goalID, stepID); err != nil {
			return err
		}
		// com tarefa, a etapa não guarda estado próprio: done_at fica nulo
		_, err := tx.ExecContext(ctx, `
			UPDATE goal_steps SET want_task = $2, task_id = $3,
				done_at = CASE WHEN $2 THEN NULL WHEN $4 THEN now() ELSE done_at END
			WHERE id = $1`, stepID, want, taskID, markDone)
		return err
	})
}

func (p *PostgresRepo) ReleaseStepTask(ctx context.Context, hh, goalID, stepID string) error {
	return p.in(ctx, hh, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			UPDATE goal_steps SET want_task = false, task_id = NULL
			WHERE id = $1 AND goal_id = $2 AND household_id = $3`, stepID, goalID, hh)
		return err
	})
}

func (p *PostgresRepo) AttachTask(ctx context.Context, hh, goalID, stepID, taskID string) error {
	return p.in(ctx, hh, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			UPDATE goal_steps SET task_id = $4
			WHERE id = $1 AND goal_id = $2 AND household_id = $3 AND want_task AND canceled_at IS NULL`,
			stepID, goalID, hh, taskID)
		return err
	})
}

// StepTaskRefs lista os ids das etapas com tarefa (todas, inclusive canceladas), para cancelar as tarefas ao apagar a meta.
func (p *PostgresRepo) StepTaskRefs(ctx context.Context, hh, goalID string) ([]string, error) {
	var out []string
	err := p.in(ctx, hh, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `SELECT id FROM goal_steps WHERE goal_id = $1 AND household_id = $2`, goalID, hh)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return err
			}
			out = append(out, id)
		}
		return rows.Err()
	})
	return out, err
}

// ---- avisos ----

// ClaimSharedNotice reserva o aviso do dia: só a primeira marcação do dia (em toda a meta) avisa o parceiro.
func (p *PostgresRepo) ClaimSharedNotice(ctx context.Context, hh, goalID, day string) (bool, error) {
	var n int64
	err := p.in(ctx, hh, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `
			UPDATE goal_shared SET notified_on = $3
			WHERE goal_id = $1 AND household_id = $2 AND (notified_on IS NULL OR notified_on < $3)`, goalID, hh, day)
		if err != nil {
			return err
		}
		n, _ = res.RowsAffected()
		return nil
	})
	return n > 0, err
}

func (p *PostgresRepo) ClaimAllDone(ctx context.Context, hh, goalID string) (bool, error) {
	var n int64
	err := p.in(ctx, hh, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `
			UPDATE goals SET all_done_notified_at = now()
			WHERE id = $1 AND household_id = $2 AND all_done_notified_at IS NULL`, goalID, hh)
		if err != nil {
			return err
		}
		n, _ = res.RowsAffected()
		return nil
	})
	return n > 0, err
}

func (p *PostgresRepo) ClearAllDone(ctx context.Context, hh, goalID string) error {
	return p.in(ctx, hh, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			UPDATE goals SET all_done_notified_at = NULL
			WHERE id = $1 AND household_id = $2 AND all_done_notified_at IS NOT NULL`, goalID, hh)
		return err
	})
}

// ---- jobs: escopo de sistema, só casais com o módulo ligado ----

func (p *PostgresRepo) HabitsForReminders(ctx context.Context, after string, limit int) ([]Goal, error) {
	var out []Goal
	err := p.system(ctx, func(tx *sql.Tx) error {
		var err error
		out, err = queryGoals(ctx, tx, "", `SELECT `+goalCols+goalFrom+`
			JOIN household_modules m ON m.household_id = g.household_id AND m.module = 'goals'
			JOIN household_members hm ON hm.household_id = g.household_id AND hm.user_id = g.owner_id
			WHERE g.kind = 'habit' AND g.status = 'active' AND h.remind_enabled AND g.id > $1
			ORDER BY g.id LIMIT $2`, after, limit)
		return err
	})
	return out, err
}

// HabitsOfHousehold lista os hábitos com lembrete ligado, em qualquer estado (para cancelar os lembretes ao
// desligar o módulo). Ignora o módulo ligado de propósito: é chamado justamente depois de desligá-lo.
func (p *PostgresRepo) HabitsOfHousehold(ctx context.Context, hh string) ([]Goal, error) {
	var out []Goal
	err := p.in(ctx, hh, func(tx *sql.Tx) error {
		var err error
		out, err = queryGoals(ctx, tx, "", `SELECT `+goalCols+goalFrom+`
			WHERE g.household_id = $1 AND g.kind = 'habit' AND h.remind_enabled`, hh)
		return err
	})
	return out, err
}

func (p *PostgresRepo) PendingSteps(ctx context.Context, after string, limit int) ([]PendingStep, error) {
	var out []PendingStep
	err := p.system(ctx, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
			SELECT s.household_id, s.goal_id, s.id, g.title, g.created_by
			FROM goal_steps s JOIN goals g ON g.id = s.goal_id
			JOIN household_modules m ON m.household_id = g.household_id AND m.module = 'goals'
			WHERE s.want_task AND s.task_id IS NULL AND s.canceled_at IS NULL
			  AND g.scope = 'household' AND g.status IN ('active', 'paused') AND s.id > $1
			ORDER BY s.id LIMIT $2`, after, limit)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var ps PendingStep
			if err := rows.Scan(&ps.HouseholdID, &ps.GoalID, &ps.StepID, &ps.GoalTitle, &ps.CreatedBy); err != nil {
				return err
			}
			out = append(out, ps)
		}
		return rows.Err()
	})
	return out, err
}
