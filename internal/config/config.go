package config

import (
	"errors"
	"os"
)

type Config struct {
	HTTPPort string
	Database string
}

func Load() (Config, error) {
	database := os.Getenv("DATABASE_URL")
	if database == "" {
		database = "postgres://backend:backend@localhost:5432/backend_challenge"
	}

	httpPort := os.Getenv("HTTP_PORT")
	if httpPort == "" {
		httpPort = "8080"
	}

	if database == "" {
		return Config{}, errors.New("database url is required")
	}

	return Config{
		HTTPPort: httpPort,
		Database: database,
	}, nil
}
