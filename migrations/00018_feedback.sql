-- +goose Up
-- +goose StatementBegin

-- Mensagens que a pessoa escreve para o time. É a única exceção ao "nunca ver conteúdo", porque a pessoa escreveu
-- aquilo para nós (a tela avisa). O casal só escreve; quem lê é o painel de suporte, pela cláusula de sistema.
CREATE TABLE feedback (
    id           uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    household_id uuid        NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    user_id      uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    message      text        NOT NULL CHECK (length(message) BETWEEN 1 AND 2000),
    screen       text        NOT NULL DEFAULT '' CHECK (screen IN (
        '', 'today', 'tasks', 'notes', 'routine', 'shopping', 'reminders', 'house', 'finance', 'goals', 'profile', 'other')),
    created_at   timestamptz NOT NULL DEFAULT now(),
    read_at      timestamptz
);
CREATE INDEX feedback_user_created_idx ON feedback (user_id, created_at DESC);
CREATE INDEX feedback_unread_idx ON feedback (created_at DESC) WHERE read_at IS NULL;

ALTER TABLE feedback ENABLE ROW LEVEL SECURITY;
CREATE POLICY feedback_tenant ON feedback
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
DROP TABLE feedback;
-- +goose StatementEnd
