include .env
export

MIGRATIONS=./migrations

db-up:
	docker compose up -d db

db-down:
	docker compose down

db-psql:
	docker compose exec db psql -U $(DB_USER) -d $(DB_NAME)

migrate-up:
	goose -dir $(MIGRATIONS) postgres "$(DATABASE_URL)" up

migrate-down:
	goose -dir $(MIGRATIONS) postgres "$(DATABASE_URL)" down

migrate-status:
	goose -dir $(MIGRATIONS) postgres "$(DATABASE_URL)" status

run:
	go run ./cmd/api

test:
	go test ./... -count=1

vet:
	go vet ./...
