# План тестирования: application context и lifecycle

## Обзор

Этап покрывает загрузку lifecycle-конфигурации, immutable application context с Zerolog и корректное завершение долгоживущих процессов.

## Зависимости

- Внешние сервисы не используются.
- Для lifecycle применяются функции-замыкания вместо моков.
- Логи записываются в `bytes.Buffer`.

## Тест-кейсы

### Config

- [x] Default и пользовательский `SHUTDOWN_TIMEOUT` для API/worker.
- [x] Нулевой, отрицательный и некорректный timeout.
- [x] Migrate не зависит от lifecycle-настроек.
- [x] Ошибки не раскрывают исходное значение.

### appctx

- [x] JSON output для production/test и console output для development.
- [x] Фильтрация по уровню и обязательные структурированные поля.
- [x] Переданный базовый logger, конфигурационное расширение и nil base context.
- [x] `WithContext`, `FromContext` и disabled logger без значения.
- [x] Immutable `WithRequestID`, включая пустой ID.
- [x] Тестовые секреты не появляются в созданных приложением логах.

### lifecycle

- [x] Штатное завершение по отмене context.
- [x] Запуск без RunFunc и shutdown tasks.
- [x] Shutdown tasks выполняются в обратном порядке.
- [x] Общий timeout ограничивает shutdown tasks и RunFunc.
- [x] Досрочная ошибка RunFunc возвращает безопасную sentinel error.
- [x] Ошибки shutdown tasks не раскрывают raw cause.

### Entry points

- [x] API и worker стартуют, получают SIGTERM и завершаются с кодом 0.
- [x] Migrate завершается сразу с кодом 0.
- [x] Некорректная конфигурация даёт код 1 и безопасный stderr.
- [x] Startup/shutdown логи не содержат test secrets.

## Покрытие

Цель: покрыть все ветви новых пакетов `appctx` и `lifecycle`, а также новые config validation cases.

## Приёмка

- [x] `go test ./...`
- [x] `go test -race ./...`
- [x] `go build ./cmd/...`
- [x] `go tool golangci-lint run`
- [x] Lefthook pre-commit
