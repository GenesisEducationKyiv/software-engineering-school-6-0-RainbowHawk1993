package store

import (
	"context"
	"errors"

	"releasesapi/internal/apperr"
	"releasesapi/internal/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CreateSubscriptionParams struct {
	Email            string
	RepoOwner        string
	RepoName         string
	ConfirmToken     string
	UnsubscribeToken string
	LastSeenTag      string
}

type PostgresSubscriptionStore struct {
	db *pgxpool.Pool
}

func NewPostgresSubscriptionStore(db *pgxpool.Pool) *PostgresSubscriptionStore {
	return &PostgresSubscriptionStore{db: db}
}

func (s *PostgresSubscriptionStore) CreateSubscription(ctx context.Context, params CreateSubscriptionParams) (model.Subscription, error) {
	subscription, err := scanSubscription(s.db.QueryRow(ctx, `
		insert into subscriptions (
			email,
			repo_owner,
			repo_name,
			confirm_token,
			unsubscribe_token,
			last_seen_tag
		)
		values ($1, $2, $3, $4, $5, $6)
		returning id, email, repo_owner, repo_name, confirmed, confirm_token, unsubscribe_token, last_seen_tag, created_at, updated_at
	`, params.Email, params.RepoOwner, params.RepoName, params.ConfirmToken, params.UnsubscribeToken, params.LastSeenTag))
	if err == nil {
		return subscription, nil
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return model.Subscription{}, apperr.ErrAlreadySubscribed
	}

	return model.Subscription{}, err
}

func (s *PostgresSubscriptionStore) DeleteSubscription(ctx context.Context, id int64) error {
	_, err := s.db.Exec(ctx, `delete from subscriptions where id = $1`, id)
	return err
}

func (s *PostgresSubscriptionStore) ConfirmByToken(ctx context.Context, token string) (model.Subscription, error) {
	subscription, err := scanSubscription(s.db.QueryRow(ctx, `
		update subscriptions
		set confirmed = true,
		    confirmed_at = coalesce(confirmed_at, now()),
		    updated_at = now()
		where confirm_token = $1
		returning id, email, repo_owner, repo_name, confirmed, confirm_token, unsubscribe_token, last_seen_tag, created_at, updated_at
	`, token))
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Subscription{}, apperr.ErrTokenNotFound
	}

	return subscription, err
}

func (s *PostgresSubscriptionStore) DeleteByUnsubscribeToken(ctx context.Context, token string) error {
	commandTag, err := s.db.Exec(ctx, `delete from subscriptions where unsubscribe_token = $1`, token)
	if err != nil {
		return err
	}
	if commandTag.RowsAffected() == 0 {
		return apperr.ErrTokenNotFound
	}

	return nil
}

func (s *PostgresSubscriptionStore) ListConfirmedByEmail(ctx context.Context, email string) ([]model.Subscription, error) {
	rows, err := s.db.Query(ctx, `
		select id, email, repo_owner, repo_name, confirmed, confirm_token, unsubscribe_token, last_seen_tag, created_at, updated_at
		from subscriptions
		where email = $1 and confirmed = true
		order by repo_owner, repo_name
	`, email)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanSubscriptions(rows)
}

func (s *PostgresSubscriptionStore) ListConfirmedForScan(ctx context.Context) ([]model.Subscription, error) {
	rows, err := s.db.Query(ctx, `
		select id, email, repo_owner, repo_name, confirmed, confirm_token, unsubscribe_token, last_seen_tag, created_at, updated_at
		from subscriptions
		where confirmed = true
		order by repo_owner, repo_name, email
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanSubscriptions(rows)
}

func (s *PostgresSubscriptionStore) UpdateLastSeenTag(ctx context.Context, id int64, tag string) error {
	_, err := s.db.Exec(ctx, `
		update subscriptions
		set last_seen_tag = $2,
		    updated_at = now()
		where id = $1
	`, id, tag)
	return err
}

func scanSubscriptions(rows pgx.Rows) ([]model.Subscription, error) {
	var subscriptions []model.Subscription
	for rows.Next() {
		subscription, err := scanSubscription(rows)
		if err != nil {
			return nil, err
		}
		subscriptions = append(subscriptions, subscription)
	}

	return subscriptions, rows.Err()
}

func scanSubscription(row interface {
	Scan(dest ...any) error
}) (model.Subscription, error) {
	var subscription model.Subscription
	err := row.Scan(
		&subscription.ID,
		&subscription.Email,
		&subscription.RepoOwner,
		&subscription.RepoName,
		&subscription.Confirmed,
		&subscription.ConfirmToken,
		&subscription.UnsubscribeToken,
		&subscription.LastSeenTag,
		&subscription.CreatedAt,
		&subscription.UpdatedAt,
	)
	if err != nil {
		return model.Subscription{}, err
	}

	return subscription, nil
}
