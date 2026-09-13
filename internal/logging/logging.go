package logging

import (
	"io"
	"time"

	"github.com/rs/zerolog"

	"github.com/pnz-pivo-zavod/teriyaki-sauce-backend/internal/config"
)

// Configure applies the validated common configuration to the entrypoint logger:
// console output without colors in development, JSON otherwise.
func Configure(logger zerolog.Logger, output io.Writer, cfg config.CommonConfig) zerolog.Logger {
	if cfg.Environment == config.EnvironmentDevelopment {
		output = zerolog.ConsoleWriter{Out: output, NoColor: true, TimeFormat: time.RFC3339}
	}

	return logger.Output(output).
		Level(cfg.LogLevel).
		With().
		Str("environment", cfg.Environment).
		Logger()
}
