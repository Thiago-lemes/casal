-- +goose Up
-- +goose StatementBegin

-- Horários em minutos desde a meia-noite, no fuso do dono (end_min 1440 = meia-noite seguinte).
-- weekdays é uma máscara: bit 0 = segunda ... bit 6 = domingo. Repetição semanal equivale a uma regra
-- iCalendar FREQ=WEEKLY;BYDAY;INTERVAL;UNTIL ancorada em start_date; 'once' vale só em start_date.
-- visibility: 'private' o parceiro não vê; 'busy' vê só o horário; 'view' vê os detalhes;
-- 'household' bloco do casal, os dois veem e editam.
CREATE TABLE routine_blocks (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    household_id   uuid        NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    owner_id       uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    title          text        NOT NULL CHECK (length(title) BETWEEN 1 AND 120),
    category       text        NOT NULL CHECK (category IN
                   ('work', 'study', 'workout', 'commute', 'appointment', 'home', 'other')),
    location       text        NOT NULL DEFAULT '' CHECK (length(location) <= 80),
    visibility     text        NOT NULL DEFAULT 'private'
                   CHECK (visibility IN ('private', 'busy', 'view', 'household')),
    recurrence     text        NOT NULL CHECK (recurrence IN ('once', 'weekly')),
    weekdays       smallint    NOT NULL DEFAULT 0 CHECK (weekdays BETWEEN 0 AND 127),
    interval_weeks smallint    NOT NULL DEFAULT 1 CHECK (interval_weeks BETWEEN 1 AND 4),
    start_date     date        NOT NULL,
    end_date       date,
    start_min      smallint    NOT NULL CHECK (start_min BETWEEN 0 AND 1439),
    end_min        smallint    NOT NULL CHECK (end_min BETWEEN 1 AND 1440),
    version        integer     NOT NULL DEFAULT 1,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    CHECK (end_min > start_min),
    CHECK ((recurrence = 'weekly' AND weekdays > 0) OR (recurrence = 'once' AND weekdays = 0)),
    CHECK (end_date IS NULL OR (recurrence = 'weekly' AND end_date >= start_date))
);
CREATE INDEX routine_blocks_household_idx ON routine_blocks (household_id, owner_id);

-- Um dia diferente num bloco: folga (skipped) ou horário trocado. household_id repete o do bloco, para o RLS.
CREATE TABLE routine_exceptions (
    block_id     uuid     NOT NULL REFERENCES routine_blocks (id) ON DELETE CASCADE,
    household_id uuid     NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    on_date      date     NOT NULL,
    skipped      boolean  NOT NULL DEFAULT false,
    start_min    smallint CHECK (start_min BETWEEN 0 AND 1439),
    end_min      smallint CHECK (end_min BETWEEN 1 AND 1440),
    PRIMARY KEY (block_id, on_date),
    CHECK ((skipped AND start_min IS NULL AND end_min IS NULL)
        OR (NOT skipped AND start_min IS NOT NULL AND end_min IS NOT NULL AND end_min > start_min))
);
CREATE INDEX routine_exceptions_household_idx ON routine_exceptions (household_id, on_date);

ALTER TABLE routine_blocks ENABLE ROW LEVEL SECURITY;
ALTER TABLE routine_exceptions ENABLE ROW LEVEL SECURITY;

CREATE POLICY routine_blocks_tenant ON routine_blocks
    USING (household_id = nullif(current_setting('app.household_id', true), '')::uuid)
    WITH CHECK (household_id = nullif(current_setting('app.household_id', true), '')::uuid);

CREATE POLICY routine_exceptions_tenant ON routine_exceptions
    USING (household_id = nullif(current_setting('app.household_id', true), '')::uuid)
    WITH CHECK (household_id = nullif(current_setting('app.household_id', true), '')::uuid);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE routine_exceptions;
DROP TABLE routine_blocks;
-- +goose StatementEnd
