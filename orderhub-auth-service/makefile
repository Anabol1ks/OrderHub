# ==========================================
# Локальная разработка (без Docker)
# ==========================================

run:
	go run ./cmd/service/main.go

migrate:
	go run cmd/migrate/main.go

cleanup-all:
	go run cmd/cleanup/main.go all

cleanup-expired:
	go run cmd/cleanup/main.go expired

cleanup-sessions:
	go run cmd/cleanup/main.go sessions

cleanup-consumed:
	go run cmd/cleanup/main.go consumed

.PHONY: test
test:
	go test -v ./test/... -count=1

# ==========================================
# Docker команды
# ==========================================

# Сборка и запуск всех сервисов (PostgreSQL, Redis, Auth Service)
.PHONY: docker-up
docker-up:
	docker-compose up -d --build

# Запуск без пересборки
.PHONY: docker-start
docker-start:
	docker-compose up -d

# Остановка всех сервисов
.PHONY: docker-stop
docker-stop:
	docker-compose stop

# Остановка и удаление контейнеров
.PHONY: docker-down
docker-down:
	docker-compose down

# Остановка и удаление контейнеров + volumes (БД будет очищена)
.PHONY: docker-down-volumes
docker-down-volumes:
	docker-compose down -v

# Пересборка только auth-service
.PHONY: docker-rebuild
docker-rebuild:
	docker-compose up -d --build auth-service

# Просмотр логов всех сервисов
.PHONY: docker-logs
docker-logs:
	docker-compose logs -f

# Просмотр логов только auth-service
.PHONY: docker-logs-app
docker-logs-app:
	docker-compose logs -f auth-service

# Просмотр логов PostgreSQL
.PHONY: docker-logs-db
docker-logs-db:
	docker-compose logs -f auth-db

# Просмотр логов Redis
.PHONY: docker-logs-redis
docker-logs-redis:
	docker-compose logs -f redis

# Статус контейнеров
.PHONY: docker-ps
docker-ps:
	docker-compose ps

# Перезапуск auth-service
.PHONY: docker-restart
docker-restart:
	docker-compose restart auth-service

# ==========================================
# Docker: Утилиты и обслуживание
# ==========================================

# Подключение к bash в контейнере auth-service
.PHONY: docker-shell
docker-shell:
	docker-compose exec auth-service /bin/sh

# Подключение к PostgreSQL через psql
.PHONY: docker-db-shell
docker-db-shell:
	docker-compose exec auth-db psql -U orderhub -d orderhub-auth-db

# Запуск миграций вручную в контейнере
.PHONY: docker-migrate
docker-migrate:
	docker-compose exec auth-service /app/migrate

# Запуск cleanup в контейнере (все задачи)
.PHONY: docker-cleanup-all
docker-cleanup-all:
	docker-compose exec auth-service /app/cleanup all

# Запуск cleanup в контейнере (только expired)
.PHONY: docker-cleanup-expired
docker-cleanup-expired:
	docker-compose exec auth-service /app/cleanup expired

# Запуск cleanup в контейнере (только sessions)
.PHONY: docker-cleanup-sessions
docker-cleanup-sessions:
	docker-compose exec auth-service /app/cleanup sessions

# Запуск cleanup в контейнере (только consumed)
.PHONY: docker-cleanup-consumed
docker-cleanup-consumed:
	docker-compose exec auth-service /app/cleanup consumed

# ==========================================
# Docker: Очистка и сброс
# ==========================================

# Полная очистка: остановка, удаление контейнеров, volumes и образов
.PHONY: docker-clean
docker-clean:
	docker-compose down -v --rmi local

# Пересоздание всего с нуля (полная очистка + сборка + запуск)
.PHONY: docker-fresh
docker-fresh: docker-clean docker-up

# ==========================================
# Помощь
# ==========================================

.PHONY: help
help:
	@echo "==========================================
	@echo "Доступные команды:"
	@echo "==========================================
	@echo ""
	@echo "Локальная разработка:"
	@echo "  make run                  - Запуск сервиса локально"
	@echo "  make migrate              - Запуск миграций локально"
	@echo "  make test                 - Запуск тестов"
	@echo "  make cleanup-all          - Запуск всех cleanup задач локально"
	@echo ""
	@echo "Docker - основное:"
	@echo "  make docker-up            - Сборка и запуск всех сервисов"
	@echo "  make docker-start         - Запуск без пересборки"
	@echo "  make docker-stop          - Остановка сервисов"
	@echo "  make docker-down          - Остановка и удаление контейнеров"
	@echo "  make docker-restart       - Перезапуск auth-service"
	@echo "  make docker-rebuild       - Пересборка auth-service"
	@echo ""
	@echo "Docker - логи:"
	@echo "  make docker-logs          - Логи всех сервисов"
	@echo "  make docker-logs-app      - Логи auth-service"
	@echo "  make docker-logs-db       - Логи PostgreSQL"
	@echo "  make docker-logs-redis    - Логи Redis"
	@echo ""
	@echo "Docker - утилиты:"
	@echo "  make docker-shell         - Подключение к контейнеру auth-service"
	@echo "  make docker-db-shell      - Подключение к PostgreSQL"
	@echo "  make docker-migrate       - Запуск миграций в контейнере"
	@echo "  make docker-cleanup-all   - Cleanup в контейнере"
	@echo ""
	@echo "Docker - очистка:"
	@echo "  make docker-clean         - Полная очистка (контейнеры + volumes + образы)"
	@echo "  make docker-fresh         - Пересоздание всего с нуля"
	@echo "=========================================="