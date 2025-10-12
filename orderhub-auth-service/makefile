run:
	go run ./cmd/service/main.go

migrate:
	go run cmd/migrate/main.go

doc:
	docker-compose -f docker-compose.yml up -d

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