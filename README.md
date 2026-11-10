# TripGo Trip Service

HTTP-сервис создаёт, возвращает и завершает поездки, хранит их в PostgreSQL и ведёт
историю статусов. Во второй лабораторной API не изменился: код разделён на слои,
покрыт тестами и инструментирован через OpenTelemetry.

## Возможности

- HTTP API по OpenAPI-контракту;
- PostgreSQL через `pgxpool`, параметризованные запросы собираются `squirrel`;
- атомарная запись поездки и истории статуса;
- защита от двух активных поездок одного водителя и конкурентное завершение;
- идемпотентное создание по `Idempotency-Key`;
- RFC 9457 `application/problem+json` со стабильными кодами;
- JSON-логи `slog`, HTTP/DB-трейсы и метрики;
- liveness `/health`, readiness `/ready` и graceful shutdown.

## Быстрый запуск

Требуются Go 1.27.1+, `tripgoctl`, Docker и `golangci-lint` v2.15.0.

```bash
tripgoctl cluster start          # один раз на машине
tripgoctl environment start      # PostgreSQL + observability
tripgoctl connect                # адреса и URL Jaeger/Grafana/Prometheus
make migrate
make run
```

`environment start` создаёт `.env`; Makefile подключает его и экспортирует переменные.
Адреса PostgreSQL и OTLP в коде не захардкожены.

```bash
curl -fsS http://localhost:8080/health
curl -fsS http://localhost:8080/ready
```

## HTTP API

| Метод | Путь | Успех | Назначение |
|---|---|---:|---|
| `POST` | `/api/v1/trips` | `201` / `200` | создать поездку или вернуть идемпотентный повтор |
| `GET` | `/api/v1/trips/{tripId}` | `200` | получить поездку |
| `POST` | `/api/v1/trips/{tripId}/finish` | `200` | завершить поездку |
| `GET` | `/health` | `200` | liveness |
| `GET` | `/ready` | `200` / `503` | readiness PostgreSQL |

Источник истины — [`contracts/openapi/trip-service.openapi.yaml`](contracts/openapi/trip-service.openapi.yaml).
Сгенерированный код вручную не редактируется.

## Архитектура

```text
cmd/trip-service          config, запуск и signal handling
internal/container        composition root
internal/config           разбор и валидация env
internal/domain/trip      инварианты, статусы, Finish, доменные ошибки
internal/usecase/trip     create/get/finish, пакет на действие
internal/http/trip        HTTP handler на действие
internal/http/domainerror единый domain error -> status + Problem
internal/http/middleware  Recover -> Tracing -> Logging -> Metrics
internal/repository/trip  SQL-реализация repository
internal/infra/postgres   pool и transaction manager
internal/infra/telemetry  slog, OTel providers и instruments
internal/api/httpv1/gen   OpenAPI-код
migrations                goose-миграции
```

Зависимости направлены внутрь:

```text
HTTP transport ──► usecase ──► domain
repository     ──────────────► domain
container      ──► конкретные реализации
```

Интерфейс объявляет потребитель в своём `contract.go`. Usecase знает только узкие
repository/transaction/clock/logging/metrics-интерфейсы, а handler — только о своём
usecase. Моки `mockgen` лежат рядо с потребителем. Границы дополнительно проверяет
`depguard`.

### Домен и ошибки

`domain/trip.Trip` не позволяет менять состояние снаружи. Переход в `completed`
выполняется только `Finish`. Домен не импортирует HTTP, PostgreSQL, OpenAPI или OTel.

Usecase возвращает доменную ошибку, обёрнутую `%w`. `internal/http/domainerror`
в одном месте выбирает HTTP status и `Problem`. Неизвестная ошибка даёт безопасный
`500 internal_error`; внутренний текст попадает только в лог.

### Транзакции и конкурентность

- transaction manager хранит `pgx.Tx` в context и поддерживает вложенный `Do`;
- rollback идёт с `context.WithoutCancel` и ограничен `DATABASE_QUERY_TIMEOUT`;
- partial unique index атомарно защищает от двух активных поездок;
- finish берёт строку `FOR UPDATE`, меняет домен и пишет историю в одной транзакции;
- `Idempotency-Key`, поездка и история также записываются атомарно.

## Observability

Сервис отправляет сигналы по OTLP в OpenTelemetry Collector и не зависит от Jaeger,
Prometheus или Grafana напрямую.

### Логи

- JSON `slog` в stdout, уровень из `LOG_LEVEL`;
- всегда есть `service`, `trace_id`, `span_id`;
- одна запись `request handled`: `Info` для 2xx/4xx, `Error` для 5xx;
- бизнес-события `trip created` и `trip completed`;
- request/response body не логируются.

### Трейсы и метрики

- HTTP span `{method} {http.route}` и PostgreSQL spans через `otelpgx`;
- `db.operation.name`, `db.collection.name`, `error.type`;
- `http.server.request.duration`, `http.server.active_requests`;
- `db.client.operation.duration`;
- `tripgo.trips.created`, `tripgo.trips.completed`, `tripgo.trips.active`.

`http.route` всегда содержит шаблон, а не UUID. Middleware выполняются так:

```text
Recover -> Tracing -> Logging -> Metrics -> Router
```

Недоступный collector не останавливает HTTP API: exporters работают асинхронно.

## Конфигурация

Полный безопасный пример — [`.env.example`](.env.example). Ключевые переменные:

| Переменная | Default | Назначение |
|---|---|---|
| `HTTP_ADDR` | `:8080` | listen address |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `SHUTDOWN_TIMEOUT` | `10s` | graceful shutdown budget |
| `DATABASE_URL` | обязателен | PostgreSQL DSN |
| `DATABASE_*` | см. `.env.example` | pool и query timeouts |
| `OTEL_SERVICE_NAME` | `trip-service` | `service.name` |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `http://localhost:4318` | collector endpoint |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | `http/protobuf` | `http/protobuf` или `grpc` |
| `OTEL_RESOURCE_ATTRIBUTES` | namespace/environment | resource attributes |
| `OTEL_TRACES_SAMPLER` | `parentbased_always_on` | trace sampler |

## Команды и тесты

| Команда | Назначение |
|---|---|
| `make generate` | OpenAPI и gomock-кодогенерация |
| `make migrate` | накатить миграции |
| `make migrate-down` | откатить до `0` |
| `make run` | запустить сервис |
| `make test` | `-race -count=2`, coverage |
| `make test-integration` | repository-тесты с PostgreSQL |
| `make lint` | линтеры и форматтеры |
| `make build` | собрать `dist/trip-service` |

```bash
tripgoctl environment start
make migrate
make test-integration
```

Integration-тесты имеют build tag `integration`, используют уникальные UUID и удаляют свои
данные. Отдельный сценарий нарушает constraint второй таблицы и проверяет полный
rollback.

## Ручная проверка observability

1. Запустить окружение, миграции и сервис.
2. Вызвать `POST /api/v1/trips`.
3. Взять `trace_id` из JSON-лога `request handled`.
4. Найти по нему трейс в Jaeger и проверить вложенный PostgreSQL span.
5. Найти в Grafana/Prometheus `http_server_request_duration`, `tripgo_trips_created`
   и `tripgo_trips_active`.
6. Остановить collector и убедиться, что API продолжает отвечать.

В PR нужно приложить скриншоты трейса и метрик.

## Docker

```bash
docker build -f deploy/Dockerfile -t trip-service:local .
```

В контейнере `DATABASE_URL` и `OTEL_EXPORTER_OTLP_ENDPOINT` должны ссылаться на
`host.docker.internal`, а не на `localhost` контейнера.
