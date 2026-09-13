# Teriyaki Sauce Backend — Project Context

Canonical working context for planning and implementation. Update this file whenever current requirements, architecture, stack, configuration, roadmap status, or deployment assumptions change. Architectural history belongs in `ADR_LOG.md`.

## Product

`teriyaki-sauce-backend` is the Go backend for a personal task tracker delivered as a Telegram Mini App. It will provide Mini App authentication, task/note/tag APIs, Telegram bot integration, reminders, and PostgreSQL persistence.

- Repository: [pnz-pivo-zavod/teriyaki-sauce-backend](https://github.com/pnz-pivo-zavod/teriyaki-sauce-backend)
- Go module: `github.com/pnz-pivo-zavod/teriyaki-sauce-backend`
- Jira project: [Teriyaki Sauce board](https://practiceilya.atlassian.net/jira/software/projects/KAN/boards/1)
- API requirements: [Confluence external specification](https://practiceilya.atlassian.net/wiki/external/ODE0MjNjOTE2ZmI2NDUzN2E0NTNkNTAwMjY2NDIxN2M)

## Fixed requirements

- Go `1.26.3` or newer patch release within Go 1.26.
- PostgreSQL 14+.
- Project organization follows [golang-standards/project-layout](https://github.com/golang-standards/project-layout/blob/master/README_ru.md).
- Production configuration comes from Dokploy process environment. `CONFIG_FILE` is forbidden in production.
- Local `.env` loading is explicit through `CONFIG_FILE=.env`; `.env` values take precedence over process environment.
- API and worker are separate long-running binaries; migrate is a one-shot CLI.
- Secrets must never appear in logs, errors, Git, Docker context, project context, or ADRs.

## Technology sources

| Area | Choice | Official source |
|---|---|---|
| Language | Go 1.26 | [Go documentation](https://go.dev/doc/) |
| Router | chi v5 | [go-chi/chi](https://github.com/go-chi/chi) |
| Telegram client | telego | [mymmrac/telego](https://github.com/mymmrac/telego) |
| Collection helpers | lo | [samber/lo](https://github.com/samber/lo) |
| Logging | zerolog | [rs/zerolog](https://github.com/rs/zerolog) |
| PostgreSQL | pgx v5 / pgxpool | [jackc/pgx](https://github.com/jackc/pgx) |
| Configuration | cleanenv | [ilyakaznacheev/cleanenv](https://github.com/ilyakaznacheev/cleanenv) |
| JWT | golang-jwt v5 | [golang-jwt/jwt](https://github.com/golang-jwt/jwt) |
| Migrations | goose v3 | [pressly/goose](https://github.com/pressly/goose) |
| Linting | golangci-lint v2 | [golangci-lint](https://github.com/golangci/golangci-lint) |
| Git hooks | Lefthook v2 | [evilmartians/lefthook](https://github.com/evilmartians/lefthook) |
| Deployment | Dokploy | [Dokploy documentation](https://docs.dokploy.com/) |

## Telegram sources and authentication

- [Telegram Bot API](https://core.telegram.org/bots/api)
- [Telegram Mini Apps](https://core.telegram.org/bots/webapps)
- [Validating Mini App init data](https://core.telegram.org/bots/webapps#validating-data-received-via-the-mini-app)

Authentication flow:

1. Telegram opens the Mini App and provides signed `Telegram.WebApp.initData` to the frontend.
2. The frontend sends the original initData string to backend `/login`.
3. Backend validates the Telegram signature and freshness before trusting user data.
4. Backend finds or creates the local user and returns application access/refresh credentials.
5. Protected endpoints authenticate with the application access token, not raw Telegram data.

Never trust `initDataUnsafe` as backend authentication evidence and never log raw initData.

## Current architecture

```text
cmd/api       long-running API process; owns migrations via InitDB; HTTP/router not connected yet
cmd/worker    long-running reminder process; connects to PostgreSQL only; loop not connected yet
cmd/migrate   one-shot goose CLI; first argument is the command, default up

internal/appctx               immutable context.Context + zerolog.Logger
internal/config               typed cleanenv configuration and validation
internal/lifecycle            SIGINT/SIGTERM and graceful shutdown orchestration
internal/repository/postgres  pgxpool lifecycle and goose migration runner
migrations                    embedded goose SQL migrations
tools                         separate module pinning golangci-lint and Lefthook
```

Packages are created by the stage that puts code into them; there are no placeholder directories. The HTTP transport package must not be named `http`, because that shadows `net/http` (for example `internal/httpapi`).

PostgreSQL access goes through `internal/repository/postgres`. `Connect` opens a pgxpool and verifies it with a ping inside `DATABASE_CONNECT_TIMEOUT`. `Migrate` runs one goose command (`up`, `down`, or `status`) against a `database/sql` handle borrowed from the pool inside `DATABASE_MIGRATE_TIMEOUT`; closing that handle leaves the pool open. `InitDB` is `Connect` plus `up` and is used only by the api process. Both long-running binaries register a `postgres` shutdown task that closes the pool.

Migration runs are serialized by a PostgreSQL session advisory lock through `goose.Provider` and `goose.WithSessionLocker`, so concurrent api instances during a rolling deploy and a manual migrate wait for each other instead of racing on `goose_db_version` and DDL. The waiting instance retries once a second for the whole `DATABASE_MIGRATE_TIMEOUT` budget. The provider replaces goose's package-level `SetBaseFS`/`SetDialect`/`SetLogger`, which were process-global state.

The schema is a single initial migration covering `users`, `refresh_sessions`, `tasks`, `notes`, `tags`, and `task_tags`. `tasks` carries `user_id` ownership plus `completed`/`deleted` soft-delete flags and `notify_at`; partial indexes serve user task lists and the reminder worker. `refresh_sessions` stores SHA-256 token hashes, never raw tokens, and does not make `user_id` unique because refresh inserts the new session before deleting the old one.

Application context is immutable. Each entrypoint creates a basic process logger before loading configuration, then passes it to `appctx.New` to apply the configured environment, level, and output format. Request-scoped data must be attached by deriving a new context; never mutate a process-wide context. Request ID support exists. Typed authenticated-user helpers will be added after the domain User and JWT middleware exist.

## Configuration contract

Common variables: `APP_ENV`, `LOG_LEVEL`, `DATABASE_URL`, `DATABASE_CONNECT_TIMEOUT`.

`DATABASE_MIGRATE_TIMEOUT` belongs to the processes that migrate, so it is part of the API and migrate contracts only. The worker uses the plain `DatabaseConfig`, which makes it a compile-time error to pass worker configuration to the migration runner and keeps a broken migration timeout from stopping a process that never migrates.

API additionally reads HTTP, Telegram update/webhook, JWT, CORS, cookie, and `SHUTDOWN_TIMEOUT` settings. Worker additionally reads Telegram sender, reminder, and `SHUTDOWN_TIMEOUT` settings. Migrate reads only common and PostgreSQL settings.

Important defaults:

- `APP_ENV=development`
- `LOG_LEVEL=info`
- `HTTP_ADDR=:8080`
- `TELEGRAM_UPDATE_MODE=polling`; production requires webhook
- `SHUTDOWN_TIMEOUT=10s`
- `DATABASE_CONNECT_TIMEOUT=15s`
- `DATABASE_MIGRATE_TIMEOUT=3m`; a migration that needs longer should be applied by hand
- `REMINDER_POLL_INTERVAL=10s`
- `REMINDER_LEASE_TIMEOUT=1m`
- `REMINDER_BATCH_SIZE=50`
- `REMINDER_MAX_ATTEMPTS=4`

Use `.env.example` as the complete safe variable inventory. Do not duplicate secret values here.

## Logging and lifecycle rules

- Development uses Zerolog console output without ANSI colors; test/production use JSON.
- Entrypoints write to stderr and add `service` and `environment` fields.
- Do not log configuration structs, Telegram/JWT/webhook secrets, cookies, Authorization, or initData.
- Configuration and lifecycle expose safe sentinel errors rather than raw causes.
- API and worker wait for SIGINT/SIGTERM and share a bounded graceful shutdown deadline.
- Named shutdown tasks run in reverse registration order. Lifecycle orchestration keeps application startup, shutdown-trigger waiting, and graceful shutdown in separate functions. A repeated signal may terminate immediately.

## Quality gates

- `.golangci.yml` is the shared strict lint configuration.
- golangci-lint and Lefthook are pinned as `tool` directives in the separate `tools/go.mod` module and run as `go tool -modfile=tools/go.mod <tool>`; the root `go.mod` carries only application dependencies.
- Pull requests run `go build ./cmd/...`, `go test -race ./...`, and `go tool golangci-lint run` through GitHub Actions; the linter version comes from `tools/go.mod` only.
- Lefthook pre-commit runs formatting and full linting (`govet` runs inside golangci-lint).
- Required acceptance for implementation stages: `go test ./...`, `go test -race ./...`, `go build ./cmd/...`, golangci-lint, and relevant smoke/integration tests.
- The test plan of a stage lives in its pull request description, not in a repository file.

## Roadmap

| Stage | Jira | Status |
|---|---|---|
| 01 — Project skeleton | [KAN-2](https://practiceilya.atlassian.net/browse/KAN-2) | Done |
| 02 — Configuration and entrypoints | [KAN-5](https://practiceilya.atlassian.net/browse/KAN-5) | Done |
| 03 — Logging and process lifecycle | [KAN-4](https://practiceilya.atlassian.net/browse/KAN-4) | Done |
| 04 — PostgreSQL and migrations | [KAN-1](https://practiceilya.atlassian.net/browse/KAN-1) | Done |
| 05 — Domain layer | [KAN-3](https://practiceilya.atlassian.net/browse/KAN-3) | Planned |
| 06 — Telegram initData validation | [KAN-6](https://practiceilya.atlassian.net/browse/KAN-6) | Planned |
| 07 — Users and tokens | [KAN-7](https://practiceilya.atlassian.net/browse/KAN-7) | Planned |
| 08 — HTTP authentication | [KAN-8](https://practiceilya.atlassian.net/browse/KAN-8) | Planned |
| 09 — Task repository and service | [KAN-9](https://practiceilya.atlassian.net/browse/KAN-9) | Planned |
| 10 — Task HTTP API | [KAN-10](https://practiceilya.atlassian.net/browse/KAN-10) | Planned |
| 11 — Tags | [KAN-11](https://practiceilya.atlassian.net/browse/KAN-11) | Planned |
| 12 — Notes | [KAN-12](https://practiceilya.atlassian.net/browse/KAN-12) | Planned |
| 13 — Router, middleware, health checks | [KAN-15](https://practiceilya.atlassian.net/browse/KAN-15) | Planned |
| 14 — Telegram bot | [KAN-18](https://practiceilya.atlassian.net/browse/KAN-18) | Planned |
| 15 — Reminders | [KAN-14](https://practiceilya.atlassian.net/browse/KAN-14) | Planned |
| 16 — OpenAPI and documentation | [KAN-13](https://practiceilya.atlassian.net/browse/KAN-13) | Planned |
| 17 — Build and Dokploy | [KAN-16](https://practiceilya.atlassian.net/browse/KAN-16) | Planned |
| 18 — Testing and final acceptance | [KAN-17](https://practiceilya.atlassian.net/browse/KAN-17) | Planned |

Before implementing a stage, read its full Jira description and reconcile it with this snapshot and the current repository.
