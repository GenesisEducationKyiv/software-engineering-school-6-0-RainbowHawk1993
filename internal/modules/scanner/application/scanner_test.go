package application

import (
	"context"
	"errors"
	"io"
	"log"
	"testing"

	"releasesapi/internal/modules/notification"
	"releasesapi/internal/modules/subscription/domain"
)

func TestScannerRunOnceSendsNotificationsForNewTags(t *testing.T) {
	t.Parallel()

	store := &fakeStore{
		listForScanResult: []domain.Subscription{
			{ID: 1, Email: "a@example.com", RepoOwner: "owner", RepoName: "repo", LastSeenTag: "v1.0.0", UnsubscribeToken: validToken("1")},
			{ID: 2, Email: "b@example.com", RepoOwner: "owner", RepoName: "repo", LastSeenTag: "v1.0.0", UnsubscribeToken: validToken("2")},
			{ID: 3, Email: "c@example.com", RepoOwner: "owner", RepoName: "repo", LastSeenTag: "v2.0.0", UnsubscribeToken: validToken("3")},
		},
	}
	github := &fakeGitHub{latestTag: "v2.0.0"}
	mailer := &fakeMailer{}
	builder := &fakeBuilder{}

	scanner := NewScanner(store, github, mailer, builder, log.New(io.Discard, "", 0), "http://localhost:8080", nil)

	if err := scanner.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}

	if len(mailer.messages) != 2 {
		t.Fatalf("expected 2 notification emails, got %d", len(mailer.messages))
	}
	if len(store.updateCalls) != 2 {
		t.Fatalf("expected 2 last_seen_tag updates, got %d", len(store.updateCalls))
	}
}

func TestScannerSkipsStateUpdateOnMailerFailure(t *testing.T) {
	t.Parallel()

	store := &fakeStore{
		listForScanResult: []domain.Subscription{
			{ID: 1, Email: "a@example.com", RepoOwner: "owner", RepoName: "repo", LastSeenTag: "", UnsubscribeToken: validToken("1")},
		},
	}
	github := &fakeGitHub{latestTag: "v1.0.0"}
	mailer := &fakeMailer{err: errors.New("smtp down")}
	builder := &fakeBuilder{}

	scanner := NewScanner(store, github, mailer, builder, log.New(io.Discard, "", 0), "http://localhost:8080", nil)

	if err := scanner.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}
	if len(store.updateCalls) != 0 {
		t.Fatalf("expected no last_seen_tag updates, got %d", len(store.updateCalls))
	}
}

type fakeStore struct {
	listForScanResult []domain.Subscription
	listForScanErr    error
	updateCalls       []updateCall
}

type updateCall struct {
	id  int64
	tag string
}

func (f *fakeStore) ListConfirmedForScan(_ context.Context) ([]domain.Subscription, error) {
	return f.listForScanResult, f.listForScanErr
}

func (f *fakeStore) UpdateLastSeenTag(_ context.Context, id int64, tag string) error {
	f.updateCalls = append(f.updateCalls, updateCall{id: id, tag: tag})
	return nil
}

type fakeGitHub struct {
	latestTag string
	latestErr error
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
	return notification.Message{To: sub.Email, Body: "/api/confirm/ /api/unsubscribe/"}
}

func (f *fakeBuilder) BuildReleaseNotification(sub domain.Subscription, tag, baseURL string) notification.Message {
	return notification.Message{To: sub.Email, Body: "Notification"}
}

func validToken(suffix string) string {
	token := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	return token[:64-len(suffix)] + suffix
}
