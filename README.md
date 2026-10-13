# TripGo Trip Service

HTTP-сервис первой лабораторной работы курса: создаёт, возвращает и завершает поездки, хранит их в PostgreSQL и ведёт историю изменений статуса.

## Возможности

- HTTP API по OpenAPI-контракту;
- PostgreSQL через `pgxpool`, запросы собираются `squirrel`;
- атомарная запись поездки и истории статуса;
- идемпотентное создание поездки по `Idempotency-Key`;
- защита от двух активных поездок одного водителя и конкурентного завершения;
- liveness `/health` и readiness `/ready`;
- graceful shutdown по `SIGINT` и `SIGTERM`;
- многостадийный Docker-образ на `distroless` без исходников и Go toolchain.

## Требования

- Go 1.27.1;
- `tripgoctl`;
- Docker — только для запуска контейнера и дополнительного задания.

## Быстрый запуск

Из корня репозитория поднять PostgreSQL курса:

```bash
tripgoctl cluster start       # достаточно выполнить один раз на машине
tripgoctl environment start   # создаёт/обновляет .env
tripgoctl connect             # печатает адреса окружения
```

Применить миграции и запустить сервис:

```bash
make migrate
make run
```

Make подключает локальный `.env` через `-include` и экспортирует его переменные рецептам. При старте сервис проверяет PostgreSQL вызовом `Ping`; если конфигурация некорректна или база недоступна, приложение не запускается. По умолчанию HTTP-сервер слушает `:8080`.

Проверить сервис:

```bash
curl -fsS http://localhost:8080/health
curl -fsS http://localhost:8080/ready
```

Обе ручки при успешной проверке возвращают:

```json
{"status":"ok"}
```

## HTTP API

| Метод | Путь | Успех | Назначение |
|---|---|---:|---|
| `POST` | `/api/v1/trips` | `201` / `200` | создать поездку или вернуть результат повтора |
| `GET` | `/api/v1/trips/{tripId}` | `200` | получить поездку |
| `POST` | `/api/v1/trips/{tripId}/finish` | `200` | завершить поездку |
| `GET` | `/health` | `200` | проверить, что процесс работает |
| `GET` | `/ready` | `200` / `503` | проверить доступность PostgreSQL |

Успешные ответы имеют `Content-Type: application/json`. Ошибки бизнес-ручек возвращаются в формате RFC 9457 с `Content-Type: application/problem+json` и стабильным полем `code` (`invalid_request`, `trip_not_found`, `trip_completed`, `driver_busy`, `idempotency_conflict`, `internal_error`).

`POST /api/v1/trips` принимает необязательный UUID-заголовок `Idempotency-Key`. Первый запрос с ключом создаёт поездку и возвращает `201`; повтор с тем же ключом и телом возвращает ту же поездку и `200`; использование ключа с другим телом возвращает `409 idempotency_conflict`. Без заголовка ручка всегда работает как обычное создание. Ключ действует 24 часа, после чего может быть использован заново.

Источник истины для API — [`contracts/openapi/trip-service.openapi.yaml`](contracts/openapi/trip-service.openapi.yaml). Типы и интерфейс сервера генерируются в `internal/api/httpv1/gen/api.gen.go`; сгенерированный файл вручную не редактируется.

## Команды

| Команда | Назначение |
|---|---|
| `make generate` | перегенерировать Go-код из OpenAPI |
| `make migrate` | применить все миграции |
| `make migrate-down` | откатить миграции до версии `0` |
| `make run` | запустить сервис с переменными из `.env` |
| `make test` | выполнить `go test -race -count=2 ./...` |
| `make build` | собрать `dist/trip-service` |

`make generate` вызывает `go generate ./...`; директива генерации находится в `internal/api/httpv1/api.go`, а параметры — в `oapi-codegen.yaml`.

## Конфигурация

Безопасный пример находится в [`.env.example`](.env.example). Все значения, кроме `DATABASE_URL`, имеют defaults в приложении.

| Переменная | Обязательность | Default | Назначение |
|---|---:|---:|---|
| `HTTP_ADDR` | нет | `:8080` | адрес HTTP-сервера |
| `HTTP_READ_TIMEOUT` | нет | `10s` | таймаут чтения запроса |
| `HTTP_READ_HEADER_TIMEOUT` | нет | `5s` | таймаут чтения заголовков |
| `HTTP_WRITE_TIMEOUT` | нет | `15s` | таймаут записи ответа |
| `HTTP_IDLE_TIMEOUT` | нет | `60s` | keep-alive таймаут |
| `LOG_LEVEL` | нет | `info` | уровень `debug`, `info`, `warn` или `error` |
| `SHUTDOWN_TIMEOUT` | нет | `10s` | общий бюджет graceful shutdown |
| `DATABASE_URL` | да | — | URL подключения к PostgreSQL |
| `DATABASE_MAX_CONNS` | нет | `10` | максимум соединений пула |
| `DATABASE_MIN_CONNS` | нет | `2` | минимум соединений пула |
| `DATABASE_MAX_CONN_LIFETIME` | нет | `30m` | максимальное время жизни соединения |
| `DATABASE_CONNECT_TIMEOUT` | нет | `5s` | таймаут каждой попытки подключения |
| `DATABASE_QUERY_TIMEOUT` | нет | `3s` | таймаут SQL-запросов, rollback, стартового и внутреннего ping, readiness-проверки |

Уровень логирования разбирается стандартным `slog.Level`; регистр не важен, некорректное значение останавливает приложение при загрузке конфигурации. Все duration должны быть положительными, `DATABASE_MIN_CONNS` не может превышать `DATABASE_MAX_CONNS`.

## Принятые решения

- Транзакции работают с уровнем `READ COMMITTED`. Конкурентные инварианты обеспечиваются атомарными SQL-операциями и ограничениями PostgreSQL, поэтому более строгий уровень изоляции здесь не нужен.
- `TxManager` хранит `pgx.Tx` в `context.Context`; репозиторий сам выбирает транзакцию или пул. Вложенный `Do` переиспользует текущую транзакцию. Ошибка и паника приводят к rollback, успех — к commit.
- Rollback выполняется с отдельным контекстом без отмены исходного запроса, но с `DATABASE_QUERY_TIMEOUT`. Это позволяет очистить транзакцию после разрыва клиентского запроса и не держать горутину бесконечно.
- Создание поездки и запись начального статуса, как и завершение поездки с записью истории, выполняются атомарно. Частичный уникальный индекс `WHERE status = 'active'` запрещает две активные поездки одного водителя; PostgreSQL-код `23505` преобразуется в `driver_busy`.
- `Idempotency-Key`, SHA-256 нормализованного тела запроса и созданный `trip_id` хранятся в отдельной таблице 24 часа. Резервирование ключа, создание поездки и запись истории входят в одну транзакцию. Уникальный ключ сериализует конкурентные запросы: повтор возвращает существующую поездку, а другое тело — `idempotency_conflict`. Истёкшие записи удаляются при следующем создании с ключом.
- Создание использует `INSERT ... RETURNING`, а завершение — `UPDATE ... WHERE status = 'active' RETURNING`. Ответ содержит фактически сохранённые PostgreSQL значения с микросекундной точностью; два конкурентных `finish` не могут оба завершить поездку.
- `TIMESTAMPTZ` декодируется `pgx` в UTC, поэтому временные поля всех ответов имеют единое представление и не зависят от часового пояса процесса.
- `/health` не обращается к БД. `/ready` выполняет `Ping` PostgreSQL с `DATABASE_QUERY_TIMEOUT` и возвращает `503`, если БД недоступна.
- Ожидаемые ошибки преобразуются в безопасные `apierror`; внутренняя причина сохраняется для логирования, но не попадает в HTTP-ответ.

## Структура проекта

```text
cmd/trip-service/             точка входа
contracts/openapi/            OpenAPI-контракт
internal/api/httpv1/          HTTP-обработчики и валидация
internal/api/httpv1/gen/      сгенерированный OpenAPI-код
internal/apierror/            публичная модель ошибок
internal/app/                 сборка и жизненный цикл приложения
internal/app/config/          загрузка и валидация конфигурации
internal/app/log/             настройка slog
internal/db/postgres/         репозиторий и менеджер транзакций
internal/model/               доменные модели и ошибки
internal/service/             бизнес-логика
migrations/                   миграции goose
deploy/Dockerfile             многостадийная сборка образа
```

## Docker

Собрать образ и вывести его размер:

```bash
docker build -f deploy/Dockerfile -t trip-service:local .
docker image inspect trip-service:local --format '{{.Size}} bytes'
```

Финальный слой основан на `gcr.io/distroless/static-debian12:nonroot`, содержит только статически собранный бинарник и запускается от пользователя `nonroot`. Последний локально измеренный размер образа — `6 064 493` байта (примерно `5.78 MiB`).

### Запуск с PostgreSQL курса

Поднять окружение, применить миграции и собрать образ:

```bash
tripgoctl cluster start
tripgoctl environment start
make migrate
docker build -f deploy/Dockerfile -t trip-service:local .
```

В созданном `tripgoctl` файле `.env` PostgreSQL доступен по `localhost` хоста. Из контейнера для обращения к хосту нужно использовать `host.docker.internal`:

```bash
DATABASE_URL_DOCKER="$(sed -n 's/^DATABASE_URL=//p' .env | sed 's/@localhost:/@host.docker.internal:/')"

docker run --rm -d \
  --name trip-service \
  --env-file .env \
  --add-host host.docker.internal:host-gateway \
  -e DATABASE_URL="$DATABASE_URL_DOCKER" \
  -p 8080:8080 \
  trip-service:local
```

Docker Desktop предоставляет имя `host.docker.internal` автоматически; `--add-host ...:host-gateway` добавляет его на Linux.

Проверить готовность и посмотреть логи:

```bash
curl -fsS http://localhost:8080/ready
docker logs trip-service
```

Ожидаемый ответ:

```json
{"status":"ok"}
```

Остановить и удалить контейнер:

```bash
docker stop trip-service
```
