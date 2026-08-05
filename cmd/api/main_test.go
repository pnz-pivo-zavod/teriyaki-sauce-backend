package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	t.Run("database unavailable", func(t *testing.T) {
		setValidEnvironment(t)
		var output bytes.Buffer

		if code := run(context.Background(), &output); code != 1 {
			t.Fatalf("run() code = %d, want 1; output=%q", code, output.String())
		}
		if !strings.Contains(output.String(), "database_initialization_failed") {
			t.Errorf("run() output = %q", output.String())
		}
		if strings.Contains(output.String(), "api-token-do-not-leak") ||
			strings.Contains(output.String(), "jwt-secret-do-not-leak") ||
			strings.Contains(output.String(), "api-dsn-do-not-leak") {
			t.Fatalf("run() leaked a secret: %q", output.String())
		}
	})

	t.Run("invalid configuration", func(t *testing.T) {
		t.Setenv("DATABASE_URL", "")
		var output bytes.Buffer

		if code := run(context.Background(), &output); code != 1 {
			t.Fatalf("run() code = %d, want 1", code)
		}
		if !strings.Contains(output.String(), "invalid_or_missing_configuration") {
			t.Errorf("run() output = %q", output.String())
		}
	})
}

func setValidEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("APP_ENV", "test")
	t.Setenv("LOG_LEVEL", "debug")
	// Port 1 refuses immediately, so the entry point fails without waiting.
	t.Setenv("DATABASE_URL", "postgres://user:api-dsn-do-not-leak@127.0.0.1:1/test")
	t.Setenv("DATABASE_CONNECT_TIMEOUT", "1s")
	t.Setenv("TELEGRAM_BOT_TOKEN", "api-token-do-not-leak")
	t.Setenv("MINI_APP_URL", "https://mini.example.test")
	t.Setenv("JWT_ACCESS_SECRET", "jwt-secret-do-not-leak-000000000")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://mini.example.test")
}
