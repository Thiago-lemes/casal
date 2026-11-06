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

# cria o papel limitado que o app usa (o RLS vale para ele); rode antes do migrate-up
# falha cedo e com mensagem clara se a senha do papel do app não estiver no .env
require-app-password:
	@test -n "$(APP_DB_PASSWORD)" || (echo "defina APP_DB_PASSWORD no .env (veja .env.example)"; exit 1)

require-ops-password:
	@test -n "$(OPS_DB_PASSWORD)" || (echo "defina OPS_DB_PASSWORD no .env (veja .env.example)"; exit 1)

db-role: require-app-password require-ops-password
	docker compose exec -T db psql -U $(DB_USER) -d $(DB_NAME) -v ON_ERROR_STOP=1 \
		-c "DO \$$\$$ BEGIN IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'casal_app') THEN CREATE ROLE casal_app LOGIN PASSWORD '$(APP_DB_PASSWORD)'; END IF; END \$$\$$;" \
		-c "GRANT CONNECT ON DATABASE $(DB_NAME) TO casal_app;" \
		-c "GRANT USAGE ON SCHEMA public TO casal_app;" 		-c "DO \$$\$$ BEGIN IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'casal_ops') THEN CREATE ROLE casal_ops LOGIN PASSWORD '$(OPS_DB_PASSWORD)'; END IF; END \$$\$$;" 		-c "ALTER ROLE casal_ops LOGIN PASSWORD '$(OPS_DB_PASSWORD)';" 		-c "GRANT CONNECT ON DATABASE $(DB_NAME) TO casal_ops;"


# ---- qualidade ----

test:  ## unitários; os de integração são pulados sem TEST_DATABASE_URL
	go test ./... -count=1

vet:
	go vet ./...

fmt-check:  ## falha se algum arquivo Go não estiver formatado
	@test -z "$$(gofmt -l .)" || (echo "arquivos sem gofmt:"; gofmt -l .; exit 1)

lint: vet fmt-check  ## vet + gofmt + staticcheck
	staticcheck ./...

TEST_DB=casal_test
TEST_URL=postgres://$(DB_USER):$(DB_PASSWORD)@localhost:5432/$(TEST_DB)?sslmode=disable
TEST_APP_URL=postgres://casal_app:$(APP_DB_PASSWORD)@localhost:5432/$(TEST_DB)?sslmode=disable
TEST_OPS_URL=postgres://casal_ops:$(OPS_DB_PASSWORD)@localhost:5432/$(TEST_DB)?sslmode=disable

test-db: require-app-password require-ops-password db-role
	-docker compose exec db createdb -U $(DB_USER) $(TEST_DB)
	goose -dir $(MIGRATIONS) postgres "$(TEST_URL)" up

# roda tudo duas vezes: como dono do banco e como casal_app (com o RLS ligado, como em produção)
test-integration: test-db
	TEST_OPS_URL="$(TEST_OPS_URL)" TEST_DATABASE_URL="$(TEST_URL)" go test ./... -count=1
	TEST_OPS_URL="$(TEST_OPS_URL)" TEST_DATABASE_URL="$(TEST_APP_URL)" go test ./... -count=1

check: lint test-integration  ## tudo o que o CI roda

.PHONY: require-app-password require-ops-password db-up db-down db-psql db-role migrate-up migrate-down migrate-status run test vet fmt-check lint test-db test-integration check
