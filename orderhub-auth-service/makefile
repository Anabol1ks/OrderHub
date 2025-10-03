run:
	go run ./cmd/service/main.go

migrate:
	go run cmd/migrate/main.go

doc:
	docker-compose -f docker-compose.yml up -d

.PHONY: test
test:
		go test ./test/... -count=1