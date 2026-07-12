# Telegram Mini App Task Tracker Backend

Backend для персонального task tracker, работающего как Telegram Mini App. Проект будет включать HTTP API, Telegram-бота, отдельный worker напоминаний и CLI для миграций PostgreSQL.

На текущем этапе реализованы каркас проекта, типизированная конфигурация, application context, структурированное логирование и lifecycle процессов. Подключение к базе данных и бизнес-логика будут добавлены на следующих этапах.

## Контекст проекта для Codex

Для каждого нового чата Codex использует корневой [`AGENTS.md`](AGENTS.md) и локальный skill [`maintain-project-context`](.codex/skills/maintain-project-context/SKILL.md). Актуальные требования, источники и roadmap находятся в [`PROJECT_CONTEXT.md`](.codex/skills/maintain-project-context/references/PROJECT_CONTEXT.md), а принятые архитектурные решения и журнал изменений — в [`ADR_LOG.md`](.codex/skills/maintain-project-context/references/ADR_LOG.md).

Эти документы обновляются в том же изменении, в котором принимается ADR или меняются требования, архитектура, конфигурация либо статус этапа.

## Требования

- Go 1.26.3 или новее в рамках ветки 1.26
- PostgreSQL 14+

## Go module

```text
github.com/pnz-pivo-zavod/teriyaki-sauce-backend
```

## Структура

```text
.
├── api/                         # OpenAPI 3.1
├── cmd/
│   ├── api/                     # HTTP API и Telegram updates
│   ├── migrate/                 # CLI миграций
│   └── worker/                  # Worker напоминаний
├── internal/
│   ├── auth/                    # JWT и Telegram-аутентификация
│   ├── appctx/                  # Application context и Zerolog
│   ├── config/                  # Загрузка конфигурации через cleanenv
│   ├── domain/                  # Доменные модели и ошибки
│   ├── reminder/                # Планирование и отправка напоминаний
│   ├── repository/postgres/     # PostgreSQL repositories
│   ├── service/                 # Бизнес-логика
│   ├── telegram/                # Интеграция с Telegram Bot API
│   ├── lifecycle/               # Signals и graceful shutdown
│   └── transport/http/          # HTTP handlers и middleware
└── migrations/                  # SQL-миграции goose
```

## Бинарники

- `api` — будущий HTTP API, авторизация Mini App и обработка Telegram updates.
- `worker` — будущая фоновая отправка напоминаний.
- `migrate` — будущие применение и откат миграций PostgreSQL.

`api` и `worker` загружают конфигурацию и остаются запущенными до SIGINT или SIGTERM. После сигнала они выполняют graceful shutdown в пределах `SHUTDOWN_TIMEOUT`. `migrate` остаётся однократной командой и завершается сразу после проверки конфигурации.

## Локальная конфигурация

Создайте локальный файл из безопасного примера:

```sh
cp .env.example .env
```

Запускайте приложения с явно указанным файлом:

```sh
CONFIG_FILE=.env go run ./cmd/api
CONFIG_FILE=.env go run ./cmd/worker
CONFIG_FILE=.env go run ./cmd/migrate
```

Значения `.env` имеют приоритет над одноимёнными переменными shell. Файл `.env` не должен попадать в Git или Docker image.

В production `CONFIG_FILE` запрещён: переменные окружения задаются напрямую в настройках приложений Dokploy. API получает HTTP, Telegram, JWT, CORS и cookie settings; worker — PostgreSQL, Telegram sender и reminder settings; migrate — только общие настройки и `DATABASE_URL`.

## Логирование и завершение процессов

В `development` Zerolog использует читаемый console output без ANSI-цветов. В `test` и `production` логи записываются в JSON. Все entrypoints пишут в stderr и добавляют поля `service` и `environment`.

Токены Telegram, JWT и webhook secrets, cookie, заголовки авторизации и Telegram `initData` запрещено передавать в logger. Конфигурационные структуры и исходные ошибки загрузки конфигурации также не логируются целиком.

Timeout graceful shutdown настраивается только для API и worker:

```text
SHUTDOWN_TIMEOUT=10s
```

Повторный сигнал во время shutdown принудительно завершает процесс. Migrate signal lifecycle не использует.

## Проверки перед коммитом

Lefthook запускает проверки для staged Go-файлов при каждом `git commit`:

- форматирование через formatters из `.golangci.yml` с автоматическим добавлением исправлений в index;
- `go vet ./...`;
- полный `golangci-lint run` с настройками проекта.

Lefthook и golangci-lint зафиксированы как Go tools в `go.mod`. После клонирования репозитория установите hook:

```sh
go tool lefthook install
```

Ручной запуск:

```sh
go tool lefthook run pre-commit
```

В исключительном случае hook можно временно отключить для одного коммита:

```sh
LEFTHOOK=0 git commit -m "message"
```
