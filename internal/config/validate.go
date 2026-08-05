package config

import (
	"errors"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var (
	webhookSecretPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,256}$`)
	domainPattern        = regexp.MustCompile(`^[A-Za-z0-9.-]+$`)
)

var ErrInvalidConfig = errors.New("invalid or missing configuration")

type validationError struct {
	field  string
	reason string
}

func (e *validationError) Error() string {
	return "configuration field " + e.field + " " + e.reason
}

func validateAPI(cfg *APIConfig) error {
	if err := validateCommon(&cfg.Common); err != nil {
		return err
	}
	if err := validateDatabaseMigration(cfg.Database); err != nil {
		return err
	}
	if err := validateHTTP(&cfg.HTTP); err != nil {
		return err
	}
	if err := validateTelegramSender(cfg.Telegram.TelegramSenderConfig); err != nil {
		return err
	}

	cfg.Telegram.UpdateMode = normalize(cfg.Telegram.UpdateMode)
	switch cfg.Telegram.UpdateMode {
	case TelegramUpdateModePolling, TelegramUpdateModeWebhook:
	default:
		return invalid("TELEGRAM_UPDATE_MODE", "must be polling or webhook")
	}
	if cfg.Common.Environment == EnvironmentProduction && cfg.Telegram.UpdateMode != TelegramUpdateModeWebhook {
		return invalid("TELEGRAM_UPDATE_MODE", "must be webhook in production")
	}
	if cfg.Telegram.UpdateMode == TelegramUpdateModeWebhook {
		if err := validateHTTPSURL("TELEGRAM_WEBHOOK_URL", cfg.Telegram.WebhookURL); err != nil {
			return err
		}
		if !webhookSecretPattern.MatchString(cfg.Telegram.WebhookSecret) {
			return invalid("TELEGRAM_WEBHOOK_SECRET", "must contain 1 to 256 letters, digits, underscores, or hyphens")
		}
	}

	if len(cfg.JWT.AccessSecret) < 32 {
		return invalid("JWT_ACCESS_SECRET", "must contain at least 32 bytes")
	}
	if strings.TrimSpace(cfg.JWT.Issuer) == "" {
		return invalid("JWT_ISSUER", "must not be empty")
	}
	if strings.TrimSpace(cfg.JWT.Audience) == "" {
		return invalid("JWT_AUDIENCE", "must not be empty")
	}

	if err := validateCORS(&cfg.CORS, cfg.Common.Environment); err != nil {
		return err
	}
	if err := validateCookie(&cfg.Cookie, cfg.Common.Environment); err != nil {
		return err
	}
	if err := validateLifecycle(cfg.Lifecycle); err != nil {
		return err
	}

	return nil
}

func validateWorker(cfg *WorkerConfig) error {
	if err := validateCommon(&cfg.Common); err != nil {
		return err
	}
	if err := validateDatabase(cfg.Database); err != nil {
		return err
	}
	if err := validateTelegramSender(cfg.Telegram); err != nil {
		return err
	}
	if cfg.Reminder.PollInterval <= 0 {
		return invalid("REMINDER_POLL_INTERVAL", "must be greater than zero")
	}
	if cfg.Reminder.LeaseTimeout <= cfg.Reminder.PollInterval {
		return invalid("REMINDER_LEASE_TIMEOUT", "must be greater than REMINDER_POLL_INTERVAL")
	}
	if cfg.Reminder.BatchSize < 1 || cfg.Reminder.BatchSize > 500 {
		return invalid("REMINDER_BATCH_SIZE", "must be between 1 and 500")
	}
	if cfg.Reminder.MaxAttempts < 1 || cfg.Reminder.MaxAttempts > 10 {
		return invalid("REMINDER_MAX_ATTEMPTS", "must be between 1 and 10")
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

	cfg.LogLevel = normalize(cfg.LogLevel)
	switch cfg.LogLevel {
	case "trace", "debug", "info", "warn", "error":
	default:
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

func validateHTTP(cfg *HTTPConfig) error {
	_, portText, err := net.SplitHostPort(strings.TrimSpace(cfg.Address))
	if err != nil {
		return invalid("HTTP_ADDR", "must be a valid host:port address")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return invalid("HTTP_ADDR", "must contain a port between 1 and 65535")
	}
	return nil
}

func validateTelegramSender(cfg TelegramSenderConfig) error {
	if strings.TrimSpace(cfg.BotToken) == "" {
		return invalid("TELEGRAM_BOT_TOKEN", "must not be empty")
	}
	return validateHTTPSURL("MINI_APP_URL", cfg.MiniAppURL)
}

func validateHTTPSURL(field, raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.Fragment != "" || parsed.User != nil {
		return invalid(field, "must be an absolute HTTPS URL without credentials or fragment")
	}
	return nil
}

func validateCORS(cfg *CORSConfig, environment string) error {
	if len(cfg.AllowedOrigins) == 0 {
		return invalid("CORS_ALLOWED_ORIGINS", "must contain at least one origin")
	}

	seen := make(map[string]struct{}, len(cfg.AllowedOrigins))
	origins := make([]string, 0, len(cfg.AllowedOrigins))
	for _, raw := range cfg.AllowedOrigins {
		origin := strings.TrimSpace(raw)
		parsed, err := url.Parse(origin)
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
			return invalid("CORS_ALLOWED_ORIGINS", "must contain valid HTTP origins without paths, credentials, queries, or fragments")
		}
		if environment == EnvironmentProduction && parsed.Scheme != "https" {
			return invalid("CORS_ALLOWED_ORIGINS", "must contain only HTTPS origins in production")
		}
		if _, ok := seen[origin]; ok {
			continue
		}
		seen[origin] = struct{}{}
		origins = append(origins, origin)
	}
	if len(origins) == 0 {
		return invalid("CORS_ALLOWED_ORIGINS", "must contain at least one origin")
	}
	cfg.AllowedOrigins = origins
	return nil
}

func validateCookie(cfg *CookieConfig, environment string) error {
	cfg.SameSite = normalize(cfg.SameSite)
	switch cfg.SameSite {
	case "lax", "strict", "none":
	default:
		return invalid("COOKIE_SAME_SITE", "must be lax, strict, or none")
	}
	if cfg.SameSite == "none" && !cfg.Secure {
		return invalid("COOKIE_SECURE", "must be true when COOKIE_SAME_SITE is none")
	}
	if environment == EnvironmentProduction && !cfg.Secure {
		return invalid("COOKIE_SECURE", "must be true in production")
	}

	domain := strings.TrimSpace(cfg.Domain)
	if domain == "" {
		return nil
	}
	bareDomain := strings.TrimPrefix(domain, ".")
	if bareDomain == "" || strings.Contains(bareDomain, "..") || !domainPattern.MatchString(bareDomain) {
		return invalid("COOKIE_DOMAIN", "must be a hostname without scheme, path, or port")
	}
	cfg.Domain = domain
	return nil
}

func invalid(field, reason string) error {
	return &validationError{field: field, reason: reason}
}

func normalize(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
