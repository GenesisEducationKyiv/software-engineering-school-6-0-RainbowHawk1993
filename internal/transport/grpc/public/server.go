package grpcapi

import (
	"context"

	releasev1 "releasesapi/gen/releasev1"
	"releasesapi/internal/modules/subscription/domain"
	"releasesapi/internal/modules/subscription/ports"
	"releasesapi/internal/platform/apperr"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	releasev1.UnimplementedSubscriptionServiceServer
	subscriptions ports.UseCase
}

func NewServer(subscriptions ports.UseCase) *Server {
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
		response.Subscriptions = append(response.Subscriptions, toProtoSubscription(subscription))
	}

	return response, nil
}

func toProtoSubscription(subscription domain.Subscription) *releasev1.Subscription {
	return &releasev1.Subscription{
		Email:       subscription.Email,
		Repo:        subscription.Repo(),
		Confirmed:   subscription.Confirmed,
		LastSeenTag: subscription.LastSeenTag,
	}
}

func toStatusError(err error) error {
	if appErr, ok := err.(apperr.AppError); ok {
		return status.Error(appErr.GRPCStatus(), appErr.Error())
	}
	return status.Error(codes.Internal, "internal server error")
}
