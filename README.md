# TripGo Trip Service

Первая лабораторная работа курса: HTTP API для создания, получения и завершения поездок с хранением данных в PostgreSQL.

## Требования

- Go 1.27.1;
- Docker;
- `tripgoctl`.

## Запуск

Поднять окружение и подключиться к нему:

```bash
tripgoctl cluster start
tripgoctl environment start
tripgoctl connect
```

`tripgoctl environment start` создаёт локальный `.env`. После этого применить миграции и запустить сервис:

```bash
make migrate
make run
```

При старте сервис создаёт пул соединений и проверяет PostgreSQL вызовом `Ping`. Сервис по умолчанию слушает `:8080`.

## HTTP API

В первой работе из OpenAPI-контракта используются следующие операции:

| Метод | Путь | Назначение |
|---|---|---|
| `POST` | `/api/v1/trips` | создать поездку |
| `GET` | `/api/v1/trips/{tripId}` | получить поездку |
| `POST` | `/api/v1/trips/{tripId}/finish` | завершить поездку |
| `GET` | `/health` | проверить состояние процесса |
| `GET` | `/ready` | проверить доступность PostgreSQL |

Типы и серверный интерфейс генерируются из `contracts/openapi/trip-service.openapi.yaml` в `internal/api/httpv1/gen/api.gen.go`. Список операций первой работы задан в `oapi-codegen.yaml`.

```bash
make generate
```

Цель вызывает `go generate ./...`; директива генерации находится в `internal/api/httpv1/api.go`. Сгенерированный файл коммитится, но вручную не редактируется.

## Команды

```bash
make generate      # перегенерировать OpenAPI-код
make migrate       # применить все миграции
make migrate-down  # откатить все миграции
make run           # запустить сервис с переменными из .env
make test          # запустить тесты с -race и -count=2
make build         # собрать dist/trip-service
```

## Переменные окружения

Полный безопасный пример находится в `.env.example`.

| Переменная | Назначение |
|---|---|
| `HTTP_ADDR` | адрес HTTP-сервера; по умолчанию `:8080` |
| `HTTP_READ_TIMEOUT` | таймаут чтения запроса; по умолчанию `10s` |
| `HTTP_READ_HEADER_TIMEOUT` | таймаут чтения заголовков; по умолчанию `5s` |
| `HTTP_WRITE_TIMEOUT` | таймаут записи ответа; по умолчанию `15s` |
| `HTTP_IDLE_TIMEOUT` | keep-alive таймаут; по умолчанию `60s` |
| `LOG_LEVEL` | уровень логирования: `debug`, `info`, `warning` или `error`; по умолчанию `info` |
| `SHUTDOWN_TIMEOUT` | общий бюджет graceful shutdown; по умолчанию `10s` |
| `DATABASE_URL` | строка подключения PostgreSQL |
| `DATABASE_MAX_CONNS` | максимальное количество соединений пула; по умолчанию `10` |
| `DATABASE_MIN_CONNS` | минимальное количество соединений пула; по умолчанию `2` |
| `DATABASE_MAX_CONN_LIFETIME` | максимальное время жизни соединения; по умолчанию `30m` |
| `DATABASE_CONNECT_TIMEOUT` | таймаут создания пула и стартового `Ping` PostgreSQL; по умолчанию `5s` |
| `DATABASE_QUERY_TIMEOUT` | таймаут SQL-запросов и readiness-проверки; по умолчанию `3s` |

`HTTP_ADDR` и `DATABASE_URL` обязательны. Размеры пула валидируются, все duration должны быть положительными.

## Структура

```text
cmd/trip-service/             точка входа
contracts/openapi/            OpenAPI-контракт
internal/api/httpv1/          HTTP v1: обработчики и валидация
internal/api/httpv1/gen/      сгенерированный OpenAPI-код
internal/apierror/            безопасные публичные ошибки API
internal/app/                 сборка и lifecycle приложения
internal/app/config/          загрузка конфигурации из env
internal/app/log/             настройка slog
internal/db/postgres/         репозиторий и менеджер транзакций
internal/model/               доменные модели и ошибки
internal/service/             бизнес-логика
migrations/                   goose-миграции PostgreSQL
deploy/Dockerfile             многостадийная сборка образа
dist/                         локальные артефакты make build
```

## Решения

- Транзакции работают с уровнем `READ COMMITTED`: операции создают и изменяют одну поездку, а конкурентные инварианты закреплены атомарными операциями PostgreSQL. Более строгий уровень добавил бы повторы транзакций без пользы для текущих сценариев.
- Менеджер транзакций кладёт `pgx.Tx` в `context.Context`; репозиторий сам выбирает транзакцию или пул. Вложенный `Do` использует текущую транзакцию. Ошибка и паника приводят к rollback, успешное выполнение — к commit.
- Создание поездки и первая запись истории выполняются в одной транзакции. Запрет двух активных поездок водителя обеспечивает частичный уникальный индекс `WHERE status = 'active'`; PostgreSQL-код `23505` отображается в ошибку `driver_busy`.
- Завершение выполнено одним `UPDATE ... WHERE status = 'active' RETURNING ...`. Поэтому два конкурентных запроса не могут оба завершить поездку и переписать `finished_at`; проигравший получает `trip_completed`.
- `/health` проверяет только жизнь процесса и не обращается к БД. `/ready` с таймаутом `DATABASE_QUERY_TIMEOUT` вызывает `Ping` PostgreSQL.
- Ожидаемые ошибки переводятся сервисом в безопасные `apierror`; внутренняя причина сохраняется для логирования, но не возвращается клиенту.

## Docker

Собрать образ и проверить его размер:

```bash
docker build -f deploy/Dockerfile -t trip-service:local .
docker image inspect trip-service:local --format '{{.Size}}'
```

Финальный образ основан на `gcr.io/distroless/static-debian12:nonroot`, содержит только бинарник сервиса и запускается от пользователя `nonroot`.

Фактический размер образа `trip-service:local`: `6 064 493` байта (примерно `5.78 MiB`).
