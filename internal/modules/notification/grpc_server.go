package notification

import (
	"context"

	"releasesapi/gen/mailv1"
	"releasesapi/internal/modules/subscription/domain"
)

type MailVerificationServer struct {
	mailv1.UnimplementedMailVerificationServiceServer
	mailer  Mailer
	builder Builder
	baseURL string
}

func NewMailVerificationServer(mailer Mailer, builder Builder, baseURL string) *MailVerificationServer {
	return &MailVerificationServer{
		mailer:  mailer,
		builder: builder,
		baseURL: baseURL,
	}
}

func (s *MailVerificationServer) SendVerificationEmail(ctx context.Context, req *mailv1.SendVerificationEmailRequest) (*mailv1.SendVerificationEmailResponse, error) {
	// Construct a dummy subscription to reuse the builder
	sub := domain.Subscription{
		Email:            req.GetEmail(),
		RepoOwner:        req.GetRepoOwner(),
		RepoName:         req.GetRepoName(),
		ConfirmToken:     req.GetConfirmToken(),
		UnsubscribeToken: req.GetUnsubscribeToken(),
	}

	msg := s.builder.BuildConfirmation(sub, s.baseURL)
	if err := s.mailer.Send(ctx, msg); err != nil {
		return nil, err
	}

	return &mailv1.SendVerificationEmailResponse{}, nil
}
