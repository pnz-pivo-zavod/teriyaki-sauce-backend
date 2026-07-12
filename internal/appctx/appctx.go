package appctx

import (
	"context"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

const (
	environmentDevelopment = "development"
	environmentTest        = "test"
	environmentProduction  = "production"
)

var ErrInvalidOptions = errors.New("invalid application context options")

type loggerContextKey struct{}

type Options struct {
	Environment string
	Level       string
	Writer      io.Writer
}

type Context struct {
	context.Context
	logger zerolog.Logger
}

func New(base context.Context, logger zerolog.Logger, options Options) (Context, error) {
	environment := strings.ToLower(strings.TrimSpace(options.Environment))
	level, err := zerolog.ParseLevel(strings.ToLower(strings.TrimSpace(options.Level)))
	if err != nil {
		return Context{}, ErrInvalidOptions
	}

	if environment != environmentDevelopment && environment != environmentTest && environment != environmentProduction {
		return Context{}, ErrInvalidOptions
	}

	writer := options.Writer
	if writer == nil {
		writer = io.Discard
	}

	if environment == environmentDevelopment {
		writer = zerolog.ConsoleWriter{Out: writer, NoColor: true, TimeFormat: time.RFC3339}
	}

	logger = logger.Output(writer).
		Level(level).
		With().
		Str("environment", environment).
		Logger()

	return withLogger(normalizeBase(base), logger), nil
}

func FromContext(ctx context.Context) Context {
	base := normalizeBase(ctx)
	logger, ok := base.Value(loggerContextKey{}).(zerolog.Logger)
	if !ok {
		logger = zerolog.Nop()
	}

	return withLogger(base, logger)
}

func (ctx Context) Logger() *zerolog.Logger {
	return &ctx.logger
}

func (ctx Context) WithContext(base context.Context) Context {
	return withLogger(normalizeBase(base), ctx.logger)
}

func (ctx Context) WithRequestID(requestID string) Context {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return ctx
	}

	logger := ctx.logger.With().Str("request_id", requestID).Logger()
	return withLogger(ctx.Context, logger)
}

func withLogger(base context.Context, logger zerolog.Logger) Context {
	base = context.WithValue(base, loggerContextKey{}, logger)
	base = logger.WithContext(base)
	return Context{Context: base, logger: logger}
}

func normalizeBase(base context.Context) context.Context {
	if base == nil {
		return context.Background()
	}

	return base
}
