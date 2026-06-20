package application

import (
	"context"
	"errors"
	"strings"
	"testing"

	"releasesapi/internal/modules/notification"
	"releasesapi/internal/modules/subscription/domain"
	"releasesapi/internal/modules/subscription/ports"
	"releasesapi/internal/platform/apperr"
	"releasesapi/internal/platform/events"
)

type fakeStore struct {
	createInput         ports.CreateSubscriptionParams
	createResult        domain.Subscription
	createErr           error
	confirmResult       domain.Subscription
	confirmErr          error
	listResult          []domain.Subscription
	listErr             error
	deleteCalledWithID  int64
	updateCalls         []updateCall
	listForScanResult   []domain.Subscription
	listForScanErr      error
	unsubscribeErr      error
	unsubscribeToken    string
	confirmToken        string
	listByEmailArgument string
}

type updateCall struct {
	id  int64
	tag string
}

func (f *fakeStore) CreateSubscription(_ context.Context, params ports.CreateSubscriptionParams) (domain.Subscription, error) {
	f.createInput = params
	if f.createErr != nil {
		return domain.Subscription{}, f.createErr
	}
	if f.createResult.ID == 0 {
		f.createResult = domain.Subscription{
			ID:               1,
			Email:            params.Email,
			RepoOwner:        params.RepoOwner,
			RepoName:         params.RepoName,
			ConfirmToken:     params.ConfirmToken,
			UnsubscribeToken: params.UnsubscribeToken,
			LastSeenTag:      params.LastSeenTag,
		}
	}
	return f.createResult, nil
}

func (f *fakeStore) DeleteSubscription(_ context.Context, id int64) error {
	f.deleteCalledWithID = id
	return nil
}

func (f *fakeStore) ConfirmByToken(_ context.Context, token string) (domain.Subscription, error) {
	f.confirmToken = token
	return f.confirmResult, f.confirmErr
}

func (f *fakeStore) DeleteByUnsubscribeToken(_ context.Context, token string) error {
	f.unsubscribeToken = token
	return f.unsubscribeErr
}

func (f *fakeStore) ListConfirmedByEmail(_ context.Context, email string) ([]domain.Subscription, error) {
	f.listByEmailArgument = email
	return f.listResult, f.listErr
}

func (f *fakeStore) ListConfirmedForScan(_ context.Context) ([]domain.Subscription, error) {
	return f.listForScanResult, f.listForScanErr
}

func (f *fakeStore) UpdateLastSeenTag(_ context.Context, id int64, tag string) error {
	f.updateCalls = append(f.updateCalls, updateCall{id: id, tag: tag})
	return nil
}

type fakeGitHub struct {
	repoExistsErr error
	latestTag     string
	latestErr     error
}

func (f *fakeGitHub) RepoExists(context.Context, string, string) error {
	return f.repoExistsErr
}

func (f *fakeGitHub) LatestReleaseTag(context.Context, string, string) (string, error) {
	return f.latestTag, f.latestErr
}

type fakeMailer struct {
	messages []notification.Message
	err      error
}

func (f *fakeMailer) Send(_ context.Context, message notification.Message) error {
	f.messages = append(f.messages, message)
	return f.err
}

type fakeBuilder struct{}

func (f *fakeBuilder) BuildConfirmation(sub domain.Subscription, baseURL string) notification.Message {
	return notification.Message{
		To:      sub.Email,
		Subject: "Confirm",
		Body:    "/api/confirm/ /api/unsubscribe/",
	}
}

func (f *fakeBuilder) BuildReleaseNotification(sub domain.Subscription, tag, baseURL string) notification.Message {
	return notification.Message{To: sub.Email, Body: "Notification"}
}

func (f *fakeBuilder) BuildReleaseNotificationFromEvent(event events.ReleaseDetected, baseURL string) notification.Message {
	return notification.Message{To: event.Email, Body: "Notification"}
}

func TestSubscribeSuccess(t *testing.T) {
	t.Parallel()

	store := &fakeStore{}
	github := &fakeGitHub{latestTag: "v1.2.3"}
	mailer := &fakeMailer{}
	builder := &fakeBuilder{}
	service := NewService(store, github, mailer, builder, "http://localhost:8080")

	subscription, err := service.Subscribe(context.Background(), "User@Example.com", "Owner/Repo")
	if err != nil {
		t.Fatalf("Subscribe returned error: %v", err)
	}

	if subscription.Email != "user@example.com" {
		t.Fatalf("expected normalized email, got %q", subscription.Email)
	}
	if store.createInput.RepoOwner != "owner" || store.createInput.RepoName != "repo" {
		t.Fatalf("expected normalized repo, got %s/%s", store.createInput.RepoOwner, store.createInput.RepoName)
	}
	if store.createInput.LastSeenTag != "v1.2.3" {
		t.Fatalf("expected last_seen_tag to be seeded, got %q", store.createInput.LastSeenTag)
	}
	if len(mailer.messages) != 1 {
		t.Fatalf("expected one confirmation email, got %d", len(mailer.messages))
	}
	if !strings.Contains(mailer.messages[0].Body, "/api/confirm/") {
		t.Fatalf("confirmation email missing confirm link: %q", mailer.messages[0].Body)
	}
	if !strings.Contains(mailer.messages[0].Body, "/api/unsubscribe/") {
		t.Fatalf("confirmation email missing unsubscribe link: %q", mailer.messages[0].Body)
	}
}

func TestSubscribeRollbackOnMailerFailure(t *testing.T) {
	t.Parallel()

	store := &fakeStore{}
	github := &fakeGitHub{}
	mailer := &fakeMailer{err: errors.New("smtp down")}
	builder := &fakeBuilder{}
	service := NewService(store, github, mailer, builder, "http://localhost:8080")

	if _, err := service.Subscribe(context.Background(), "user@example.com", "owner/repo"); err == nil {
		t.Fatal("expected mailer error")
	}
	if store.deleteCalledWithID != 1 {
		t.Fatalf("expected created subscription rollback, got id %d", store.deleteCalledWithID)
	}
}

func TestSubscribeDuplicate(t *testing.T) {
	t.Parallel()

	service := NewService(
		&fakeStore{createErr: apperr.ErrAlreadySubscribed},
		&fakeGitHub{},
		&fakeMailer{},
		&fakeBuilder{},
		"http://localhost:8080",
	)

	_, err := service.Subscribe(context.Background(), "user@example.com", "owner/repo")
	if !errors.Is(err, apperr.ErrAlreadySubscribed) {
		t.Fatalf("expected ErrAlreadySubscribed, got %v", err)
	}
}

func TestConfirmRejectsInvalidToken(t *testing.T) {
	t.Parallel()

	service := NewService(&fakeStore{}, &fakeGitHub{}, &fakeMailer{}, &fakeBuilder{}, "http://localhost:8080")
	_, err := service.Confirm(context.Background(), "bad-token")
	if !errors.Is(err, apperr.ErrInvalidToken) {
		t.Fatalf("expected invalid token error, got %v", err)
	}
}

func TestListByEmailRejectsInvalidEmail(t *testing.T) {
	t.Parallel()

	service := NewService(&fakeStore{}, &fakeGitHub{}, &fakeMailer{}, &fakeBuilder{}, "http://localhost:8080")
	if _, err := service.ListByEmail(context.Background(), "not-an-email"); !errors.Is(err, apperr.ErrInvalidEmail) {
		t.Fatalf("expected invalid email error, got %v", err)
	}
}
