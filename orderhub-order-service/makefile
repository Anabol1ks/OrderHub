run:
	go run ./cmd/service/main.go

migrate:
	go run cmd/migrate/main.go

.PHONY: test
test:
	go test -v ./test/... -count=1

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