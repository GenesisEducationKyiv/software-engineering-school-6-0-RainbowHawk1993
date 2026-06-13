package application

import (
	"context"
	"errors"
	"io"
	"log"
	"testing"

	"releasesapi/internal/modules/subscription/domain"
	"releasesapi/internal/platform/events"
)

func TestScannerRunOncePublishesEventsForNewTags(t *testing.T) {
	t.Parallel()

	store := &fakeStore{
		listForScanResult: []domain.Subscription{
			{ID: 1, Email: "a@example.com", RepoOwner: "owner", RepoName: "repo", LastSeenTag: "v1.0.0", UnsubscribeToken: validToken("1")},
			{ID: 2, Email: "b@example.com", RepoOwner: "owner", RepoName: "repo", LastSeenTag: "v1.0.0", UnsubscribeToken: validToken("2")},
			{ID: 3, Email: "c@example.com", RepoOwner: "owner", RepoName: "repo", LastSeenTag: "v2.0.0", UnsubscribeToken: validToken("3")},
		},
	}
	github := &fakeGitHub{latestTag: "v2.0.0"}
	publisher := &fakePublisher{}

	scanner := NewScanner(store, github, publisher, log.New(io.Discard, "", 0), "http://localhost:8080", nil)

	if err := scanner.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}

	if len(publisher.events) != 2 {
		t.Fatalf("expected 2 published events, got %d", len(publisher.events))
	}
	if len(store.updateCalls) != 2 {
		t.Fatalf("expected 2 last_seen_tag updates, got %d", len(store.updateCalls))
	}
}

func TestScannerSkipsPublishWhenTagUnchanged(t *testing.T) {
	t.Parallel()

	store := &fakeStore{
		listForScanResult: []domain.Subscription{
			{ID: 1, Email: "a@example.com", RepoOwner: "owner", RepoName: "repo", LastSeenTag: "v1.0.0", UnsubscribeToken: validToken("1")},
		},
	}
	github := &fakeGitHub{latestTag: "v1.0.0"}
	publisher := &fakePublisher{}

	scanner := NewScanner(store, github, publisher, log.New(io.Discard, "", 0), "http://localhost:8080", nil)

	if err := scanner.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}

	if len(publisher.events) != 0 {
		t.Fatalf("expected no published events, got %d", len(publisher.events))
	}
	if len(store.updateCalls) != 0 {
		t.Fatalf("expected no last_seen_tag updates, got %d", len(store.updateCalls))
	}
}

func TestScannerContinuesOnPublishFailure(t *testing.T) {
	t.Parallel()

	store := &fakeStore{
		listForScanResult: []domain.Subscription{
			{ID: 1, Email: "a@example.com", RepoOwner: "owner", RepoName: "repo", LastSeenTag: "", UnsubscribeToken: validToken("1")},
		},
	}
	github := &fakeGitHub{latestTag: "v1.0.0"}
	publisher := &fakePublisher{err: errors.New("nats down")}

	scanner := NewScanner(store, github, publisher, log.New(io.Discard, "", 0), "http://localhost:8080", nil)

	if err := scanner.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}

	// Tag should still be updated (at-least-once: update first, then publish)
	if len(store.updateCalls) != 1 {
		t.Fatalf("expected 1 last_seen_tag update, got %d", len(store.updateCalls))
	}
}

func TestScannerPublishesCorrectEventData(t *testing.T) {
	t.Parallel()

	store := &fakeStore{
		listForScanResult: []domain.Subscription{
			{ID: 1, Email: "user@example.com", RepoOwner: "golang", RepoName: "go", LastSeenTag: "v1.22.0", UnsubscribeToken: "unsub-token"},
		},
	}
	github := &fakeGitHub{latestTag: "v1.23.0"}
	publisher := &fakePublisher{}

	scanner := NewScanner(store, github, publisher, log.New(io.Discard, "", 0), "http://localhost:8080", nil)

	if err := scanner.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}

	if len(publisher.events) != 1 {
		t.Fatalf("expected 1 published event, got %d", len(publisher.events))
	}

	event := publisher.events[0]
	if event.subject != events.SubjectReleaseDetected {
		t.Fatalf("expected subject %q, got %q", events.SubjectReleaseDetected, event.subject)
	}

	releaseEvent, ok := event.payload.(events.ReleaseDetected)
	if !ok {
		t.Fatalf("expected ReleaseDetected payload, got %T", event.payload)
	}
	if releaseEvent.Email != "user@example.com" {
		t.Fatalf("expected email user@example.com, got %q", releaseEvent.Email)
	}
	if releaseEvent.RepoOwner != "golang" || releaseEvent.RepoName != "go" {
		t.Fatalf("expected repo golang/go, got %s/%s", releaseEvent.RepoOwner, releaseEvent.RepoName)
	}
	if releaseEvent.Tag != "v1.23.0" {
		t.Fatalf("expected tag v1.23.0, got %q", releaseEvent.Tag)
	}
	if releaseEvent.UnsubscribeToken != "unsub-token" {
		t.Fatalf("expected unsubscribe token unsub-token, got %q", releaseEvent.UnsubscribeToken)
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

type publishedEvent struct {
	subject string
	payload any
}

type fakePublisher struct {
	events []publishedEvent
	err    error
}

func (f *fakePublisher) Publish(subject string, event any) error {
	f.events = append(f.events, publishedEvent{subject: subject, payload: event})
	return f.err
}

func validToken(suffix string) string {
	token := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	return token[:64-len(suffix)] + suffix
}
