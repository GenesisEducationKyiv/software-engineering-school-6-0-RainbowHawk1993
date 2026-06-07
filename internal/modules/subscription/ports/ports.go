package ports

import (
	"context"

	"releasesapi/internal/modules/subscription/domain"
)

type CreateSubscriptionParams struct {
	Email            string
	RepoOwner        string
	RepoName         string
	ConfirmToken     string
	UnsubscribeToken string
	LastSeenTag      string
}

type Repository interface {
	CreateSubscription(context.Context, CreateSubscriptionParams) (domain.Subscription, error)
	DeleteSubscription(context.Context, int64) error
	ConfirmByToken(context.Context, string) (domain.Subscription, error)
	DeleteByUnsubscribeToken(context.Context, string) error
	ListConfirmedByEmail(context.Context, string) ([]domain.Subscription, error)
	ListConfirmedForScan(context.Context) ([]domain.Subscription, error)
	UpdateLastSeenTag(context.Context, int64, string) error
}

type UseCase interface {
	Subscribe(context.Context, string, string) (domain.Subscription, error)
	Confirm(context.Context, string) (domain.Subscription, error)
	Unsubscribe(context.Context, string) error
	ListByEmail(context.Context, string) ([]domain.Subscription, error)
}
