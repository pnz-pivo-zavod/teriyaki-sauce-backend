package main

import (
	"context"
	"io"
	"os"

	"github.com/rs/zerolog"

	"github.com/pnz-pivo-zavod/teriyaki-sauce-backend/internal/config"
	"github.com/pnz-pivo-zavod/teriyaki-sauce-backend/internal/logging"
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

	logger = logging.Configure(logger, output, cfg.Common)
	ctx := logger.WithContext(base)

	command := postgres.CommandUp
	if len(args) > 0 {
		command = args[0]
	}
	if !postgres.SupportedCommand(command) {
		logger.Error().Str("command", command).Msg("unknown_migration_command")
		return 1
	}

	pool, err := postgres.Connect(ctx, cfg.Database.DatabaseConfig)
	if err != nil {
		logger.Error().Msg("database_connection_failed")
		return 1
	}
	defer pool.Close()

	if err := postgres.Migrate(ctx, pool, cfg.Database, command); err != nil {
		return 1
	}

	return 0
}
