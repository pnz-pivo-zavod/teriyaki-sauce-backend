# Telegram Mini App Task Tracker Backend

Backend для персонального task tracker, работающего как Telegram Mini App. Проект будет включать HTTP API, Telegram-бота, отдельный worker напоминаний и CLI для миграций PostgreSQL.

На текущем этапе реализованы каркас проекта, типизированная загрузка конфигурации через `cleanenv` и минимальные точки входа. Подключение к базе данных и бизнес-логика будут добавлены на следующих этапах.

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
│   ├── config/                  # Загрузка конфигурации через cleanenv
│   ├── domain/                  # Доменные модели и ошибки
│   ├── reminder/                # Планирование и отправка напоминаний
│   ├── repository/postgres/     # PostgreSQL repositories
│   ├── service/                 # Бизнес-логика
│   ├── telegram/                # Интеграция с Telegram Bot API
│   └── transport/http/          # HTTP handlers и middleware
└── migrations/                  # SQL-миграции goose
```

## Бинарники

- `api` — будущий HTTP API, авторизация Mini App и обработка Telegram updates.
- `worker` — будущая фоновая отправка напоминаний.
- `migrate` — будущие применение и откат миграций PostgreSQL.

Пока каждый бинарник только загружает и проверяет предназначенную ему конфигурацию, записывает структурированное Zerolog-событие и завершается.

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
