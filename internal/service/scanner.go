package service

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"releasesapi/internal/apperr"
	"releasesapi/internal/mailer"
	"releasesapi/internal/model"
)

type Scanner struct {
	store   SubscriptionStore
	github  GitHubClient
	mailer  Mailer
	logger  *log.Logger
	baseURL string
}

func NewScanner(store SubscriptionStore, github GitHubClient, mailer Mailer, logger *log.Logger, baseURL string) *Scanner {
	if logger == nil {
		logger = log.New(nilWriter{}, "", 0)
	}

	return &Scanner{
		store:   store,
		github:  github,
		mailer:  mailer,
		logger:  logger,
		baseURL: strings.TrimRight(baseURL, "/"),
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
		return err
	}

	repoGroups := make(map[string][]model.Subscription)
	for _, subscription := range subscriptions {
		repoGroups[subscription.Repo()] = append(repoGroups[subscription.Repo()], subscription)
	}

	for _, group := range repoGroups {
		repoOwner := group[0].RepoOwner
		repoName := group[0].RepoName

		tag, err := s.github.LatestReleaseTag(ctx, repoOwner, repoName)
		switch {
		case err == nil:
		case err == apperr.ErrRateLimited:
			s.logger.Printf("github rate limit reached while scanning %s/%s", repoOwner, repoName)
			continue
		case err == apperr.ErrRepoNotFound:
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

			message := mailer.Message{
				To:      subscription.Email,
				Subject: fmt.Sprintf("New release for %s: %s", subscription.Repo(), tag),
				Body: strings.Join([]string{
					fmt.Sprintf("A new release is available for %s.", subscription.Repo()),
					fmt.Sprintf("Latest tag: %s", tag),
					"",
					fmt.Sprintf("Unsubscribe: %s/api/unsubscribe/%s", s.baseURL, subscription.UnsubscribeToken),
				}, "\n"),
			}

			if err := s.mailer.Send(ctx, message); err != nil {
				s.logger.Printf("email send failed for %s: %v", subscription.Email, err)
				continue
			}

			if err := s.store.UpdateLastSeenTag(ctx, subscription.ID, tag); err != nil {
				s.logger.Printf("failed to update last_seen_tag for subscription %d: %v", subscription.ID, err)
			}
		}
	}

	return nil
}

func isScannerSoftError(err error) bool {
	return err == nil || err == context.Canceled
}

type nilWriter struct{}

func (nilWriter) Write(p []byte) (int, error) {
	return len(p), nil
}
