package notification

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"releasesapi/internal/platform/events"
)

func TestHandleReleaseDetected_Success(t *testing.T) {
	t.Parallel()

	mailer := &fakeMailer{}
	builder := NewDefaultBuilder()
	consumer := NewConsumer(mailer, builder, "http://localhost:8080", nil)

	event := events.ReleaseDetected{
		Email:            "user@example.com",
		RepoOwner:        "owner",
		RepoName:         "repo",
		Tag:              "v2.0.0",
		UnsubscribeToken: "abc123",
	}

	if err := consumer.HandleReleaseDetected(context.Background(), event); err != nil {
		t.Fatalf("HandleReleaseDetected returned error: %v", err)
	}

	if len(mailer.messages) != 1 {
		t.Fatalf("expected 1 message sent, got %d", len(mailer.messages))
	}

	msg := mailer.messages[0]
	if msg.To != "user@example.com" {
		t.Fatalf("expected To=user@example.com, got %q", msg.To)
	}
}

func TestHandleReleaseDetected_MailerFailure(t *testing.T) {
	t.Parallel()

	mailer := &fakeMailer{err: errors.New("smtp down")}
	builder := NewDefaultBuilder()
	consumer := NewConsumer(mailer, builder, "http://localhost:8080", nil)

	event := events.ReleaseDetected{
		Email:            "user@example.com",
		RepoOwner:        "owner",
		RepoName:         "repo",
		Tag:              "v1.0.0",
		UnsubscribeToken: "token",
	}

	err := consumer.HandleReleaseDetected(context.Background(), event)
	if err == nil {
		t.Fatal("expected error from HandleReleaseDetected, got nil")
	}
	if !strings.Contains(err.Error(), "send notification") {
		t.Fatalf("expected 'send notification' in error, got %q", err.Error())
	}
}

func TestHandleReleaseDetected_BuildsCorrectMessage(t *testing.T) {
	t.Parallel()

	mailer := &fakeMailer{}
	builder := NewDefaultBuilder()
	consumer := NewConsumer(mailer, builder, "http://localhost:8080", nil)

	event := events.ReleaseDetected{
		Email:            "dev@example.com",
		RepoOwner:        "golang",
		RepoName:         "go",
		Tag:              "v1.23.0",
		UnsubscribeToken: "unsub-token-123",
	}

	if err := consumer.HandleReleaseDetected(context.Background(), event); err != nil {
		t.Fatalf("HandleReleaseDetected returned error: %v", err)
	}

	msg := mailer.messages[0]

	if msg.To != "dev@example.com" {
		t.Fatalf("expected To=dev@example.com, got %q", msg.To)
	}

	if !strings.Contains(msg.Subject, "golang/go") {
		t.Fatalf("expected subject to contain repo name, got %q", msg.Subject)
	}
	if !strings.Contains(msg.Subject, "v1.23.0") {
		t.Fatalf("expected subject to contain tag, got %q", msg.Subject)
	}

	if !strings.Contains(msg.Body, "golang/go") {
		t.Fatalf("expected body to contain repo name, got %q", msg.Body)
	}
	if !strings.Contains(msg.Body, "v1.23.0") {
		t.Fatalf("expected body to contain tag, got %q", msg.Body)
	}
	if !strings.Contains(msg.Body, "/api/unsubscribe/unsub-token-123") {
		t.Fatalf("expected body to contain unsubscribe link, got %q", msg.Body)
	}
}

func TestHandleReleaseDetectedRaw_Success(t *testing.T) {
	t.Parallel()

	mailer := &fakeMailer{}
	builder := NewDefaultBuilder()
	consumer := NewConsumer(mailer, builder, "http://localhost:8080", nil)

	event := events.ReleaseDetected{
		Email:            "user@example.com",
		RepoOwner:        "owner",
		RepoName:         "repo",
		Tag:              "v3.0.0",
		UnsubscribeToken: "token",
	}
	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("failed to marshal event: %v", err)
	}

	if err := consumer.HandleReleaseDetectedRaw(context.Background(), data); err != nil {
		t.Fatalf("HandleReleaseDetectedRaw returned error: %v", err)
	}

	if len(mailer.messages) != 1 {
		t.Fatalf("expected 1 message sent, got %d", len(mailer.messages))
	}
}

func TestHandleReleaseDetectedRaw_InvalidJSON(t *testing.T) {
	t.Parallel()

	mailer := &fakeMailer{}
	builder := NewDefaultBuilder()
	consumer := NewConsumer(mailer, builder, "http://localhost:8080", nil)

	err := consumer.HandleReleaseDetectedRaw(context.Background(), []byte("not-json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
	if !strings.Contains(err.Error(), "unmarshal event") {
		t.Fatalf("expected 'unmarshal event' in error, got %q", err.Error())
	}
}

type fakeMailer struct {
	messages []Message
	err      error
}

func (f *fakeMailer) Send(_ context.Context, message Message) error {
	f.messages = append(f.messages, message)
	return f.err
}
