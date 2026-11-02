-- +goose Up
-- +goose StatementBegin

-- Lista de compras única do casal. Tudo é dos dois: não há itens pessoais.
-- name_key é o nome sem diferença de maiúsculas e espaços; dois itens abertos iguais (mesmo nome, unidade
-- e categoria) viram um só e somam a quantidade, e o índice único garante isso mesmo com dois celulares ao mesmo tempo.
-- recurring: item que o casal sempre compra; depois de comprado, volta à lista em "repor" (restock).
-- source: de onde veio ('menu' é do módulo Marmita, que ainda não existe).
CREATE TABLE shopping_items (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    household_id uuid          NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    name         text          NOT NULL CHECK (length(name) BETWEEN 1 AND 120),
    name_key     text          GENERATED ALWAYS AS (lower(btrim(name))) STORED,
    quantity     numeric(10,3) CHECK (quantity > 0 AND quantity <= 100000),
    unit         text          NOT NULL DEFAULT '' CHECK (length(unit) <= 10),
    category     text          NOT NULL DEFAULT 'other'
                 CHECK (category IN ('produce', 'grocery', 'cleaning', 'butcher', 'other')),
    recurring    boolean       NOT NULL DEFAULT false,
    source       text          NOT NULL DEFAULT 'manual' CHECK (source IN ('manual', 'menu', 'recurring')),
    added_by     uuid          NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    bought_at    timestamptz,
    bought_by    uuid          REFERENCES users (id) ON DELETE SET NULL,
    created_at   timestamptz   NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX shopping_open_unique ON shopping_items (household_id, name_key, unit, category)
    WHERE bought_at IS NULL;
CREATE INDEX shopping_household_idx ON shopping_items (household_id, bought_at);

ALTER TABLE shopping_items ENABLE ROW LEVEL SECURITY;

-- o escopo de sistema existe para a limpeza periódica, que atravessa todos os casais
CREATE POLICY shopping_items_tenant ON shopping_items
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
DROP TABLE shopping_items;
-- +goose StatementEnd
