package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

const (
	testDatabaseURL = "postgres://user:password@localhost:5432/teriyaki_sauce?sslmode=disable"
	testBotToken    = "123456789:test_bot_token"
	testMiniAppURL  = "https://mini.example.test"
	testJWTSecret   = "0123456789abcdef0123456789abcdef"
)

var configEnvironmentNames = []string{
	"CONFIG_FILE",
	"APP_ENV",
	"LOG_LEVEL",
	"SHUTDOWN_TIMEOUT",
	"DATABASE_URL",
	"DATABASE_CONNECT_TIMEOUT",
	"DATABASE_MIGRATE_TIMEOUT",
	"TELEGRAM_BOT_TOKEN",
	"MINI_APP_URL",
	"HTTP_ADDR",
	"JWT_ACCESS_SECRET",
	"JWT_ISSUER",
	"JWT_AUDIENCE",
	"TELEGRAM_UPDATE_MODE",
	"TELEGRAM_WEBHOOK_URL",
	"TELEGRAM_WEBHOOK_SECRET",
	"CORS_ALLOWED_ORIGINS",
	"COOKIE_DOMAIN",
	"COOKIE_SECURE",
	"COOKIE_SAME_SITE",
	"REMINDER_POLL_INTERVAL",
	"REMINDER_LEASE_TIMEOUT",
	"REMINDER_BATCH_SIZE",
	"REMINDER_MAX_ATTEMPTS",
}

func TestLoadAPIFromEnvironment(t *testing.T) {
	cleanEnvironment(t)
	setValidAPIEnvironment(t)
	t.Setenv("APP_ENV", " TEST ")
	t.Setenv("LOG_LEVEL", "DEBUG")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://one.example.test, https://two.example.test,https://one.example.test")

	cfg, err := LoadAPI()
	if err != nil {
		t.Fatalf("LoadAPI() error = %v", err)
	}

	if cfg.Common.Environment != EnvironmentTest {
		t.Errorf("Environment = %q, want %q", cfg.Common.Environment, EnvironmentTest)
	}
	if cfg.Common.LogLevel != zerolog.DebugLevel {
		t.Errorf("LogLevel = %s, want debug", cfg.Common.LogLevel)
	}
	if cfg.HTTP.Address != ":8080" {
		t.Errorf("HTTP address = %q, want :8080", cfg.HTTP.Address)
	}
	if cfg.Telegram.UpdateMode != TelegramUpdateModePolling {
		t.Errorf("UpdateMode = %q, want polling", cfg.Telegram.UpdateMode)
	}
	if len(cfg.CORS.AllowedOrigins) != 2 {
		t.Errorf("AllowedOrigins = %v, want two unique origins", cfg.CORS.AllowedOrigins)
	}
	if cfg.Lifecycle.ShutdownTimeout != 10*time.Second {
		t.Errorf("ShutdownTimeout = %s, want 10s", cfg.Lifecycle.ShutdownTimeout)
	}
}

func TestLoadWorkerFromEnvironment(t *testing.T) {
	cleanEnvironment(t)
	setValidWorkerEnvironment(t)

	cfg, err := LoadWorker()
	if err != nil {
		t.Fatalf("LoadWorker() error = %v", err)
	}

	if cfg.Reminder.PollInterval != 10*time.Second {
		t.Errorf("PollInterval = %s, want 10s", cfg.Reminder.PollInterval)
	}
	if cfg.Reminder.LeaseTimeout != time.Minute {
		t.Errorf("LeaseTimeout = %s, want 1m", cfg.Reminder.LeaseTimeout)
	}
	if cfg.Reminder.BatchSize != 50 || cfg.Reminder.MaxAttempts != 4 {
		t.Errorf("Reminder limits = %d/%d, want 50/4", cfg.Reminder.BatchSize, cfg.Reminder.MaxAttempts)
	}
	if cfg.Lifecycle.ShutdownTimeout != 10*time.Second {
		t.Errorf("ShutdownTimeout = %s, want 10s", cfg.Lifecycle.ShutdownTimeout)
	}
}

func TestLoadLifecycleConfig(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    time.Duration
		wantErr bool
	}{
		{name: "default", want: 10 * time.Second},
		{name: "custom", value: "25s", want: 25 * time.Second},
		{name: "zero", value: "0s", wantErr: true},
		{name: "negative", value: "-1s", wantErr: true},
		{name: "invalid", value: "secret-invalid-duration", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanEnvironment(t)
			setValidWorkerEnvironment(t)
			if tt.value != "" {
				t.Setenv("SHUTDOWN_TIMEOUT", tt.value)
			}

			cfg, err := LoadWorker()
			if tt.wantErr {
				if err == nil {
					t.Fatal("LoadWorker() error = nil, want error")
				}
				if strings.Contains(err.Error(), tt.value) {
					t.Fatalf("error exposed value: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("LoadWorker() error = %v", err)
			}
			if cfg.Lifecycle.ShutdownTimeout != tt.want {
				t.Errorf("ShutdownTimeout = %s, want %s", cfg.Lifecycle.ShutdownTimeout, tt.want)
			}
		})
	}
}

func TestLoadDatabaseTimeouts(t *testing.T) {
	tests := []struct {
		name     string
		variable string
		value    string
		want     time.Duration
		wantErr  bool
	}{
		{name: "connect default", variable: "DATABASE_CONNECT_TIMEOUT", want: 15 * time.Second},
		{name: "connect custom", variable: "DATABASE_CONNECT_TIMEOUT", value: "5s", want: 5 * time.Second},
		{name: "connect zero", variable: "DATABASE_CONNECT_TIMEOUT", value: "0s", wantErr: true},
		{name: "connect negative", variable: "DATABASE_CONNECT_TIMEOUT", value: "-1s", wantErr: true},
		{name: "connect invalid", variable: "DATABASE_CONNECT_TIMEOUT", value: "secret-invalid-duration", wantErr: true},
		{name: "migrate default", variable: "DATABASE_MIGRATE_TIMEOUT", want: 3 * time.Minute},
		{name: "migrate custom", variable: "DATABASE_MIGRATE_TIMEOUT", value: "30s", want: 30 * time.Second},
		{name: "migrate zero", variable: "DATABASE_MIGRATE_TIMEOUT", value: "0s", wantErr: true},
		{name: "migrate negative", variable: "DATABASE_MIGRATE_TIMEOUT", value: "-1s", wantErr: true},
		{name: "migrate invalid", variable: "DATABASE_MIGRATE_TIMEOUT", value: "secret-invalid-duration", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanEnvironment(t)
			t.Setenv("DATABASE_URL", testDatabaseURL)
			if tt.value != "" {
				t.Setenv(tt.variable, tt.value)
			}

			cfg, err := LoadMigrate()
			if tt.wantErr {
				if err == nil {
					t.Fatal("LoadMigrate() error = nil, want error")
				}
				if strings.Contains(err.Error(), tt.value) {
					t.Fatalf("error exposed value: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("LoadMigrate() error = %v", err)
			}

			got := cfg.Database.ConnectTimeout
			if tt.variable == "DATABASE_MIGRATE_TIMEOUT" {
				got = cfg.Database.MigrateTimeout
			}
			if got != tt.want {
				t.Errorf("%s = %s, want %s", tt.variable, got, tt.want)
			}
		})
	}
}

func TestMigrationTimeoutAppliesOnlyToMigratingProcesses(t *testing.T) {
	t.Run("worker ignores it", func(t *testing.T) {
		cleanEnvironment(t)
		setValidWorkerEnvironment(t)
		t.Setenv("DATABASE_MIGRATE_TIMEOUT", "invalid-for-worker")

		if _, err := LoadWorker(); err != nil {
			t.Fatalf("LoadWorker() error = %v, the worker never runs migrations", err)
		}
	})

	t.Run("api rejects it", func(t *testing.T) {
		cleanEnvironment(t)
		setValidAPIEnvironment(t)
		t.Setenv("DATABASE_MIGRATE_TIMEOUT", "0s")

		if _, err := LoadAPI(); err == nil {
			t.Fatal("LoadAPI() error = nil, want an error for a zero migration timeout")
		}
	})
}

func TestLoadMigrateRequiresOnlyDatabaseAndCommonConfig(t *testing.T) {
	cleanEnvironment(t)
	t.Setenv("DATABASE_URL", testDatabaseURL)
	t.Setenv("SHUTDOWN_TIMEOUT", "invalid-for-migrate")

	cfg, err := LoadMigrate()
	if err != nil {
		t.Fatalf("LoadMigrate() error = %v", err)
	}
	if cfg.Common.Environment != EnvironmentDevelopment || cfg.Common.LogLevel != zerolog.InfoLevel {
		t.Errorf("common defaults = %#v, want development/info", cfg.Common)
	}
}

func TestMissingRequiredFieldIsSanitized(t *testing.T) {
	cleanEnvironment(t)

	_, err := LoadMigrate()
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("error = %v, want ErrInvalidConfig", err)
	}
}

func TestConfigFileOverridesProcessEnvironment(t *testing.T) {
	cleanEnvironment(t)
	t.Setenv("APP_ENV", EnvironmentDevelopment)
	t.Setenv("DATABASE_URL", "postgres://shell:shell@localhost:5432/shell")

	path := writeConfigFile(t, strings.Join([]string{
		"APP_ENV=test",
		"LOG_LEVEL=error",
		"DATABASE_URL=" + testDatabaseURL,
	}, "\n"))
	t.Setenv("CONFIG_FILE", path)

	cfg, err := LoadMigrate()
	if err != nil {
		t.Fatalf("LoadMigrate() error = %v", err)
	}
	if cfg.Common.Environment != EnvironmentTest || cfg.Common.LogLevel != zerolog.ErrorLevel {
		t.Errorf("file common config = %#v, want test/error", cfg.Common)
	}
	if cfg.Database.URL != testDatabaseURL {
		t.Errorf("Database URL did not come from config file")
	}
}

func TestConfigFileIsForbiddenInProduction(t *testing.T) {
	t.Run("process environment", func(t *testing.T) {
		cleanEnvironment(t)
		t.Setenv("APP_ENV", EnvironmentProduction)
		t.Setenv("CONFIG_FILE", writeConfigFile(t, "APP_ENV=development\n"))

		_, err := LoadMigrate()
		assertErrorContains(t, err, "CONFIG_FILE")
	})

	t.Run("config file", func(t *testing.T) {
		cleanEnvironment(t)
		t.Setenv("CONFIG_FILE", writeConfigFile(t, strings.Join([]string{
			"APP_ENV=production",
			"DATABASE_URL=" + testDatabaseURL,
		}, "\n")))

		_, err := LoadMigrate()
		assertErrorContains(t, err, "CONFIG_FILE")
	})
}

func TestLoadAPIValidation(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testing.T)
		field  string
	}{
		{name: "environment", mutate: func(t *testing.T) { t.Setenv("APP_ENV", "staging") }, field: "APP_ENV"},
		{name: "log level", mutate: func(t *testing.T) { t.Setenv("LOG_LEVEL", "fatal") }, field: "LOG_LEVEL"},
		{name: "database scheme", mutate: func(t *testing.T) { t.Setenv("DATABASE_URL", "mysql://localhost/db") }, field: "DATABASE_URL"},
		{name: "database name", mutate: func(t *testing.T) { t.Setenv("DATABASE_URL", "postgres://localhost") }, field: "DATABASE_URL"},
		{name: "HTTP address", mutate: func(t *testing.T) { t.Setenv("HTTP_ADDR", "8080") }, field: "HTTP_ADDR"},
		{name: "HTTP port", mutate: func(t *testing.T) { t.Setenv("HTTP_ADDR", ":0") }, field: "HTTP_ADDR"},
		{name: "bot token", mutate: func(t *testing.T) { t.Setenv("TELEGRAM_BOT_TOKEN", "") }, field: "TELEGRAM_BOT_TOKEN"},
		{name: "Mini App URL", mutate: func(t *testing.T) { t.Setenv("MINI_APP_URL", "http://mini.example.test") }, field: "MINI_APP_URL"},
		{name: "update mode", mutate: func(t *testing.T) { t.Setenv("TELEGRAM_UPDATE_MODE", "queue") }, field: "TELEGRAM_UPDATE_MODE"},
		{name: "JWT secret", mutate: func(t *testing.T) { t.Setenv("JWT_ACCESS_SECRET", "short") }, field: "JWT_ACCESS_SECRET"},
		{name: "CORS path", mutate: func(t *testing.T) { t.Setenv("CORS_ALLOWED_ORIGINS", "https://example.test/path") }, field: "CORS_ALLOWED_ORIGINS"},
		{name: "SameSite", mutate: func(t *testing.T) { t.Setenv("COOKIE_SAME_SITE", "sometimes") }, field: "COOKIE_SAME_SITE"},
		{name: "SameSite none without secure", mutate: func(t *testing.T) { t.Setenv("COOKIE_SAME_SITE", "none") }, field: "COOKIE_SECURE"},
		{name: "cookie domain", mutate: func(t *testing.T) { t.Setenv("COOKIE_DOMAIN", "https://example.test") }, field: "COOKIE_DOMAIN"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanEnvironment(t)
			setValidAPIEnvironment(t)
			tt.mutate(t)

			_, err := LoadAPI()
			assertErrorContains(t, err, tt.field)
		})
	}
}

func TestLoadAPIWebhookValidation(t *testing.T) {
	t.Run("valid production webhook", func(t *testing.T) {
		cleanEnvironment(t)
		setValidAPIEnvironment(t)
		t.Setenv("APP_ENV", EnvironmentProduction)
		t.Setenv("TELEGRAM_UPDATE_MODE", TelegramUpdateModeWebhook)
		t.Setenv("TELEGRAM_WEBHOOK_URL", "https://api.example.test/telegram/webhook")
		t.Setenv("TELEGRAM_WEBHOOK_SECRET", "valid_webhook-secret")
		t.Setenv("COOKIE_SECURE", "true")

		if _, err := LoadAPI(); err != nil {
			t.Fatalf("LoadAPI() error = %v", err)
		}
	})

	tests := []struct {
		name   string
		mutate func(*testing.T)
		field  string
	}{
		{name: "polling in production", mutate: func(t *testing.T) {
			t.Setenv("APP_ENV", EnvironmentProduction)
			t.Setenv("COOKIE_SECURE", "true")
		}, field: "TELEGRAM_UPDATE_MODE"},
		{name: "missing webhook URL", mutate: func(t *testing.T) {
			t.Setenv("TELEGRAM_UPDATE_MODE", TelegramUpdateModeWebhook)
			t.Setenv("TELEGRAM_WEBHOOK_SECRET", "secret")
		}, field: "TELEGRAM_WEBHOOK_URL"},
		{name: "invalid webhook secret", mutate: func(t *testing.T) {
			t.Setenv("TELEGRAM_UPDATE_MODE", TelegramUpdateModeWebhook)
			t.Setenv("TELEGRAM_WEBHOOK_URL", "https://api.example.test/webhook")
			t.Setenv("TELEGRAM_WEBHOOK_SECRET", "invalid secret")
		}, field: "TELEGRAM_WEBHOOK_SECRET"},
		{name: "HTTP CORS in production", mutate: func(t *testing.T) {
			t.Setenv("APP_ENV", EnvironmentProduction)
			t.Setenv("TELEGRAM_UPDATE_MODE", TelegramUpdateModeWebhook)
			t.Setenv("TELEGRAM_WEBHOOK_URL", "https://api.example.test/webhook")
			t.Setenv("TELEGRAM_WEBHOOK_SECRET", "secret")
			t.Setenv("COOKIE_SECURE", "true")
			t.Setenv("CORS_ALLOWED_ORIGINS", "http://app.example.test")
		}, field: "CORS_ALLOWED_ORIGINS"},
		{name: "insecure production cookie", mutate: func(t *testing.T) {
			t.Setenv("APP_ENV", EnvironmentProduction)
			t.Setenv("TELEGRAM_UPDATE_MODE", TelegramUpdateModeWebhook)
			t.Setenv("TELEGRAM_WEBHOOK_URL", "https://api.example.test/webhook")
			t.Setenv("TELEGRAM_WEBHOOK_SECRET", "secret")
		}, field: "COOKIE_SECURE"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanEnvironment(t)
			setValidAPIEnvironment(t)
			tt.mutate(t)

			_, err := LoadAPI()
			assertErrorContains(t, err, tt.field)
		})
	}
}

func TestLoadWorkerValidation(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testing.T)
		field  string
	}{
		{name: "poll interval", mutate: func(t *testing.T) { t.Setenv("REMINDER_POLL_INTERVAL", "0s") }, field: "REMINDER_POLL_INTERVAL"},
		{name: "lease timeout", mutate: func(t *testing.T) {
			t.Setenv("REMINDER_POLL_INTERVAL", "10s")
			t.Setenv("REMINDER_LEASE_TIMEOUT", "10s")
		}, field: "REMINDER_LEASE_TIMEOUT"},
		{name: "batch size low", mutate: func(t *testing.T) { t.Setenv("REMINDER_BATCH_SIZE", "0") }, field: "REMINDER_BATCH_SIZE"},
		{name: "batch size high", mutate: func(t *testing.T) { t.Setenv("REMINDER_BATCH_SIZE", "501") }, field: "REMINDER_BATCH_SIZE"},
		{name: "attempts low", mutate: func(t *testing.T) { t.Setenv("REMINDER_MAX_ATTEMPTS", "0") }, field: "REMINDER_MAX_ATTEMPTS"},
		{name: "attempts high", mutate: func(t *testing.T) { t.Setenv("REMINDER_MAX_ATTEMPTS", "11") }, field: "REMINDER_MAX_ATTEMPTS"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanEnvironment(t)
			setValidWorkerEnvironment(t)
			tt.mutate(t)

			_, err := LoadWorker()
			assertErrorContains(t, err, tt.field)
		})
	}
}

func TestLoadErrorsDoNotExposeSecrets(t *testing.T) {
	tests := []struct {
		name   string
		secret string
		setup  func(*testing.T, string)
		load   func() error
	}{
		{
			name:   "database URL",
			secret: "database-password-do-not-leak",
			setup: func(t *testing.T, secret string) {
				t.Setenv("DATABASE_URL", "postgres://user:"+secret+"@")
			},
			load: func() error { _, err := LoadMigrate(); return err },
		},
		{
			name:   "JWT secret",
			secret: "jwt-secret-do-not-leak",
			setup: func(t *testing.T, secret string) {
				setValidAPIEnvironment(t)
				t.Setenv("JWT_ACCESS_SECRET", secret)
			},
			load: func() error { _, err := LoadAPI(); return err },
		},
		{
			name:   "invalid config file",
			secret: "file-secret-do-not-leak",
			setup: func(t *testing.T, secret string) {
				t.Setenv("CONFIG_FILE", writeConfigFile(t, "BROKEN='"+secret))
			},
			load: func() error { _, err := LoadMigrate(); return err },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanEnvironment(t)
			tt.setup(t, tt.secret)
			err := tt.load()
			if err == nil {
				t.Fatal("load error = nil, want error")
			}
			if strings.Contains(err.Error(), tt.secret) {
				t.Fatalf("error exposed secret: %v", err)
			}
		})
	}
}

func TestCleanenvErrorsAreSanitized(t *testing.T) {
	cleanEnvironment(t)
	t.Setenv("DATABASE_URL", testDatabaseURL)
	t.Setenv("LOG_LEVEL", "info")
	t.Setenv("APP_ENV", "development")
	t.Setenv("REMINDER_BATCH_SIZE", "not-a-number-with-secret")
	t.Setenv("TELEGRAM_BOT_TOKEN", testBotToken)
	t.Setenv("MINI_APP_URL", testMiniAppURL)

	_, err := LoadWorker()
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("error = %v, want ErrInvalidConfig", err)
	}
	if err.Error() != ErrInvalidConfig.Error() {
		t.Fatalf("error = %q, want sanitized error", err)
	}
}

func TestUnknownLogLevelIsSanitized(t *testing.T) {
	cleanEnvironment(t)
	t.Setenv("DATABASE_URL", testDatabaseURL)
	t.Setenv("LOG_LEVEL", "verbose")

	_, err := LoadMigrate()
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("error = %v, want ErrInvalidConfig", err)
	}
}

func setValidAPIEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", testDatabaseURL)
	t.Setenv("TELEGRAM_BOT_TOKEN", testBotToken)
	t.Setenv("MINI_APP_URL", testMiniAppURL)
	t.Setenv("JWT_ACCESS_SECRET", testJWTSecret)
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://app.example.test")
}

func setValidWorkerEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", testDatabaseURL)
	t.Setenv("TELEGRAM_BOT_TOKEN", testBotToken)
	t.Setenv("MINI_APP_URL", testMiniAppURL)
}

func writeConfigFile(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte(contents+"\n"), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}
	return path
}

func cleanEnvironment(t *testing.T) {
	t.Helper()
	type originalValue struct {
		value string
		set   bool
	}
	original := make(map[string]originalValue, len(configEnvironmentNames))
	for _, name := range configEnvironmentNames {
		value, set := os.LookupEnv(name)
		original[name] = originalValue{value: value, set: set}
		if err := os.Unsetenv(name); err != nil {
			t.Fatalf("unset %s: %v", name, err)
		}
	}
	t.Cleanup(func() {
		for _, name := range configEnvironmentNames {
			value := original[name]
			if value.set {
				_ = os.Setenv(name, value.value)
			} else {
				_ = os.Unsetenv(name)
			}
		}
	})
}

func assertErrorContains(t *testing.T, err error, field string) {
	t.Helper()
	if err == nil {
		t.Fatalf("error = nil, want error containing %q", field)
	}
	if !strings.Contains(err.Error(), field) {
		t.Fatalf("error = %q, want field %q", err, field)
	}
}
