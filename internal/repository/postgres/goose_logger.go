package postgres

import (
	"fmt"
	"strings"

	"github.com/rs/zerolog"
)

type gooseLogger struct {
	logger *zerolog.Logger
}

func (l gooseLogger) Printf(format string, v ...any) {
	l.logger.Info().Msg(strings.TrimSpace(fmt.Sprintf(format, v...)))
}

func (l gooseLogger) Fatalf(format string, v ...any) {
	l.logger.Error().Msg(strings.TrimSpace(fmt.Sprintf(format, v...)))
}
