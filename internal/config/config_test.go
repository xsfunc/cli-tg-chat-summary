package config

import (
	"os"
	"testing"
)

// Helper to set env vars and clean up
func setEnv(t *testing.T, key, value string) {
	t.Helper()
	if err := os.Setenv(key, value); err != nil {
		t.Fatalf("failed to set %s: %v", key, err)
	}
	t.Cleanup(func() {
		_ = os.Unsetenv(key)
	})
}

func unsetEnv(t *testing.T, key string) {
	t.Helper()
	_ = os.Unsetenv(key)
}

func TestLoad_MissingAppID(t *testing.T) {
	// Clear env
	unsetEnv(t, "TG_APP_ID")
	unsetEnv(t, "TG_APP_HASH")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when TG_APP_ID is missing")
	}
	if err.Error() != "TG_APP_ID environment variable is required" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoad_InvalidAppID(t *testing.T) {
	setEnv(t, "TG_APP_ID", "not_a_number")
	setEnv(t, "TG_APP_HASH", "somehash")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when TG_APP_ID is not a number")
	}
	if err.Error() != "TG_APP_ID must be an integer" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoad_MissingAppHash(t *testing.T) {
	setEnv(t, "TG_APP_ID", "12345")
	unsetEnv(t, "TG_APP_HASH")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when TG_APP_HASH is missing")
	}
	if err.Error() != "TG_APP_HASH environment variable is required" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoad_Success(t *testing.T) {
	setEnv(t, "TG_APP_ID", "12345")
	setEnv(t, "TG_APP_HASH", "testhash")
	setEnv(t, "TG_PHONE", "+1234567890")
	setEnv(t, "LOG_LEVEL", "debug")
	setEnv(t, "RATE_LIMIT_MS", "500")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.TelegramAppID != 12345 {
		t.Errorf("expected AppID 12345, got %d", cfg.TelegramAppID)
	}
	if cfg.TelegramAppHash != "testhash" {
		t.Errorf("expected AppHash 'testhash', got %s", cfg.TelegramAppHash)
	}
	if cfg.Phone != "+1234567890" {
		t.Errorf("expected Phone '+1234567890', got %s", cfg.Phone)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected LogLevel 'debug', got %s", cfg.LogLevel)
	}
	if cfg.RateLimitMs != 500 {
		t.Errorf("expected RateLimitMs 500, got %d", cfg.RateLimitMs)
	}
}

func TestLoad_Defaults(t *testing.T) {
	setEnv(t, "TG_APP_ID", "12345")
	setEnv(t, "TG_APP_HASH", "testhash")
	unsetEnv(t, "TG_PHONE")
	unsetEnv(t, "LOG_LEVEL")
	unsetEnv(t, "RATE_LIMIT_MS")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Phone != "" {
		t.Errorf("expected empty Phone, got %s", cfg.Phone)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("expected default LogLevel 'info', got %s", cfg.LogLevel)
	}
	if cfg.RateLimitMs != 350 {
		t.Errorf("expected default RateLimitMs 350, got %d", cfg.RateLimitMs)
	}
}

func TestLoad_InvalidRateLimitFallsBackToDefault(t *testing.T) {
	setEnv(t, "TG_APP_ID", "12345")
	setEnv(t, "TG_APP_HASH", "testhash")
	setEnv(t, "RATE_LIMIT_MS", "not_a_number")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.RateLimitMs != 350 {
		t.Errorf("expected default RateLimitMs 350 on invalid input, got %d", cfg.RateLimitMs)
	}
}
