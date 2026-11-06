-- +goose Up
-- +goose StatementBegin

-- Códigos de cadastro do beta (SIGNUP_MODE=invite). Não é o convite do parceiro (household_invites): este controla
-- quem pode criar conta. O código é mostrado uma vez; só o hash fica aqui. Não há household_id: o código existe antes da pessoa.
CREATE TABLE signup_codes (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    code_hash  text        NOT NULL UNIQUE,
    note       text        NOT NULL DEFAULT '' CHECK (length(note) <= 100),  -- rótulo para quem administra ("amiga da Carla")
    expires_at timestamptz NOT NULL,
    max_uses   int         NOT NULL DEFAULT 1 CHECK (max_uses BETWEEN 1 AND 100),
    uses       int         NOT NULL DEFAULT 0 CHECK (uses >= 0),
    revoked_at timestamptz,
    created_by uuid,       -- conta de administração; a chave estrangeira entra com a tabela admin_users
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (uses <= max_uses)
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE signup_codes;
-- +goose StatementEnd
