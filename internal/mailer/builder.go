package mailer

import (
	"fmt"
	"strings"

	"releasesapi/internal/model"
)

type NotificationBuilder interface {
	BuildConfirmation(sub model.Subscription, baseURL string) Message
	BuildReleaseNotification(sub model.Subscription, tag string, baseURL string) Message
}

type DefaultNotificationBuilder struct{}

func NewDefaultNotificationBuilder() *DefaultNotificationBuilder {
	return &DefaultNotificationBuilder{}
}

func (b *DefaultNotificationBuilder) BuildConfirmation(sub model.Subscription, baseURL string) Message {
	return Message{
		To:      sub.Email,
		Subject: fmt.Sprintf("Confirm release subscription for %s", sub.Repo()),
		Body: strings.Join([]string{
			fmt.Sprintf("Confirm your subscription for %s.", sub.Repo()),
			"",
			fmt.Sprintf("Confirm: %s/api/confirm/%s", baseURL, sub.ConfirmToken),
			fmt.Sprintf("Unsubscribe: %s/api/unsubscribe/%s", baseURL, sub.UnsubscribeToken),
		}, "\n"),
	}
}

func (b *DefaultNotificationBuilder) BuildReleaseNotification(sub model.Subscription, tag string, baseURL string) Message {
	return Message{
		To:      sub.Email,
		Subject: fmt.Sprintf("New release for %s: %s", sub.Repo(), tag),
		Body: strings.Join([]string{
			fmt.Sprintf("A new release is available for %s.", sub.Repo()),
			fmt.Sprintf("Latest tag: %s", tag),
			"",
			fmt.Sprintf("Unsubscribe: %s/api/unsubscribe/%s", baseURL, sub.UnsubscribeToken),
		}, "\n"),
	}
}
