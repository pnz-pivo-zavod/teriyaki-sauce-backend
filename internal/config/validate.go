package config

import (
	"errors"
	"net/url"
	"strings"

	"github.com/rs/zerolog"
)

var ErrInvalidConfig = errors.New("invalid or missing configuration")

// ValidationError names the invalid field and never includes its value.
type ValidationError struct {
	Field  string
	Reason string
}

func (e *ValidationError) Error() string {
	return "configuration field " + e.Field + " " + e.Reason
}

func validateAPI(cfg *APIConfig) error {
	if err := validateCommon(&cfg.Common); err != nil {
		return err
	}

	if err := validateDatabaseMigration(cfg.Database); err != nil {
		return err
	}

	return validateLifecycle(cfg.Lifecycle)
}

func validateWorker(cfg *WorkerConfig) error {
	if err := validateCommon(&cfg.Common); err != nil {
		return err
	}

	if err := validateDatabase(cfg.Database); err != nil {
		return err
	}

	return validateLifecycle(cfg.Lifecycle)
}

func validateMigrate(cfg *MigrateConfig) error {
	if err := validateCommon(&cfg.Common); err != nil {
		return err
	}

	return validateDatabaseMigration(cfg.Database)
}

func validateCommon(cfg *CommonConfig) error {
	cfg.Environment = normalize(cfg.Environment)
	switch cfg.Environment {
	case EnvironmentDevelopment, EnvironmentTest, EnvironmentProduction:
	default:
		return invalid("APP_ENV", "must be development, test, or production")
	}

	if cfg.LogLevel < zerolog.TraceLevel || cfg.LogLevel > zerolog.ErrorLevel {
		return invalid("LOG_LEVEL", "must be trace, debug, info, warn, or error")
	}

	return nil
}

func validateDatabase(cfg DatabaseConfig) error {
	raw := strings.TrimSpace(cfg.URL)
	if raw == "" {
		return invalid("DATABASE_URL", "must not be empty")
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return invalid("DATABASE_URL", "must be a valid PostgreSQL URL")
	}

	if parsed.Scheme != "postgres" && parsed.Scheme != "postgresql" {
		return invalid("DATABASE_URL", "must use the postgres or postgresql scheme")
	}

	if parsed.Host == "" || strings.Trim(parsed.Path, "/") == "" {
		return invalid("DATABASE_URL", "must include a host and database name")
	}

	if cfg.ConnectTimeout <= 0 {
		return invalid("DATABASE_CONNECT_TIMEOUT", "must be greater than zero")
	}

	return nil
}

func validateDatabaseMigration(cfg DatabaseMigrationConfig) error {
	if err := validateDatabase(cfg.DatabaseConfig); err != nil {
		return err
	}

	if cfg.MigrateTimeout <= 0 {
		return invalid("DATABASE_MIGRATE_TIMEOUT", "must be greater than zero")
	}

	return nil
}

func validateLifecycle(cfg LifecycleConfig) error {
	if cfg.ShutdownTimeout <= 0 {
		return invalid("SHUTDOWN_TIMEOUT", "must be greater than zero")
	}

	return nil
}

func invalid(field, reason string) error {
	return &ValidationError{Field: field, Reason: reason}
}

func normalize(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
