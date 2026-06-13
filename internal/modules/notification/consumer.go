package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"releasesapi/internal/platform/events"
)

type Mailer interface {
	Send(context.Context, Message) error
}

type Consumer struct {
	mailer  Mailer
	builder Builder
	baseURL string
	logger  *log.Logger
}

func NewConsumer(mailer Mailer, builder Builder, baseURL string, logger *log.Logger) *Consumer {
	if logger == nil {
		logger = log.New(nilWriter{}, "", 0)
	}

	return &Consumer{
		mailer:  mailer,
		builder: builder,
		baseURL: strings.TrimRight(baseURL, "/"),
		logger:  logger,
	}
}

func (c *Consumer) HandleReleaseDetected(ctx context.Context, event events.ReleaseDetected) error {
	message := c.builder.BuildReleaseNotificationFromEvent(event, c.baseURL)

	if err := c.mailer.Send(ctx, message); err != nil {
		c.logger.Printf("failed to send notification to %s for %s/%s@%s: %v",
			event.Email, event.RepoOwner, event.RepoName, event.Tag, err)
		return fmt.Errorf("send notification: %w", err)
	}

	c.logger.Printf("sent release notification to %s for %s/%s@%s",
		event.Email, event.RepoOwner, event.RepoName, event.Tag)
	return nil
}

func (c *Consumer) HandleReleaseDetectedRaw(ctx context.Context, data []byte) error {
	var event events.ReleaseDetected
	if err := json.Unmarshal(data, &event); err != nil {
		c.logger.Printf("failed to unmarshal ReleaseDetected event: %v", err)
		return fmt.Errorf("unmarshal event: %w", err)
	}

	return c.HandleReleaseDetected(ctx, event)
}

type nilWriter struct{}

func (nilWriter) Write(p []byte) (int, error) { return len(p), nil }
