// Package config provides configuration management for the application.
package config

import (
	"errors"
	"os"
	"strconv"
)

// Config holds the application configuration.
type Config struct {
	TelegramAppID   int
	TelegramAppHash string
	Phone           string
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	// Simple loader from environment for now
	// In the future, we can add .env support

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

	return &Config{
		TelegramAppID:   appID,
		TelegramAppHash: appHash,
		Phone:           os.Getenv("TG_PHONE"),
	}, nil
}
