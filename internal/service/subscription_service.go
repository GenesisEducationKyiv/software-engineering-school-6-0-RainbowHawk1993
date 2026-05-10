package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/mail"
	"regexp"
	"strings"

	"releasesapi/internal/apperr"
	"releasesapi/internal/mailer"
	"releasesapi/internal/model"
	"releasesapi/internal/store"
)

var tokenPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

type SubscriptionManager interface {
	CreateSubscription(context.Context, store.CreateSubscriptionParams) (model.Subscription, error)
	DeleteSubscription(context.Context, int64) error
	ConfirmByToken(context.Context, string) (model.Subscription, error)
	DeleteByUnsubscribeToken(context.Context, string) error
	ListConfirmedByEmail(context.Context, string) ([]model.Subscription, error)
}

type GitHubClient interface {
	RepoExists(context.Context, string, string) error
	LatestReleaseTag(context.Context, string, string) (string, error)
}

type Mailer interface {
	Send(context.Context, mailer.Message) error
}

type SubscriptionService struct {
	store   SubscriptionManager
	github  GitHubClient
	mailer  Mailer
	baseURL string
}

func NewSubscriptionService(store SubscriptionManager, github GitHubClient, mailer Mailer, baseURL string) *SubscriptionService {
	return &SubscriptionService{
		store:   store,
		github:  github,
		mailer:  mailer,
		baseURL: strings.TrimRight(baseURL, "/"),
	}
}

func (s *SubscriptionService) Subscribe(ctx context.Context, email, repo string) (model.Subscription, error) {
	normalizedEmail, err := normalizeEmail(email)
	if err != nil {
		return model.Subscription{}, err
	}

	owner, name, err := normalizeRepo(repo)
	if err != nil {
		return model.Subscription{}, err
	}

	if err := s.github.RepoExists(ctx, owner, name); err != nil {
		return model.Subscription{}, err
	}

	lastSeenTag, err := s.github.LatestReleaseTag(ctx, owner, name)
	if err != nil {
		return model.Subscription{}, err
	}

	confirmToken, err := generateToken()
	if err != nil {
		return model.Subscription{}, err
	}

	unsubscribeToken, err := generateToken()
	if err != nil {
		return model.Subscription{}, err
	}

	subscription, err := s.store.CreateSubscription(ctx, store.CreateSubscriptionParams{
		Email:            normalizedEmail,
		RepoOwner:        owner,
		RepoName:         name,
		ConfirmToken:     confirmToken,
		UnsubscribeToken: unsubscribeToken,
		LastSeenTag:      lastSeenTag,
	})
	if err != nil {
		return model.Subscription{}, err
	}

	message := mailer.Message{
		To:      subscription.Email,
		Subject: fmt.Sprintf("Confirm release subscription for %s", subscription.Repo()),
		Body: strings.Join([]string{
			fmt.Sprintf("Confirm your subscription for %s.", subscription.Repo()),
			"",
			fmt.Sprintf("Confirm: %s/api/confirm/%s", s.baseURL, subscription.ConfirmToken),
			fmt.Sprintf("Unsubscribe: %s/api/unsubscribe/%s", s.baseURL, subscription.UnsubscribeToken),
		}, "\n"),
	}

	if err := s.mailer.Send(ctx, message); err != nil {
		_ = s.store.DeleteSubscription(ctx, subscription.ID)
		return model.Subscription{}, err
	}

	return subscription, nil
}

func (s *SubscriptionService) Confirm(ctx context.Context, token string) (model.Subscription, error) {
	if !tokenPattern.MatchString(strings.TrimSpace(token)) {
		return model.Subscription{}, apperr.ErrInvalidToken
	}

	return s.store.ConfirmByToken(ctx, token)
}

func (s *SubscriptionService) Unsubscribe(ctx context.Context, token string) error {
	if !tokenPattern.MatchString(strings.TrimSpace(token)) {
		return apperr.ErrInvalidToken
	}

	return s.store.DeleteByUnsubscribeToken(ctx, token)
}

func (s *SubscriptionService) ListByEmail(ctx context.Context, email string) ([]model.Subscription, error) {
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
