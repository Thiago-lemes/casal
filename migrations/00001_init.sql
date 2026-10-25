-- +goose Up
-- +goose StatementBegin

CREATE TABLE users (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email         text        NOT NULL,
    name          text        NOT NULL,
    password_hash text        NOT NULL,
    timezone      text        NOT NULL DEFAULT 'America/Sao_Paulo',
    created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX users_email_lower_idx ON users (lower(email));

CREATE TABLE households (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE household_members (
    household_id uuid        NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    user_id      uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    joined_at    timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (household_id, user_id),
    UNIQUE (user_id)  -- cada pessoa pertence a um casal por vez
);

CREATE TABLE household_invites (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    household_id uuid        NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    token_hash   text        NOT NULL UNIQUE,
    created_by   uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    expires_at   timestamptz NOT NULL,
    accepted_by  uuid        REFERENCES users (id) ON DELETE SET NULL,
    accepted_at  timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX household_invites_household_idx ON household_invites (household_id);

CREATE TABLE sessions (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash text        NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sessions_user_idx ON sessions (user_id);

CREATE TABLE reminders (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    household_id uuid        NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    created_by   uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    recipient_id uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    body         text        NOT NULL CHECK (length(body) BETWEEN 1 AND 500),
    remind_at    timestamptz NOT NULL,
    status       text        NOT NULL DEFAULT 'scheduled'
                 CHECK (status IN ('scheduled', 'sent', 'resolved', 'canceled')),
    sent_at      timestamptz,
    resolved_at  timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now()
);
-- o agendador procura só os pendentes, por isso o índice parcial
CREATE INDEX reminders_due_idx ON reminders (remind_at) WHERE status = 'scheduled';
CREATE INDEX reminders_household_recipient_idx ON reminders (household_id, recipient_id, status);

CREATE TABLE push_subscriptions (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    endpoint   text        NOT NULL UNIQUE,
    p256dh     text        NOT NULL,
    auth       text        NOT NULL,
    user_agent text        NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX push_subscriptions_user_idx ON push_subscriptions (user_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS push_subscriptions;
DROP TABLE IF EXISTS reminders;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS household_invites;
DROP TABLE IF EXISTS household_members;
DROP TABLE IF EXISTS households;
DROP TABLE IF EXISTS users;
-- +goose StatementEnd
