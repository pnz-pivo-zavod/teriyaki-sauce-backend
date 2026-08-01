package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	t.Run("database unavailable", func(t *testing.T) {
		t.Setenv("APP_ENV", "test")
		// Port 1 refuses immediately, so the entry point fails without waiting.
		t.Setenv("DATABASE_URL", "postgres://user:worker-dsn-do-not-leak@127.0.0.1:1/test")
		t.Setenv("DATABASE_CONNECT_TIMEOUT", "1s")
		t.Setenv("TELEGRAM_BOT_TOKEN", "worker-token-do-not-leak")
		t.Setenv("MINI_APP_URL", "https://mini.example.test")
		var output bytes.Buffer

		if code := run(context.Background(), &output); code != 1 {
			t.Fatalf("run() code = %d, want 1; output=%q", code, output.String())
		}
		if !strings.Contains(output.String(), "database_initialization_failed") {
			t.Errorf("run() output = %q", output.String())
		}
		if strings.Contains(output.String(), "worker-token-do-not-leak") ||
			strings.Contains(output.String(), "worker-dsn-do-not-leak") {
			t.Fatalf("run() leaked a secret: %q", output.String())
		}
	})

	t.Run("invalid configuration", func(t *testing.T) {
		t.Setenv("DATABASE_URL", "")
		var output bytes.Buffer
		if code := run(context.Background(), &output); code != 1 {
			t.Fatalf("run() code = %d, want 1", code)
		}
	})
}
