-- +goose Up
-- +goose StatementBegin

-- ---- última atividade ----
-- sessions.last_used_at é atualizada no máximo uma vez por hora por sessão (para não escrever a cada requisição).
-- user_activity guarda um dia por pessoa que a usou: é o que permite contar ativos por semana e a retenção, que
-- um único "último uso" não permitiria (ele apaga o histórico). Só ids e datas, nenhum conteúdo.
ALTER TABLE sessions ADD COLUMN last_used_at timestamptz;

CREATE TABLE user_activity (
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    day     date NOT NULL,
    PRIMARY KEY (user_id, day)
);
CREATE INDEX user_activity_day_idx ON user_activity (day);

-- ---- eventos de produto ----
-- Só nomes e contagens: sem coluna de texto livre nem JSON, para não haver onde gravar conteúdo por engano.
-- household_id e user_id viram nulos se o casal ou a pessoa forem apagados; o evento em si não guarda mais nada.
CREATE TABLE product_events (
    id           bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    occurred_at  timestamptz NOT NULL DEFAULT now(),
    household_id uuid        REFERENCES households (id) ON DELETE SET NULL,
    user_id      uuid        REFERENCES users (id) ON DELETE SET NULL,
    name         text        NOT NULL CHECK (name IN (
        'user_registered', 'household_created', 'partner_joined', 'module_enabled', 'module_disabled',
        'reminder_created_for_partner', 'push_enabled', 'feedback_sent'))
);
CREATE INDEX product_events_occurred_idx ON product_events (occurred_at);

-- Quem grava atravessa casais (o evento nasce antes do vínculo, ou no cadastro), então a política leva a cláusula de sistema.
ALTER TABLE product_events ENABLE ROW LEVEL SECURITY;
CREATE POLICY product_events_tenant ON product_events
    USING (
        household_id = nullif(current_setting('app.household_id', true), '')::uuid
        OR current_setting('app.system', true) = 'on'
    )
    WITH CHECK (
        household_id = nullif(current_setting('app.household_id', true), '')::uuid
        OR current_setting('app.system', true) = 'on'
    );

-- ---- papel e schema de agregados ----
-- O Grafana não lê tabelas: o papel casal_ops só enxerga views do schema ops, que devolvem contagens.
-- O papel é criado sem login aqui (make db-role / db-init.sh dão o login e a senha). As views pertencem ao dono do
-- banco, que ignora o RLS (nenhuma tabela usa FORCE ROW LEVEL SECURITY); por isso devolvem só números.
DO $$ BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'casal_ops') THEN
        CREATE ROLE casal_ops NOLOGIN;
    END IF;
END $$;

CREATE SCHEMA ops;

-- Célula com menos de 3 casais (ou pessoas) vira nula, para um casal de teste não ser reconhecível num gráfico.
CREATE FUNCTION ops.mask(n bigint) RETURNS bigint
    LANGUAGE sql IMMUTABLE PARALLEL SAFE
    AS $f$ SELECT CASE WHEN n >= 3 THEN n END $f$;

CREATE VIEW ops.daily_activation AS
WITH s AS (SELECT (created_at AT TIME ZONE 'UTC')::date AS day, count(*) AS n FROM users GROUP BY 1),
     h AS (SELECT (created_at AT TIME ZONE 'UTC')::date AS day, count(*) AS n FROM households GROUP BY 1),
     p AS (SELECT (accepted_at AT TIME ZONE 'UTC')::date AS day, count(*) AS n
           FROM household_invites WHERE accepted_at IS NOT NULL GROUP BY 1),
     days AS (SELECT day FROM s UNION SELECT day FROM h UNION SELECT day FROM p)
SELECT days.day,
       ops.mask(s.n) AS signups,
       ops.mask(h.n) AS households_created,
       ops.mask(p.n) AS partners_joined
FROM days
LEFT JOIN s USING (day)
LEFT JOIN h USING (day)
LEFT JOIN p USING (day);

CREATE VIEW ops.weekly_active AS
SELECT date_trunc('week', a.day)::date AS week,
       ops.mask(count(DISTINCT a.user_id)) AS active_people,
       ops.mask(count(DISTINCT m.household_id)) AS active_households
FROM user_activity a
LEFT JOIN household_members m ON m.user_id = a.user_id
GROUP BY 1;

CREATE VIEW ops.module_adoption AS
SELECT module, ops.mask(count(*)) AS households
FROM household_modules
GROUP BY module;

-- Casais que criaram algo no módulo na semana. Só a contagem: não distingue item pessoal de item do casal.
CREATE VIEW ops.module_usage AS
SELECT module, week, ops.mask(n) AS households
FROM (
    SELECT 'reminders' AS module, date_trunc('week', created_at AT TIME ZONE 'UTC')::date AS week, count(DISTINCT household_id) AS n FROM reminders GROUP BY 2
    UNION ALL
    SELECT 'tasks', date_trunc('week', created_at AT TIME ZONE 'UTC')::date, count(DISTINCT household_id) FROM tasks GROUP BY 2
    UNION ALL
    SELECT 'notes', date_trunc('week', created_at AT TIME ZONE 'UTC')::date, count(DISTINCT household_id) FROM notes GROUP BY 2
    UNION ALL
    SELECT 'shopping', date_trunc('week', created_at AT TIME ZONE 'UTC')::date, count(DISTINCT household_id) FROM shopping_items GROUP BY 2
    UNION ALL
    SELECT 'routine', date_trunc('week', created_at AT TIME ZONE 'UTC')::date, count(DISTINCT household_id) FROM routine_blocks GROUP BY 2
    UNION ALL
    SELECT 'finance', date_trunc('week', created_at AT TIME ZONE 'UTC')::date, count(DISTINCT household_id) FROM expenses GROUP BY 2
    UNION ALL
    SELECT 'goals', date_trunc('week', created_at AT TIME ZONE 'UTC')::date, count(DISTINCT household_id) FROM goals GROUP BY 2
) u;

-- Coorte semanal de cadastro x semanas seguintes com atividade (0 = a semana do cadastro).
CREATE VIEW ops.retention AS
WITH cohort AS (
    SELECT id, date_trunc('week', created_at AT TIME ZONE 'UTC')::date AS cohort_week FROM users
), sizes AS (
    SELECT cohort_week, count(*) AS n FROM cohort GROUP BY 1
), act AS (
    SELECT DISTINCT user_id, date_trunc('week', day)::date AS active_week FROM user_activity
)
SELECT c.cohort_week,
       ((a.active_week - c.cohort_week) / 7)::int AS weeks_since_signup,
       ops.mask(count(DISTINCT c.id)) AS active_people,
       ops.mask(s.n) AS cohort_size
FROM cohort c
JOIN sizes s ON s.cohort_week = c.cohort_week
JOIN act a ON a.user_id = c.id AND a.active_week >= c.cohort_week
GROUP BY c.cohort_week, s.n, ((a.active_week - c.cohort_week) / 7);

CREATE VIEW ops.push_coverage AS
SELECT ops.mask(count(*)) AS households,
       ops.mask(count(*) FILTER (WHERE has_push)) AS households_with_push
FROM (
    SELECT h.id,
           EXISTS (SELECT 1 FROM household_members m JOIN push_subscriptions p ON p.user_id = m.user_id
                   WHERE m.household_id = h.id) AS has_push
    FROM households h
) x;

GRANT USAGE ON SCHEMA ops TO casal_ops;
GRANT SELECT ON ALL TABLES IN SCHEMA ops TO casal_ops;
-- o papel do app não deve ler as views de agregados
REVOKE ALL ON SCHEMA ops FROM PUBLIC;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP SCHEMA ops CASCADE;
DROP TABLE product_events;
DROP TABLE user_activity;
ALTER TABLE sessions DROP COLUMN last_used_at;
-- o papel casal_ops é do cluster, não desta base: fica
-- +goose StatementEnd
