# Ревью репозитория: overhead и рефакторинг

- Дата: 2026-09-13
- Срез: `main` @ `1bd7fda` (этапы 01–04 смержены)
- Линзы: скиллы `ponytail` / `ponytail-audit` (over-engineering), `go-code-style`, `go-tests`
- Состояние: `go build`, `go vet`, `go test ./...` — зелёные; `golangci-lint run` — 0 issues
- Декомпозиция по задачам: [TASKS.md](TASKS.md)

Всё ниже — **предложения** (статус `Proposed`). Пункты, отменяющие принятые ADR, требуют решения
и нового ADR с `Supersedes`, а не правки старых.

## Итог в цифрах

| | Сейчас | После | Δ |
|---|---:|---:|---:|
| Go-код (prod + tests) | 2335 строк | ~1400 строк | **≈ −950** |
| `//nolint` в коде | 6 | 0 | −6 |
| Косвенные зависимости в `go.mod` приложения | 246 | ~15 | **−230** (уезжают в tools-модуль) |
| Пустые пакеты-заглушки (`.gitkeep`) | 7 | 0 | −7 |
| Дублирующие проверки в pre-commit | vet + lint | lint | −1 джоба |

`net: ≈ −950 строк Go, −230 зависимостей из графа приложения, 0 новых зависимостей.`

---

## 1. От чего можно отказаться (overhead, ранжировано по размеру выреза)

Формат: `тег` · что вырезать · чем заменить · где.

### 1.1 `yagni:` конфигурация, которую никто не читает — ≈ −520 строк

`HTTPConfig`, `TelegramAPIConfig`, `TelegramSenderConfig`, `JWTConfig`, `CORSConfig`, `CookieConfig`,
`ReminderConfig` загружаются и строго валидируются, но **ни одно поле не используется** ни одним
бинарником. Потребители появятся на этапах 06–15.

- [internal/config/config.go](../../internal/config/config.go) — 7 из 10 структур.
- [internal/config/validate.go](../../internal/config/validate.go) — ~170 из 258 строк: webhook-regexp,
  CORS-дедупликация, cookie-domain regexp, JWT-длины, reminder-диапазоны.
- [internal/config/consts.go](../../internal/config/consts.go) — `TelegramUpdateMode*`.
- [internal/config/config_test.go](../../internal/config/config_test.go) — ~280 из 537 строк.
- [.env.example](../../.env.example), [cmd/api/main_test.go](../../cmd/api/main_test.go) — лишние переменные.

Побочный вред: `api` не стартует без `TELEGRAM_BOT_TOKEN`, `JWT_ACCESS_SECRET`, `CORS_ALLOWED_ORIGINS`,
`MINI_APP_URL`, хотя ничего с ними не делает. Секрет, который валидируется, но не используется, —
чистая поверхность атаки и трение при локальном запуске.

Замена: ничего. Каждое поле добавляется в этапе, где у него появляется потребитель, и сразу в
нативном типе (`http.SameSite`, `[]*url.URL` и т.п.), а не строкой с ручной валидацией.
Недостижимая ветка `len(origins) == 0` в `validateCORS` уходит вместе с кодом.

> Требует решения: пересматривает объём этапа 02 ([ADR-002](../../.codex/skills/maintain-project-context/references/ADR_LOG.md)).
> Разделение конфигов по процессам (`APIConfig` / `WorkerConfig` / `MigrateConfig`) **сохраняется**.

### 1.2 `yagni:` + `native:` пакет `internal/appctx` — ≈ −210 строк, −6 `nolint`

Собственный тип `appctx.Context` (встраивает `context.Context` + приватный логгер) повторяет то, что
zerolog уже даёт через `logger.WithContext(ctx)` / `zerolog.Ctx(ctx)`.

- Логгер кладётся в контекст **дважды**: своим `loggerContextKey{}` и `logger.WithContext` —
  [appctx.go:91-95](../../internal/appctx/appctx.go:91).
- `FromContext` = `zerolog.Ctx`; `WithContext` — перезаворачивание; `normalizeBase` существует только
  ради `nil`-контекста, который в prod никто не передаёт.
- `WithRequestID` вызывается только из тестов. Для HTTP-этапа есть нативный `zerolog/hlog`
  (`hlog.RequestIDHandler`) — зависимость уже подключена.
- `New` повторно нормализует и валидирует `APP_ENV` / `LOG_LEVEL`, которые уже провалидировал `config`,
  поэтому `ErrInvalidOptions` в prod недостижим.
- Все 6 `//nolint:contextcheck` в `cmd/*/main.go` появились только из-за этого типа: линтер не видит,
  что `application` содержит `base`.

Замена: `context.Context` везде + одна функция настройки логгера (~20 строк) по окружению.
`LOG_LEVEL` можно сразу грузить как `zerolog.Level` — cleanenv поддерживает
`encoding.TextUnmarshaler` (проверено в v1.5.0), что убирает ручной `switch` уровней.

> Требует решения: supersedes [ADR-005](../../.codex/skills/maintain-project-context/references/ADR_LOG.md)
> (immutable appctx). Иммутабельность сохраняется бесплатно: `context.Context` и так иммутабелен.

### 1.3 `shrink:` `internal/lifecycle` — ≈ −160 строк (191 → ~80 prod)

[lifecycle.go](../../internal/lifecycle/lifecycle.go) решает простую задачу (ждать сигнал → выполнить задачи
в обратном порядке в пределах дедлайна) через 10 функций и 3-полевой `triggerResult`.

- `Run` → `run` с параметром `stop` и `nil`-guard — это шов для тестов. Не нужен: `signal.NotifyContext`
  от уже отменённого родителя сразу `Done`, тесты могут звать `Run` напрямую.
- `runFn` в обоих прод-вызовах `nil` → в коде 4 ветки `runDone == nil`
  (`startApplication`, `waitForShutdown`, `shouldWaitForApplication`, `applicationRunFailed`).
  Замена одной строкой: `if run == nil { run = func(ctx context.Context) error { <-ctx.Done(); return nil } }`.
- `timeout <= 0` — уже гарантирует `config.validateLifecycle`.
- Фолбэк имени `"unnamed"` и пропуск `task.Run == nil` — оба вызова всегда передают имя и функцию.
- Горутина **на каждую** задачу → одна горутина на весь проход задач, один `select` по дедлайну.
- `waitForApplication(...) (error, error)` — два неименованных `error` (нарушение `go-code-style`).
- Типы `RunFunc` / `ShutdownFunc` — алиасы ради одного использования.

Бонус: если `signal.NotifyContext` создавать в начале `run` в entrypoint, SIGTERM во время ожидания
advisory lock миграции будет обработан штатно — это закрывает открытое последствие
[ADR-010](../../.codex/skills/maintain-project-context/references/ADR_LOG.md).

### 1.4 `delete:` зависимости инструментов в графе приложения — −230 косвенных зависимостей

`tool` в [go.mod](../../go.mod) тянет golangci-lint и lefthook в граф модуля приложения: 246 строк
`// indirect`, `go.sum` ≈ 100 КБ. Риск не только в шуме: общие транзитивные зависимости линтера
могут поднять версии библиотек, с которыми собирается сервис.

Замена: отдельный `tools/go.mod` (или `tools.mod`) и `go tool -modfile=tools/go.mod golangci-lint`.
Пин версий остаётся в Go-модуле, как требует ADR-004.

Там же: версия golangci-lint закреплена **дважды** — `go.mod` (v2.12.2) и
[workflow](../../.github/workflows/golangci-lint.yml) (`version: v2.12.2`, `install-mode: goinstall`).
В CI достаточно `go tool ... golangci-lint run`.

> Частично supersedes [ADR-004](../../.codex/skills/maintain-project-context/references/ADR_LOG.md).

### 1.5 `delete:` scaffolding «на потом» — 7 пустых пакетов, 2 файла

- `.gitkeep` в `api/`, `internal/auth`, `internal/domain`, `internal/reminder`, `internal/service`,
  `internal/telegram`, `internal/transport/http`. Структура, в которой ещё нет кода, фиксирует слои
  до того, как понятно, нужны ли они. Директория создаётся коммитом, который кладёт в неё код.
  Отдельно по `go-code-style`: пакет `internal/transport/http` будет называться `http` и
  **конфликтовать с `net/http`** — при создании назвать иначе (`httpapi`).
- [.dockerignore](../../.dockerignore) без Dockerfile — вернуть на этапе 17.
- [TESTING_PLAN.md](../../TESTING_PLAN.md) в корне — артефакт этапа 04 с чекбоксами. Место — описание PR /
  change log; правило «maintain TESTING_PLAN.md» в `PROJECT_CONTEXT.md` порождает перезаписываемый
  файл без истории.

### 1.6 `delete:` мёртвый адаптер логгера goose — −20 строк

[goose_logger.go](../../internal/repository/postgres/goose_logger.go) и `goose.WithLogger(...)` в
[postgres.go:118](../../internal/repository/postgres/postgres.go:118) ничего не делают: `Provider.logf`
в goose v3.27.3 выходит сразу при `!verbose`, а `WithVerbose` не включён; session locker не логирует.
Утверждение ADR-010 «адаптер удерживает вывод goose вне stdout» неверно — вывода нет и без него.

Замена: ничего. Результаты миграций уже логируются структурно (`logResult`, `migration_status`).

### 1.7 `shrink:` тройная проверка команды миграции — −25 строк

Команда проверяется в [cmd/migrate/main.go:43](../../cmd/migrate/main.go:43) (`SupportedCommand`), повторно в
начале [`Migrate`](../../internal/repository/postgres/postgres.go:77) и третий раз в `default` у `runCommand`.
Гейт до подключения к БД (решение ADR-010) — в `main`. В `Migrate` достаточно `default`-ветки `switch`;
вместе с верхней проверкой уходит тест `TestMigrateRejectsUnknownCommandBeforeUsingThePool`.

### 1.8 `delete:` дубли в tooling

- [lefthook.yml](../../lefthook.yml): джоба `vet` дублирует `govet` с `enable-all` внутри golangci-lint.
- [.golangci.yml](../../.golangci.yml):
  - 6 пересекающихся линтеров сложности: `cyclop`, `gocyclo`, `gocognit`, `maintidx`, `nestif`,
    `funlen` → оставить `gocognit` + `funlen`;
  - форматтеры `gci` и `goimports` оба управляют группировкой импортов → оставить `gci` + `gofumpt`;
  - `testifylint` (testify не используется), `testableexamples` (примеров нет) — no-op;
  - `run.go: "1.26"` дублирует `go.mod`; блок `output` и `run.tests: true` повторяют дефолты;
  - `lll: 200` расходится с жёстким лимитом `go-code-style` — **180**.

### 1.9 `shrink:` ручное восстановление env в тестах конфига — −20 строк

`cleanEnvironment` в [config_test.go:503](../../internal/config/config_test.go:503) вручную сохраняет и
восстанавливает 24 переменные. Stdlib: `t.Setenv(name, ""); os.Unsetenv(name)` — `t.Setenv` сам
регистрирует восстановление исходного значения.

### 1.10 `shrink:` дублирование README и PROJECT_CONTEXT

[README.md](../../README.md) пересказывает разделы о миграциях, конфигурации и логировании из
`PROJECT_CONTEXT.md` — два источника правды, которые уже расходятся. README: quickstart + ссылки.

---

## 2. Рефакторинг по `go-code-style`

Делать **после** вырезов из раздела 1 — половина нарушений исчезнет вместе с кодом.

| Правило | Где нарушено |
|---|---|
| Пакетные неэкспортируемые `var`/`const` с `_`-префиксом | `serviceName` (3× `cmd/*/main.go`), `environment*` в `appctx`, `webhookSecretPattern`, `domainPattern`, `configFileField`, `envField`, `lockRetryPeriodSeconds`; в тестах `schemaTables`, `configEnvironmentNames`, `test*` |
| Тип ошибки — `*Error`, экспортирован | `config.validationError` → `ValidationError` |
| Встраиваемые типы вверху, отделены пустой строкой | `DatabaseMigrationConfig`, `TelegramAPIConfig`, `appctx.Context` |
| Жёсткий лимит строки 180 | [config.go:20](../../internal/config/config.go:20) (183), [validate.go:207](../../internal/config/validate.go:207) (188) |
| Мягкий лимит 99 (не теги) | сигнатуры `lifecycle.go:38,43,82` (105–150), `postgres.go:75,108,122`, `validate.go:48,56,191,208`, `appctx.go:41`, `load.go:66` |
| Пустая строка после проверки ошибки / перед `return` после блока | цепочки `if err := ...` в `validateAPI`/`validateWorker`/`validateDatabase`/`validateCommon`/`validateCORS`/`validateCookie`; `cmd/migrate/main.go:39-46`; `postgres.Connect`, `postgres.Migrate`; `appctx.withLogger` |
| Named return только для однотипных результатов с понятными именами | `lifecycle.waitForApplication (error, error)`, `runShutdownTasks (bool, error)` |
| Функции в порядке вызова | `lifecycle.go`: `shutdownApplication` вызывает `runShutdownTasks` → `waitForApplication` → `applicationRunFailed`, а объявлены в другом порядке |
| Пакет не конфликтует со stdlib | будущий `internal/transport/http` |
| Fewest files | `config/consts.go` (8 констант) → в `config.go` |

Что **уже соответствует**: инициализация структур по именам полей, `&T{}` вместо `new(T)`, `Err*`-
sentinel-ы, `goimports`/`gofumpt`, нет `util`/`common`, нет затенения встроенных имён.

## 3. Тесты по `go-tests`

Применять к тестам, которые переписываются в задачах раздела 1, а не отдельным «массовым» проходом.

- Поля кейсов `value` / `want` / `wantErr bool` / `field` / `mutate` → `give*` / `want*` / `setup*`;
  `wantErr error` для sentinel + `ErrorIs`, `wantErrContains` для текста.
- Имена кейсов `"default"`, `"zero"`, `"invalid"`, `"environment"` → `"[условие] -> [результат]"`.
- `appctx_test` передаёт `nil` в качестве `context.Context` (`var base context.Context`) — уходит с пакетом.

## 4. Противоречия, которые нужно решить до рефакторинга

1. **`tt := tt`** требует `go-tests`, но на Go ≥ 1.22 переменная цикла уже per-iteration, а включённый
   `copyloopvar` пометит такую строку. Рекомендация: не добавлять, поправить скилл.
2. **`require` / `assert` (testify)** — `go-tests` на них опирается, в репо только stdlib `testing`.
   Сейчас testify косвенно есть в графе через golangci-lint; после выноса tools (1.4) это станет новой
   прямой зависимостью. Рекомендация ponytail: остаться на stdlib.
3. **Порядок импортов**: `go-code-style` пишет «stdlib → внутренние → внешние», но там же ссылается на
   `goimports`, который ставит локальные пакеты последними. Репо консистентно и форсится `gci`:
   stdlib → внешние → внутренние. Рекомендация: оставить как есть, поправить формулировку скилла.
4. **Интеграционные тесты**: `go-tests` — `tests/integration/` + testcontainers; в репо — пропуск по
   `TEST_DATABASE_URL` рядом с кодом. testcontainers — тяжёлая зависимость ради одного пакета.
   Рекомендация: оставить текущий подход до появления второго пакета с БД.

## 5. Осознанно оставляем

- `migrations/embed.go` как отдельный пакет — `go:embed` не видит родительские директории.
- `postgres.InitDB` — 12 строк, один вызов, но закрывает пул при ошибке миграции и явно называет шаг.
- Advisory lock через `goose.Provider` — реальная защита от гонки при rolling deploy, проверена тестом.
- Разные `APIConfig` / `WorkerConfig` / `MigrateConfig` и санитизированные sentinel-ошибки — граница
  безопасности, не overhead.
- Двойная проверка `CONFIG_FILE` в production в `load()` — обе нужны: `.env` перекрывает окружение
  через `os.Setenv`, поэтому одна проверка до чтения файла, одна после.
- Похожие `run` в трёх entrypoint-ах — после 1.2 это ~25 строк на бинарник; общая обёртка спрятала бы
  различия (миграции в api, lifecycle не в migrate).

## 6. Замечено попутно (вне скоупа over-engineering)

- **CI не запускает `go test` / `go build`** — только линтер, хотя quality gates в `PROJECT_CONTEXT.md`
  их требуют. Включено в задачу по tooling.
- cleanenv при чтении `.env` делает `os.Setenv` для каждой переменной файла — секреты из `.env`
  наследуются дочерними процессами. Только dev (`CONFIG_FILE` запрещён в production), приоритет низкий.
- Документация устарела: roadmap в `PROJECT_CONTEXT.md` показывает этап 04 как «Implemented in working
  tree», хотя он смержен (#4); ADR-006 и ADR-007 заменены ADR-009, но их статус всё ещё `Accepted`.
