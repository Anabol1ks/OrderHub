# OrderHub Auth Service

Микросервис аутентификации и авторизации для OrderHub. Предоставляет gRPC API для регистрации, входа, обновления и отзыва токенов, проверки токенов, восстановления пароля и верификации email. Работает с Postgres, Redis (опционально для кэша и rate limit) и Kafka для отправки email-сообщений.

## Назначение и функции

- Регистрация пользователей с ролью по умолчанию ROLE_CUSTOMER и отправкой письма с подтверждением email
- Вход (login) и выдача пары токенов: JWT Access и opaque Refresh
- Обновление (refresh) пары токенов с ротацией refresh-токена
- Logout одного устройства (по refresh токену) и массовый Logout всех сессий пользователя
- JWKS (JSON Web Key Set) для верификации JWT на стороне клиентов/гейтвея
- Интроспекция access-токена (проверка активности и извлечение субьекта/роли/exp)
- Восстановление пароля (запрос кода и подтверждение с изменением пароля)
- Верификация email (запрос/подтверждение)
- Периодические задачи очистки: просроченные токены, старые/осиротевшие сессии, использованные токены

## Архитектура и компоненты

- gRPC сервер (порт `APP_PORT`, по умолчанию `:8081`)
  - Health-check (`grpc_health_v1`), включена серверная рефлексия
  - Unary-интерцептор авторизации: публичные методы пропускаются, остальные требуют заголовок `Authorization: Bearer <access>`
- Сервисная логика (`internal/service`)
  - Управление пользователями, сессиями, access/refresh токенами, пароль/почта, JWKS
  - Кэш/Rate limit и blacklist в Redis (если включено)
  - Отправка email через Kafka (topic из `KAFKA_TOPIC_EMAIL`)
- Репозитории (`internal/repository`) через Postgres
- Токены (`internal/token`)
  - JWT RSA (подпись access токенов), JWKS из БД, опционально кэшируется в Redis
- Планировщик очистки (`internal/cleanup/scheduler.go`)
  - Запускается вместе с сервисом и периодически запускает процедуры очистки
- Docker образ
  - Многоступенчатая сборка: билд Go бинарей и финальный lightweight-образ
  - `entrypoint.sh` выполняет миграции перед стартом сервиса
- Интеграции
  - Postgres 17 (контейнер `auth-db`)
  - Redis 7 (контейнер `redis`)
  - Kafka (в docker-compose не поднимается; ожидается доступный брокер)

## Пример .env и .env.docker с таблицами переменных окружения

Ниже приведены содержимое файлов окружения и сводные таблицы переменных.

### .env (локальная разработка)

Содержимое:

```
ENV=development

DB_HOST=localhost
DB_PORT=5432
DB_USER=orderhub
DB_PASSWORD=12341
DB_NAME=orderhub-auth-db
DB_SSLMODE=disable

REDIS_ENABLED=true
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=lo-ren-ru
REDIS_DB=0
CACHE_TTL_SECONDS=60

APP_PORT=:8081

JWT_ISSUER=auth-service
JWT_AUDIENCE=orderhub
ACCESS_EXP=15m
REFRESH_EXP=7d

KAFKA_BROKERS=host.docker.internal:9092
KAFKA_TOPIC_EMAIL=emails.send
```

Таблица переменных окружения (.env):

| Переменная          | Обязат. | Назначение                                           | Пример                      | Примечание |
|---------------------|---------|------------------------------------------------------|-----------------------------|------------|
| ENV                 | Да      | Режим работы (development/production)                | development                 | Влияет на логгер |
| DB_HOST             | Да      | Адрес БД Postgres                                    | localhost                   | - |
| DB_PORT             | Да      | Порт БД Postgres                                     | 5432                        | - |
| DB_USER             | Да      | Пользователь БД                                      | orderhub                    | - |
| DB_PASSWORD         | Да      | Пароль БД                                            | 12341                       | - |
| DB_NAME             | Да      | Имя БД                                               | orderhub-auth-db            | - |
| DB_SSLMODE          | Да      | Режим SSL для БД                                     | disable                     | - |
| REDIS_ENABLED       | Да      | Включить Redis кэш/блеклист/лимиты                   | true                        | Если false — Redis не используется, но переменные всё равно читаются кодом |
| REDIS_ADDR          | Да      | Адрес Redis                                          | localhost:6379              | Используется, если REDIS_ENABLED=true |
| REDIS_PASSWORD      | Да      | Пароль Redis                                         | lo-ren-ru                   | Используется, если REDIS_ENABLED=true |
| REDIS_DB            | Да      | Номер БД Redis                                       | 0                           | Используется, если REDIS_ENABLED=true |
| CACHE_TTL_SECONDS   | Да      | TTL по умолчанию для кэша/ключей                     | 60                          | seconds |
| APP_PORT            | Да      | gRPC адрес/порт                                      | :8081                       | Формат ":порт" |
| JWT_ISSUER          | Да      | Issuer для JWT                                       | auth-service                | - |
| JWT_AUDIENCE        | Да      | Audience для JWT                                     | orderhub                    | - |
| ACCESS_EXP          | Да      | Время жизни Access токена                            | 15m                         | Поддерживается суффикс d (дни), например 1d |
| REFRESH_EXP         | Да      | Время жизни Refresh токена                           | 7d                          | Поддерживается суффикс d (дни) |
| KAFKA_BROKERS       | Нет     | Список брокеров Kafka (comma-separated)              | host.docker.internal:9092   | Может быть пустым; читает через os.Getenv |
| KAFKA_TOPIC_EMAIL   | Да      | Топик Kafka для email-сообщений                      | emails.send                 | - |

### .env.docker (запуск в Docker)

Содержимое:

```
ENV=production

# Хосты должны указывать на имена сервисов в Docker Compose
DB_HOST=auth-db
DB_PORT=5432
DB_USER=orderhub
DB_PASSWORD=12341
DB_NAME=orderhub-auth-db
DB_SSLMODE=disable

REDIS_ENABLED=true
REDIS_ADDR=redis:6379
REDIS_PASSWORD=lo-ren-ru
REDIS_DB=0
CACHE_TTL_SECONDS=60

APP_PORT=:8081

JWT_ISSUER=auth-service
JWT_AUDIENCE=orderhub
ACCESS_EXP=15m
REFRESH_EXP=7d

KAFKA_BROKERS=host.docker.internal:9092
KAFKA_TOPIC_EMAIL=emails.send
```

Таблица переменных окружения (.env.docker):

| Переменная          | Обязат. | Назначение                                           | Пример            | Примечание |
|---------------------|---------|------------------------------------------------------|-------------------|------------|
| ENV                 | Да      | Режим работы                                         | production        | - |
| DB_HOST             | Да      | Адрес БД Postgres (имя сервиса в compose)            | auth-db           | - |
| DB_PORT             | Да      | Порт БД Postgres                                     | 5432              | - |
| DB_USER             | Да      | Пользователь БД                                      | orderhub          | - |
| DB_PASSWORD         | Да      | Пароль БД                                            | 12341             | - |
| DB_NAME             | Да      | Имя БД                                               | orderhub-auth-db  | - |
| DB_SSLMODE          | Да      | Режим SSL для БД                                     | disable           | - |
| REDIS_ENABLED       | Да      | Включить Redis кэш/блеклист/лимиты                   | true              | - |
| REDIS_ADDR          | Да      | Адрес Redis (имя сервиса в compose)                  | redis:6379        | - |
| REDIS_PASSWORD      | Да      | Пароль Redis                                         | lo-ren-ru         | - |
| REDIS_DB            | Да      | Номер БД Redis                                       | 0                 | - |
| CACHE_TTL_SECONDS   | Да      | TTL по умолчанию для кэша/ключей                     | 60                | seconds |
| APP_PORT            | Да      | gRPC адрес/порт                                      | :8081             | - |
| JWT_ISSUER          | Да      | Issuer для JWT                                       | auth-service      | - |
| JWT_AUDIENCE        | Да      | Audience для JWT                                     | orderhub          | - |
| ACCESS_EXP          | Да      | Время жизни Access токена                            | 15m               | Поддерживается суффикс d |
| REFRESH_EXP         | Да      | Время жизни Refresh токена                           | 7d                | Поддерживается суффикс d |
| KAFKA_BROKERS       | Нет     | Список брокеров Kafka                                | host.docker.internal:9092 | Kafka не в compose; укажите доступный брокер |
| KAFKA_TOPIC_EMAIL   | Да      | Топик Kafka для email-сообщений                      | emails.send       | - |

Примечание: файл `.env` в репозитории присутствует для локального запуска; для контейнера используется `.env.docker` через `env_file` в docker-compose.

## Способы запуска

### Требования

- Go 1.25+
- Docker и Docker Compose (для контейнерного запуска)
- Доступный брокер Kafka (например, `host.docker.internal:9092`)
- Свободные порты: 8081 (gRPC), 5432 (Postgres), 6379 (Redis)

### Запуск локально (без Docker)

1) Поднимите Postgres и Redis локально (или укажите корректные хосты в `.env`).
2) Убедитесь, что в корне `orderhub-auth-service` есть `.env` с валидными значениями.
3) Выполните:

- Миграции: `make migrate`
- Запуск сервиса: `make run`
- Тесты: `make test`

Сервис поднимет gRPC на `:8081`.

### Запуск через Docker Compose

В каталоге `orderhub-auth-service`:

- Сборка и старт: `make docker-up`
- Только старт (без сборки): `make docker-start`
- Пересборка только сервиса: `make docker-rebuild`
- Стоп: `make docker-stop`
- Полный стоп и удаление: `make docker-down` или с volumes: `make docker-down-volumes`
- Логи: `make docker-logs` или `make docker-logs-app`

Что делает compose:
- Поднимает контейнеры `auth-db` (Postgres 17) и `redis` (Redis 7)
- Собирает и запускает `auth-service`
- В `entrypoint.sh` сервис: ждёт БД, запускает миграции (`/app/migrate`), стартует gRPC (`/app/auth-service`)

Kafka в compose не поднимается — укажите внешний брокер через `KAFKA_BROKERS`.

## gRPC API и ссылки на .proto

Proto-файлы находятся в репозитории: https://github.com/Anabol1ks/orderhub-pkg-proto/tree/main/proto (пакет `auth.v1`). Клиентский код импортирует `github.com/Anabol1ks/orderhub-pkg-proto/proto/auth/v1`.

Общие замечания по авторизации:
- Публичные методы (не требуют `Authorization`): Register, Login, Refresh, Logout, GetJwks, ConfirmEmailVerification, RequestPasswordReset, ConfirmPasswordReset, Introspect
- Приватные (требуют Bearer): RequestEmailVerification (см. нюанс ниже)
- Нюанс Logout: при `all=true` выполняется массовый logout, и для него требуется идентичность пользователя (Bearer), для одиночного logout по `refresh_token` авторизация не нужна

Таблица методов:

| Метод                       | Запрос                                           | Ответ                                                     | Авторизация          | Назначение |
|-----------------------------|--------------------------------------------------|-----------------------------------------------------------|----------------------|------------|
| Register                    | email, password                                  | user_id, email, role, created_at                          | не требуется         | Регистрация пользователя и запуск верификации email |
| Login                       | email, password                                  | user_id, role, tokens(access_token, refresh_token, exp)   | не требуется         | Выдача пары токенов и создание сессии |
| Refresh                     | refresh_token (opaque)                           | tokens(access_token, refresh_token, exp)                  | не требуется         | Ротация refresh, обновление пары токенов |
| Logout                      | oneof: refresh_token или all: bool               | Empty                                                     | зависит (см. прим.)  | Отзыв refresh токена или всех токенов пользователя |
| GetJwks                     | Empty                                            | keys: [kid,kty,alg,use,n,e]                               | не требуется         | Публикация JWKS для верификации JWT |
| Introspect                  | access_token: string                             | active, user_id, role, exp_unix, scopes                   | не требуется         | Проверка валидности access токена |
| RequestPasswordReset        | email: string                                    | Empty                                                     | не требуется         | Запрос кода/ссылки для сброса пароля |
| ConfirmPasswordReset        | code: string, new_password: string               | Empty                                                     | не требуется         | Подтверждение сброса, установка нового пароля |
| RequestEmailVerification    | email: string (опц.; чаще пусто для автор. юзера) | Empty                                                     | требуется            | Запрос кода подтверждения email (авторизованный пользователь) |
| ConfirmEmailVerification    | code: string                                     | Empty                                                     | не требуется         | Подтверждение email по коду |

Примечания:
- В `RequestEmailVerification` есть ветка для неавторизованного пользователя по email, но текущий интерцептор помечает метод как приватный — для вызова требуется Bearer. Если нужен публичный сценарий, добавьте метод в список публичных в `internal/transport/grpc/interceptor.go`.
- В ответах времена экспирации отдаются в Unix-секундах; refresh в ответе — opaque, а в БД хранится его хэш.

## Прочее

- Health-check: сервис регистрирует `grpc_health_v1.HealthServer` и включает gRPC Reflection.
- Безопасность: предусмотрен blacklist для access-токенов при logout; содержимое хранится в Redis при наличии.
- Очистка: планировщик запускает регулярные задачи (истёкшие/использованные токены, старые/осиротевшие сессии).
- Миграции: в контейнере автоматически выполняются перед стартом сервиса (`entrypoint.sh`). Для локального запуска используйте `make migrate`.
- Конфигурация JWT: поддерживаются продолжительности с суффиксом `d` (например, `7d`).
- Примеры запросов в Postman: https://web.postman.co/workspace/My-Workspace~4f573858-3f68-4a9d-8cba-f984d14376dc/folder/68e0dba4af7cf0200a9fed1f
