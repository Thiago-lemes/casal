package goal

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"time"

	"casal/internal/db"
)

// NewStep é uma etapa a gravar junto com a meta (ou depois).
type NewStep struct {
	Title      string
	AssigneeID *string
	DueOn      *string
	WantTask   bool
}

// NewGoal é o que o service entrega ao repository para criar: a meta com os parâmetros do tipo e as etapas.
type NewGoal struct {
	Goal  Goal
	Steps []NewStep
}

// GoalPatch troca título, notas, prazo e visibilidade, e os parâmetros que podem mudar em cada tipo.
type GoalPatch struct {
	Version    int
	Title      string
	Notes      string
	DueOn      *string
	Visibility string

	TargetCents *int64  // financeira
	Habit       *Habit  // hábito: alvo, dias e lembrete
	Shared      *Shared // do casal: alvo e requires_both
}

// StepPatch muda uma etapa; ponteiro nulo deixa o campo como está.
type StepPatch struct {
	Title       *string
	SetAssignee bool
	AssigneeID  *string
	SetDue      bool
	DueOn       *string
	Position    *int
}

// PendingStep é uma etapa que quer tarefa e ainda não tem (a tarefa será refeita pelo job).
type PendingStep struct {
	HouseholdID string
	GoalID      string
	StepID      string
	GoalTitle   string
	CreatedBy   string
}

type Repository interface {
	CreateGoal(ctx context.Context, n NewGoal) (Goal, error)
	// GetGoal só devolve a meta que a pessoa pode ver (visibleTo); as outras são ErrNotFound.
	GetGoal(ctx context.Context, hh, viewer, id string) (Goal, error)
	// GetGoalSystem lê qualquer meta do casal, para avisos e jobs: nunca use numa resposta a uma pessoa.
	GetGoalSystem(ctx context.Context, hh, id string) (Goal, error)
	ListGoals(ctx context.Context, hh, viewer string, f ListFilter) ([]Goal, error)
	CountActive(ctx context.Context, hh, viewer string) (int, error)
	UpdateGoal(ctx context.Context, hh, actor, id string, p GoalPatch) error
	SetStatus(ctx context.Context, hh, actor, id string, from []string, to string) error
	DeleteGoal(ctx context.Context, hh, actor, id string) error

	AddContribution(ctx context.Context, hh, actor, goalID string, c Contribution) (Contribution, error)
	ListContributions(ctx context.Context, hh, viewer, goalID, before string, limit int) ([]Contribution, error)
	DeleteContribution(ctx context.Context, hh, actor, goalID, id string) error
	SumContributions(ctx context.Context, hh, viewer string, goalIDs []string) (map[string]int64, error)
	AllContributions(ctx context.Context, hh, viewer string, goalIDs []string) (map[string][]Contribution, error)

	AddCheckin(ctx context.Context, hh, actor, goalID, day string) (created bool, err error)
	RemoveCheckin(ctx context.Context, hh, actor, goalID, day string) error
	Checkins(ctx context.Context, hh, viewer string, goalIDs []string, from, to time.Time) (map[string][]Checkin, error)

	AddStep(ctx context.Context, hh, actor, goalID string, s NewStep) (Step, error)
	GetStep(ctx context.Context, hh, goalID, stepID string) (Step, error)
	UpdateStep(ctx context.Context, hh, actor, goalID, stepID string, p StepPatch) error
	CancelStep(ctx context.Context, hh, actor, goalID, stepID string) error
	SetStepDone(ctx context.Context, hh, actor, goalID, stepID string, done bool) error
	// SetStepTask liga (want = true) ou desliga a tarefa da etapa. Desligando, markDone leva a etapa a concluída.
	SetStepTask(ctx context.Context, hh, actor, goalID, stepID string, want bool, taskID *string, markDone bool) error
	// ReleaseStepTask solta a tarefa da etapa quando ela foi cancelada por fora (a etapa continua, aberta).
	ReleaseStepTask(ctx context.Context, hh, goalID, stepID string) error
	// AttachTask grava o id da tarefa criada, só se a etapa ainda a quer.
	AttachTask(ctx context.Context, hh, goalID, stepID, taskID string) error
	StepTaskRefs(ctx context.Context, hh, goalID string) ([]string, error)

	// avisos
	ClaimSharedNotice(ctx context.Context, hh, goalID, day string) (bool, error)
	ClaimAllDone(ctx context.Context, hh, goalID string) (bool, error)
	ClearAllDone(ctx context.Context, hh, goalID string) error

	// jobs (escopo de sistema, só casais com o módulo ligado)
	HabitsForReminders(ctx context.Context, after string, limit int) ([]Goal, error)
	HabitsOfHousehold(ctx context.Context, hh string) ([]Goal, error)
	PendingSteps(ctx context.Context, after string, limit int) ([]PendingStep, error)
}

type PostgresRepo struct{ db *sql.DB }

func NewPostgresRepo(db *sql.DB) *PostgresRepo { return &PostgresRepo{db: db} }

func (p *PostgresRepo) in(ctx context.Context, hh string, fn func(tx *sql.Tx) error) error {
	return db.InTx(ctx, p.db, db.Scope{HouseholdID: hh}, fn)
}

func (p *PostgresRepo) system(ctx context.Context, fn func(tx *sql.Tx) error) error {
	return db.InTx(ctx, p.db, db.Scope{System: true}, fn)
}

func isNoRows(err error) bool { return errors.Is(err, sql.ErrNoRows) }

// visibleTo é a única trava de privacidade entre os parceiros (o RLS só separa um casal do outro).
// A meta do casal é dos dois; a pessoal é do dono, e o parceiro só a enxerga se for "view".
// Todo SQL de leitura de meta usa este trecho, e as tabelas filhas sempre entram juntadas a goals.
// $n é o id de quem pergunta.
func visibleTo(n string) string {
	return `(g.scope = 'household' OR g.owner_id = ` + n + `::uuid OR g.visibility = 'view')`
}

// writableBy: pode alterar quem é dono, ou qualquer um dos dois numa meta do casal.
func writableBy(n string) string {
	return `(g.scope = 'household' OR g.owner_id = ` + n + `::uuid)`
}

// fullTo: pode ver o detalhe (notas, aportes, marcações, etapas) quem é dono, ou numa meta do casal.
func fullTo(n string) string { return writableBy(n) }

// ---- leitura de metas ----

const goalCols = `g.id, g.household_id, g.created_by, g.owner_id, g.scope, g.visibility, g.kind, g.title, g.notes,
	g.status, g.due_on, g.completed_at, g.completed_by, g.version, g.created_at, g.updated_at,
	f.target_cents, f.source, f.category_id, f.baseline_on,
	h.target_count, h.period, h.weekdays, h.remind_min, h.remind_enabled,
	s.target_count, s.period, s.requires_both`

const goalFrom = ` FROM goals g
	LEFT JOIN goal_financial f ON f.goal_id = g.id
	LEFT JOIN goal_habit h ON h.goal_id = g.id
	LEFT JOIN goal_shared s ON s.goal_id = g.id `

type scanner interface{ Scan(dest ...any) error }

func scanGoal(r scanner, viewer string) (Goal, error) {
	var g Goal
	var owner, completedBy, category sql.NullString
	var due, baseline sql.NullTime
	var completedAt sql.NullTime
	var fTarget, hCount, hWeekdays, hRemind, sCount sql.NullInt64
	var fSource, hPeriod, sPeriod sql.NullString
	var hEnabled, sBoth sql.NullBool
	err := r.Scan(&g.ID, &g.HouseholdID, &g.CreatedBy, &owner, &g.Scope, &g.Visibility, &g.Kind, &g.Title, &g.Notes,
		&g.Status, &due, &completedAt, &completedBy, &g.Version, &g.CreatedAt, &g.UpdatedAt,
		&fTarget, &fSource, &category, &baseline,
		&hCount, &hPeriod, &hWeekdays, &hRemind, &hEnabled,
		&sCount, &sPeriod, &sBoth)
	if err != nil {
		return Goal{}, err
	}
	if owner.Valid {
		g.OwnerID = &owner.String
	}
	if due.Valid {
		d := fmtDay(due.Time)
		g.DueOn = &d
	}
	if completedAt.Valid {
		g.CompletedAt = &completedAt.Time
	}
	if completedBy.Valid {
		g.CompletedBy = &completedBy.String
	}
	if fTarget.Valid {
		f := &Financial{TargetCents: fTarget.Int64, Source: fSource.String, BaselineOn: fmtDay(baseline.Time)}
		if category.Valid {
			f.CategoryID = &category.String
		}
		g.Financial = f
	}
	if hCount.Valid {
		g.Habit = &Habit{TargetCount: int(hCount.Int64), Period: hPeriod.String, Weekdays: int(hWeekdays.Int64),
			RemindMin: int(hRemind.Int64), RemindEnabled: hEnabled.Bool}
	}
	if sCount.Valid {
		g.Shared = &Shared{TargetCount: int(sCount.Int64), Period: sPeriod.String, RequiresBoth: sBoth.Bool}
	}
	g.Access = AccessView
	if g.Scope == ScopeHousehold || (g.OwnerID != nil && *g.OwnerID == viewer) {
		g.Access = AccessFull
	}
	g.CanEdit = g.Access == AccessFull
	return g, nil
}

func queryGoals(ctx context.Context, tx *sql.Tx, viewer, query string, args ...any) ([]Goal, error) {
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Goal{}
	for rows.Next() {
		g, err := scanGoal(rows, viewer)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (p *PostgresRepo) GetGoal(ctx context.Context, hh, viewer, id string) (Goal, error) {
	var out Goal
	err := p.in(ctx, hh, func(tx *sql.Tx) error {
		gs, err := queryGoals(ctx, tx, viewer, `SELECT `+goalCols+goalFrom+`
			WHERE g.id = $1 AND g.household_id = $2 AND `+visibleTo("$3"), id, hh, viewer)
		if err != nil {
			return err
		}
		if len(gs) == 0 {
			return ErrNotFound
		}
		out = gs[0]
		return attachSteps(ctx, tx, hh, []*Goal{&out})
	})
	return out, err
}

func (p *PostgresRepo) GetGoalSystem(ctx context.Context, hh, id string) (Goal, error) {
	var out Goal
	err := p.in(ctx, hh, func(tx *sql.Tx) error {
		gs, err := queryGoals(ctx, tx, "", `SELECT `+goalCols+goalFrom+`
			WHERE g.id = $1 AND g.household_id = $2`, id, hh)
		if err != nil {
			return err
		}
		if len(gs) == 0 {
			return ErrNotFound
		}
		out = gs[0]
		return attachSteps(ctx, tx, hh, []*Goal{&out})
	})
	return out, err
}

func (p *PostgresRepo) ListGoals(ctx context.Context, hh, viewer string, f ListFilter) ([]Goal, error) {
	var out []Goal
	err := p.in(ctx, hh, func(tx *sql.Tx) error {
		q := `SELECT ` + goalCols + goalFrom + `WHERE g.household_id = $1 AND ` + visibleTo("$2")
		args := []any{hh, viewer}
		switch f.Scope {
		case ScopeHousehold:
			q += ` AND g.scope = 'household'`
		case "mine":
			q += ` AND g.scope = 'personal' AND g.owner_id = $2`
		}
		if f.Kind != "" {
			args = append(args, f.Kind)
			q += ` AND g.kind = $` + itoa(len(args))
		}
		switch f.Status {
		case "":
			q += ` AND g.status <> 'archived'`
		case "all":
		default:
			args = append(args, f.Status)
			q += ` AND g.status = $` + itoa(len(args))
		}
		q += ` ORDER BY (g.status = 'active') DESC, g.due_on NULLS LAST, g.created_at DESC LIMIT 500`
		gs, err := queryGoals(ctx, tx, viewer, q, args...)
		if err != nil {
			return err
		}
		ptrs := make([]*Goal, len(gs))
		for i := range gs {
			ptrs[i] = &gs[i]
		}
		out = gs
		return attachSteps(ctx, tx, hh, ptrs)
	})
	return out, err
}

func itoa(n int) string { return strconv.Itoa(n) }

// attachSteps carrega as etapas (as não canceladas) dos projetos da lista, em ordem.
func attachSteps(ctx context.Context, tx *sql.Tx, hh string, goals []*Goal) error {
	var ids []string
	byID := map[string]*Goal{}
	for _, g := range goals {
		if g.Kind == KindProject {
			ids = append(ids, g.ID)
			byID[g.ID] = g
		}
	}
	if len(ids) == 0 {
		return nil
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT id, goal_id, position, title, assignee_id, due_on, done_at, want_task, task_id
		FROM goal_steps
		WHERE household_id = $1 AND goal_id = ANY($2) AND canceled_at IS NULL
		ORDER BY goal_id, position, created_at`, hh, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		s, goalID, err := scanStep(rows)
		if err != nil {
			return err
		}
		byID[goalID].Steps = append(byID[goalID].Steps, s)
	}
	return rows.Err()
}

func scanStep(r scanner) (Step, string, error) {
	var s Step
	var goalID string
	var assignee, taskID sql.NullString
	var due, doneAt sql.NullTime
	if err := r.Scan(&s.ID, &goalID, &s.Position, &s.Title, &assignee, &due, &doneAt, &s.wantTask, &taskID); err != nil {
		return Step{}, "", err
	}
	s.goalID = goalID
	if assignee.Valid {
		s.AssigneeID = &assignee.String
		s.rawAssignee = &assignee.String
	}
	if due.Valid {
		d := fmtDay(due.Time)
		s.DueOn = &d
	}
	if doneAt.Valid {
		s.rawDone = &doneAt.Time
		s.DoneAt = &doneAt.Time
		s.Done = true
	}
	if taskID.Valid {
		s.taskID = taskID.String
	}
	return s, goalID, nil
}

func (p *PostgresRepo) CountActive(ctx context.Context, hh, viewer string) (int, error) {
	var n int
	err := p.in(ctx, hh, func(tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `
			SELECT count(*) FROM goals g
			WHERE g.household_id = $1 AND g.status IN ('active', 'paused')
			  AND (g.scope = 'household' OR g.owner_id = $2)`, hh, viewer).Scan(&n)
	})
	return n, err
}

// ---- escrita de metas ----

func (p *PostgresRepo) CreateGoal(ctx context.Context, n NewGoal) (Goal, error) {
	g := n.Goal
	var id string
	err := p.in(ctx, g.HouseholdID, func(tx *sql.Tx) error {
		// dois celulares criando ao mesmo tempo não furam o teto
		if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, "goals:"+g.HouseholdID); err != nil {
			return err
		}
		var count int
		if err := tx.QueryRowContext(ctx, `
			SELECT count(*) FROM goals g WHERE g.household_id = $1 AND g.status IN ('active', 'paused')
			  AND (g.scope = 'household' OR g.owner_id = $2)`, g.HouseholdID, g.CreatedBy).Scan(&count); err != nil {
			return err
		}
		if count >= MaxActiveGoals {
			return ErrLimit
		}
		if err := tx.QueryRowContext(ctx, `
			INSERT INTO goals (household_id, created_by, owner_id, scope, visibility, kind, title, notes, due_on)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id`,
			g.HouseholdID, g.CreatedBy, g.OwnerID, g.Scope, g.Visibility, g.Kind, g.Title, g.Notes, g.DueOn).Scan(&id); err != nil {
			return err
		}
		switch {
		case g.Financial != nil:
			f := g.Financial
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO goal_financial (goal_id, household_id, target_cents, source, category_id, baseline_on)
				VALUES ($1, $2, $3, $4, $5, $6)`, id, g.HouseholdID, f.TargetCents, f.Source, f.CategoryID, f.BaselineOn); err != nil {
				return err
			}
		case g.Habit != nil:
			h := g.Habit
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO goal_habit (goal_id, household_id, target_count, period, weekdays, remind_min, remind_enabled)
				VALUES ($1, $2, $3, $4, $5, $6, $7)`, id, g.HouseholdID, h.TargetCount, h.Period, h.Weekdays, h.RemindMin, h.RemindEnabled); err != nil {
				return err
			}
		case g.Shared != nil:
			s := g.Shared
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO goal_shared (goal_id, household_id, target_count, period, requires_both)
				VALUES ($1, $2, $3, $4, $5)`, id, g.HouseholdID, s.TargetCount, s.Period, s.RequiresBoth); err != nil {
				return err
			}
		}
		for i, st := range n.Steps {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO goal_steps (goal_id, household_id, position, title, assignee_id, due_on, want_task)
				VALUES ($1, $2, $3, $4, $5, $6, $7)`, id, g.HouseholdID, i+1, st.Title, st.AssigneeID, st.DueOn, st.WantTask); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return Goal{}, err
	}
	return p.GetGoal(ctx, g.HouseholdID, g.CreatedBy, id)
}

// failedWrite explica por que um UPDATE/DELETE não tocou nenhuma linha: a meta não existe (ou não é visível),
// a pessoa só enxerga, ou a versão está velha.
func failedWrite(ctx context.Context, tx *sql.Tx, hh, actor, id string, version int) error {
	var writable bool
	var current int
	var status string
	err := tx.QueryRowContext(ctx, `
		SELECT `+writableBy("$3")+`, g.version, g.status FROM goals g
		WHERE g.id = $1 AND g.household_id = $2 AND `+visibleTo("$3"), id, hh, actor).Scan(&writable, &current, &status)
	if isNoRows(err) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if !writable {
		return ErrForbidden
	}
	if version != 0 && (status == StatusDone || status == StatusArchived) {
		return ErrClosed // só o PUT passa version: concluída e arquivada se reabrem antes de editar
	}
	if version != 0 && current != version {
		return ErrConflict
	}
	return ErrNotFound
}

func (p *PostgresRepo) UpdateGoal(ctx context.Context, hh, actor, id string, pa GoalPatch) error {
	return p.in(ctx, hh, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `
			UPDATE goals g SET title = $5, notes = $6, due_on = $7, visibility = $8,
				version = version + 1, updated_at = now()
			WHERE g.id = $1 AND g.household_id = $2 AND g.version = $4 AND g.status NOT IN ('done', 'archived') AND `+writableBy("$3"),
			id, hh, actor, pa.Version, pa.Title, pa.Notes, pa.DueOn, pa.Visibility)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return failedWrite(ctx, tx, hh, actor, id, pa.Version)
		}
		if pa.TargetCents != nil {
			if _, err := tx.ExecContext(ctx, `UPDATE goal_financial SET target_cents = $2 WHERE goal_id = $1`, id, *pa.TargetCents); err != nil {
				return err
			}
		}
		if h := pa.Habit; h != nil {
			if _, err := tx.ExecContext(ctx, `
				UPDATE goal_habit SET target_count = $2, weekdays = $3, remind_min = $4, remind_enabled = $5
				WHERE goal_id = $1`, id, h.TargetCount, h.Weekdays, h.RemindMin, h.RemindEnabled); err != nil {
				return err
			}
		}
		if s := pa.Shared; s != nil {
			if _, err := tx.ExecContext(ctx, `
				UPDATE goal_shared SET target_count = $2, requires_both = $3 WHERE goal_id = $1`,
				id, s.TargetCount, s.RequiresBoth); err != nil {
				return err
			}
		}
		return nil
	})
}

func (p *PostgresRepo) SetStatus(ctx context.Context, hh, actor, id string, from []string, to string) error {
	return p.in(ctx, hh, func(tx *sql.Tx) error {
		if to == StatusActive {
			// reabrir e desarquivar também respeitam o teto, sob o mesmo trinco da criação
			if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, "goals:"+hh); err != nil {
				return err
			}
			var n int
			if err := tx.QueryRowContext(ctx, `
				SELECT count(*) FROM goals g WHERE g.household_id = $1 AND g.status IN ('active', 'paused')
				  AND (g.scope = 'household' OR g.owner_id = $2)`, hh, actor).Scan(&n); err != nil {
				return err
			}
			var reopening bool
			if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM goals WHERE id = $1 AND household_id = $2 AND status IN ('done', 'archived'))`, id, hh).Scan(&reopening); err != nil {
				return err
			}
			if reopening && n >= MaxActiveGoals {
				return ErrLimit
			}
		}
		res, err := tx.ExecContext(ctx, `
			UPDATE goals g SET status = $4,
				completed_at = CASE WHEN $4 = 'done' THEN now() WHEN $4 = 'archived' THEN g.completed_at END,
				completed_by = CASE WHEN $4 = 'done' THEN $3::uuid WHEN $4 = 'archived' THEN g.completed_by END,
				updated_at = now()
			WHERE g.id = $1 AND g.household_id = $2 AND g.status = ANY($5) AND `+writableBy("$3"),
			id, hh, actor, to, from)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return failedWrite(ctx, tx, hh, actor, id, 0)
		}
		return nil
	})
}

func (p *PostgresRepo) DeleteGoal(ctx context.Context, hh, actor, id string) error {
	return p.in(ctx, hh, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `
			DELETE FROM goals g WHERE g.id = $1 AND g.household_id = $2 AND `+writableBy("$3"), id, hh, actor)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return failedWrite(ctx, tx, hh, actor, id, 0)
		}
		return nil
	})
}

// locked é o estado de uma meta travada (FOR UPDATE) no começo de uma escrita em tabela filha.
type locked struct {
	Kind, Status, Scope, Source string
	OwnerID                     *string
}

// lockGoal trava a meta e confere, no banco, que a pessoa a enxerga e pode escrever nela.
func lockGoal(ctx context.Context, tx *sql.Tx, hh, actor, id string) (locked, error) {
	var l locked
	var owner sql.NullString
	var writable bool
	err := tx.QueryRowContext(ctx, `
		SELECT g.kind, g.status, g.scope, g.owner_id, coalesce(f.source, ''), `+writableBy("$3")+`
		FROM goals g LEFT JOIN goal_financial f ON f.goal_id = g.id
		WHERE g.id = $1 AND g.household_id = $2 AND `+visibleTo("$3")+`
		FOR UPDATE OF g`, id, hh, actor).Scan(&l.Kind, &l.Status, &l.Scope, &owner, &l.Source, &writable)
	if isNoRows(err) {
		return l, ErrNotFound
	}
	if err != nil {
		return l, err
	}
	if owner.Valid {
		l.OwnerID = &owner.String
	}
	if !writable {
		return l, ErrForbidden
	}
	return l, nil
}

func (l locked) closed() bool { return l.Status == StatusDone || l.Status == StatusArchived }
