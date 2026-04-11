package service

import (
	"context"
	"errors"
	"io"
	"log"
	"testing"

	"releasesapi/internal/model"
)

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
	scanner := NewScanner(store, github, mailer, log.New(io.Discard, "", 0), "http://localhost:8080", nil)

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
	scanner := NewScanner(store, github, mailer, log.New(io.Discard, "", 0), "http://localhost:8080", nil)

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
