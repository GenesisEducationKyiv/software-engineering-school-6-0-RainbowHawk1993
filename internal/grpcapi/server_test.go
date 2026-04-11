package grpcapi

import (
	"context"
	"errors"
	"net"
	"testing"

	releasev1 "releasesapi/gen/releasev1"
	"releasesapi/internal/apperr"
	"releasesapi/internal/model"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

type fakeSubscriptions struct {
	subscribeResult model.Subscription
	subscribeErr    error
	listResult      []model.Subscription
	listErr         error
}

func (f *fakeSubscriptions) Subscribe(context.Context, string, string) (model.Subscription, error) {
	return f.subscribeResult, f.subscribeErr
}

func (f *fakeSubscriptions) Confirm(context.Context, string) (model.Subscription, error) {
	return model.Subscription{}, nil
}

func (f *fakeSubscriptions) Unsubscribe(context.Context, string) error {
	return nil
}

func (f *fakeSubscriptions) ListByEmail(context.Context, string) ([]model.Subscription, error) {
	return f.listResult, f.listErr
}

func TestSubscribeReturnsAlreadyExists(t *testing.T) {
	t.Parallel()

	client, cleanup := newTestClient(t, &fakeSubscriptions{subscribeErr: apperr.ErrAlreadySubscribed})
	defer cleanup()

	_, err := client.Subscribe(context.Background(), &releasev1.SubscribeRequest{
		Email: "user@example.com",
		Repo:  "owner/repo",
	})
	if status.Code(err) != codes.AlreadyExists {
		t.Fatalf("expected AlreadyExists, got %v", err)
	}
}

func TestGetSubscriptionsReturnsMappedItems(t *testing.T) {
	t.Parallel()

	client, cleanup := newTestClient(t, &fakeSubscriptions{
		listResult: []model.Subscription{
			{Email: "user@example.com", RepoOwner: "owner", RepoName: "repo", Confirmed: true, LastSeenTag: "v1.0.0"},
		},
	})
	defer cleanup()

	response, err := client.GetSubscriptions(context.Background(), &releasev1.GetSubscriptionsRequest{
		Email: "user@example.com",
	})
	if err != nil {
		t.Fatalf("GetSubscriptions returned error: %v", err)
	}
	if len(response.Subscriptions) != 1 {
		t.Fatalf("expected 1 subscription, got %d", len(response.Subscriptions))
	}
	if response.Subscriptions[0].Repo != "owner/repo" {
		t.Fatalf("expected repo owner/repo, got %q", response.Subscriptions[0].Repo)
	}
}

func newTestClient(t *testing.T, subscriptions SubscriptionUseCase) (releasev1.SubscriptionServiceClient, func()) {
	t.Helper()

	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	releasev1.RegisterSubscriptionServiceServer(server, NewServer(subscriptions))

	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			t.Errorf("Serve returned error: %v", err)
		}
	}()

	connection, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return listener.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("grpc.NewClient returned error: %v", err)
	}

	cleanup := func() {
		_ = connection.Close()
		server.Stop()
		_ = listener.Close()
	}

	return releasev1.NewSubscriptionServiceClient(connection), cleanup
}
