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

const _testDatabaseURL = "postgres://user:password@localhost:5432/teriyaki_sauce?sslmode=disable"

var _configEnvironmentNames = []string{
	"CONFIG_FILE",
	"APP_ENV",
	"LOG_LEVEL",
	"SHUTDOWN_TIMEOUT",
	"DATABASE_URL",
	"DATABASE_CONNECT_TIMEOUT",
	"DATABASE_MIGRATE_TIMEOUT",
}

func TestLoadDefaults(t *testing.T) {
	cleanEnvironment(t)
	t.Setenv("DATABASE_URL", _testDatabaseURL)

	api, err := LoadAPI()
	if err != nil {
		t.Fatalf("LoadAPI() error = %v, want only DATABASE_URL to be required", err)
	}
	worker, err := LoadWorker()
	if err != nil {
		t.Fatalf("LoadWorker() error = %v, want only DATABASE_URL to be required", err)
	}

	if api.Common.Environment != EnvironmentDevelopment || api.Common.LogLevel != zerolog.InfoLevel {
		t.Errorf("common defaults = %#v, want development/info", api.Common)
	}
	if db := api.Database; db.ConnectTimeout != 15*time.Second || db.MigrateTimeout != 3*time.Minute {
		t.Errorf("database timeouts = %s/%s, want 15s/3m", db.ConnectTimeout, db.MigrateTimeout)
	}
	if api.Lifecycle.ShutdownTimeout != 10*time.Second {
		t.Errorf("api ShutdownTimeout = %s, want 10s", api.Lifecycle.ShutdownTimeout)
	}
	if worker.Lifecycle.ShutdownTimeout != 10*time.Second {
		t.Errorf("worker ShutdownTimeout = %s, want 10s", worker.Lifecycle.ShutdownTimeout)
	}
}

func TestLoadNormalizesCommonConfig(t *testing.T) {
	cleanEnvironment(t)
	t.Setenv("DATABASE_URL", _testDatabaseURL)
	t.Setenv("APP_ENV", " TEST ")
	t.Setenv("LOG_LEVEL", "DEBUG")

	cfg, err := LoadMigrate()
	if err != nil {
		t.Fatalf("LoadMigrate() error = %v", err)
	}

	if cfg.Common.Environment != EnvironmentTest || cfg.Common.LogLevel != zerolog.DebugLevel {
		t.Errorf("common config = %#v, want test/debug", cfg.Common)
	}
}

func TestLoadDurations(t *testing.T) {
	tests := []struct {
		name string

		giveVariable string
		giveValue    string

		wantDuration time.Duration
		wantErr      bool
	}{
		{
			name:         "SHUTDOWN_TIMEOUT=25s -> 25s",
			giveVariable: "SHUTDOWN_TIMEOUT",
			giveValue:    "25s",
			wantDuration: 25 * time.Second,
		},
		{
			name:         "SHUTDOWN_TIMEOUT=0s -> error",
			giveVariable: "SHUTDOWN_TIMEOUT",
			giveValue:    "0s",
			wantErr:      true,
		},
		{
			name:         "SHUTDOWN_TIMEOUT negative -> error",
			giveVariable: "SHUTDOWN_TIMEOUT",
			giveValue:    "-1s",
			wantErr:      true,
		},
		{
			name:         "SHUTDOWN_TIMEOUT unparsable -> error without the value",
			giveVariable: "SHUTDOWN_TIMEOUT",
			giveValue:    "secret-invalid-duration",
			wantErr:      true,
		},
		{
			name:         "DATABASE_CONNECT_TIMEOUT=5s -> 5s",
			giveVariable: "DATABASE_CONNECT_TIMEOUT",
			giveValue:    "5s",
			wantDuration: 5 * time.Second,
		},
		{
			name:         "DATABASE_CONNECT_TIMEOUT=0s -> error",
			giveVariable: "DATABASE_CONNECT_TIMEOUT",
			giveValue:    "0s",
			wantErr:      true,
		},
		{
			name:         "DATABASE_CONNECT_TIMEOUT negative -> error",
			giveVariable: "DATABASE_CONNECT_TIMEOUT",
			giveValue:    "-1s",
			wantErr:      true,
		},
		{
			name:         "DATABASE_CONNECT_TIMEOUT unparsable -> error without the value",
			giveVariable: "DATABASE_CONNECT_TIMEOUT",
			giveValue:    "secret-invalid-duration",
			wantErr:      true,
		},
		{
			name:         "DATABASE_MIGRATE_TIMEOUT=30s -> 30s",
			giveVariable: "DATABASE_MIGRATE_TIMEOUT",
			giveValue:    "30s",
			wantDuration: 30 * time.Second,
		},
		{
			name:         "DATABASE_MIGRATE_TIMEOUT=0s -> error",
			giveVariable: "DATABASE_MIGRATE_TIMEOUT",
			giveValue:    "0s",
			wantErr:      true,
		},
		{
			name:         "DATABASE_MIGRATE_TIMEOUT negative -> error",
			giveVariable: "DATABASE_MIGRATE_TIMEOUT",
			giveValue:    "-1s",
			wantErr:      true,
		},
		{
			name:         "DATABASE_MIGRATE_TIMEOUT unparsable -> error without the value",
			giveVariable: "DATABASE_MIGRATE_TIMEOUT",
			giveValue:    "secret-invalid-duration",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanEnvironment(t)
			t.Setenv("DATABASE_URL", _testDatabaseURL)
			t.Setenv(tt.giveVariable, tt.giveValue)

			// The api is the only process that reads all three durations.
			cfg, err := LoadAPI()

			if tt.wantErr {
				if err == nil {
					t.Fatal("LoadAPI() error = nil, want error")
				}
				if strings.Contains(err.Error(), tt.giveValue) {
					t.Fatalf("error exposed value: %v", err)
				}

				return
			}
			if err != nil {
				t.Fatalf("LoadAPI() error = %v", err)
			}

			got := map[string]time.Duration{
				"SHUTDOWN_TIMEOUT":         cfg.Lifecycle.ShutdownTimeout,
				"DATABASE_CONNECT_TIMEOUT": cfg.Database.ConnectTimeout,
				"DATABASE_MIGRATE_TIMEOUT": cfg.Database.MigrateTimeout,
			}[tt.giveVariable]
			if got != tt.wantDuration {
				t.Errorf("%s = %s, want %s", tt.giveVariable, got, tt.wantDuration)
			}
		})
	}
}

func TestLoadReadsOnlyVariablesOfItsProcess(t *testing.T) {
	t.Run("worker never migrates -> broken DATABASE_MIGRATE_TIMEOUT is ignored", func(t *testing.T) {
		cleanEnvironment(t)
		t.Setenv("DATABASE_URL", _testDatabaseURL)
		t.Setenv("DATABASE_MIGRATE_TIMEOUT", "invalid-for-worker")

		if _, err := LoadWorker(); err != nil {
			t.Fatalf("LoadWorker() error = %v", err)
		}
	})

	t.Run("migrate is one-shot -> broken SHUTDOWN_TIMEOUT is ignored", func(t *testing.T) {
		cleanEnvironment(t)
		t.Setenv("DATABASE_URL", _testDatabaseURL)
		t.Setenv("SHUTDOWN_TIMEOUT", "invalid-for-migrate")

		if _, err := LoadMigrate(); err != nil {
			t.Fatalf("LoadMigrate() error = %v", err)
		}
	})
}

func TestLoadValidation(t *testing.T) {
	tests := []struct {
		name string

		giveVariable string
		giveValue    string

		wantErrContains string
	}{
		{
			name:            "unknown APP_ENV -> APP_ENV error",
			giveVariable:    "APP_ENV",
			giveValue:       "staging",
			wantErrContains: "APP_ENV",
		},
		{
			name:            "LOG_LEVEL=fatal would hide errors -> LOG_LEVEL error",
			giveVariable:    "LOG_LEVEL",
			giveValue:       "fatal",
			wantErrContains: "LOG_LEVEL",
		},
		{
			name:            "empty LOG_LEVEL -> LOG_LEVEL error",
			giveVariable:    "LOG_LEVEL",
			giveValue:       "",
			wantErrContains: "LOG_LEVEL",
		},
		{
			name:            "mysql DATABASE_URL -> DATABASE_URL error",
			giveVariable:    "DATABASE_URL",
			giveValue:       "mysql://localhost/db",
			wantErrContains: "DATABASE_URL",
		},
		{
			name:            "DATABASE_URL without database name -> DATABASE_URL error",
			giveVariable:    "DATABASE_URL",
			giveValue:       "postgres://localhost",
			wantErrContains: "DATABASE_URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanEnvironment(t)
			t.Setenv("DATABASE_URL", _testDatabaseURL)
			t.Setenv(tt.giveVariable, tt.giveValue)

			_, err := LoadMigrate()

			if err == nil || !strings.Contains(err.Error(), tt.wantErrContains) {
				t.Fatalf("LoadMigrate() error = %v, want it to contain %q", err, tt.wantErrContains)
			}
		})
	}
}

func TestLoadSanitizesCleanenvErrors(t *testing.T) {
	tests := []struct {
		name string

		giveVariable string
		giveValue    string
	}{
		{
			name:         "missing DATABASE_URL -> ErrInvalidConfig",
			giveVariable: "DATABASE_URL",
			giveValue:    "",
		},
		{
			name:         "unparsable LOG_LEVEL -> ErrInvalidConfig",
			giveVariable: "LOG_LEVEL",
			giveValue:    "verbose-secret",
		},
		{
			name:         "unparsable duration -> ErrInvalidConfig",
			giveVariable: "DATABASE_CONNECT_TIMEOUT",
			giveValue:    "not-a-duration-secret",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanEnvironment(t)
			if tt.giveVariable != "DATABASE_URL" {
				t.Setenv("DATABASE_URL", _testDatabaseURL)
				t.Setenv(tt.giveVariable, tt.giveValue)
			}

			_, err := LoadMigrate()

			if !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("LoadMigrate() error = %v, want ErrInvalidConfig", err)
			}
			if err.Error() != ErrInvalidConfig.Error() {
				t.Fatalf("LoadMigrate() error = %q, want the sanitized error", err)
			}
		})
	}
}

func TestLoadErrorsDoNotExposeSecrets(t *testing.T) {
	tests := []struct {
		name string

		giveSecret string
		setupEnv   func(t *testing.T, secret string)
	}{
		{
			name:       "DATABASE_URL with password but no host -> error without the password",
			giveSecret: "database-password-do-not-leak",
			setupEnv: func(t *testing.T, secret string) {
				t.Setenv("DATABASE_URL", "postgres://user:"+secret+"@")
			},
		},
		{
			name:       "broken config file -> error without the file contents",
			giveSecret: "file-secret-do-not-leak",
			setupEnv: func(t *testing.T, secret string) {
				t.Setenv("CONFIG_FILE", writeConfigFile(t, "BROKEN='"+secret))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanEnvironment(t)
			tt.setupEnv(t, tt.giveSecret)

			_, err := LoadMigrate()

			if err == nil {
				t.Fatal("LoadMigrate() error = nil, want error")
			}
			if strings.Contains(err.Error(), tt.giveSecret) {
				t.Fatalf("error exposed secret: %v", err)
			}
		})
	}
}

func TestConfigFileOverridesProcessEnvironment(t *testing.T) {
	cleanEnvironment(t)
	t.Setenv("APP_ENV", EnvironmentDevelopment)
	t.Setenv("DATABASE_URL", "postgres://shell:shell@localhost:5432/shell")
	t.Setenv("CONFIG_FILE", writeConfigFile(t, strings.Join([]string{
		"APP_ENV=test",
		"LOG_LEVEL=error",
		"DATABASE_URL=" + _testDatabaseURL,
	}, "\n")))

	cfg, err := LoadMigrate()
	if err != nil {
		t.Fatalf("LoadMigrate() error = %v", err)
	}

	if cfg.Common.Environment != EnvironmentTest || cfg.Common.LogLevel != zerolog.ErrorLevel {
		t.Errorf("file common config = %#v, want test/error", cfg.Common)
	}
	if cfg.Database.URL != _testDatabaseURL {
		t.Errorf("Database URL did not come from config file")
	}
}

func TestConfigFileIsForbiddenInProduction(t *testing.T) {
	t.Run("production process environment -> CONFIG_FILE error before reading", func(t *testing.T) {
		cleanEnvironment(t)
		t.Setenv("APP_ENV", EnvironmentProduction)
		t.Setenv("CONFIG_FILE", writeConfigFile(t, "APP_ENV=development\n"))

		_, err := LoadMigrate()
		assertErrorContains(t, err, "CONFIG_FILE")
	})

	t.Run("APP_ENV=production inside the file -> CONFIG_FILE error", func(t *testing.T) {
		cleanEnvironment(t)
		t.Setenv("CONFIG_FILE", writeConfigFile(t, strings.Join([]string{
			"APP_ENV=production",
			"DATABASE_URL=" + _testDatabaseURL,
		}, "\n")))

		_, err := LoadMigrate()
		assertErrorContains(t, err, "CONFIG_FILE")
	})
}

func writeConfigFile(t *testing.T, contents string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte(contents+"\n"), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	return path
}

// cleanEnvironment unsets every configuration variable for the test;
// t.Setenv restores the original values afterwards.
func cleanEnvironment(t *testing.T) {
	t.Helper()

	for _, name := range _configEnvironmentNames {
		t.Setenv(name, "")
		if err := os.Unsetenv(name); err != nil {
			t.Fatalf("unset %s: %v", name, err)
		}
	}
}

func assertErrorContains(t *testing.T, err error, field string) {
	t.Helper()

	if err == nil || !strings.Contains(err.Error(), field) {
		t.Fatalf("error = %v, want it to contain %q", err, field)
	}
}
