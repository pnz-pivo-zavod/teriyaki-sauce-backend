package main

import (
	"context"
	"io"
	"os"

	"github.com/rs/zerolog"

	"github.com/pnz-pivo-zavod/teriyaki-sauce-backend/internal/appctx"
	"github.com/pnz-pivo-zavod/teriyaki-sauce-backend/internal/config"
	"github.com/pnz-pivo-zavod/teriyaki-sauce-backend/internal/lifecycle"
)

const serviceName = "worker"

func main() {
	os.Exit(run(context.Background(), os.Stderr))
}

func run(base context.Context, output io.Writer) int {
	logger := zerolog.New(output).With().Timestamp().Str("service", serviceName).Logger()
	cfg, err := config.LoadWorker()
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

	application.Logger().Info().Msg("application_started")
	//nolint:contextcheck // application already contains the base context passed to run.
	if err := lifecycle.Run(application, cfg.Lifecycle.ShutdownTimeout, nil); err != nil {
		application.Logger().Error().Msg("application_lifecycle_failed")
		return 1
	}

	return 0
}
