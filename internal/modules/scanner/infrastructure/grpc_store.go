package infrastructure

import (
	"context"

	internalv1 "releasesapi/gen/internalv1"
	"releasesapi/internal/modules/subscription/domain"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

type GRPCStore struct {
	client internalv1.InternalSubscriptionServiceClient
	apiKey string
}

func NewGRPCStore(addr, apiKey string) (*GRPCStore, error) {
	connection, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}

	return &GRPCStore{
		client: internalv1.NewInternalSubscriptionServiceClient(connection),
		apiKey: apiKey,
	}, nil
}

func (s *GRPCStore) ListConfirmedForScan(ctx context.Context) ([]domain.Subscription, error) {
	response, err := s.client.ListConfirmedForScan(s.withAPIKey(ctx), &internalv1.Empty{})
	if err != nil {
		return nil, err
	}

	subscriptions := make([]domain.Subscription, 0, len(response.GetSubscriptions()))
	for _, item := range response.GetSubscriptions() {
		subscriptions = append(subscriptions, fromInternalSubscription(item))
	}

	return subscriptions, nil
}

func (s *GRPCStore) UpdateLastSeenTag(ctx context.Context, id int64, tag string) error {
	_, err := s.client.UpdateLastSeenTag(s.withAPIKey(ctx), &internalv1.UpdateLastSeenTagRequest{
		SubscriptionId: id,
		Tag:            tag,
	})
	return err
}

func (s *GRPCStore) withAPIKey(ctx context.Context) context.Context {
	if s.apiKey == "" {
		return ctx
	}

	return metadata.AppendToOutgoingContext(ctx, "x-api-key", s.apiKey)
}

func fromInternalSubscription(item *internalv1.InternalSubscription) domain.Subscription {
	if item == nil {
		return domain.Subscription{}
	}

	return domain.Subscription{
		ID:               item.GetId(),
		Email:            item.GetEmail(),
		RepoOwner:        item.GetRepoOwner(),
		RepoName:         item.GetRepoName(),
		Confirmed:        item.GetConfirmed(),
		ConfirmToken:     item.GetConfirmToken(),
		UnsubscribeToken: item.GetUnsubscribeToken(),
		LastSeenTag:      item.GetLastSeenTag(),
	}
}
