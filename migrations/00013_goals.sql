-- +goose Up
-- +goose StatementBegin

-- Módulo Metas. Valores em centavos inteiros (bigint), teto de R$ 10.000.000,00 por valor.
-- Meta pessoal tem owner_id; a separação entre os parceiros do mesmo casal NÃO é do RLS, é só do código (visibleTo).

CREATE TABLE goals (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    household_id uuid        NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    created_by   uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    owner_id     uuid        REFERENCES users (id) ON DELETE CASCADE,
    scope        text        NOT NULL CHECK (scope IN ('household', 'personal')),
    visibility   text        NOT NULL CHECK (visibility IN ('household', 'private', 'view')),
    kind         text        NOT NULL CHECK (kind IN ('financial', 'habit', 'shared', 'project')),
    title        text        NOT NULL CHECK (length(btrim(title)) BETWEEN 1 AND 120),
    notes        text        NOT NULL DEFAULT '' CHECK (length(notes) <= 1000),
    status       text        NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'paused', 'done', 'archived')),
    due_on       date,
    completed_at timestamptz,
    completed_by uuid        REFERENCES users (id) ON DELETE SET NULL,
    -- aviso "todas as etapas estão prontas" já enviado (um por vez; limpo quando uma etapa reabre)
    all_done_notified_at timestamptz,
    version      integer     NOT NULL DEFAULT 1,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    CHECK ((scope = 'personal' AND owner_id = created_by AND visibility IN ('private', 'view'))
        OR (scope = 'household' AND owner_id IS NULL AND visibility = 'household'))
);
CREATE INDEX goals_household_idx ON goals (household_id, status);
CREATE INDEX goals_owner_idx ON goals (owner_id) WHERE scope = 'personal';

-- a meta de economia do orçamento aponta para a categoria COM o casal: o banco recusa a de outro casal
ALTER TABLE finance_categories ADD CONSTRAINT finance_categories_id_household_key UNIQUE (id, household_id);

CREATE TABLE goal_financial (
    goal_id      uuid PRIMARY KEY REFERENCES goals (id) ON DELETE CASCADE,
    household_id uuid   NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    target_cents bigint NOT NULL CHECK (target_cents > 0 AND target_cents <= 1000000000),
    source       text   NOT NULL DEFAULT 'contributions' CHECK (source IN ('contributions', 'budget_savings')),
    category_id  uuid,
    baseline_on  date   NOT NULL,
    CHECK ((source = 'contributions' AND category_id IS NULL)
        OR (source = 'budget_savings' AND category_id IS NOT NULL)),
    FOREIGN KEY (category_id, household_id) REFERENCES finance_categories (id, household_id) ON DELETE RESTRICT
);

-- o que já foi guardado; retirada é negativa. Aporte NÃO é gasto: não aparece em Finanças.
CREATE TABLE goal_contributions (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    goal_id      uuid        NOT NULL REFERENCES goals (id) ON DELETE CASCADE,
    household_id uuid        NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    created_by   uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    amount_cents bigint      NOT NULL CHECK (amount_cents <> 0 AND abs(amount_cents) <= 1000000000),
    made_on      date        NOT NULL,
    note         text        NOT NULL DEFAULT '' CHECK (length(note) <= 200),
    created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX goal_contributions_goal_idx ON goal_contributions (goal_id, made_on DESC, created_at DESC);

-- weekdays: máscara de dias (bit 0 = segunda ... bit 6 = domingo); 0 significa qualquer dia
CREATE TABLE goal_habit (
    goal_id        uuid PRIMARY KEY REFERENCES goals (id) ON DELETE CASCADE,
    household_id   uuid     NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    target_count   smallint NOT NULL CHECK (target_count BETWEEN 1 AND 31),
    period         text     NOT NULL CHECK (period IN ('week', 'month')),
    weekdays       smallint NOT NULL DEFAULT 0 CHECK (weekdays BETWEEN 0 AND 127),
    remind_min     smallint NOT NULL DEFAULT 540 CHECK (remind_min BETWEEN 0 AND 1439),
    remind_enabled boolean  NOT NULL DEFAULT false
);

CREATE TABLE goal_shared (
    goal_id       uuid PRIMARY KEY REFERENCES goals (id) ON DELETE CASCADE,
    household_id  uuid     NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    target_count  smallint NOT NULL CHECK (target_count BETWEEN 1 AND 31),
    period        text     NOT NULL CHECK (period IN ('week', 'month')),
    requires_both boolean  NOT NULL DEFAULT false,
    -- último dia em que o parceiro foi avisado de uma marcação (no máximo um aviso por meta por dia)
    notified_on   date
);

-- uma marcação por pessoa por dia
CREATE TABLE goal_checkins (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    goal_id      uuid        NOT NULL REFERENCES goals (id) ON DELETE CASCADE,
    household_id uuid        NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    user_id      uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    day          date        NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    UNIQUE (goal_id, user_id, day)
);
CREATE INDEX goal_checkins_goal_day_idx ON goal_checkins (goal_id, day);

-- Etapas de projeto. Com tarefa (want_task), o estado "concluída" vem SÓ da tarefa (done_at fica nulo);
-- sem tarefa, é um item de checklist e done_at é dela.
CREATE TABLE goal_steps (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    goal_id      uuid        NOT NULL REFERENCES goals (id) ON DELETE CASCADE,
    household_id uuid        NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    position     integer     NOT NULL,
    title        text        NOT NULL CHECK (length(btrim(title)) BETWEEN 1 AND 200),
    assignee_id  uuid        REFERENCES users (id) ON DELETE SET NULL,
    due_on       date,
    done_at      timestamptz,
    want_task    boolean     NOT NULL DEFAULT false,
    task_id      uuid,   -- sem FK: o vínculo de verdade é o ref da tarefa (goal:<meta>:step:<etapa>)
    canceled_at  timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now(),
    CHECK (NOT want_task OR done_at IS NULL)
);
CREATE INDEX goal_steps_goal_idx ON goal_steps (goal_id, position);
CREATE INDEX goal_steps_pending_task_idx ON goal_steps (household_id) WHERE want_task AND task_id IS NULL AND canceled_at IS NULL;

-- RLS: por casal em todas. A cláusula de sistema (app.system = 'on') existe porque os jobs (lembretes de hábito,
-- reconciliação de tarefas) atravessam casais e os ganchos de mudança de casal movem as metas pessoais.
DO $$
DECLARE
    t text;
BEGIN
    FOREACH t IN ARRAY ARRAY['goals', 'goal_financial', 'goal_contributions', 'goal_habit', 'goal_shared',
                             'goal_checkins', 'goal_steps'] LOOP
        EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
        EXECUTE format($p$CREATE POLICY %I ON %I
            USING (household_id = nullif(current_setting('app.household_id', true), '')::uuid
                   OR current_setting('app.system', true) = 'on')
            WITH CHECK (household_id = nullif(current_setting('app.household_id', true), '')::uuid
                        OR current_setting('app.system', true) = 'on')$p$, t || '_tenant', t);
    END LOOP;
END $$;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE goal_steps;
DROP TABLE goal_checkins;
DROP TABLE goal_shared;
DROP TABLE goal_habit;
DROP TABLE goal_contributions;
DROP TABLE goal_financial;
ALTER TABLE finance_categories DROP CONSTRAINT finance_categories_id_household_key;
DROP TABLE goals;
-- +goose StatementEnd
