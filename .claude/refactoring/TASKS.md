# Задачи рефакторинга

Источник: [AUDIT.md](AUDIT.md) (ревью 2026-09-13, `main` @ `1bd7fda`).
Все задачи — `Proposed`. Размер: S ≤ 0.5 дня, M ≤ 1.5 дня.

Общие критерии приёмки для каждой задачи (из quality gates проекта):
`go build ./cmd/...`, `go test ./...`, `go test -race ./...`, `go tool golangci-lint run`,
`TEST_DATABASE_URL=... go test ./internal/repository/...` если задача трогает postgres,
плюс обновлённые `PROJECT_CONTEXT.md` / `ADR_LOG.md` в том же PR.

## Порядок и зависимости

```text
REF-01 ─┐
REF-05 ─┼─ независимые, можно параллельно
REF-06 ─┤
REF-07 ─┘
REF-02 ──► REF-03 ──► REF-08
REF-04 ─────────────► REF-08
REF-09 — вместе с REF-03 и REF-04, не отдельным проходом
```

| ID | Задача | Размер | Приоритет | Δ строк | Нужно решение | ADR |
|---|---|:-:|:-:|--:|:-:|---|
| REF-01 | Удалить мёртвый код в postgres | S | P1 | −45 | нет | change log, уточнение к ADR-010 |
| REF-02 | Заменить `appctx` на `context.Context` + zerolog | M | P1 | −210 | да | supersedes ADR-005 |
| REF-03 | Упростить `lifecycle` | M | P1 | −160 | нет | change log к ADR-009 |
| REF-04 | Урезать конфигурацию до используемых полей | M | P1 | −520 | да | supersedes ADR-002 (частично) |
| REF-05 | Почистить tooling и добавить тесты в CI | S | P2 | −30 | нет | change log |
| REF-06 | Вынести tool-зависимости из `go.mod` | S | P2 | −230 deps | да | supersedes ADR-004 (частично) |
| REF-07 | Убрать scaffolding, актуализировать документацию | S | P2 | −100 | нет | change log |
| REF-08 | Привести оставшийся код к `go-code-style` | S | P3 | ~0 | нет | нет |
| REF-09 | Привести затронутые тесты к `go-tests` | S | P3 | −20 | да (testify) | нет |

---

## REF-01 — Удалить мёртвый код в postgres

Аудит: 1.6, 1.7.

- Удалить `internal/repository/postgres/goose_logger.go` и опцию `goose.WithLogger(...)`.
- Убрать проверку `SupportedCommand` в начале `postgres.Migrate`; оставить гейт в `cmd/migrate/main.go`
  и `default: return ErrCommand` в `runCommand`.
- Удалить тест `TestMigrateRejectsUnknownCommandBeforeUsingThePool`.

Приёмка:
- `migrate reset` при недоступной БД — код 1, `unknown_migration_command`, без попытки подключения
  (поведение из ADR-010 сохранено).
- `migrate up/status/down` на живой БД не пишет ничего в stdout.
- В change log отмечено, что адаптер логгера goose был no-op.

## REF-02 — Заменить `appctx` на `context.Context` + zerolog

Аудит: 1.2. **Решение:** принять отказ от `appctx.Context` (новый ADR, supersedes ADR-005).

- `LogLevel` в `CommonConfig` грузить как `zerolog.Level` (cleanenv + `encoding.TextUnmarshaler`);
  убрать ручной `switch` уровней в `validateCommon`. Решить, допустимы ли `fatal`/`panic`/`disabled`.
- Одна функция настройки логгера по окружению (console без цвета в development, JSON иначе,
  поле `environment`) — вместо `appctx.New`.
- Entrypoint: `ctx := logger.WithContext(base)`; во всех пакетах `context.Context` и `zerolog.Ctx(ctx)`.
- Удалить `internal/appctx` целиком вместе с тестами; снять 6 `//nolint:contextcheck`.
- Request ID — отложить до HTTP-этапа, там `zerolog/hlog`.

Приёмка:
- В `cmd/`, `internal/` нет импорта `appctx` и ни одного `//nolint`.
- Формат логов не изменился: поля `service`, `environment`, JSON в test/production, console в development —
  покрыто тестом функции настройки логгера.
- Тесты утечки секретов в entrypoint-ах проходят без изменений ожиданий.

## REF-03 — Упростить `lifecycle`

Аудит: 1.3. Зависит от REF-02 (сигнатуры на `context.Context`).

- Один публичный `Run(ctx, timeout, run, tasks...)`; удалить внутренний `run` с параметром `stop`.
- `nil` run → одна строка с функцией ожидания `ctx.Done()`; удалить `startApplication`,
  `shouldWaitForApplication`, `applicationRunFailed`, `triggerResult`, типы `RunFunc` / `ShutdownFunc`.
- Удалить ветку `timeout <= 0`, фолбэк `"unnamed"`, пропуск `task.Run == nil`.
- Одна горутина на весь проход задач вместо горутины на задачу.
- Опционально: создавать `signal.NotifyContext` в entrypoint до `InitDB`, чтобы SIGTERM во время
  ожидания advisory lock обрабатывался штатно (закрывает открытое последствие ADR-010).

Приёмка:
- Сохранены все инварианты ADR-009: обратный порядок задач, общий дедлайн, sentinel-ошибки
  `ErrRuntime` / `ErrShutdown` / `ErrShutdownTimeout`, `context.Canceled` после сигнала — не ошибка,
  повторный сигнал завершает процесс.
- Существующие сценарии `lifecycle_test.go` проходят через публичный `Run`.
- `lifecycle.go` ≤ ~90 строк.

## REF-04 — Урезать конфигурацию до используемых полей

Аудит: 1.1. **Решение:** согласиться, что поля добавляются в этапах их потребителей
(новый ADR, частично supersedes ADR-002; разделение конфигов по процессам остаётся).

- Удалить из `APIConfig`: `HTTP`, `Telegram`, `JWT`, `CORS`, `Cookie`; из `WorkerConfig`: `Telegram`,
  `Reminder`, вместе с валидацией, регулярками, константами `TelegramUpdateMode*` и тестами.
- Слить `consts.go` в `config.go`.
- Урезать `.env.example` и env в `cmd/*/main_test.go`.
- В описания будущих этапов (KAN-6/7/8/13/14/18) перенести требования к удалённым полям, чтобы
  контракт не потерялся: дефолты, production-ограничения (webhook, HTTPS CORS, secure cookie),
  минимальная длина JWT-секрета.
- Опционально: вместо `DatabaseMigrationConfig` со встраиванием передавать `MigrateTimeout` в
  `postgres.Migrate` явным `time.Duration`.

Приёмка:
- `api` и `worker` стартуют только с `DATABASE_URL` (+ дефолты).
- Проверки `APP_ENV`, `LOG_LEVEL`, `DATABASE_*`, `SHUTDOWN_TIMEOUT`, запрет `CONFIG_FILE` в production
  и санитизация ошибок покрыты тестами как раньше.
- Раздел «Configuration contract» в `PROJECT_CONTEXT.md` отражает новый объём.

## REF-05 — Почистить tooling и добавить тесты в CI

Аудит: 1.8, 6.

- `lefthook.yml`: удалить джобу `vet`.
- `.golangci.yml`: оставить из линтеров сложности `gocognit` + `funlen`; убрать форматтер `goimports`
  (остаются `gci` + `gofumpt`), `testifylint`, `testableexamples`, `run.go`, `run.tests`, блок `output`;
  `lll.line-length: 180`.
- Workflow: запуск линтера через `go tool golangci-lint run` без второго пина версии;
  добавить шаги `go build ./cmd/...` и `go test -race ./...`.

Приёмка:
- PR с заведомо падающим тестом краснеет в CI.
- Линт по-прежнему 0 issues (после `lll: 180` — починить две строки > 180 или сделать это в REF-08
  раньше этой задачи).

## REF-06 — Вынести tool-зависимости из `go.mod`

Аудит: 1.4. **Решение:** принять отдельный модуль инструментов (частично supersedes ADR-004).

- Создать `tools/go.mod` с `tool`-директивами golangci-lint и lefthook; убрать `tool` из корневого `go.mod`,
  `go mod tidy`.
- Команды: `go tool -modfile=tools/go.mod golangci-lint ...` в `lefthook.yml`, workflow и README.

Приёмка:
- В корневом `go.mod` нет golangci-lint / lefthook и их транзитивных зависимостей.
- `go tool lefthook install` / pre-commit / CI работают по новой команде.

## REF-07 — Убрать scaffolding, актуализировать документацию

Аудит: 1.5, 1.10, 6.

- Удалить 7 `.gitkeep` и пустые директории; `.dockerignore` (вернуть с Dockerfile на этапе 17).
- Удалить `TESTING_PLAN.md`; убрать правило о нём из `PROJECT_CONTEXT.md` (план тестов — в описании PR).
- README: оставить quickstart и ссылки на `PROJECT_CONTEXT.md`, убрать пересказ архитектуры;
  дерево структуры — только существующие директории.
- `PROJECT_CONTEXT.md`: этап 04 → `Done`; в архитектуре убрать `future`-пакеты; зафиксировать,
  что HTTP-пакет не будет называться `http`.
- `ADR_LOG.md`: пометить ADR-006 и ADR-007 как `Superseded by ADR-009`.

Приёмка: в репо нет пустых директорий-заглушек; README и PROJECT_CONTEXT не противоречат коду.

## REF-08 — Привести оставшийся код к `go-code-style`

Аудит: раздел 2. Делать после REF-02…REF-04, чтобы не править удаляемый код.

- `_`-префикс у неэкспортируемых пакетных `var`/`const` (`_serviceName`, `_lockRetryPeriodSeconds`, …).
- `validationError` → экспортируемый `ValidationError` (если переживёт REF-04).
- Пустая строка после встраиваемых полей; после проверок ошибок; перед `return` после логического блока.
- Строки ≤ 180 жёстко, ≤ 99 по возможности (кроме struct-тегов).
- Функции в порядке вызова, приватные после публичных.

Приёмка: чеклист `go-code-style` пройден по всем `.go`-файлам; `golangci-lint run` чистый.

## REF-09 — Привести затронутые тесты к `go-tests`

Аудит: разделы 3, 4. Выполнять в PR задач REF-03 и REF-04. **Решение:** stdlib или testify;
`tt := tt` не добавлять (Go ≥ 1.22, `copyloopvar`).

- Поля кейсов `give*` / `want*` / `setup*`; `wantErr error` + `errors.Is`, `wantErrContains` для текста.
- Имена кейсов в формате `"[условие] -> [результат]"`.
- `cleanEnvironment` → `t.Setenv(name, ""); os.Unsetenv(name)`.

Приёмка: тесты `config` и `lifecycle` соответствуют чеклисту `go-tests` за вычетом пунктов,
отменённых решениями из раздела 4 аудита.
