# Telegram Mini App Task Tracker Backend

Backend для персонального task tracker, работающего как Telegram Mini App.

Требования, архитектура, контракт конфигурации и roadmap — в [`PROJECT_CONTEXT.md`](.codex/skills/maintain-project-context/references/PROJECT_CONTEXT.md), принятые решения и журнал изменений — в [`ADR_LOG.md`](.codex/skills/maintain-project-context/references/ADR_LOG.md). Для агентов точка входа — [`AGENTS.md`](AGENTS.md) и skill [`maintain-project-context`](.codex/skills/maintain-project-context/SKILL.md).

## Требования

- Go 1.26.3 или новее в рамках ветки 1.26
- PostgreSQL 14+

## Структура

```text
.
├── cmd/
│   ├── api/                     # HTTP API и Telegram updates
│   ├── migrate/                 # CLI миграций
│   └── worker/                  # Worker напоминаний
├── internal/
│   ├── config/                  # Загрузка конфигурации через cleanenv
│   ├── lifecycle/               # Signals и graceful shutdown
│   ├── logging/                 # Настройка Zerolog по окружению
│   └── repository/postgres/     # PostgreSQL и миграции
├── migrations/                  # SQL-миграции goose
└── tools/                       # go.mod с golangci-lint и Lefthook
```

## Быстрый старт

Локальная база:

```sh
docker run --rm -d --name teriyaki-pg -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=teriyaki_sauce -p 5432:5432 postgres:16
```

Конфигурация из безопасного примера:

```sh
cp .env.example .env
```

Запуск (`api` сам применяет миграции при старте):

```sh
CONFIG_FILE=.env go run ./cmd/api
CONFIG_FILE=.env go run ./cmd/worker
```

Ручные миграции: `up` (по умолчанию), `status` или `down`:

```sh
CONFIG_FILE=.env go run ./cmd/migrate status
```

## Тесты

```sh
go test -race ./...
```

Интеграционные тесты миграций пропускаются без `TEST_DATABASE_URL`:

```sh
TEST_DATABASE_URL='postgres://postgres:postgres@localhost:5432/teriyaki_sauce?sslmode=disable' go test ./internal/repository/...
```

## Проверки перед коммитом

Lefthook запускает форматирование и golangci-lint. После клонирования установите hook:

```sh
go tool -modfile=tools/go.mod lefthook install
```

Отключить для одного коммита:

```sh
LEFTHOOK=0 git commit -m "message"
```
