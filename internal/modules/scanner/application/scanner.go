package application

import (
	"context"
	"log"
	"strings"
	"time"

	"releasesapi/internal/modules/notification"
	"releasesapi/internal/modules/subscription/domain"
	"releasesapi/internal/platform/apperr"
	"releasesapi/internal/platform/metrics"
)

type Store interface {
	ListConfirmedForScan(context.Context) ([]domain.Subscription, error)
	UpdateLastSeenTag(context.Context, int64, string) error
}

type GitHubClient interface {
	LatestReleaseTag(context.Context, string, string) (string, error)
}

type Mailer interface {
	Send(context.Context, notification.Message) error
}

type Scanner struct {
	store   Store
	github  GitHubClient
	mailer  Mailer
	builder notification.Builder
	logger  *log.Logger
	baseURL string
	metrics *metrics.ServiceMetrics
}

func NewScanner(store Store, github GitHubClient, mailer Mailer, builder notification.Builder, logger *log.Logger, baseURL string, serviceMetrics *metrics.ServiceMetrics) *Scanner {
	if logger == nil {
		logger = log.New(nilWriter{}, "", 0)
	}

	return &Scanner{
		store:   store,
		github:  github,
		mailer:  mailer,
		builder: builder,
		logger:  logger,
		baseURL: strings.TrimRight(baseURL, "/"),
		metrics: serviceMetrics,
	}
}

func (s *Scanner) Run(ctx context.Context, interval time.Duration) error {
	if interval <= 0 {
		interval = 15 * time.Minute
	}

	if err := s.RunOnce(ctx); err != nil && !isScannerSoftError(err) {
		s.logger.Printf("initial scan failed: %v", err)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := s.RunOnce(ctx); err != nil && !isScannerSoftError(err) {
				s.logger.Printf("scan failed: %v", err)
			}
		}
	}
}

func (s *Scanner) RunOnce(ctx context.Context) error {
	subscriptions, err := s.store.ListConfirmedForScan(ctx)
	if err != nil {
		s.observeScannerRun("error")
		return err
	}

	repoGroups := make(map[string][]domain.Subscription)
	for _, subscription := range subscriptions {
		repoGroups[subscription.Repo()] = append(repoGroups[subscription.Repo()], subscription)
	}

	for _, group := range repoGroups {
		if s.metrics != nil {
			s.metrics.ScannerRepositoriesTotal.Inc()
		}
		repoOwner := group[0].RepoOwner
		repoName := group[0].RepoName

		tag, err := s.github.LatestReleaseTag(ctx, repoOwner, repoName)
		switch err {
		case nil:
		case apperr.ErrRateLimited:
			s.logger.Printf("github rate limit reached while scanning %s/%s", repoOwner, repoName)
			continue
		case apperr.ErrRepoNotFound:
			s.logger.Printf("repository missing during scan: %s/%s", repoOwner, repoName)
			continue
		default:
			s.logger.Printf("release lookup failed for %s/%s: %v", repoOwner, repoName, err)
			continue
		}

		if tag == "" {
			continue
		}

		for _, subscription := range group {
			if subscription.LastSeenTag == tag {
				continue
			}

			message := s.builder.BuildReleaseNotification(subscription, tag, s.baseURL)

			if err := s.mailer.Send(ctx, message); err != nil {
				if s.metrics != nil {
					s.metrics.NotificationsFailedTotal.Inc()
				}
				s.logger.Printf("email send failed for %s: %v", subscription.Email, err)
				continue
			}

			if s.metrics != nil {
				s.metrics.NotificationsSentTotal.Inc()
			}

			if err := s.store.UpdateLastSeenTag(ctx, subscription.ID, tag); err != nil {
				s.logger.Printf("failed to update last_seen_tag for subscription %d: %v", subscription.ID, err)
			}
		}
	}

	s.observeScannerRun("success")
	return nil
}

func (s *Scanner) observeScannerRun(outcome string) {
	if s.metrics == nil {
		return
	}
	s.metrics.ObserveScannerRun(outcome)
}

func isScannerSoftError(err error) bool {
	return err == nil || err == context.Canceled
}

type nilWriter struct{}

func (nilWriter) Write(p []byte) (int, error) {
	return len(p), nil
}
