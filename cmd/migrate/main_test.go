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
		// Port 1 refuses immediately, so the command fails without waiting.
		t.Setenv("DATABASE_URL", "postgres://user:migrate-secret-do-not-leak@127.0.0.1:1/test")
		t.Setenv("DATABASE_CONNECT_TIMEOUT", "1s")
		t.Setenv("SHUTDOWN_TIMEOUT", "invalid-but-unused")
		var output bytes.Buffer

		if code := run(context.Background(), &output, nil); code != 1 {
			t.Fatalf("run() code = %d, want 1; output=%q", code, output.String())
		}
		if !strings.Contains(output.String(), "database_connection_failed") ||
			strings.Contains(output.String(), "migrate-secret-do-not-leak") {
			t.Errorf("run() output = %q", output.String())
		}
	})

	t.Run("invalid configuration", func(t *testing.T) {
		t.Setenv("DATABASE_URL", "")
		var output bytes.Buffer
		if code := run(context.Background(), &output, []string{"status"}); code != 1 {
			t.Fatalf("run() code = %d, want 1", code)
		}
	})
}
