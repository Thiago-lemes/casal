package household

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"casal/internal/apperr"
	"casal/internal/db"
)

const MaxMembers = 2

var (
	ErrNoHousehold        = apperr.NewCoded(apperr.Forbidden, "no_household", "você ainda não tem um casal")
	ErrAlreadyInHousehold = apperr.NewCoded(apperr.Conflict, "already_in_household", "você já pertence a um casal")
	ErrInviteInvalid      = apperr.NewCoded(apperr.NotFound, "invite_invalid", "convite inválido, expirado ou já usado")
	ErrHouseholdFull      = apperr.NewCoded(apperr.Conflict, "household_full", "o casal já tem duas pessoas")
)

type Member struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	JoinedAt time.Time `json:"joined_at"`
}

type Repository interface {
	// CreateHousehold, AcceptInvite e LeaveHousehold rodam os ganchos (db.Hook) dentro da mesma transação:
	// se um falhar, a mudança de casal toda é desfeita.
	CreateHousehold(ctx context.Context, userID string, hooks ...db.Hook) (string, error)
	HouseholdOf(ctx context.Context, userID string) (string, error)
	Members(ctx context.Context, householdID string) ([]Member, error)
	IsMember(ctx context.Context, householdID, userID string) (bool, error)
	CreateInvite(ctx context.Context, householdID, createdBy, tokenHash string, expiresAt time.Time) error
	AcceptInvite(ctx context.Context, tokenHash, userID string, now time.Time, hooks ...db.Hook) (string, error)
	// LeaveHousehold tira a pessoa do casal. Pode sair mesmo sendo a única: o casal fica vazio.
	LeaveHousehold(ctx context.Context, userID string, hooks ...db.Hook) error
	// DeleteStaleInvites apaga convites usados ou vencidos há mais de olderThan.
	DeleteStaleInvites(ctx context.Context, now time.Time, olderThan time.Duration) (int64, error)
}

type PostgresRepo struct{ db *sql.DB }

func NewPostgresRepo(db *sql.DB) *PostgresRepo { return &PostgresRepo{db: db} }

func (r *PostgresRepo) CreateHousehold(ctx context.Context, userID string, hooks ...db.Hook) (string, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	var id string
	if err := tx.QueryRowContext(ctx, `INSERT INTO households DEFAULT VALUES RETURNING id`).Scan(&id); err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO household_members (household_id, user_id) VALUES ($1, $2)`, id, userID); err != nil {
		if db.IsUniqueViolation(err) {
			return "", ErrAlreadyInHousehold
		}
		return "", err
	}
	if err := runHooks(ctx, tx, hooks, db.Handoff{UserID: userID, To: id}); err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return id, nil
}

// runHooks roda cada gancho na transação da mudança de casal; o primeiro erro desfaz tudo.
func runHooks(ctx context.Context, tx *sql.Tx, hooks []db.Hook, h db.Handoff) error {
	for _, hook := range hooks {
		if err := hook(ctx, tx, h); err != nil {
			return err
		}
	}
	return nil
}

// LeaveHousehold tira a pessoa do casal. O casal é travado para não correr junto com um convite sendo aceito.
// Os convites que ela criou e ninguém usou são revogados (e todos, se o casal ficou vazio), para ninguém
// entrar num casal por um link que quem ficou não gerou.
func (r *PostgresRepo) LeaveHousehold(ctx context.Context, userID string, hooks ...db.Hook) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var householdID string
	err = tx.QueryRowContext(ctx, `SELECT household_id FROM household_members WHERE user_id = $1`, userID).Scan(&householdID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNoHousehold
	}
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `SELECT 1 FROM households WHERE id = $1 FOR UPDATE`, householdID); err != nil {
		return err
	}

	var remaining sql.NullString
	err = tx.QueryRowContext(ctx, `
		SELECT user_id FROM household_members WHERE household_id = $1 AND user_id <> $2 LIMIT 1`,
		householdID, userID).Scan(&remaining)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM household_members WHERE household_id = $1 AND user_id = $2`, householdID, userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM household_invites
		WHERE household_id = $1 AND accepted_at IS NULL AND (created_by = $2 OR $3 = '')`,
		householdID, userID, remaining.String); err != nil {
		return err
	}
	h := db.Handoff{UserID: userID, From: householdID, RemainingUserID: remaining.String}
	if err := runHooks(ctx, tx, hooks, h); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *PostgresRepo) HouseholdOf(ctx context.Context, userID string) (string, error) {
	var id string
	err := r.db.QueryRowContext(ctx,
		`SELECT household_id FROM household_members WHERE user_id = $1`, userID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNoHousehold
	}
	return id, err
}

// Members faz um JOIN somente leitura em users, só para mostrar o nome.
func (r *PostgresRepo) Members(ctx context.Context, householdID string) ([]Member, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT u.id, u.name, m.joined_at
		FROM household_members m JOIN users u ON u.id = m.user_id
		WHERE m.household_id = $1
		ORDER BY m.joined_at`, householdID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Member
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.ID, &m.Name, &m.JoinedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *PostgresRepo) IsMember(ctx context.Context, householdID, userID string) (bool, error) {
	var ok bool
	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS (SELECT 1 FROM household_members WHERE household_id = $1 AND user_id = $2)`,
		householdID, userID).Scan(&ok)
	return ok, err
}

func (r *PostgresRepo) CreateInvite(ctx context.Context, householdID, createdBy, tokenHash string, expiresAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO household_invites (household_id, token_hash, created_by, expires_at)
		VALUES ($1, $2, $3, $4)`, householdID, tokenHash, createdBy, expiresAt)
	return err
}

// AcceptInvite faz tudo em uma transação: valida o convite, trava o casal,
// confere a vaga e registra a entrada.
func (r *PostgresRepo) AcceptInvite(ctx context.Context, tokenHash, userID string, now time.Time, hooks ...db.Hook) (string, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	var inviteID, householdID string
	var expiresAt time.Time
	var acceptedAt sql.NullTime
	err = tx.QueryRowContext(ctx, `
		SELECT id, household_id, expires_at, accepted_at
		FROM household_invites WHERE token_hash = $1 FOR UPDATE`, tokenHash).
		Scan(&inviteID, &householdID, &expiresAt, &acceptedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrInviteInvalid
	}
	if err != nil {
		return "", err
	}
	if acceptedAt.Valid || !expiresAt.After(now) {
		return "", ErrInviteInvalid
	}

	// A trava no casal impede que dois convites aceitos ao mesmo tempo passem de duas pessoas.
	if _, err := tx.ExecContext(ctx, `SELECT 1 FROM households WHERE id = $1 FOR UPDATE`, householdID); err != nil {
		return "", err
	}
	var count int
	if err := tx.QueryRowContext(ctx,
		`SELECT count(*) FROM household_members WHERE household_id = $1`, householdID).Scan(&count); err != nil {
		return "", err
	}
	if count >= MaxMembers {
		return "", ErrHouseholdFull
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO household_members (household_id, user_id) VALUES ($1, $2)`, householdID, userID); err != nil {
		if db.IsUniqueViolation(err) {
			return "", ErrAlreadyInHousehold
		}
		return "", err
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE household_invites SET accepted_by = $1, accepted_at = $2 WHERE id = $3`,
		userID, now, inviteID); err != nil {
		return "", err
	}
	if err := runHooks(ctx, tx, hooks, db.Handoff{UserID: userID, To: householdID}); err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return householdID, nil
}

func (r *PostgresRepo) DeleteStaleInvites(ctx context.Context, now time.Time, olderThan time.Duration) (int64, error) {
	cutoff := now.Add(-olderThan)
	res, err := r.db.ExecContext(ctx, `
		DELETE FROM household_invites
		WHERE expires_at <= $1 OR (accepted_at IS NOT NULL AND accepted_at <= $1)`, cutoff)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
