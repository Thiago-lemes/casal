-- +goose Up
-- +goose StatementBegin

-- Cartões de crédito. São PESSOAIS: só o dono lê (o RLS não separa os parceiros, então o código filtra por owner_id).
-- O cartão serve para dizer onde o gasto foi feito; é sempre opcional no gasto.
CREATE TABLE finance_cards (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    household_id uuid        NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    owner_id     uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name         text        NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 40),
    name_key     text        GENERATED ALWAYS AS (lower(btrim(name))) STORED,
    brand        text        NOT NULL DEFAULT '' CHECK (length(brand) <= 20),
    last4        text        NOT NULL DEFAULT '' CHECK (last4 = '' OR last4 ~ '^[0-9]{4}$'),
    closing_day  smallint    CHECK (closing_day BETWEEN 1 AND 31),
    due_day      smallint    CHECK (due_day BETWEEN 1 AND 31),
    archived_at  timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX finance_cards_name_idx ON finance_cards (owner_id, name_key);
CREATE INDEX finance_cards_household_idx ON finance_cards (household_id);

-- o cartão do gasto: nulo = sem cartão. Cartão nunca é apagado (só arquivado), então a chave não precisa de ação.
ALTER TABLE expenses ADD COLUMN card_id uuid REFERENCES finance_cards (id) ON DELETE SET NULL;
CREATE INDEX expenses_card_idx ON expenses (card_id) WHERE card_id IS NOT NULL;

-- cláusula de sistema: o cartão acompanha a pessoa quando ela entra em outro casal (gancho OnJoin)
ALTER TABLE finance_cards ENABLE ROW LEVEL SECURITY;
CREATE POLICY finance_cards_tenant ON finance_cards
    USING (household_id = nullif(current_setting('app.household_id', true), '')::uuid
           OR current_setting('app.system', true) = 'on')
    WITH CHECK (household_id = nullif(current_setting('app.household_id', true), '')::uuid
                OR current_setting('app.system', true) = 'on');

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX expenses_card_idx;
ALTER TABLE expenses DROP COLUMN card_id;
DROP TABLE finance_cards;
-- +goose StatementEnd
