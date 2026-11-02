-- +goose Up
-- +goose StatementBegin

-- Módulos opcionais por casal (Finanças primeiro). A ausência da linha significa desligado:
-- desligar apaga só esta linha; os dados do módulo ficam e voltam quando for religado.
CREATE TABLE household_modules (
    household_id uuid        NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    module       text        NOT NULL CHECK (module IN ('finance')),
    enabled_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (household_id, module)
);

ALTER TABLE household_modules ENABLE ROW LEVEL SECURITY;

CREATE POLICY household_modules_tenant ON household_modules
    USING (household_id = nullif(current_setting('app.household_id', true), '')::uuid)
    WITH CHECK (household_id = nullif(current_setting('app.household_id', true), '')::uuid);

-- Lembretes criados por outro módulo (contas a pagar) levam um ref estável, para serem cancelados e refeitos
-- quando a conta muda. Um ref só pode estar ativo uma vez por casal: criar de novo com o mesmo ref é idempotente,
-- e cancelado libera o ref para um lembrete novo.
ALTER TABLE reminders ADD COLUMN ref text CHECK (ref IS NULL OR length(ref) BETWEEN 1 AND 100);
CREATE UNIQUE INDEX reminders_ref_active_idx ON reminders (household_id, ref)
    WHERE ref IS NOT NULL AND status <> 'canceled';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX reminders_ref_active_idx;
ALTER TABLE reminders DROP COLUMN ref;
DROP TABLE household_modules;
-- +goose StatementEnd
