# Этап 1: Сборка приложения
# Используем golang:1.25-alpine (без patch версии для лучшей совместимости с зеркалами)
FROM golang:1.25-alpine AS builder

# Устанавливаем необходимые инструменты для сборки
RUN apk add --no-cache git make

# Устанавливаем рабочую директорию
WORKDIR /app

# Копируем go.mod и go.sum для загрузки зависимостей
COPY go.mod go.sum ./

# Загружаем зависимости
RUN go mod download

# Копируем весь исходный код
COPY . .

# Собираем бинарники для всех компонентов
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /bin/auth-service ./cmd/service/main.go
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /bin/migrate ./cmd/migrate/main.go
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /bin/cleanup ./cmd/cleanup/main.go

# Этап 2: Финальный образ
FROM alpine:latest

# Устанавливаем сертификаты для HTTPS, timezone data и dos2unix
RUN apk --no-cache add ca-certificates tzdata dos2unix

# Создаем непривилегированного пользователя
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

WORKDIR /app

# Копируем бинарники из builder
COPY --from=builder /bin/auth-service /app/auth-service
COPY --from=builder /bin/migrate /app/migrate
COPY --from=builder /bin/cleanup /app/cleanup

# Копируем entrypoint скрипт и конвертируем окончания строк
COPY entrypoint.sh /app/entrypoint.sh
RUN dos2unix /app/entrypoint.sh && chmod +x /app/entrypoint.sh

# Меняем владельца файлов
RUN chown -R appuser:appuser /app

# Переключаемся на непривилегированного пользователя
USER appuser

# Экспонируем порт gRPC
EXPOSE 8081

# Используем entrypoint для запуска миграций и сервиса
ENTRYPOINT ["/app/entrypoint.sh"]
CMD ["/app/auth-service"]
