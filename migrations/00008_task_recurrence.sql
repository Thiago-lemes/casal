-- +goose Up
-- +goose StatementBegin

-- recurrence: a tarefa reaparece sozinha depois de concluída ('daily', 'weekly', 'monthly'), a partir do prazo.
-- rotation (rodízio): a cada ciclo o responsável passa para a outra pessoa do casal; exige repetição e responsável.
ALTER TABLE tasks
    ADD COLUMN recurrence text    NOT NULL DEFAULT 'none' CHECK (recurrence IN ('none', 'daily', 'weekly', 'monthly')),
    ADD COLUMN rotation   boolean NOT NULL DEFAULT false,
    ADD CONSTRAINT tasks_rotation_needs_repeat CHECK (NOT rotation OR (recurrence <> 'none' AND assignee_id IS NOT NULL));

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE tasks
    DROP CONSTRAINT tasks_rotation_needs_repeat,
    DROP COLUMN rotation,
    DROP COLUMN recurrence;
-- +goose StatementEnd
