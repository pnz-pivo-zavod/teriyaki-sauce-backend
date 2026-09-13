package main

import (
	"context"
	"io"
	"os"

	"github.com/rs/zerolog"

	"github.com/pnz-pivo-zavod/teriyaki-sauce-backend/internal/config"
	"github.com/pnz-pivo-zavod/teriyaki-sauce-backend/internal/lifecycle"
	"github.com/pnz-pivo-zavod/teriyaki-sauce-backend/internal/logging"
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

	logger = logging.Configure(logger, output, cfg.Common)
	// Signals cancel startup too, so SIGTERM while waiting for the migration lock exits cleanly.
	ctx := lifecycle.NotifyContext(logger.WithContext(base))

	pool, err := postgres.InitDB(ctx, cfg.Database)
	if err != nil {
		logger.Error().Msg("database_initialization_failed")
		return 1
	}

	logger.Info().Msg("application_started")
	if err := lifecycle.Run(ctx, cfg.Lifecycle.ShutdownTimeout, nil, lifecycle.ShutdownTask{
		Name: "postgres",
		Run: func(context.Context) error {
			pool.Close()
			return nil
		},
	}); err != nil {
		logger.Error().Msg("application_lifecycle_failed")
		return 1
	}

	return 0
}
