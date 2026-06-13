package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"

	"releasesapi/internal/modules/github"
	scanapp "releasesapi/internal/modules/scanner/application"
	scannerinfra "releasesapi/internal/modules/scanner/infrastructure"
	"releasesapi/internal/platform/config"
	"releasesapi/internal/platform/metrics"
	nats_platform "releasesapi/internal/platform/nats"

	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
)

func main() {
	logger := log.New(os.Stdout, "", log.LstdFlags|log.LUTC)

	if err := run(logger); err != nil {
		logger.Printf("fatal error: %v", err)
		os.Exit(1)
	}
}

func run(logger *log.Logger) error {
	cfg, err := config.LoadScanner()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	_, serviceMetrics := metrics.NewRegistry()
	serviceMetrics.ServiceUp.Set(1)
	defer serviceMetrics.ServiceUp.Set(0)

	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	defer func() {
		if err := redisClient.Close(); err != nil {
			logger.Printf("failed to close redis connection: %v", err)
		}
	}()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		return err
	}

	store, err := scannerinfra.NewGRPCStore(cfg.SubscriptionAPIGRPCAddr, cfg.APIKey)
	if err != nil {
		return err
	}

	natsConn, err := nats.Connect(cfg.NATS.URL)
	if err != nil {
		return err
	}
	defer natsConn.Close()

	publisher := nats_platform.NewPublisher(natsConn)

	githubCache := github.NewRedisCache(redisClient)
	githubClient := github.NewClient(cfg.GitHubToken, githubCache, serviceMetrics, logger)
	scanner := scanapp.NewScanner(store, githubClient, publisher, logger, cfg.AppBaseURL, serviceMetrics)

	logger.Printf("scanner connected to subscription api at %s", cfg.SubscriptionAPIGRPCAddr)

	if err := scanner.Run(ctx, cfg.ScanInterval); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}

	return nil
}
