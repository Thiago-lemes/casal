package push

import (
	"context"
	"database/sql"
)

type PostgresRepo struct{ db *sql.DB }

func NewPostgresRepo(db *sql.DB) *PostgresRepo { return &PostgresRepo{db: db} }

// Save grava ou atualiza a assinatura. Se o mesmo aparelho trocar de conta,
// o endereço passa a pertencer à nova pessoa.
func (r *PostgresRepo) Save(ctx context.Context, userID string, sub Subscription, userAgent string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO push_subscriptions (user_id, endpoint, p256dh, auth, user_agent)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (endpoint) DO UPDATE
		SET user_id = EXCLUDED.user_id, p256dh = EXCLUDED.p256dh,
		    auth = EXCLUDED.auth, user_agent = EXCLUDED.user_agent`,
		userID, sub.Endpoint, sub.P256dh, sub.Auth, userAgent)
	return err
}

func (r *PostgresRepo) Delete(ctx context.Context, userID, endpoint string) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM push_subscriptions WHERE user_id = $1 AND endpoint = $2`, userID, endpoint)
	return err
}

func (r *PostgresRepo) DeleteEndpoint(ctx context.Context, endpoint string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM push_subscriptions WHERE endpoint = $1`, endpoint)
	return err
}

func (r *PostgresRepo) ByUser(ctx context.Context, userID string) ([]Subscription, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT endpoint, p256dh, auth FROM push_subscriptions WHERE user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Subscription
	for rows.Next() {
		var s Subscription
		if err := rows.Scan(&s.Endpoint, &s.P256dh, &s.Auth); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
