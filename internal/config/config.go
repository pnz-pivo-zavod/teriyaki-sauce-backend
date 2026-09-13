package config

import (
	"time"

	"github.com/rs/zerolog"
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

type TelegramSenderConfig struct {
	BotToken   string `env:"TELEGRAM_BOT_TOKEN" env-required:"true" env-description:"Telegram bot token"`
	MiniAppURL string `env:"MINI_APP_URL" env-required:"true" env-description:"Public HTTPS URL of the Telegram Mini App"`
}

type TelegramAPIConfig struct {
	TelegramSenderConfig
	UpdateMode    string `env:"TELEGRAM_UPDATE_MODE" env-default:"polling" env-description:"Telegram update mode: polling or webhook"`
	WebhookURL    string `env:"TELEGRAM_WEBHOOK_URL" env-description:"Public HTTPS Telegram webhook URL"`
	WebhookSecret string `env:"TELEGRAM_WEBHOOK_SECRET" env-description:"Secret used to authenticate Telegram webhook requests"`
}

type HTTPConfig struct {
	Address string `env:"HTTP_ADDR" env-default:":8080" env-description:"HTTP listen address"`
}

type JWTConfig struct {
	AccessSecret string `env:"JWT_ACCESS_SECRET" env-required:"true" env-description:"Secret used to sign access tokens"`
	Issuer       string `env:"JWT_ISSUER" env-default:"tg-task-tracker" env-description:"Access token issuer"`
	Audience     string `env:"JWT_AUDIENCE" env-default:"tg-mini-app" env-description:"Access token audience"`
}

type CORSConfig struct {
	AllowedOrigins []string `env:"CORS_ALLOWED_ORIGINS" env-required:"true" env-separator:"," env-description:"Comma-separated allowed CORS origins"`
}

type CookieConfig struct {
	Domain   string `env:"COOKIE_DOMAIN" env-description:"Optional refresh cookie domain"`
	Secure   bool   `env:"COOKIE_SECURE" env-default:"false" env-description:"Send refresh cookie only over HTTPS"`
	SameSite string `env:"COOKIE_SAME_SITE" env-default:"lax" env-description:"Refresh cookie SameSite mode: lax, strict, or none"`
}

type ReminderConfig struct {
	PollInterval time.Duration `env:"REMINDER_POLL_INTERVAL" env-default:"10s" env-description:"Interval between reminder queue polls"`
	LeaseTimeout time.Duration `env:"REMINDER_LEASE_TIMEOUT" env-default:"1m" env-description:"Reminder processing lease duration"`
	BatchSize    int           `env:"REMINDER_BATCH_SIZE" env-default:"50" env-description:"Maximum reminders claimed per batch"`
	MaxAttempts  int           `env:"REMINDER_MAX_ATTEMPTS" env-default:"4" env-description:"Maximum reminder delivery attempts"`
}

type LifecycleConfig struct {
	ShutdownTimeout time.Duration `env:"SHUTDOWN_TIMEOUT" env-default:"10s" env-description:"Maximum graceful shutdown duration"`
}

type APIConfig struct {
	Common    CommonConfig
	Database  DatabaseMigrationConfig
	HTTP      HTTPConfig
	Telegram  TelegramAPIConfig
	JWT       JWTConfig
	CORS      CORSConfig
	Cookie    CookieConfig
	Lifecycle LifecycleConfig
}

type WorkerConfig struct {
	Common    CommonConfig
	Database  DatabaseConfig
	Telegram  TelegramSenderConfig
	Reminder  ReminderConfig
	Lifecycle LifecycleConfig
}

type MigrateConfig struct {
	Common   CommonConfig
	Database DatabaseMigrationConfig
}
