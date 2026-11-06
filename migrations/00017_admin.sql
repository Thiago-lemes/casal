-- +goose Up
-- +goose StatementBegin

-- ---- bloqueio de conta ----
-- Bloquear não apaga nada: as sessões caem, o login responde account_blocked e os dados ficam onde estão.
ALTER TABLE users ADD COLUMN blocked_at timestamptz;

-- ---- contas de administração (separadas das contas dos casais) ----
-- Não há cadastro: as contas nascem pelo comando go run ./cmd/admin admin create. Sem household_id: o admin
-- não pertence a casal nenhum, então estas tabelas não entram no RLS.
CREATE TABLE admin_users (
    id             uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    email          text        NOT NULL,
    password_hash  text        NOT NULL,
    totp_secret    text        NOT NULL,
    totp_last_step bigint      NOT NULL DEFAULT 0,  -- último passo de 30 s aceito: um código não vale duas vezes
    created_at     timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX admin_users_email_lower_idx ON admin_users (lower(email));

CREATE TABLE admin_sessions (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    admin_id   uuid        NOT NULL REFERENCES admin_users (id) ON DELETE CASCADE,
    token_hash text        NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX admin_sessions_admin_idx ON admin_sessions (admin_id);

ALTER TABLE signup_codes
    ADD CONSTRAINT signup_codes_created_by_fk FOREIGN KEY (created_by) REFERENCES admin_users (id) ON DELETE SET NULL;

-- ---- auditoria ----
-- Somente inserção: o papel do app tem INSERT e SELECT, sem UPDATE nem DELETE. admin_email guarda quem foi mesmo
-- que a conta seja removida depois (admin_id vira nulo). before/after só recebem valores estruturados (estado do
-- módulo, bloqueio), nunca texto de casal.
CREATE TABLE admin_audit (
    id           bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    occurred_at  timestamptz NOT NULL DEFAULT now(),
    admin_id     uuid        REFERENCES admin_users (id) ON DELETE SET NULL,
    admin_email  text        NOT NULL,
    action       text        NOT NULL CHECK (action IN (
        'admin_login', 'household_viewed', 'user_viewed', 'module_enabled', 'module_disabled',
        'user_blocked', 'user_unblocked', 'invite_created', 'invite_revoked', 'feedback_read')),
    target_type  text        NOT NULL CHECK (target_type IN ('admin', 'household', 'user', 'invite', 'feedback')),
    target_id    uuid,
    before       jsonb,
    after        jsonb
);
CREATE INDEX admin_audit_occurred_idx ON admin_audit (occurred_at DESC);

REVOKE UPDATE, DELETE, TRUNCATE ON admin_audit FROM casal_app;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE admin_audit;
ALTER TABLE signup_codes DROP CONSTRAINT signup_codes_created_by_fk;
DROP TABLE admin_sessions;
DROP TABLE admin_users;
ALTER TABLE users DROP COLUMN blocked_at;
-- +goose StatementEnd
