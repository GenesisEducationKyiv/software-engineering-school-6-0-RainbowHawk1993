package application

import (
	"context"
	"log"

	"releasesapi/internal/modules/subscription/domain"
	"releasesapi/internal/platform/events"
)

type ReleaseSagaOrchestrator struct {
	store     Store
	publisher EventPublisher
	logger    *log.Logger
}

func NewReleaseSagaOrchestrator(store Store, publisher EventPublisher, logger *log.Logger) *ReleaseSagaOrchestrator {
	if logger == nil {
		logger = log.New(nilWriter{}, "", 0)
	}

	return &ReleaseSagaOrchestrator{
		store:     store,
		publisher: publisher,
		logger:    logger,
	}
}

func (o *ReleaseSagaOrchestrator) ProcessRelease(ctx context.Context, sub domain.Subscription, newTag string) error {
	if err := o.store.UpdateLastSeenTag(ctx, sub.ID, newTag); err != nil {
		return err
	}

	event := events.ReleaseDetected{
		Email:            sub.Email,
		RepoOwner:        sub.RepoOwner,
		RepoName:         sub.RepoName,
		Tag:              newTag,
		UnsubscribeToken: sub.UnsubscribeToken,
	}

	if err := o.publisher.Publish(events.SubjectReleaseDetected, event); err != nil {
		o.logger.Printf("publish failed for sub %d, executing saga compensation: reverting tag to %s", sub.ID, sub.LastSeenTag)

		if compErr := o.store.UpdateLastSeenTag(ctx, sub.ID, sub.LastSeenTag); compErr != nil {
			o.logger.Printf("CRITICAL: saga compensation failed for sub %d: %v", sub.ID, compErr)
		}

		return err
	}

	return nil
}
