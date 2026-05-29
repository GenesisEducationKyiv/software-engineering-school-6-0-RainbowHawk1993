package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"releasesapi/internal/mailer"
	"releasesapi/internal/model"
)

type fakeBuilder struct{}

func (f *fakeBuilder) BuildConfirmation(sub model.Subscription, baseURL string) mailer.Message {
	return mailer.Message{
		To:      sub.Email,
		Subject: "Confirm",
		Body:    "/api/confirm/ /api/unsubscribe/",
	}
}

func (f *fakeBuilder) BuildReleaseNotification(sub model.Subscription, tag, baseURL string) mailer.Message {
	return mailer.Message{To: sub.Email, Body: "Notification"}
}

func TestScannerRunOnceSendsNotificationsForNewTags(t *testing.T) {
	t.Parallel()

	store := &fakeStore{
		listForScanResult: []model.Subscription{
			{ID: 1, Email: "a@example.com", RepoOwner: "owner", RepoName: "repo", LastSeenTag: "v1.0.0", UnsubscribeToken: validToken("1")},
			{ID: 2, Email: "b@example.com", RepoOwner: "owner", RepoName: "repo", LastSeenTag: "v1.0.0", UnsubscribeToken: validToken("2")},
			{ID: 3, Email: "c@example.com", RepoOwner: "owner", RepoName: "repo", LastSeenTag: "v2.0.0", UnsubscribeToken: validToken("3")},
		},
	}
	github := &fakeGitHub{latestTag: "v2.0.0"}
	mailer := &fakeMailer{}
	builder := &fakeBuilder{}

	discardLogger := slog.New(slog.NewTextHandler(io.Discard, nil))

	scanner := NewScanner(store, github, mailer, builder, discardLogger, "http://localhost:8080", nil)

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
		listForScanResult: []model.Subscription{
			{ID: 1, Email: "a@example.com", RepoOwner: "owner", RepoName: "repo", LastSeenTag: "", UnsubscribeToken: validToken("1")},
		},
	}
	github := &fakeGitHub{latestTag: "v1.0.0"}
	mailer := &fakeMailer{err: errors.New("smtp down")}
	builder := &fakeBuilder{}

	discardLogger := slog.New(slog.NewTextHandler(io.Discard, nil))

	scanner := NewScanner(store, github, mailer, builder, discardLogger, "http://localhost:8080", nil)

	if err := scanner.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}
	if len(store.updateCalls) != 0 {
		t.Fatalf("expected no last_seen_tag updates, got %d", len(store.updateCalls))
	}
}

func validToken(suffix string) string {
	token := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	return token[:64-len(suffix)] + suffix
}
