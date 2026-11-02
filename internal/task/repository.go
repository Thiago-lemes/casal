package task

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"casal/internal/apperr"
	"casal/internal/db"
)

var (
	ErrInvalidInput = apperr.New(apperr.Invalid, "dados inválidos")
	ErrNotFound     = apperr.NewCoded(apperr.NotFound, "task_not_found", "tarefa não encontrada")
)

const (
	StatusOpen     = "open"
	StatusDone     = "done"
	StatusDeclined = "declined"
	StatusCanceled = "canceled"

	RepeatNone    = "none"
	RepeatDaily   = "daily"
	RepeatWeekly  = "weekly"
	RepeatMonthly = "monthly"
)

type Task struct {
	ID            string     `json:"id"`
	HouseholdID   string     `json:"-"`
	CreatedBy     string     `json:"created_by"`
	CreatedByName string     `json:"created_by_name"`
	AssigneeID    *string    `json:"assignee_id"` // nulo: tarefa do casal
	Title         string     `json:"title"`
	DueAt         *time.Time `json:"due_at,omitempty"`
	Status        string     `json:"status"`
	Repeat        string     `json:"repeat"`   // none, daily, weekly ou monthly
	Rotation      bool       `json:"rotation"` // rodízio: a cada ciclo o responsável passa para a outra pessoa
	DeclineReason *string    `json:"decline_reason,omitempty"`
	CompletedBy   *string    `json:"completed_by,omitempty"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

type Repository interface {
	Create(ctx context.Context, t Task) (Task, error)
	Get(ctx context.Context, householdID, id string) (Task, error)
	List(ctx context.Context, householdID, status string) ([]Task, error)
	// Complete conclui a tarefa e, se next não for nulo, cria a próxima ocorrência na mesma transação:
	// ou as duas coisas acontecem, ou nenhuma. next só é criada se a conclusão valer.
	Complete(ctx context.Context, householdID, id, userID string, now time.Time, next *Task) (done Task, created *Task, err error)
	Decline(ctx context.Context, householdID, id, userID, reason string) (Task, error)
	Reassign(ctx context.Context, householdID, id, userID string, assigneeID *string) (Task, error)
	Cancel(ctx context.Context, householdID, id, userID string) error
}

type PostgresRepo struct{ db *sql.DB }

func NewPostgresRepo(db *sql.DB) *PostgresRepo { return &PostgresRepo{db: db} }

// Colunas lidas em todas as consultas. O JOIN em users é só leitura, para mostrar o nome.
const cols = `t.id, t.household_id, t.created_by, u.name, t.assignee_id, t.title, t.due_at,
	t.status, t.recurrence, t.rotation, t.decline_reason, t.completed_by, t.completed_at, t.created_at`

type scanner interface{ Scan(dest ...any) error }

func scan(s scanner) (Task, error) {
	var t Task
	var assignee, reason, completedBy sql.NullString
	var due, completedAt sql.NullTime
	err := s.Scan(&t.ID, &t.HouseholdID, &t.CreatedBy, &t.CreatedByName, &assignee, &t.Title, &due,
		&t.Status, &t.Repeat, &t.Rotation, &reason, &completedBy, &completedAt, &t.CreatedAt)
	if err != nil {
		return Task{}, err
	}
	if assignee.Valid {
		t.AssigneeID = &assignee.String
	}
	if due.Valid {
		t.DueAt = &due.Time
	}
	if reason.Valid {
		t.DeclineReason = &reason.String
	}
	if completedBy.Valid {
		t.CompletedBy = &completedBy.String
	}
	if completedAt.Valid {
		t.CompletedAt = &completedAt.Time
	}
	return t, nil
}

func (p *PostgresRepo) Create(ctx context.Context, in Task) (Task, error) {
	var out Task
	err := db.InTx(ctx, p.db, db.Scope{HouseholdID: in.HouseholdID}, func(tx *sql.Tx) error {
		var err error
		out, err = insert(ctx, tx, in)
		return err
	})
	return out, err
}

func insert(ctx context.Context, tx *sql.Tx, in Task) (Task, error) {
	if in.Repeat == "" {
		in.Repeat = RepeatNone
	}
	return scan(tx.QueryRowContext(ctx, `
		WITH ins AS (
			INSERT INTO tasks (household_id, created_by, assignee_id, title, due_at, recurrence, rotation)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			RETURNING *
		)
		SELECT `+cols+` FROM ins t JOIN users u ON u.id = t.created_by`,
		in.HouseholdID, in.CreatedBy, in.AssigneeID, in.Title, in.DueAt, in.Repeat, in.Rotation))
}

func (p *PostgresRepo) Get(ctx context.Context, householdID, id string) (Task, error) {
	var out Task
	err := db.InTx(ctx, p.db, db.Scope{HouseholdID: householdID}, func(tx *sql.Tx) error {
		var err error
		out, err = scan(tx.QueryRowContext(ctx, `
			SELECT `+cols+` FROM tasks t JOIN users u ON u.id = t.created_by
			WHERE t.id = $1 AND t.household_id = $2`, id, householdID))
		return err
	})
	if errors.Is(err, sql.ErrNoRows) {
		return Task{}, ErrNotFound
	}
	return out, err
}

// List devolve as tarefas do casal com o status pedido. Todas são visíveis aos dois.
func (p *PostgresRepo) List(ctx context.Context, householdID, status string) ([]Task, error) {
	order := `t.due_at NULLS LAST, t.created_at DESC`
	if status == StatusDone {
		order = `t.completed_at DESC`
	}
	out := []Task{}
	err := db.InTx(ctx, p.db, db.Scope{HouseholdID: householdID}, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
			SELECT `+cols+`
			FROM tasks t JOIN users u ON u.id = t.created_by
			WHERE t.household_id = $1 AND t.status = $2
			ORDER BY `+order+`
			LIMIT 200`, householdID, status)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			t, err := scan(rows)
			if err != nil {
				return err
			}
			out = append(out, t)
		}
		return rows.Err()
	})
	return out, err
}

// Conclui quem é o responsável; numa tarefa do casal, qualquer um dos dois. Quem criou também
// pode concluir (é a confirmação do que foi atribuído ao parceiro).
func (p *PostgresRepo) Complete(ctx context.Context, householdID, id, userID string, now time.Time, next *Task) (Task, *Task, error) {
	var done Task
	var created *Task
	err := db.InTx(ctx, p.db, db.Scope{HouseholdID: householdID}, func(tx *sql.Tx) error {
		var err error
		done, err = scan(tx.QueryRowContext(ctx, `
			WITH upd AS (
				UPDATE tasks SET status = 'done', completed_by = $3, completed_at = $4
				WHERE id = $1 AND household_id = $2 AND status = 'open'
				  AND (assignee_id IS NULL OR assignee_id = $3 OR created_by = $3)
				RETURNING *
			)
			SELECT `+cols+` FROM upd t JOIN users u ON u.id = t.created_by`,
			id, householdID, userID, now))
		if err != nil || next == nil {
			return err
		}
		n := *next
		n.HouseholdID = householdID
		made, err := insert(ctx, tx, n)
		if err != nil {
			return err
		}
		created = &made
		return nil
	})
	if errors.Is(err, sql.ErrNoRows) {
		return Task{}, nil, ErrNotFound
	}
	if err != nil {
		return Task{}, nil, err
	}
	return done, created, nil
}

// Só o responsável recusa, e não a própria tarefa.
func (p *PostgresRepo) Decline(ctx context.Context, householdID, id, userID, reason string) (Task, error) {
	return p.update(ctx, householdID, `
		WITH upd AS (
			UPDATE tasks SET status = 'declined', decline_reason = $4
			WHERE id = $1 AND household_id = $2 AND status = 'open'
			  AND assignee_id = $3 AND created_by <> $3
			RETURNING *
		)
		SELECT `+cols+` FROM upd t JOIN users u ON u.id = t.created_by`,
		id, householdID, userID, reason)
}

// Só quem criou reatribui. A tarefa volta a ficar aberta e a justificativa da recusa sai.
func (p *PostgresRepo) Reassign(ctx context.Context, householdID, id, userID string, assigneeID *string) (Task, error) {
	return p.update(ctx, householdID, `
		WITH upd AS (
			UPDATE tasks SET assignee_id = $4, status = 'open', decline_reason = NULL,
				rotation = rotation AND $4::uuid IS NOT NULL
			WHERE id = $1 AND household_id = $2 AND created_by = $3
			  AND status IN ('open', 'declined')
			RETURNING *
		)
		SELECT `+cols+` FROM upd t JOIN users u ON u.id = t.created_by`,
		id, householdID, userID, assigneeID)
}

// update roda um UPDATE ... RETURNING dentro do escopo do casal; sem linha, é ErrNotFound.
func (p *PostgresRepo) update(ctx context.Context, householdID, query string, args ...any) (Task, error) {
	var out Task
	err := db.InTx(ctx, p.db, db.Scope{HouseholdID: householdID}, func(tx *sql.Tx) error {
		var err error
		out, err = scan(tx.QueryRowContext(ctx, query, args...))
		return err
	})
	if errors.Is(err, sql.ErrNoRows) {
		return Task{}, ErrNotFound
	}
	return out, err
}

// Só quem criou cancela.
func (p *PostgresRepo) Cancel(ctx context.Context, householdID, id, userID string) error {
	return db.InTx(ctx, p.db, db.Scope{HouseholdID: householdID}, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `
			UPDATE tasks SET status = 'canceled'
			WHERE id = $1 AND household_id = $2 AND created_by = $3
			  AND status IN ('open', 'declined')`, id, householdID, userID)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrNotFound
		}
		return nil
	})
}

// HandleLeave roda quando alguém sai do casal: as tarefas dela ainda abertas ou recusadas (que criou ou
// recebeu) passam para quem ficou, que assume o que faltava. O que já foi concluído fica como histórico.
// Uma tarefa recusada cujo responsável passa a ser quem a criou reabre, pois ninguém recusa a própria tarefa.
// Sem ninguém que fique, não há a quem passar. Roda na transação da saída.
func (p *PostgresRepo) HandleLeave(ctx context.Context, tx *sql.Tx, h db.Handoff) error {
	if h.RemainingUserID == "" {
		return nil
	}
	if err := db.SetScope(ctx, tx, db.Scope{System: true}); err != nil {
		return err
	}
	for _, q := range []string{
		`UPDATE tasks SET created_by = $3 WHERE household_id = $1 AND created_by = $2 AND status IN ('open', 'declined')`,
		`UPDATE tasks SET assignee_id = $3 WHERE household_id = $1 AND assignee_id = $2 AND status IN ('open', 'declined')`,
	} {
		if _, err := tx.ExecContext(ctx, q, h.From, h.UserID, h.RemainingUserID); err != nil {
			return err
		}
	}
	// recusada e agora com o mesmo responsável e criador: ninguém recusa a própria tarefa, ela reabre
	_, err := tx.ExecContext(ctx, `
		UPDATE tasks SET status = 'open', decline_reason = NULL
		WHERE household_id = $1 AND status = 'declined' AND assignee_id = created_by AND created_by = $2`,
		h.From, h.RemainingUserID)
	return err
}
