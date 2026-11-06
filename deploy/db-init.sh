#!/bin/sh
# roda uma única vez, na criação do volume do Postgres: cria os papéis limitados: o do app (o RLS vale para ele) e o do Grafana (só lê views de agregados)
set -e
psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" <<SQL
CREATE ROLE casal_app LOGIN PASSWORD '${APP_DB_PASSWORD}';
GRANT CONNECT ON DATABASE ${POSTGRES_DB} TO casal_app;
GRANT USAGE ON SCHEMA public TO casal_app;
CREATE ROLE casal_ops LOGIN PASSWORD '${OPS_DB_PASSWORD}';
GRANT CONNECT ON DATABASE ${POSTGRES_DB} TO casal_ops;
SQL
