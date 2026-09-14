package logging

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/rs/zerolog"

	"github.com/pnz-pivo-zavod/teriyaki-sauce-backend/internal/config"
)

func TestConfigure(t *testing.T) {
	tests := []struct {
		name            string
		giveEnvironment string
		wantJSON        bool
	}{
		{name: "development -> console", giveEnvironment: config.EnvironmentDevelopment},
		{name: "test -> JSON", giveEnvironment: config.EnvironmentTest, wantJSON: true},
		{name: "production -> JSON", giveEnvironment: config.EnvironmentProduction, wantJSON: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output bytes.Buffer
			base := zerolog.New(&output).With().Timestamp().Str("service", "api").Logger()
			cfg := config.CommonConfig{Environment: tt.giveEnvironment, LogLevel: zerolog.InfoLevel}

			logger := Configure(base, &output, cfg)
			logger.Info().Msg("started")

			if !tt.wantJSON {
				line := output.String()
				hasColors := strings.Contains(line, "\x1b[")
				if hasColors || !strings.Contains(line, "INF") || !strings.Contains(line, "started") {
					t.Errorf("console output = %q", line)
				}

				return
			}

			var event map[string]any
			if err := json.Unmarshal(output.Bytes(), &event); err != nil {
				t.Fatalf("log is not JSON: %v", err)
			}
			if event["service"] != "api" || event["environment"] != tt.giveEnvironment ||
				event["message"] != "started" {
				t.Errorf("event = %#v, want structured fields", event)
			}
		})
	}
}

func TestConfigureLevel(t *testing.T) {
	var output bytes.Buffer
	cfg := config.CommonConfig{Environment: config.EnvironmentTest, LogLevel: zerolog.WarnLevel}

	logger := Configure(zerolog.New(&output), &output, cfg)
	logger.Info().Msg("filtered")
	logger.Warn().Msg("visible")

	got := output.String()
	if strings.Contains(got, "filtered") || !strings.Contains(got, "visible") {
		t.Errorf("level-filtered output = %q", output.String())
	}
}
