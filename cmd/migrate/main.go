package main

import (
	"context"
	"io"
	"os"

	"github.com/rs/zerolog"

	"github.com/pnz-pivo-zavod/teriyaki-sauce-backend/internal/appctx"
	"github.com/pnz-pivo-zavod/teriyaki-sauce-backend/internal/config"
	"github.com/pnz-pivo-zavod/teriyaki-sauce-backend/internal/repository/postgres"
)

const serviceName = "migrate"

func main() {
	os.Exit(run(context.Background(), os.Stderr, os.Args[1:]))
}

func run(base context.Context, output io.Writer, args []string) int {
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

	//nolint:contextcheck // application already contains the base context passed to run.
	pool, err := postgres.Connect(application, cfg.Database)
	if err != nil {
		application.Logger().Error().Msg("database_connection_failed")
		return 1
	}
	defer pool.Close()

	command := postgres.CommandUp
	if len(args) > 0 {
		command = args[0]
	}

	//nolint:contextcheck // application already contains the base context passed to run.
	if err := postgres.Migrate(application, pool, cfg.Database, command); err != nil {
		return 1
	}

	return 0
}
