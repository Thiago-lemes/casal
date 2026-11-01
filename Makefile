include .env
export

MIGRATIONS=./migrations
# as migrações rodam com o dono do banco; sem MIGRATE_DATABASE_URL, usa DATABASE_URL
MIGRATE_DATABASE_URL ?= $(DATABASE_URL)

db-up:
	docker compose up -d db

db-down:
	docker compose down

db-psql:
	docker compose exec db psql -U $(DB_USER) -d $(DB_NAME)

migrate-up:
	goose -dir $(MIGRATIONS) postgres "$(MIGRATE_DATABASE_URL)" up

migrate-down:
	goose -dir $(MIGRATIONS) postgres "$(MIGRATE_DATABASE_URL)" down

migrate-status:
	goose -dir $(MIGRATIONS) postgres "$(MIGRATE_DATABASE_URL)" status

run:
	go run ./cmd/api

test:
	go test ./... -count=1

vet:
	go vet ./...

# cria o papel limitado que o app usa (o RLS vale para ele); rode antes do migrate-up
db-role:
	docker compose exec -T db psql -U $(DB_USER) -d $(DB_NAME) -v ON_ERROR_STOP=1 \
		-c "DO \$$\$$ BEGIN IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'casal_app') THEN CREATE ROLE casal_app LOGIN PASSWORD '$(APP_DB_PASSWORD)'; END IF; END \$$\$$;" \
		-c "GRANT CONNECT ON DATABASE $(DB_NAME) TO casal_app;" \
		-c "GRANT USAGE ON SCHEMA public TO casal_app;"
