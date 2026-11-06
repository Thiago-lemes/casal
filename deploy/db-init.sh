#!/bin/sh
# roda uma única vez, na criação do volume do Postgres: cria o papel limitado que o app usa (o RLS vale para ele)
set -e
psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" <<SQL
CREATE ROLE casal_app LOGIN PASSWORD '${APP_DB_PASSWORD}';
GRANT CONNECT ON DATABASE ${POSTGRES_DB} TO casal_app;
GRANT USAGE ON SCHEMA public TO casal_app;
SQL
