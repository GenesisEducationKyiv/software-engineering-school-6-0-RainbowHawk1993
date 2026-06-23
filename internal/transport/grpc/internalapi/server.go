package grpcinternal

import (
	"context"

	internalv1 "releasesapi/gen/internalv1"
	"releasesapi/internal/modules/subscription/domain"
	"releasesapi/internal/modules/subscription/ports"
)

type Server struct {
	internalv1.UnimplementedInternalSubscriptionServiceServer
	store ports.Repository
}

func NewServer(store ports.Repository) *Server {
	return &Server{store: store}
}

func (s *Server) ListConfirmedForScan(ctx context.Context, _ *internalv1.Empty) (*internalv1.ListConfirmedForScanResponse, error) {
	subscriptions, err := s.store.ListConfirmedForScan(ctx)
	if err != nil {
		return nil, err
	}

	response := &internalv1.ListConfirmedForScanResponse{
		Subscriptions: make([]*internalv1.InternalSubscription, 0, len(subscriptions)),
	}
	for _, subscription := range subscriptions {
		response.Subscriptions = append(response.Subscriptions, toInternalSubscription(subscription))
	}

	return response, nil
}

func (s *Server) UpdateLastSeenTag(ctx context.Context, request *internalv1.UpdateLastSeenTagRequest) (*internalv1.Empty, error) {
	if err := s.store.UpdateLastSeenTag(ctx, request.GetSubscriptionId(), request.GetTag()); err != nil {
		return nil, err
	}

	return &internalv1.Empty{}, nil
}

func toInternalSubscription(subscription domain.Subscription) *internalv1.InternalSubscription {
	return &internalv1.InternalSubscription{
		Id:               subscription.ID,
		Email:            subscription.Email,
		RepoOwner:        subscription.RepoOwner,
		RepoName:         subscription.RepoName,
		Confirmed:        subscription.Confirmed,
		ConfirmToken:     subscription.ConfirmToken,
		UnsubscribeToken: subscription.UnsubscribeToken,
		LastSeenTag:      subscription.LastSeenTag,
	}
}
