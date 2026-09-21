package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPPort string
	Database string

	AWSRegion          string
	AWSAccessKeyID     string
	AWSSecretAccessKey string
	AWSEndpointURL     string

	SQSQueueURL       string
	SQSEventsQueueURL string
	SQSDLQURL         string

	OutboxPollInterval time.Duration
	OutboxBatchSize    int
	OutboxWorkerID     string

	OIDCIssuerURL string
	OIDCClientID  string
	OIDCAudience  string
}

func Load() (Config, error) {
	cfg := Config{
		HTTPPort: getEnv(
			"HTTP_PORT",
			"8080",
		),
		Database: getEnv(
			"DATABASE_URL",
			"postgres://backend:backend@localhost:5432/backend_challenge",
		),

		AWSRegion: getEnv(
			"AWS_REGION",
			"us-east-1",
		),
		AWSAccessKeyID: getEnv(
			"AWS_ACCESS_KEY_ID",
			"test",
		),
		AWSSecretAccessKey: getEnv(
			"AWS_SECRET_ACCESS_KEY",
			"test",
		),
		AWSEndpointURL: getEnv(
			"AWS_ENDPOINT_URL",
			"http://localhost:4566",
		),

		SQSQueueURL: getEnv(
			"SQS_QUEUE_URL",
			"http://localhost:4566/000000000000/wager-transactions.fifo",
		),
		SQSEventsQueueURL: getEnv(
			"SQS_EVENTS_QUEUE_URL",
			"http://localhost:4566/000000000000/wager-events.fifo",
		),
		SQSDLQURL: getEnv(
			"SQS_DLQ_URL",
			"http://localhost:4566/000000000000/wager-transactions-dlq.fifo",
		),

		OutboxPollInterval: getDurationEnv(
			"OUTBOX_POLL_INTERVAL",
			2*time.Second,
		),
		OutboxBatchSize: getIntEnv(
			"OUTBOX_BATCH_SIZE",
			50,
		),
		OutboxWorkerID: getEnv(
			"OUTBOX_WORKER_ID",
			"outbox-publisher",
		),

		OIDCIssuerURL: getEnv(
			"OIDC_ISSUER_URL",
			"",
		),
		OIDCClientID: getEnv(
			"OIDC_CLIENT_ID",
			"",
		),
		OIDCAudience: getEnv(
			"OIDC_AUDIENCE",
			"",
		),
	}

	if cfg.HTTPPort == "" {
		return Config{}, fmt.Errorf(
			"HTTP_PORT is required",
		)
	}

	if cfg.Database == "" {
		return Config{}, fmt.Errorf(
			"DATABASE_URL is required",
		)
	}

	if cfg.AWSRegion == "" {
		return Config{}, fmt.Errorf(
			"AWS_REGION is required",
		)
	}

	if cfg.SQSQueueURL == "" {
		return Config{}, fmt.Errorf(
			"SQS_QUEUE_URL is required",
		)
	}

	if cfg.SQSEventsQueueURL == "" {
		return Config{}, fmt.Errorf(
			"SQS_EVENTS_QUEUE_URL is required",
		)
	}

	if cfg.SQSDLQURL == "" {
		return Config{}, fmt.Errorf(
			"SQS_DLQ_URL is required",
		)
	}

	if cfg.OutboxPollInterval <= 0 {
		return Config{}, fmt.Errorf(
			"OUTBOX_POLL_INTERVAL must be greater than zero",
		)
	}

	if cfg.OutboxBatchSize <= 0 {
		return Config{}, fmt.Errorf(
			"OUTBOX_BATCH_SIZE must be greater than zero",
		)
	}

	if cfg.OutboxWorkerID == "" {
		return Config{}, fmt.Errorf(
			"OUTBOX_WORKER_ID is required",
		)
	}

	return cfg, nil
}

func getEnv(key string, fallback string) string {
	value := os.Getenv(key)

	if value == "" {
		return fallback
	}

	return value
}

func getIntEnv(key string, fallback int) int {
	value := os.Getenv(key)

	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}

	return parsed
}

func getDurationEnv(
	key string,
	fallback time.Duration,
) time.Duration {
	value := os.Getenv(key)

	if value == "" {
		return fallback
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}

	return parsed
}
