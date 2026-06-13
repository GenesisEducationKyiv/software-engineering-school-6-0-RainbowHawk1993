package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/mail"
	"regexp"
	"strings"

	"releasesapi/internal/modules/notification"
	"releasesapi/internal/modules/subscription/domain"
	"releasesapi/internal/modules/subscription/ports"
	"releasesapi/internal/platform/apperr"
)

var tokenPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

type GitHubClient interface {
	RepoExists(context.Context, string, string) error
	LatestReleaseTag(context.Context, string, string) (string, error)
}

type Mailer interface {
	Send(context.Context, notification.Message) error
}

type Service struct {
	store   ports.Repository
	github  GitHubClient
	mailer  Mailer
	builder notification.Builder
	baseURL string
}

func NewService(store ports.Repository, github GitHubClient, mailer Mailer, builder notification.Builder, baseURL string) *Service {
	return &Service{
		store:   store,
		github:  github,
		mailer:  mailer,
		builder: builder,
		baseURL: strings.TrimRight(baseURL, "/"),
	}
}

func (s *Service) Subscribe(ctx context.Context, email, repo string) (domain.Subscription, error) {
	normalizedEmail, err := normalizeEmail(email)
	if err != nil {
		return domain.Subscription{}, err
	}

	owner, name, err := normalizeRepo(repo)
	if err != nil {
		return domain.Subscription{}, err
	}

	if err := s.github.RepoExists(ctx, owner, name); err != nil {
		return domain.Subscription{}, err
	}

	lastSeenTag, err := s.github.LatestReleaseTag(ctx, owner, name)
	if err != nil {
		return domain.Subscription{}, err
	}

	confirmToken, err := generateToken()
	if err != nil {
		return domain.Subscription{}, err
	}

	unsubscribeToken, err := generateToken()
	if err != nil {
		return domain.Subscription{}, err
	}

	subscription, err := s.store.CreateSubscription(ctx, ports.CreateSubscriptionParams{
		Email:            normalizedEmail,
		RepoOwner:        owner,
		RepoName:         name,
		ConfirmToken:     confirmToken,
		UnsubscribeToken: unsubscribeToken,
		LastSeenTag:      lastSeenTag,
	})
	if err != nil {
		return domain.Subscription{}, err
	}

	message := s.builder.BuildConfirmation(subscription, s.baseURL)

	if err := s.mailer.Send(ctx, message); err != nil {
		_ = s.store.DeleteSubscription(ctx, subscription.ID)
		return domain.Subscription{}, err
	}

	return subscription, nil
}

func (s *Service) Confirm(ctx context.Context, token string) (domain.Subscription, error) {
	if !tokenPattern.MatchString(strings.TrimSpace(token)) {
		return domain.Subscription{}, apperr.ErrInvalidToken
	}

	return s.store.ConfirmByToken(ctx, token)
}

func (s *Service) Unsubscribe(ctx context.Context, token string) error {
	if !tokenPattern.MatchString(strings.TrimSpace(token)) {
		return apperr.ErrInvalidToken
	}

	return s.store.DeleteByUnsubscribeToken(ctx, token)
}

func (s *Service) ListByEmail(ctx context.Context, email string) ([]domain.Subscription, error) {
	normalizedEmail, err := normalizeEmail(email)
	if err != nil {
		return nil, err
	}

	return s.store.ListConfirmedByEmail(ctx, normalizedEmail)
}

func normalizeEmail(email string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(email))
	if normalized == "" {
		return "", apperr.ErrInvalidEmail
	}

	if _, err := mail.ParseAddress(normalized); err != nil {
		return "", apperr.ErrInvalidEmail
	}

	return normalized, nil
}

func normalizeRepo(repo string) (string, string, error) {
	trimmed := strings.ToLower(strings.TrimSpace(repo))
	parts := strings.Split(trimmed, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", apperr.ErrInvalidRepoFormat
	}
	if strings.ContainsAny(trimmed, " \t\n\r") {
		return "", "", apperr.ErrInvalidRepoFormat
	}

	return parts[0], parts[1], nil
}

func generateToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	return hex.EncodeToString(bytes), nil
}
