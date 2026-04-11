package grpcapi

import (
	"context"
	"errors"

	releasev1 "releasesapi/gen/releasev1"
	"releasesapi/internal/apperr"
	"releasesapi/internal/model"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type SubscriptionUseCase interface {
	Subscribe(context.Context, string, string) (model.Subscription, error)
	Confirm(context.Context, string) (model.Subscription, error)
	Unsubscribe(context.Context, string) error
	ListByEmail(context.Context, string) ([]model.Subscription, error)
}

type Server struct {
	releasev1.UnimplementedSubscriptionServiceServer
	subscriptions SubscriptionUseCase
}

func NewServer(subscriptions SubscriptionUseCase) *Server {
	return &Server{subscriptions: subscriptions}
}

func (s *Server) Subscribe(ctx context.Context, request *releasev1.SubscribeRequest) (*releasev1.MessageReply, error) {
	if _, err := s.subscriptions.Subscribe(ctx, request.GetEmail(), request.GetRepo()); err != nil {
		return nil, toStatusError(err)
	}

	return &releasev1.MessageReply{Message: "subscription created; confirmation email sent"}, nil
}

func (s *Server) ConfirmSubscription(ctx context.Context, request *releasev1.TokenRequest) (*releasev1.MessageReply, error) {
	if _, err := s.subscriptions.Confirm(ctx, request.GetToken()); err != nil {
		return nil, toStatusError(err)
	}

	return &releasev1.MessageReply{Message: "subscription confirmed successfully"}, nil
}

func (s *Server) Unsubscribe(ctx context.Context, request *releasev1.TokenRequest) (*releasev1.MessageReply, error) {
	if err := s.subscriptions.Unsubscribe(ctx, request.GetToken()); err != nil {
		return nil, toStatusError(err)
	}

	return &releasev1.MessageReply{Message: "unsubscribed successfully"}, nil
}

func (s *Server) GetSubscriptions(ctx context.Context, request *releasev1.GetSubscriptionsRequest) (*releasev1.GetSubscriptionsResponse, error) {
	subscriptions, err := s.subscriptions.ListByEmail(ctx, request.GetEmail())
	if err != nil {
		return nil, toStatusError(err)
	}

	response := &releasev1.GetSubscriptionsResponse{
		Subscriptions: make([]*releasev1.Subscription, 0, len(subscriptions)),
	}
	for _, subscription := range subscriptions {
		response.Subscriptions = append(response.Subscriptions, &releasev1.Subscription{
			Email:       subscription.Email,
			Repo:        subscription.Repo(),
			Confirmed:   subscription.Confirmed,
			LastSeenTag: subscription.LastSeenTag,
		})
	}

	return response, nil
}

func toStatusError(err error) error {
	switch {
	case errors.Is(err, apperr.ErrInvalidEmail), errors.Is(err, apperr.ErrInvalidRepoFormat), errors.Is(err, apperr.ErrInvalidToken):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, apperr.ErrRepoNotFound), errors.Is(err, apperr.ErrTokenNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, apperr.ErrAlreadySubscribed):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, apperr.ErrRateLimited):
		return status.Error(codes.Unavailable, "github api rate limit reached")
	default:
		return status.Error(codes.Internal, "internal server error")
	}
}
