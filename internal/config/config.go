package config

import (
	"time"

	"github.com/rs/zerolog"
)

// Fields are added in the stage that introduces their consumer; see the
// configuration contract in PROJECT_CONTEXT.md for requirements of future fields.

const (
	EnvironmentDevelopment = "development"
	EnvironmentTest        = "test"
	EnvironmentProduction  = "production"
)

const (
	configFileField = "CONFIG_FILE"
	envField        = "APP_ENV"
)

type CommonConfig struct {
	Environment string        `env:"APP_ENV" env-default:"development" env-description:"Application environment: development, test, or production"`
	LogLevel    zerolog.Level `env:"LOG_LEVEL" env-default:"info" env-description:"Log level: trace, debug, info, warn, or error"`
}

type DatabaseConfig struct {
	URL            string        `env:"DATABASE_URL" env-required:"true" env-description:"PostgreSQL connection URL"`
	ConnectTimeout time.Duration `env:"DATABASE_CONNECT_TIMEOUT" env-default:"15s" env-description:"Maximum duration of the PostgreSQL connect and ping"`
}

// DatabaseMigrationConfig is the database contract of the processes that run
// migrations. The worker uses the plain DatabaseConfig so a broken migration
// timeout cannot stop a process that never migrates.
type DatabaseMigrationConfig struct {
	DatabaseConfig
	MigrateTimeout time.Duration `env:"DATABASE_MIGRATE_TIMEOUT" env-default:"3m" env-description:"Maximum duration of a migration run, including the lock wait"`
}

type LifecycleConfig struct {
	ShutdownTimeout time.Duration `env:"SHUTDOWN_TIMEOUT" env-default:"10s" env-description:"Maximum graceful shutdown duration"`
}

type APIConfig struct {
	Common    CommonConfig
	Database  DatabaseMigrationConfig
	Lifecycle LifecycleConfig
}

type WorkerConfig struct {
	Common    CommonConfig
	Database  DatabaseConfig
	Lifecycle LifecycleConfig
}

type MigrateConfig struct {
	Common   CommonConfig
	Database DatabaseMigrationConfig
}
