-- +goose Up
-- +goose StatementBegin

-- Sair do casal e entrar em outro move linhas entre casais (o que é pessoal acompanha a pessoa), e isso só
-- é possível no escopo de sistema, como já era para lembretes e compras. Quem usa o escopo de sistema são
-- caminhos do código (db.Scope{System: true}), nunca a requisição.
ALTER POLICY notes_tenant ON notes
    USING (
        household_id = nullif(current_setting('app.household_id', true), '')::uuid
        OR current_setting('app.system', true) = 'on'
    )
    WITH CHECK (
        household_id = nullif(current_setting('app.household_id', true), '')::uuid
        OR current_setting('app.system', true) = 'on'
    );

ALTER POLICY tasks_tenant ON tasks
    USING (
        household_id = nullif(current_setting('app.household_id', true), '')::uuid
        OR current_setting('app.system', true) = 'on'
    )
    WITH CHECK (
        household_id = nullif(current_setting('app.household_id', true), '')::uuid
        OR current_setting('app.system', true) = 'on'
    );

ALTER POLICY routine_blocks_tenant ON routine_blocks
    USING (
        household_id = nullif(current_setting('app.household_id', true), '')::uuid
        OR current_setting('app.system', true) = 'on'
    )
    WITH CHECK (
        household_id = nullif(current_setting('app.household_id', true), '')::uuid
        OR current_setting('app.system', true) = 'on'
    );

ALTER POLICY routine_exceptions_tenant ON routine_exceptions
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
ALTER POLICY notes_tenant ON notes
    USING (household_id = nullif(current_setting('app.household_id', true), '')::uuid)
    WITH CHECK (household_id = nullif(current_setting('app.household_id', true), '')::uuid);
ALTER POLICY tasks_tenant ON tasks
    USING (household_id = nullif(current_setting('app.household_id', true), '')::uuid)
    WITH CHECK (household_id = nullif(current_setting('app.household_id', true), '')::uuid);
ALTER POLICY routine_blocks_tenant ON routine_blocks
    USING (household_id = nullif(current_setting('app.household_id', true), '')::uuid)
    WITH CHECK (household_id = nullif(current_setting('app.household_id', true), '')::uuid);
ALTER POLICY routine_exceptions_tenant ON routine_exceptions
    USING (household_id = nullif(current_setting('app.household_id', true), '')::uuid)
    WITH CHECK (household_id = nullif(current_setting('app.household_id', true), '')::uuid);
-- +goose StatementEnd
