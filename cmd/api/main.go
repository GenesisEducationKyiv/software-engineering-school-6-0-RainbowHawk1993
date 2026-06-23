package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	releasev1 "releasesapi/gen/releasev1"
	internalv1 "releasesapi/gen/internalv1"
	"releasesapi/internal/modules/github"
	"releasesapi/internal/modules/notification"
	subapp "releasesapi/internal/modules/subscription/application"
	subinfra "releasesapi/internal/modules/subscription/infrastructure"
	"releasesapi/internal/platform/config"
	"releasesapi/internal/platform/metrics"
	"releasesapi/internal/platform/migrations"
	"releasesapi/internal/transport/grpc/internalapi"
	grpcpublic "releasesapi/internal/transport/grpc/public"
	"releasesapi/internal/transport/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	logger := log.New(os.Stdout, "", log.LstdFlags|log.LUTC)

	if err := run(logger); err != nil {
		logger.Printf("fatal error: %v", err)
		os.Exit(1)
	}
}

func run(logger *log.Logger) error {
	cfg, err := config.LoadAPI()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	registry, serviceMetrics := metrics.NewRegistry()
	serviceMetrics.ServiceUp.Set(1)
	defer serviceMetrics.ServiceUp.Set(0)

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

	subscriptionStore := subinfra.NewPostgresSubscriptionStore(db)
	githubCache := github.NewRedisCache(redisClient)
	githubClient := github.NewClient(cfg.GitHubToken, githubCache, serviceMetrics, logger)
	smtpMailer := notification.NewSMTPMailer(cfg.SMTP)
	notificationBuilder := notification.NewDefaultBuilder()

	var grpcSender subapp.VerificationSender
	if cfg.NotificationGRPCAddr != "" {
		grpcClient, err := subinfra.NewNotificationGRPCClient(cfg.NotificationGRPCAddr)
		if err != nil {
			logger.Printf("failed to connect to notification grpc: %v", err)
		} else {
			grpcSender = grpcClient
			logger.Printf("api connected to notification grpc at %s", cfg.NotificationGRPCAddr)
		}
	}

	subscriptionService := subapp.NewService(subscriptionStore, githubClient, smtpMailer, grpcSender, notificationBuilder, cfg.AppBaseURL)

	router := api.NewRouter(api.NewHandler(subscriptionService), logger, serviceMetrics, promhttp.HandlerFor(registry, promhttp.HandlerOpts{}), cfg.APIKey)
	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	publicGRPCListener, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		return err
	}
	publicGRPCServer := grpc.NewServer(grpc.UnaryInterceptor(grpcpublic.UnaryAPIKeyInterceptor(cfg.APIKey)))
	grpcpublicHandler := grpcpublic.NewServer(subscriptionService)
	releasev1.RegisterSubscriptionServiceServer(publicGRPCServer, grpcpublicHandler)
	reflection.Register(publicGRPCServer)

	internalGRPCListener, err := net.Listen("tcp", ":"+cfg.InternalGRPCPort)
	if err != nil {
		_ = publicGRPCListener.Close()
		return err
	}
	internalGRPCServer := grpc.NewServer(grpc.UnaryInterceptor(grpcpublic.UnaryAPIKeyInterceptor(cfg.APIKey)))
	internalv1.RegisterInternalSubscriptionServiceServer(internalGRPCServer, grpcinternal.NewServer(subscriptionStore))

	serverErr := make(chan error, 3)
	go func() {
		logger.Printf("http listening on :%s", cfg.Port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()
	go func() {
		logger.Printf("public grpc listening on :%s", cfg.GRPCPort)
		if err := publicGRPCServer.Serve(publicGRPCListener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			serverErr <- err
		}
	}()
	go func() {
		logger.Printf("internal grpc listening on :%s", cfg.InternalGRPCPort)
		if err := internalGRPCServer.Serve(internalGRPCListener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		publicGRPCServer.Stop()
		internalGRPCServer.Stop()
		_ = publicGRPCListener.Close()
		_ = internalGRPCListener.Close()
		return err
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	publicStopped := make(chan struct{})
	go func() {
		publicGRPCServer.GracefulStop()
		close(publicStopped)
	}()

	internalStopped := make(chan struct{})
	go func() {
		internalGRPCServer.GracefulStop()
		close(internalStopped)
	}()

	select {
	case <-publicStopped:
	case <-shutdownCtx.Done():
		publicGRPCServer.Stop()
	}
	select {
	case <-internalStopped:
	case <-shutdownCtx.Done():
		internalGRPCServer.Stop()
	}

	_ = publicGRPCListener.Close()
	_ = internalGRPCListener.Close()

	return server.Shutdown(shutdownCtx)
}
