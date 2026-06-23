package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"releasesapi/gen/mailv1"
	"releasesapi/internal/modules/notification"
	"releasesapi/internal/platform/config"
	"releasesapi/internal/platform/events"

	"github.com/nats-io/nats.go"
	"google.golang.org/grpc"
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
	defer func() {
		if err := sub.Unsubscribe(); err != nil {
			logger.Printf("failed to unsubscribe: %v", err)
		}
	}()

	logger.Printf("listening for events on %s", events.SubjectReleaseDetected)

	grpcListener, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		return err
	}
	grpcServer := grpc.NewServer()
	mailVerificationServer := notification.NewMailVerificationServer(smtpMailer, builder, cfg.AppBaseURL)
	mailv1.RegisterMailVerificationServiceServer(grpcServer, mailVerificationServer)

	serverErr := make(chan error, 1)
	go func() {
		logger.Printf("grpc listening on :%s", cfg.GRPCPort)
		if err := grpcServer.Serve(grpcListener); err != nil {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		return err
	case <-ctx.Done():
	}

	logger.Printf("shutting down notification service")
	grpcServer.GracefulStop()
	return nil
}
