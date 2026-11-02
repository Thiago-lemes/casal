-- +goose Up
-- +goose StatementBegin

-- G0: o módulo Metas é opcional por casal e as etapas de projeto viram tarefas.
ALTER TABLE household_modules DROP CONSTRAINT household_modules_module_check;
ALTER TABLE household_modules ADD CONSTRAINT household_modules_module_check CHECK (module IN ('finance', 'goals'));

-- Tarefa criada por outro módulo leva um ref estável (goal:<meta>:step:<etapa>). Um ref só pode estar ativo uma vez
-- por casal: criar de novo é idempotente, e cancelada libera o ref (igual a reminders.ref).
ALTER TABLE tasks ADD COLUMN ref text CHECK (ref IS NULL OR length(ref) BETWEEN 1 AND 100);
CREATE UNIQUE INDEX tasks_ref_active_idx ON tasks (household_id, ref)
    WHERE ref IS NOT NULL AND status <> 'canceled';
CREATE INDEX tasks_ref_idx ON tasks (household_id, ref) WHERE ref IS NOT NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX tasks_ref_idx;
DROP INDEX tasks_ref_active_idx;
ALTER TABLE tasks DROP COLUMN ref;
DELETE FROM household_modules WHERE module = 'goals';
ALTER TABLE household_modules DROP CONSTRAINT household_modules_module_check;
ALTER TABLE household_modules ADD CONSTRAINT household_modules_module_check CHECK (module IN ('finance'));
-- +goose StatementEnd
