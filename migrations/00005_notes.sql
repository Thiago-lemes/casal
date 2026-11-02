-- +goose Up
-- +goose StatementBegin

-- visibility: 'private' só o dono vê; 'view' o parceiro lê; 'household' os dois leem e editam.
-- items (checklist) e tags são jsonb: sempre lidos e gravados junto com a nota.
-- version serve à edição a dois: gravar com uma versão velha dá conflito em vez de apagar o trabalho do outro.
CREATE TABLE notes (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    household_id uuid        NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    owner_id     uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    title        text        NOT NULL CHECK (length(title) BETWEEN 1 AND 200),
    kind         text        NOT NULL CHECK (kind IN ('text', 'checklist')),
    body         text        NOT NULL DEFAULT '' CHECK (length(body) <= 20000),
    items        jsonb       NOT NULL DEFAULT '[]' CHECK (jsonb_typeof(items) = 'array'),
    tags         jsonb       NOT NULL DEFAULT '[]' CHECK (jsonb_typeof(tags) = 'array'),
    visibility   text        NOT NULL DEFAULT 'private' CHECK (visibility IN ('private', 'view', 'household')),
    pinned       boolean     NOT NULL DEFAULT false,
    version      integer     NOT NULL DEFAULT 1,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX notes_household_updated_idx ON notes (household_id, updated_at DESC);

ALTER TABLE notes ENABLE ROW LEVEL SECURITY;

CREATE POLICY notes_tenant ON notes
    USING (household_id = nullif(current_setting('app.household_id', true), '')::uuid)
    WITH CHECK (household_id = nullif(current_setting('app.household_id', true), '')::uuid);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE notes;
-- +goose StatementEnd
