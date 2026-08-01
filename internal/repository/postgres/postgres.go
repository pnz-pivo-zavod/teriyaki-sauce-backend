package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
	"github.com/rs/zerolog"

	"github.com/pnz-pivo-zavod/teriyaki-sauce-backend/internal/appctx"
	"github.com/pnz-pivo-zavod/teriyaki-sauce-backend/internal/config"
	"github.com/pnz-pivo-zavod/teriyaki-sauce-backend/migrations"
)

const (
	CommandUp     = "up"
	CommandDown   = "down"
	CommandStatus = "status"

	lockRetryPeriodSeconds = 1
)

var (
	ErrConnect = errors.New("postgres connection failed")
	ErrMigrate = errors.New("postgres migration failed")
	ErrCommand = errors.New("unknown migration command")
)

func InitDB(ctx appctx.Context, cfg config.DatabaseConfig) (*pgxpool.Pool, error) {
	pool, err := Connect(ctx, cfg)
	if err != nil {
		return nil, err
	}

	if err := Migrate(ctx, pool, cfg, CommandUp); err != nil {
		pool.Close()
		return nil, err
	}

	return pool, nil
}

func Connect(ctx appctx.Context, cfg config.DatabaseConfig) (*pgxpool.Pool, error) {
	connectContext, cancel := context.WithTimeout(ctx, cfg.ConnectTimeout)
	defer cancel()

	pool, err := pgxpool.New(connectContext, cfg.URL)
	if err != nil {
		return nil, ErrConnect
	}

	if err := pool.Ping(connectContext); err != nil {
		pool.Close()
		return nil, ErrConnect
	}

	ctx.Logger().Info().Msg("postgres_connected")
	return pool, nil
}

func Migrate(ctx appctx.Context, pool *pgxpool.Pool, cfg config.DatabaseConfig, command string) error {
	migrateContext, cancel := context.WithTimeout(ctx, cfg.MigrateTimeout)
	defer cancel()

	logger := ctx.Logger()

	// Closing this handle returns the borrowed connections; the pool stays open.
	db := stdlib.OpenDBFromPool(pool)
	defer func() {
		if err := db.Close(); err != nil {
			logger.Error().Msg("migration_handle_close_failed")
		}
	}()

	provider, err := newProvider(db, cfg, logger)
	if err != nil {
		logger.Error().Msg("migration_provider_failed")
		return ErrMigrate
	}

	if err := runCommand(migrateContext, logger, provider, command); err != nil {
		if errors.Is(err, ErrCommand) {
			logger.Error().Str("command", command).Msg("unknown_migration_command")
			return ErrCommand
		}

		logger.Error().Str("command", command).Msg("migration_failed")
		return ErrMigrate
	}

	logger.Info().Str("command", command).Msg("migration_completed")
	return nil
}

func newProvider(db *sql.DB, cfg config.DatabaseConfig, logger *zerolog.Logger) (*goose.Provider, error) {
	retries := max(cfg.MigrateTimeout/time.Second, 1)

	locker, err := lock.NewPostgresSessionLocker(lock.WithLockTimeout(lockRetryPeriodSeconds, uint64(retries)))
	if err != nil {
		return nil, err
	}

	return goose.NewProvider(goose.DialectPostgres, db, migrations.FS,
		goose.WithSessionLocker(locker),
		goose.WithLogger(gooseLogger{logger: logger}),
	)
}

func runCommand(ctx context.Context, logger *zerolog.Logger, provider *goose.Provider, command string) error {
	switch command {
	case CommandUp:
		results, err := provider.Up(ctx)
		if err != nil {
			return err
		}
		for _, result := range results {
			logResult(logger, result)
		}
		return nil

	case CommandDown:
		result, err := provider.Down(ctx)
		if err != nil {
			return err
		}
		logResult(logger, result)
		return nil

	case CommandStatus:
		items, err := provider.Status(ctx)
		if err != nil {
			return err
		}
		for _, item := range items {
			event := logger.Info().
				Int64("version", item.Source.Version).
				Str("migration", item.Source.Path).
				Str("state", string(item.State))
			if !item.AppliedAt.IsZero() {
				event = event.Time("applied_at", item.AppliedAt)
			}
			event.Msg("migration_status")
		}
		return nil

	default:
		return ErrCommand
	}
}

func logResult(logger *zerolog.Logger, result *goose.MigrationResult) {
	logger.Info().
		Int64("version", result.Source.Version).
		Str("migration", result.Source.Path).
		Str("direction", result.Direction).
		Dur("duration", result.Duration).
		Msg("migration_applied")
}
