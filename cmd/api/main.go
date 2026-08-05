package main

import (
	"context"
	"io"
	"os"

	"github.com/rs/zerolog"

	"github.com/pnz-pivo-zavod/teriyaki-sauce-backend/internal/appctx"
	"github.com/pnz-pivo-zavod/teriyaki-sauce-backend/internal/config"
	"github.com/pnz-pivo-zavod/teriyaki-sauce-backend/internal/lifecycle"
	"github.com/pnz-pivo-zavod/teriyaki-sauce-backend/internal/repository/postgres"
)

const serviceName = "api"

func main() {
	os.Exit(run(context.Background(), os.Stderr))
}

func run(base context.Context, output io.Writer) int {
	logger := zerolog.New(output).With().Timestamp().Str("service", serviceName).Logger()
	cfg, err := config.LoadAPI()
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

	//nolint:contextcheck // application already contains the base context passed to run.
	pool, err := postgres.InitDB(application, cfg.Database)
	if err != nil {
		application.Logger().Error().Msg("database_initialization_failed")
		return 1
	}

	application.Logger().Info().Msg("application_started")
	//nolint:contextcheck // application already contains the base context passed to run.
	if err := lifecycle.Run(application, cfg.Lifecycle.ShutdownTimeout, nil, lifecycle.ShutdownTask{
		Name: "postgres",
		Run: func(appctx.Context) error {
			pool.Close()
			return nil
		},
	}); err != nil {
		application.Logger().Error().Msg("application_lifecycle_failed")
		return 1
	}

	return 0
}
