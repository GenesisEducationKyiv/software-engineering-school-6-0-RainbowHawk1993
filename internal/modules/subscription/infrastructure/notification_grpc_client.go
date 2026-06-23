package infrastructure

import (
	"context"

	"releasesapi/gen/mailv1"
	"releasesapi/internal/modules/subscription/domain"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type NotificationGRPCClient struct {
	client mailv1.MailVerificationServiceClient
}

func NewNotificationGRPCClient(addr string) (*NotificationGRPCClient, error) {
	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}

	return &NotificationGRPCClient{
		client: mailv1.NewMailVerificationServiceClient(conn),
	}, nil
}

func (c *NotificationGRPCClient) SendVerification(ctx context.Context, sub domain.Subscription) error {
	_, err := c.client.SendVerificationEmail(ctx, &mailv1.SendVerificationEmailRequest{
		Email:            sub.Email,
		RepoOwner:        sub.RepoOwner,
		RepoName:         sub.RepoName,
		ConfirmToken:     sub.ConfirmToken,
		UnsubscribeToken: sub.UnsubscribeToken,
	})
	return err
}
