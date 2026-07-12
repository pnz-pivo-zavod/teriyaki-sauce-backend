package appctx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

func TestNewOutputFormats(t *testing.T) {
	tests := []struct {
		name        string
		environment string
		wantJSON    bool
	}{
		{name: "development console", environment: environmentDevelopment},
		{name: "test JSON", environment: environmentTest, wantJSON: true},
		{name: "production JSON", environment: environmentProduction, wantJSON: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output bytes.Buffer
			var base context.Context
			logger := zerolog.New(&output).With().Timestamp().Str("service", "api").Logger()
			ctx, err := New(base, logger, Options{Environment: tt.environment, Level: "info", Writer: &output})
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}

			ctx.Logger().Info().Msg("started")
			line := output.String()
			if tt.wantJSON {
				var event map[string]any
				if err := json.Unmarshal(output.Bytes(), &event); err != nil {
					t.Fatalf("log is not JSON: %v", err)
				}
				if event["service"] != "api" || event["environment"] != tt.environment || event["message"] != "started" {
					t.Errorf("event = %#v, want structured fields", event)
				}
				return
			}
			if !strings.Contains(line, "INF") || !strings.Contains(line, "started") || strings.Contains(line, "\x1b[") {
				t.Errorf("console output = %q", line)
			}
		})
	}
}

func TestNewValidationAndLevel(t *testing.T) {
	tests := []struct {
		name    string
		options Options
	}{
		{name: "invalid environment", options: Options{Environment: "stage", Level: "info"}},
		{name: "invalid level", options: Options{Environment: environmentTest, Level: "verbose"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := New(context.Background(), zerolog.Nop(), tt.options); !errors.Is(err, ErrInvalidOptions) {
				t.Fatalf("New() error = %v, want ErrInvalidOptions", err)
			}
		})
	}

	var output bytes.Buffer
	var base context.Context
	logger := zerolog.New(&output)
	ctx, err := New(base, logger, Options{Environment: environmentTest, Level: "warn", Writer: &output})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	ctx.Logger().Info().Msg("filtered")
	ctx.Logger().Warn().Msg("visible")
	if strings.Contains(output.String(), "filtered") || !strings.Contains(output.String(), "visible") {
		t.Errorf("level-filtered output = %q", output.String())
	}
}

func TestContextDerivation(t *testing.T) {
	var output bytes.Buffer
	var base context.Context
	logger := zerolog.New(&output).With().Timestamp().Str("service", "api").Logger()
	root, err := New(base, logger, Options{Environment: environmentTest, Level: "info", Writer: &output})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	request := root.WithContext(context.Background()).WithRequestID(" request-42 ")
	FromContext(request).Logger().Info().Msg("request")
	root.Logger().Info().Msg("root")

	lines := bytes.Split(bytes.TrimSpace(output.Bytes()), []byte("\n"))
	if len(lines) != 2 {
		t.Fatalf("log lines = %d, want 2", len(lines))
	}
	var requestEvent, rootEvent map[string]any
	if err := json.Unmarshal(lines[0], &requestEvent); err != nil {
		t.Fatalf("request log: %v", err)
	}
	if err := json.Unmarshal(lines[1], &rootEvent); err != nil {
		t.Fatalf("root log: %v", err)
	}
	if requestEvent["request_id"] != "request-42" {
		t.Errorf("request_id = %#v, want request-42", requestEvent["request_id"])
	}
	if _, ok := rootEvent["request_id"]; ok {
		t.Errorf("root context was mutated: %#v", rootEvent)
	}

	empty := root.WithRequestID("  ")
	if empty.Logger().GetLevel() != root.Logger().GetLevel() {
		t.Error("empty request ID changed logger")
	}
}

func TestUnknownContext(t *testing.T) {
	unknown := FromContext(context.Background())
	if unknown.Logger().GetLevel() != zerolog.Disabled {
		t.Errorf("unknown logger level = %s, want disabled", unknown.Logger().GetLevel())
	}
}
