# Telegram Mini App Task Tracker Backend

Backend для персонального task tracker, работающего как Telegram Mini App. Проект будет включать HTTP API, Telegram-бота, отдельный worker напоминаний и CLI для миграций PostgreSQL.

На текущем этапе создан только каркас проекта. Точки входа, загрузка конфигурации, подключение к базе данных и бизнес-логика будут добавлены на следующих этапах.

## Требования

- Go 1.26 или новее в рамках ветки 1.26
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

## Будущие бинарники

- `api` — HTTP API, авторизация Mini App и обработка Telegram updates.
- `worker` — фоновая отправка напоминаний.
- `migrate` — применение и откат миграций PostgreSQL.

## Локальная конфигурация

Создайте локальный файл из безопасного примера:

```sh
cp .env.example .env
```

После реализации точек входа приложения будут запускаться с явно указанным файлом:

```sh
CONFIG_FILE=.env make run-api
CONFIG_FILE=.env make run-worker
```

Файл `.env` не должен попадать в Git или Docker image. В production `CONFIG_FILE` использоваться не будет: переменные окружения задаются в настройках приложений Dokploy.
