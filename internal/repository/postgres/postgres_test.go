package postgres

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3/lock"
	"github.com/rs/zerolog"

	"github.com/pnz-pivo-zavod/teriyaki-sauce-backend/internal/appctx"
	"github.com/pnz-pivo-zavod/teriyaki-sauce-backend/internal/config"
)

// Every table the initial schema must create.
var schemaTables = []string{"users", "refresh_sessions", "tasks", "notes", "tags", "task_tags"}

func TestNewProviderReadsEmbeddedMigrations(t *testing.T) {
	// sql.Open is lazy, so the provider collects the embedded migrations
	// without ever reaching a database.
	db, err := sql.Open("pgx", "postgres://user@127.0.0.1:1/test")
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := db.Close(); closeErr != nil {
			t.Errorf("db.Close() error = %v", closeErr)
		}
	})

	provider, err := newProvider(db, config.DatabaseMigrationConfig{MigrateTimeout: time.Minute})
	if err != nil {
		t.Fatalf("newProvider() error = %v, the embedded FS is empty or misnamed", err)
	}
	if len(provider.ListSources()) == 0 {
		t.Fatal("newProvider() found no migrations in the embedded FS")
	}
}

func TestConnectUnreachable(t *testing.T) {
	ctx, output := testContext(t)
	cfg := config.DatabaseConfig{
		// Port 1 refuses immediately; the timeout only bounds a blackholed host.
		URL:            "postgres://user:connect-secret-do-not-leak@127.0.0.1:1/test",
		ConnectTimeout: time.Second,
	}

	start := time.Now()
	pool, err := Connect(ctx, cfg)
	if !errors.Is(err, ErrConnect) {
		t.Fatalf("Connect() error = %v, want ErrConnect", err)
	}
	if pool != nil {
		t.Fatal("Connect() returned a pool alongside an error")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("Connect() took %s, want the configured timeout to bound it", elapsed)
	}
	if strings.Contains(err.Error(), "connect-secret-do-not-leak") ||
		strings.Contains(output.String(), "connect-secret-do-not-leak") {
		t.Errorf("Connect() leaked the DSN password: err=%q output=%q", err, output.String())
	}
}

func TestConnectInvalidURL(t *testing.T) {
	ctx, _ := testContext(t)
	cfg := config.DatabaseConfig{
		URL:            "postgres://user@127.0.0.1:1/test?sslmode=nonsense",
		ConnectTimeout: time.Second,
	}

	if _, err := Connect(ctx, cfg); !errors.Is(err, ErrConnect) {
		t.Fatalf("Connect() error = %v, want ErrConnect", err)
	}
}

func TestSupportedCommand(t *testing.T) {
	tests := []struct {
		command string
		want    bool
	}{
		{command: CommandUp, want: true},
		{command: CommandDown, want: true},
		{command: CommandStatus, want: true},
		{command: "reset", want: false},
		{command: "create", want: false},
		{command: "fix", want: false},
		{command: "down-to", want: false},
		{command: "UP", want: false},
		{command: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			if got := SupportedCommand(tt.command); got != tt.want {
				t.Errorf("SupportedCommand(%q) = %t, want %t", tt.command, got, tt.want)
			}
		})
	}
}

// TestMigrationCycle covers the up/down/up requirement of KAN-1.
func TestMigrationCycle(t *testing.T) {
	ctx, cfg := integrationContext(t)
	pool := connectForTest(t, ctx, cfg)
	resetSchema(t, ctx, pool)

	if err := Migrate(ctx, pool, cfg, CommandUp); err != nil {
		t.Fatalf("Migrate(up) error = %v", err)
	}
	for _, table := range schemaTables {
		if !tableExists(t, ctx, pool, table) {
			t.Errorf("table %q is missing after up", table)
		}
	}

	if err := Migrate(ctx, pool, cfg, CommandStatus); err != nil {
		t.Fatalf("Migrate(status) error = %v", err)
	}

	if err := Migrate(ctx, pool, cfg, CommandDown); err != nil {
		t.Fatalf("Migrate(down) error = %v", err)
	}
	for _, table := range schemaTables {
		if tableExists(t, ctx, pool, table) {
			t.Errorf("table %q still exists after down", table)
		}
	}

	if err := Migrate(ctx, pool, cfg, CommandUp); err != nil {
		t.Fatalf("Migrate(up) after down error = %v", err)
	}
	for _, table := range schemaTables {
		if !tableExists(t, ctx, pool, table) {
			t.Errorf("table %q is missing after the second up", table)
		}
	}
}

// TestMigrationWaitsForSessionLock is the regression test for concurrent
// migration runs during a rolling deploy. Racing several goroutines does not
// reproduce the collision reliably, so this holds goose's own advisory lock the
// way another process would and asserts that a migration waits for it instead
// of touching the schema. Verified to fail when goose.WithSessionLocker is
// removed: the first migration below then succeeds immediately.
func TestMigrationWaitsForSessionLock(t *testing.T) {
	ctx, cfg := integrationContext(t)
	pool := connectForTest(t, ctx, cfg)
	resetSchema(t, ctx, pool)

	holder, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	if _, err := holder.Exec(ctx, "SELECT pg_advisory_lock($1)", lock.DefaultLockID); err != nil {
		t.Fatalf("pg_advisory_lock() error = %v", err)
	}

	blocked := cfg
	blocked.MigrateTimeout = 2 * time.Second
	if err := Migrate(ctx, pool, blocked, CommandUp); !errors.Is(err, ErrMigrate) {
		holder.Release()
		t.Fatalf("Migrate(up) error = %v, want ErrMigrate while the advisory lock is held", err)
	}
	for _, table := range schemaTables {
		if tableExists(t, ctx, pool, table) {
			t.Errorf("table %q was created while the advisory lock was held", table)
		}
	}

	if _, err := holder.Exec(ctx, "SELECT pg_advisory_unlock($1)", lock.DefaultLockID); err != nil {
		t.Fatalf("pg_advisory_unlock() error = %v", err)
	}
	holder.Release()

	if err := Migrate(ctx, pool, cfg, CommandUp); err != nil {
		t.Fatalf("Migrate(up) after the lock was released error = %v", err)
	}
	for _, table := range schemaTables {
		if !tableExists(t, ctx, pool, table) {
			t.Errorf("table %q is missing after the lock was released", table)
		}
	}
}

// integrationContext skips the test unless TEST_DATABASE_URL points at a
// throwaway database, since these tests create and drop the whole schema.
func integrationContext(t *testing.T) (appctx.Context, config.DatabaseMigrationConfig) {
	t.Helper()

	databaseURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	ctx, _ := testContext(t)
	return ctx, config.DatabaseMigrationConfig{
		DatabaseConfig: config.DatabaseConfig{
			URL:            databaseURL,
			ConnectTimeout: 15 * time.Second,
		},
		MigrateTimeout: 3 * time.Minute,
	}
}

func connectForTest(t *testing.T, ctx appctx.Context, cfg config.DatabaseMigrationConfig) *pgxpool.Pool {
	t.Helper()

	pool, err := Connect(ctx, cfg.DatabaseConfig)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	t.Cleanup(pool.Close)

	return pool
}

func resetSchema(t *testing.T, ctx appctx.Context, pool *pgxpool.Pool) {
	t.Helper()

	if _, err := pool.Exec(ctx, "DROP SCHEMA public CASCADE; CREATE SCHEMA public"); err != nil {
		t.Fatalf("resetting the schema failed: %v", err)
	}
}

func tableExists(t *testing.T, ctx appctx.Context, pool *pgxpool.Pool, table string) bool {
	t.Helper()

	var exists bool
	query := "SELECT EXISTS (SELECT 1 FROM pg_tables WHERE schemaname = current_schema() AND tablename = $1)"
	if err := pool.QueryRow(ctx, query, table).Scan(&exists); err != nil {
		t.Fatalf("QueryRow(%q) error = %v", table, err)
	}

	return exists
}

func testContext(t *testing.T) (appctx.Context, *bytes.Buffer) {
	t.Helper()

	output := &bytes.Buffer{}
	logger := zerolog.New(output).With().Timestamp().Logger()
	ctx, err := appctx.New(context.Background(), logger, appctx.Options{
		Environment: config.EnvironmentTest,
		Level:       "debug",
		Writer:      output,
	})
	if err != nil {
		t.Fatalf("appctx.New() error = %v", err)
	}

	return ctx, output
}
