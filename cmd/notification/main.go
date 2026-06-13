package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"releasesapi/internal/modules/notification"
	"releasesapi/internal/platform/config"
	"releasesapi/internal/platform/events"

	"github.com/nats-io/nats.go"
)

func main() {
	logger := log.New(os.Stdout, "", log.LstdFlags|log.LUTC)

	if err := run(logger); err != nil {
		logger.Printf("fatal error: %v", err)
		os.Exit(1)
	}
}

func run(logger *log.Logger) error {
	cfg, err := config.LoadNotification()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	natsConn, err := nats.Connect(cfg.NATS.URL)
	if err != nil {
		return err
	}
	defer natsConn.Close()

	logger.Printf("connected to nats at %s", cfg.NATS.URL)

	smtpMailer := notification.NewSMTPMailer(cfg.SMTP)
	builder := notification.NewDefaultBuilder()
	consumer := notification.NewConsumer(smtpMailer, builder, cfg.AppBaseURL, logger)

	sub, err := natsConn.Subscribe(events.SubjectReleaseDetected, func(msg *nats.Msg) {
		_ = consumer.HandleReleaseDetectedRaw(context.Background(), msg.Data)
	})
	if err != nil {
		return err
	}
	defer sub.Unsubscribe()

	logger.Printf("listening for events on %s", events.SubjectReleaseDetected)

	<-ctx.Done()

	logger.Printf("shutting down notification service")
	return nil
}
