-- +goose Up
-- +goose StatementBegin

-- O papel casal_app precisa existir antes (make db-role).
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO casal_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO casal_app;

ALTER TABLE reminders ENABLE ROW LEVEL SECURITY;

CREATE POLICY reminders_tenant ON reminders
    USING (
        household_id = nullif(current_setting('app.household_id', true), '')::uuid
        OR current_setting('app.system', true) = 'on'
    )
    WITH CHECK (
        household_id = nullif(current_setting('app.household_id', true), '')::uuid
        OR current_setting('app.system', true) = 'on'
    );

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP POLICY IF EXISTS reminders_tenant ON reminders;
ALTER TABLE reminders DISABLE ROW LEVEL SECURITY;
-- +goose StatementEnd
