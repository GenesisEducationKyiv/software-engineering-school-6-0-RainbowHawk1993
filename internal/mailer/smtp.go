package mailer

import (
	"context"
	"fmt"
	"net/smtp"
	"strings"
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
