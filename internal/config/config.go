// Package config provides configuration management for the application.
package config

import (
	"errors"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds the application configuration.
type Config struct {
	TelegramAppID   int
	TelegramAppHash string
	Phone           string
	LogLevel        string
	RateLimitMs     int
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	// Load .env file if it exists
	_ = godotenv.Load()

	appIDStr := os.Getenv("TG_APP_ID")
	if appIDStr == "" {
		return nil, errors.New("TG_APP_ID environment variable is required")
	}
	appID, err := strconv.Atoi(appIDStr)
	if err != nil {
		return nil, errors.New("TG_APP_ID must be an integer")
	}

	appHash := os.Getenv("TG_APP_HASH")
	if appHash == "" {
		return nil, errors.New("TG_APP_HASH environment variable is required")
	}

	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}

	rateLimitStr := os.Getenv("RATE_LIMIT_MS")
	rateLimit := 350 // Default safe limit
	if rateLimitStr != "" {
		if r, err := strconv.Atoi(rateLimitStr); err == nil {
			rateLimit = r
		}
	}

	return &Config{
		TelegramAppID:   appID,
		TelegramAppHash: appHash,
		Phone:           os.Getenv("TG_PHONE"),
		LogLevel:        logLevel,
		RateLimitMs:     rateLimit,
	}, nil
}
