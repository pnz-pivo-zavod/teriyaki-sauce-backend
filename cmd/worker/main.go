package main

import (
	"os"

	"github.com/pnz-pivo-zavod/teriyaki-sauce-backend/internal/config"
	"github.com/rs/zerolog"
)

func main() {
	logger := zerolog.New(os.Stderr).With().Timestamp().Str("service", "worker").Logger()

	cfg, err := config.LoadWorker()
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
