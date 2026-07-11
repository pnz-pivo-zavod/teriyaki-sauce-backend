package main

import (
	"os"

	"github.com/rs/zerolog"

	"github.com/pnz-pivo-zavod/teriyaki-sauce-backend/internal/config"
)

func main() {
	logger := zerolog.New(os.Stderr).With().Timestamp().Str("service", "migrate").Logger()

	cfg, err := config.LoadMigrate()
	if err != nil {
		logger.Error().Msg("invalid or missing configuration")
		os.Exit(1)
	}

	level, err := zerolog.ParseLevel(cfg.Common.LogLevel)
	if err != nil {
		logger.Error().Msg("invalid log level")
		os.Exit(1)
	}

	logger = logger.Level(level)
	logger.Info().Msg("configuration is valid")
}
