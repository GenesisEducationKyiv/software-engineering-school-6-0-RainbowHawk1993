package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"releasesapi/internal/mailer"
)

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type Config struct {
	Port         string
	GRPCPort     string
	APIKey       string
	DatabaseURL  string
	GitHubToken  string
	AppBaseURL   string
	ScanInterval time.Duration
	SMTP         mailer.SMTPConfig
	Redis        RedisConfig
}

func Load() (Config, error) {
	scanInterval := 15 * time.Minute
	if raw := strings.TrimSpace(os.Getenv("SCAN_INTERVAL")); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil {
			return Config{}, fmt.Errorf("parse SCAN_INTERVAL: %w", err)
		}
		scanInterval = parsed
	}

	smtpPort := 1025
	if raw := strings.TrimSpace(os.Getenv("SMTP_PORT")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			return Config{}, fmt.Errorf("parse SMTP_PORT: %w", err)
		}
		smtpPort = parsed
	}

	redisDB := 0
	if raw := strings.TrimSpace(os.Getenv("REDIS_DB")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			return Config{}, fmt.Errorf("parse REDIS_DB: %w", err)
		}
		redisDB = parsed
	}

	return Config{
		Port:         valueOrDefault("PORT", "8080"),
		GRPCPort:     valueOrDefault("GRPC_PORT", "9090"),
		APIKey:       valueOrDefault("API_KEY", "dev-api-key"),
		DatabaseURL:  valueOrDefault("DATABASE_URL", "postgres://postgres:postgres@db:5432/releases?sslmode=disable"),
		GitHubToken:  strings.TrimSpace(os.Getenv("GITHUB_TOKEN")),
		AppBaseURL:   strings.TrimRight(valueOrDefault("APP_BASE_URL", "http://localhost:8080"), "/"),
		ScanInterval: scanInterval,
		SMTP: mailer.SMTPConfig{
			Host:     valueOrDefault("SMTP_HOST", "mailpit"),
			Port:     smtpPort,
			Username: strings.TrimSpace(os.Getenv("SMTP_USERNAME")),
			Password: strings.TrimSpace(os.Getenv("SMTP_PASSWORD")),
			From:     valueOrDefault("SMTP_FROM", "noreply@releases-api.local"),
		},
		Redis: RedisConfig{
			Addr:     valueOrDefault("REDIS_ADDR", "redis:6379"),
			Password: strings.TrimSpace(os.Getenv("REDIS_PASSWORD")),
			DB:       redisDB,
		},
	}, nil
}

func valueOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	return value
}
