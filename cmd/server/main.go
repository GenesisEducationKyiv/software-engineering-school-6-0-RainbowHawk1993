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
	"releasesapi/internal/api"
	"releasesapi/internal/config"
	githubapi "releasesapi/internal/github"
	"releasesapi/internal/grpcapi"
	"releasesapi/internal/mailer"
	appmetrics "releasesapi/internal/metrics"
	"releasesapi/internal/migrations"
	"releasesapi/internal/service"
	"releasesapi/internal/store"

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
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	registry, serviceMetrics := appmetrics.NewRegistry()
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
	defer redisClient.Close()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		return err
	}

	subscriptionStore := store.NewPostgresSubscriptionStore(db)
	githubCache := githubapi.NewRedisCache(redisClient)
	githubClient := githubapi.NewClient(cfg.GitHubToken, githubCache, serviceMetrics)
	smtpMailer := mailer.NewSMTPMailer(cfg.SMTP)
	subscriptionService := service.NewSubscriptionService(subscriptionStore, githubClient, smtpMailer, cfg.AppBaseURL)
	scanner := service.NewScanner(subscriptionStore, githubClient, smtpMailer, logger, cfg.AppBaseURL, serviceMetrics)

	go func() {
		if err := scanner.Run(ctx, cfg.ScanInterval); err != nil && !errors.Is(err, context.Canceled) {
			logger.Printf("scanner stopped: %v", err)
		}
	}()

	router := api.NewRouter(api.NewHandler(subscriptionService), logger, serviceMetrics, promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	grpcListener, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		return err
	}
	grpcServer := grpc.NewServer()
	grpcHandler := grpcapi.NewServer(subscriptionService)
	releasev1.RegisterSubscriptionServiceServer(grpcServer, grpcHandler)
	reflection.Register(grpcServer)

	serverErr := make(chan error, 2)
	go func() {
		logger.Printf("http listening on :%s", cfg.Port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()
	go func() {
		logger.Printf("grpc listening on :%s", cfg.GRPCPort)
		if err := grpcServer.Serve(grpcListener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		grpcServer.Stop()
		_ = grpcListener.Close()
		return err
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	grpcStopped := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(grpcStopped)
	}()

	select {
	case <-grpcStopped:
	case <-shutdownCtx.Done():
		grpcServer.Stop()
	}

	if err := grpcListener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
		return err
	}

	return server.Shutdown(shutdownCtx)
}
