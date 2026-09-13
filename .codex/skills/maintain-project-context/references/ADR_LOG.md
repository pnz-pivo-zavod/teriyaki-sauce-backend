# Architecture Decisions and Project Changes

Append-only record. Do not rewrite accepted decisions; supersede them with a new ADR.

## ADR-001 — Repository identity and Go layout

- Status: Accepted
- Date: 2026-07-11
- Related: [repository](https://github.com/pnz-pivo-zavod/teriyaki-sauce-backend), [project-layout](https://github.com/golang-standards/project-layout/blob/master/README_ru.md)

### Context

The backend needed a stable module identity and structure before feature work.

### Decision

Use module `github.com/pnz-pivo-zavod/teriyaki-sauce-backend` with `cmd/api`, `cmd/worker`, `cmd/migrate`, and internal packages following the Go project-layout conventions.

### Consequences

API, background processing, and migration concerns are separately deployable while implementation remains private under `internal`.

## ADR-002 — Process-specific typed configuration

- Status: Accepted
- Date: 2026-07-11
- Related: [cleanenv](https://github.com/ilyakaznacheev/cleanenv), [KAN-5](https://practiceilya.atlassian.net/browse/KAN-5)

### Context

Each binary needs a different subset of environment variables and must not receive unrelated secrets.

### Decision

Use separate `APIConfig`, `WorkerConfig`, and `MigrateConfig` loaded with cleanenv. Use explicit `CONFIG_FILE` locally; forbid config files in production and use Dokploy process environment.

### Consequences

Configuration errors are sanitized, production cannot be silently downgraded by `.env`, and each process validates only its contract.

## ADR-003 — Telegram Mini App authentication boundary

- Status: Accepted
- Date: 2026-07-11
- Related: [Telegram validation](https://core.telegram.org/bots/webapps#validating-data-received-via-the-mini-app)

### Context

Frontend-visible Telegram user data can be modified and cannot directly establish backend identity.

### Decision

Frontend sends the original signed `initData` to `/login`; backend validates its signature and age with the bot token before creating local access and refresh credentials.

### Consequences

`initDataUnsafe` is display-only. Protected application endpoints use local access tokens, and raw initData must never be logged.

## ADR-004 — Quality gates and local hooks

- Status: Accepted
- Date: 2026-07-11
- Related: [golangci-lint](https://github.com/golangci/golangci-lint), [Lefthook](https://github.com/evilmartians/lefthook)

### Context

The repository needs consistent local and pull-request checks without Node.js tooling.

### Decision

Use strict golangci-lint v2 in GitHub Actions and Lefthook v2 for sequential format, vet, and lint pre-commit jobs. Pin tools through Go tool directives.

### Consequences

The module requires Go 1.26.3, local and CI lint behavior is reproducible, and tool dependencies appear in the Go module graph.

## ADR-005 — Immutable application context

- Status: Accepted
- Date: 2026-07-12
- Related: [Zerolog context integration](https://pkg.go.dev/github.com/rs/zerolog#Logger.WithContext), [KAN-4](https://practiceilya.atlassian.net/browse/KAN-4)

### Context

Process logging and future HTTP request data need a shared context without allowing request data to leak across concurrent requests.

### Decision

Use immutable `internal/appctx.Context`, embedding `context.Context` and a private Zerolog logger. Derivation methods return copies. Request ID is supported now; authenticated-user helpers wait for domain User and JWT middleware.

### Consequences

The same type can flow through lifecycle and standard context-aware APIs. Request-scoped values must always be attached to derived contexts.

## ADR-006 — Logging formats and secret handling

- Status: Accepted
- Date: 2026-07-12
- Related: [zerolog](https://github.com/rs/zerolog), [KAN-4](https://practiceilya.atlassian.net/browse/KAN-4)

### Context

Local development needs readable output while production needs machine-readable logs, and configuration errors may contain credentials.

### Decision

Use console output without ANSI colors in development and JSON in test/production. Write entrypoint logs to stderr, use safe bootstrap logging, and never emit raw config/lifecycle causes or sensitive request values.

### Consequences

Logs are suitable for Dokploy ingestion and remain safe by construction at current boundaries; future integrations must preserve the forbidden-field policy.

## ADR-007 — Bounded process lifecycle

- Status: Accepted
- Date: 2026-07-12
- Related: [KAN-4](https://practiceilya.atlassian.net/browse/KAN-4)

### Context

API and worker must be long-running and eventually own resources requiring ordered cleanup.

### Decision

Handle SIGINT/SIGTERM centrally, use configurable `SHUTDOWN_TIMEOUT=10s`, run shutdown hooks in reverse registration order, and expose only safe sentinel failures. API/worker use lifecycle; migrate remains one-shot.

### Consequences

Later PostgreSQL, HTTP, Telegram, and worker components can register cleanup without redesigning entrypoints. A repeated signal may force termination.

## ADR-008 — Persistence and migration stack

- Status: Accepted
- Date: 2026-07-11
- Related: [pgx](https://github.com/jackc/pgx), [goose](https://github.com/pressly/goose), [KAN-1](https://practiceilya.atlassian.net/browse/KAN-1)

### Context

The service requires PostgreSQL access, pooling, and reproducible schema migrations.

### Decision

Use PostgreSQL 14+, pgx v5/pgxpool for runtime access, and goose v3 for SQL migrations through the migrate binary.

### Consequences

Stage 4 will implement pool lifecycle, schema migrations, indexes, and up/down verification.

## ADR-009 — Entrypoint logger initialization and explicit lifecycle phases

- Status: Accepted
- Date: 2026-07-12
- Related: [zerolog](https://github.com/rs/zerolog), [KAN-4](https://practiceilya.atlassian.net/browse/KAN-4)
- Supersedes: ADR-006, ADR-007

### Context

The dedicated bootstrap application context obscured that its only purpose was logging configuration-load failures. The lifecycle implementation also used generic hook terminology and mixed application startup with graceful shutdown in one function body.

### Decision

Each entrypoint creates a basic Zerolog logger with timestamp and service before loading configuration. It logs configuration failures directly and passes the logger to `appctx.New`, which applies the configured environment, level, and output format. Preserve the immutable application context and existing secret-handling rules.

Name registered cleanup operations `ShutdownTask` and execute them in reverse order within the shared shutdown deadline. Keep application startup, shutdown-trigger waiting, and graceful shutdown as separate lifecycle functions while retaining centralized SIGINT/SIGTERM handling and safe sentinel errors.

### Consequences

Startup logging no longer requires a temporary application context or a special bootstrap environment. Entrypoint ownership of process metadata is explicit, cleanup registrations describe their intent, and startup/shutdown control flow can be read and tested as distinct phases. API and worker remain long-running; migrate remains one-shot.

## ADR-010 — PostgreSQL pool ownership and initial schema

- Status: Accepted
- Date: 2026-08-01
- Related: [pgxpool](https://pkg.go.dev/github.com/jackc/pgx/v5/pgxpool), [goose](https://github.com/pressly/goose), [KAN-1](https://practiceilya.atlassian.net/browse/KAN-1)

### Context

ADR-008 fixed the persistence stack but left open who applies migrations, how the pool is bounded and released, and how the stage specification maps onto an actual schema. The specification also conflicted with itself: it required a unique `user_id` in `refresh_sessions` while describing a refresh flow that inserts the new session before deleting the old one, marked `notes.task_id` unique although a task may carry several notes, and gave `tasks` no owner even though the Jira checklist requires ownership fields.

### Decision

Only `cmd/api` applies migrations, through `postgres.InitDB`, which connects, pings, and runs `goose up`. `cmd/worker` only connects. `cmd/migrate` stays a one-shot CLI whose first argument is `up`, `down`, or `status`, defaulting to `up`, for manual rollback and for migrations that should be watched by hand. Migrations are embedded with `go:embed` so every binary carries them.

The CLI accepts only that allowlist and rejects anything else in `cmd/migrate` before opening a database connection; `postgres.Migrate` itself only keeps the `default` branch of its command switch, because goose also understands destructive commands such as `reset` and `create`, which would write a file next to the running process rather than into `migrations/`.

The database contract is split by capability. `DatabaseConfig` carries the URL and connect timeout; `DatabaseMigrationConfig` embeds it and adds `DATABASE_MIGRATE_TIMEOUT`. API and migrate use the wider type, the worker the narrower one, so passing worker configuration to the migration runner is a compile error and a broken migration timeout cannot stop a process that never migrates.

Owning migrations in one binary does not serialize that binary's own instances, so migrations run through `goose.Provider` with `goose.WithSessionLocker` and a PostgreSQL session advisory lock. A rolling deploy, a second api instance, and a manual migrate therefore wait for each other. The waiting side retries once a second for the whole `DATABASE_MIGRATE_TIMEOUT` budget instead of goose's five-second default period. The provider also replaces goose's package-level `SetBaseFS`, `SetDialect`, and `SetLogger`, which mutated process-global state shared by every caller.

Connect and ping share `DATABASE_CONNECT_TIMEOUT` (15s); a migration run, including the wait for the lock, is bounded by `DATABASE_MIGRATE_TIMEOUT` (3m). Both long-running binaries register a `postgres` shutdown task so the pool closes inside the shared shutdown deadline. Migration results are logged as structured events; goose itself writes nothing, because the provider is not verbose.

The initial schema is a single migration. `tasks` gains `user_id` ownership. `refresh_sessions.user_id` is a plain index and uniqueness moves to the token columns. `notes.task_id` is a plain index. Tokens are stored as SHA-256 hashes in `bytea`, never as raw tokens.

### Consequences

A single `go run ./cmd/api` brings up a working schema, and a Dokploy deployment needs no separate migration step, at the cost of api startup depending on migration success. Concurrent starts are safe: verified with three api instances and a manual migrate racing on an empty schema, where exactly one applied the migration and every process exited cleanly.

Two consequences remain open. An instance waiting for the lock is still inside startup, before lifecycle installs signal handling, so a SIGTERM during a long migration terminates it without graceful shutdown; it holds no resources at that point. And the advisory lock only covers processes that migrate through this code, not a schema change applied by hand at the same time.

Stage 7 must hash tokens before storing or comparing them.

## Project change log

### 2026-07-11

- Stage 1 created the Go module and project skeleton.
- The repository was renamed to `teriyaki-sauce-backend`.
- Stage 2 added typed cleanenv configuration and three entrypoints.
- Added strict golangci-lint GitHub Actions checks.
- Added Lefthook pre-commit checks and raised the minimum Go version to 1.26.3.

### 2026-07-12

- Stage 3 implemented immutable application context, environment-aware Zerolog output, request ID derivation, SIGINT/SIGTERM handling, bounded reverse-order shutdown hooks, and long-running API/worker entrypoints.
- Added `SHUTDOWN_TIMEOUT=10s` for API and worker; migrate intentionally ignores it.
- Added this project-local context skill and made ADR/context maintenance a repository-wide instruction.
- Refined stage 3 so entrypoints initialize the base logger directly, `appctx.New` applies configuration to it, and lifecycle uses explicit startup/shutdown phases with named shutdown tasks.

### 2026-08-01

- Stage 4 added `internal/repository/postgres` with pgxpool connect/ping, an embedded goose migration runner, and the initial six-table schema with list and reminder indexes.
- Added `DATABASE_CONNECT_TIMEOUT=15s` and `DATABASE_MIGRATE_TIMEOUT=3m` to the database configuration of all three processes.
- `cmd/api` now applies migrations on startup, `cmd/worker` only connects, and both close the pool through a named `postgres` shutdown task.
- `cmd/migrate` became a real goose CLI taking an `up`, `down`, or `status` argument.
- Review of stage 4 replaced the legacy package-level goose calls with `goose.Provider` and added a PostgreSQL session advisory lock, because owning migrations in the api binary does not serialize concurrent instances of that binary.
- Review of stage 4 also split `DatabaseConfig` and `DatabaseMigrationConfig` so the worker no longer validates a migration timeout it never uses, and moved the migrate command allowlist ahead of the database connection.
- Entry point tests now assert the database failure path; the migration up/down/up cycle runs in `internal/repository/postgres` and is skipped unless `TEST_DATABASE_URL` is set.
- Recorded that stage 3 is merged, correcting a stale roadmap status.

### 2026-09-13

- REF-01 removed the goose logger adapter: it was a no-op, since goose v3 `Provider` logs only with `WithVerbose`, which is not enabled. Clarified ADR-010 accordingly.
- REF-01 removed the duplicate command check at the start of `postgres.Migrate`; the allowlist gate before connecting stays in `cmd/migrate`, and `Migrate` keeps its `default` branch returning `ErrCommand`.
