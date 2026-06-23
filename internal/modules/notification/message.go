package notification

import (
	"context"
	"fmt"
	"net/smtp"
	"strings"

	"releasesapi/internal/modules/subscription/domain"
	"releasesapi/internal/platform/events"
)

type Message struct {
	To      string
	Subject string
	Body    string
}

type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

type Builder interface {
	BuildConfirmation(sub domain.Subscription, baseURL string) Message
	BuildReleaseNotification(sub domain.Subscription, tag string, baseURL string) Message
	BuildReleaseNotificationFromEvent(event events.ReleaseDetected, baseURL string) Message
}

type DefaultBuilder struct{}

func NewDefaultBuilder() *DefaultBuilder {
	return &DefaultBuilder{}
}

func (b *DefaultBuilder) BuildConfirmation(sub domain.Subscription, baseURL string) Message {
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

func (b *DefaultBuilder) BuildReleaseNotification(sub domain.Subscription, tag string, baseURL string) Message {
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

func (b *DefaultBuilder) BuildReleaseNotificationFromEvent(event events.ReleaseDetected, baseURL string) Message {
	repo := event.RepoOwner + "/" + event.RepoName
	return Message{
		To:      event.Email,
		Subject: fmt.Sprintf("New release for %s: %s", repo, event.Tag),
		Body: strings.Join([]string{
			fmt.Sprintf("A new release is available for %s.", repo),
			fmt.Sprintf("Latest tag: %s", event.Tag),
			"",
			fmt.Sprintf("Unsubscribe: %s/api/unsubscribe/%s", baseURL, event.UnsubscribeToken),
		}, "\n"),
	}
}

type SMTPMailer struct {
	config SMTPConfig
}

func NewSMTPMailer(config SMTPConfig) *SMTPMailer {
	return &SMTPMailer{config: config}
}

func (m *SMTPMailer) Send(_ context.Context, message Message) error {
	address := fmt.Sprintf("%s:%d", m.config.Host, m.config.Port)
	payload := strings.Join([]string{
		fmt.Sprintf("To: %s", message.To),
		fmt.Sprintf("From: %s", m.config.From),
		fmt.Sprintf("Subject: %s", message.Subject),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		message.Body,
	}, "\r\n")

	var auth smtp.Auth
	if m.config.Username != "" {
		auth = smtp.PlainAuth("", m.config.Username, m.config.Password, m.config.Host)
	}

	return smtp.SendMail(address, auth, m.config.From, []string{message.To}, []byte(payload))
}
