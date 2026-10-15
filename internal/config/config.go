package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Port     int
	DBPath   string
	NATSURL  string
	LogLevel string
}

func Load() (*Config, error) {
	port := 8080
	if v := os.Getenv("PORT"); v != "" {
		var err error
		port, err = strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid PORT %q: %w", v, err)
		}
	}

	dbPath := "./data/tenant.db"
	if v := os.Getenv("DB_PATH"); v != "" {
		dbPath = v
	}

	natsURL := "nats://localhost:4222"
	if v := os.Getenv("NATS_URL"); v != "" {
		natsURL = v
	}

	logLevel := "INFO"
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		logLevel = v
	}

	return &Config{
		Port:     port,
		DBPath:   dbPath,
		NATSURL:  natsURL,
		LogLevel: logLevel,
	}, nil
}
