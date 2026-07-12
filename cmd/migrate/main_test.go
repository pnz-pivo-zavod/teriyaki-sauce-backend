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
		t.Setenv("DATABASE_URL", "postgres://user:migrate-secret-do-not-leak@localhost:5432/test")
		t.Setenv("SHUTDOWN_TIMEOUT", "invalid-but-unused")
		var output bytes.Buffer

		if code := run(context.Background(), &output); code != 0 {
			t.Fatalf("run() code = %d, want 0; output=%q", code, output.String())
		}
		if !strings.Contains(output.String(), "configuration_valid") || strings.Contains(output.String(), "migrate-secret-do-not-leak") {
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
