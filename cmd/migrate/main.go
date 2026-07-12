package main

import (
	"context"
	"io"
	"os"

	"github.com/rs/zerolog"

	"github.com/pnz-pivo-zavod/teriyaki-sauce-backend/internal/appctx"
	"github.com/pnz-pivo-zavod/teriyaki-sauce-backend/internal/config"
)

const serviceName = "migrate"

func main() {
	os.Exit(run(context.Background(), os.Stderr))
}

func run(base context.Context, output io.Writer) int {
	logger := zerolog.New(output).With().Timestamp().Str("service", serviceName).Logger()
	cfg, err := config.LoadMigrate()
	if err != nil {
		logger.Error().Msg("invalid_or_missing_configuration")
		return 1
	}

	application, err := appctx.New(base, logger, appctx.Options{
		Environment: cfg.Common.Environment,
		Level:       cfg.Common.LogLevel,
		Writer:      output,
	})
	if err != nil {
		logger.Error().Msg("logger_initialization_failed")
		return 1
	}

	application.Logger().Info().Msg("configuration_valid")
	return 0
}
