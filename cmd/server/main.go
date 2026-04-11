package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"releasesapi/internal/api"
	"releasesapi/internal/config"
	githubapi "releasesapi/internal/github"
	"releasesapi/internal/mailer"
	"releasesapi/internal/migrations"
	"releasesapi/internal/service"
	"releasesapi/internal/store"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	logger := log.New(os.Stdout, "", log.LstdFlags|log.LUTC)

	if err := run(logger); err != nil {
		logger.Printf("fatal error: %v", err)
		os.Exit(1)
	}
}

func run(logger *log.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := db.Ping(ctx); err != nil {
		return err
	}

	if err := migrations.Run(ctx, db); err != nil {
		return err
	}

	subscriptionStore := store.NewPostgresSubscriptionStore(db)
	githubClient := githubapi.NewClient(cfg.GitHubToken)
	smtpMailer := mailer.NewSMTPMailer(cfg.SMTP)
	subscriptionService := service.NewSubscriptionService(subscriptionStore, githubClient, smtpMailer, cfg.AppBaseURL)
	scanner := service.NewScanner(subscriptionStore, githubClient, smtpMailer, logger, cfg.AppBaseURL)

	go func() {
		if err := scanner.Run(ctx, cfg.ScanInterval); err != nil && !errors.Is(err, context.Canceled) {
			logger.Printf("scanner stopped: %v", err)
		}
	}()

	router := api.NewRouter(api.NewHandler(subscriptionService), logger)
	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		logger.Printf("listening on :%s", cfg.Port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		return err
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return server.Shutdown(shutdownCtx)
}
