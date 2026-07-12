package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	t.Run("valid configuration", func(t *testing.T) {
		t.Setenv("APP_ENV", "test")
		t.Setenv("DATABASE_URL", "postgres://user:password@localhost:5432/test")
		t.Setenv("TELEGRAM_BOT_TOKEN", "worker-token-do-not-leak")
		t.Setenv("MINI_APP_URL", "https://mini.example.test")
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		var output bytes.Buffer

		if code := run(ctx, &output); code != 0 {
			t.Fatalf("run() code = %d, want 0; output=%q", code, output.String())
		}
		if !strings.Contains(output.String(), "shutdown_completed") || strings.Contains(output.String(), "worker-token-do-not-leak") {
			t.Errorf("run() output = %q", output.String())
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
