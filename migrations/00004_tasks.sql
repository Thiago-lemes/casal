-- +goose Up
-- +goose StatementBegin

-- assignee_id vazio significa tarefa do casal: qualquer um dos dois pode concluir.
CREATE TABLE tasks (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    household_id   uuid        NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    created_by     uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    assignee_id    uuid        REFERENCES users (id) ON DELETE CASCADE,
    title          text        NOT NULL CHECK (length(title) BETWEEN 1 AND 200),
    due_at         timestamptz,
    status         text        NOT NULL DEFAULT 'open'
                   CHECK (status IN ('open', 'done', 'declined', 'canceled')),
    decline_reason text        CHECK (decline_reason IS NULL OR length(decline_reason) BETWEEN 1 AND 500),
    completed_by   uuid        REFERENCES users (id) ON DELETE SET NULL,
    completed_at   timestamptz,
    created_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX tasks_household_status_idx ON tasks (household_id, status, due_at);

ALTER TABLE tasks ENABLE ROW LEVEL SECURITY;

CREATE POLICY tasks_tenant ON tasks
    USING (household_id = nullif(current_setting('app.household_id', true), '')::uuid)
    WITH CHECK (household_id = nullif(current_setting('app.household_id', true), '')::uuid);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE tasks;
-- +goose StatementEnd
