-- +goose Up
-- +goose StatementBegin

-- Módulo Finanças. Valores em centavos inteiros (bigint), moeda só BRL por enquanto.
-- Teto de R$ 10.000.000,00 por valor (1.000.000.000 centavos).

CREATE TABLE finance_categories (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    household_id uuid        NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    name         text        NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 40),
    name_key     text        GENERATED ALWAYS AS (lower(btrim(name))) STORED,
    -- chave das categorias padrão (a tela traduz por ela); nula nas criadas pelo casal
    key          text        CHECK (key IN ('housing', 'groceries', 'transport', 'leisure', 'health', 'bills', 'other')),
    archived_at  timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX finance_categories_name_idx ON finance_categories (household_id, name_key);
CREATE UNIQUE INDEX finance_categories_key_idx ON finance_categories (household_id, key) WHERE key IS NOT NULL;

-- divisão padrão do casal. Sem linha vale 'equal'.
CREATE TABLE finance_settings (
    household_id         uuid PRIMARY KEY REFERENCES households (id) ON DELETE CASCADE,
    split_mode           text        NOT NULL DEFAULT 'equal' CHECK (split_mode IN ('equal', 'fixed', 'proportional')),
    fixed_split          jsonb,
    proportional_consent jsonb       NOT NULL DEFAULT '[]',
    updated_at           timestamptz NOT NULL DEFAULT now()
);

-- Renda mensal de cada pessoa. É PRIVADA: o RLS não separa os parceiros, então só o código lê por user_id = quem pergunta
-- (a divisão proporcional lê as duas, mas só devolve a divisão resultante, nunca os valores).
CREATE TABLE finance_incomes (
    user_id              uuid PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    household_id         uuid        NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    monthly_income_cents bigint      NOT NULL CHECK (monthly_income_cents > 0 AND monthly_income_cents <= 1000000000),
    currency             text        NOT NULL DEFAULT 'BRL' CHECK (currency = 'BRL'),
    updated_at           timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX finance_incomes_household_idx ON finance_incomes (household_id);

CREATE TABLE settlements (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    household_id uuid        NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    through_on   date        NOT NULL,
    from_user    uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    to_user      uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    amount_cents bigint      NOT NULL CHECK (amount_cents > 0),
    currency     text        NOT NULL DEFAULT 'BRL' CHECK (currency = 'BRL'),
    kind         text        NOT NULL CHECK (kind IN ('manual', 'leave')),
    created_by   uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at   timestamptz NOT NULL DEFAULT now(),
    CHECK (from_user <> to_user)
);
CREATE INDEX settlements_household_idx ON settlements (household_id, created_at DESC);

CREATE TABLE recurring_expenses (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    household_id uuid        NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    created_by   uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    paid_by      uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    amount_cents bigint      NOT NULL CHECK (amount_cents > 0 AND amount_cents <= 1000000000),
    currency     text        NOT NULL DEFAULT 'BRL' CHECK (currency = 'BRL'),
    category_id  uuid        NOT NULL REFERENCES finance_categories (id),
    description  text        NOT NULL DEFAULT '' CHECK (length(description) <= 140),
    scope        text        NOT NULL CHECK (scope IN ('household', 'personal')),
    visibility   text        NOT NULL CHECK (visibility IN ('household', 'private', 'view')),
    split        jsonb,
    day_of_month smallint    NOT NULL CHECK (day_of_month BETWEEN 1 AND 31),
    start_on     date        NOT NULL,
    end_on       date,
    active       boolean     NOT NULL DEFAULT true,
    -- último dia gerado: apagar um gasto gerado não o faz reaparecer na próxima rodada do job
    last_generated_on date,
    created_at   timestamptz NOT NULL DEFAULT now(),
    CHECK (end_on IS NULL OR end_on >= start_on),
    CHECK ((scope = 'household' AND visibility = 'household' AND split IS NOT NULL)
        OR (scope = 'personal' AND visibility IN ('private', 'view') AND split IS NULL AND paid_by = created_by))
);
CREATE INDEX recurring_expenses_household_idx ON recurring_expenses (household_id);

-- expense_id ganha a chave estrangeira depois, porque expenses também aponta para bills
CREATE TABLE bills (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    household_id   uuid        NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    created_by     uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    title          text        NOT NULL CHECK (length(btrim(title)) BETWEEN 1 AND 120),
    -- opcional: a conta de luz só tem o valor depois que chega
    amount_cents   bigint      CHECK (amount_cents > 0 AND amount_cents <= 1000000000),
    currency       text        NOT NULL DEFAULT 'BRL' CHECK (currency = 'BRL'),
    due_on         date        NOT NULL,
    due_day        smallint    NOT NULL CHECK (due_day BETWEEN 1 AND 31), -- dia do primeiro vencimento, para o fim de mês curto
    responsible_id uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    category_id    uuid        NOT NULL REFERENCES finance_categories (id),
    notify_both    boolean     NOT NULL DEFAULT false,
    repeat         text        NOT NULL DEFAULT 'none' CHECK (repeat IN ('none', 'monthly')),
    paid_at        timestamptz,
    paid_by        uuid        REFERENCES users (id) ON DELETE SET NULL,
    expense_id     uuid,
    -- refs dos lembretes já tratados (veja a feature reminder), para cancelar e refazer quando a conta muda
    reminder_refs  jsonb       NOT NULL DEFAULT '[]',
    created_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX bills_household_idx ON bills (household_id, due_on);
CREATE INDEX bills_open_idx ON bills (due_on) WHERE paid_at IS NULL;

CREATE TABLE expenses (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    household_id        uuid        NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    created_by          uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    updated_by          uuid        REFERENCES users (id) ON DELETE SET NULL,
    paid_by             uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    scope               text        NOT NULL CHECK (scope IN ('household', 'personal')),
    visibility          text        NOT NULL CHECK (visibility IN ('household', 'private', 'view')),
    amount_cents        bigint      NOT NULL CHECK (amount_cents > 0 AND amount_cents <= 1000000000),
    currency            text        NOT NULL DEFAULT 'BRL' CHECK (currency = 'BRL'),
    category_id         uuid        NOT NULL REFERENCES finance_categories (id),
    spent_on            date        NOT NULL,
    description         text        NOT NULL DEFAULT '' CHECK (length(description) <= 140),
    -- {"<user_id>": pontos-base}, soma 10000; gravado em cada gasto na criação
    split               jsonb,
    source              text        NOT NULL DEFAULT 'manual' CHECK (source IN ('manual', 'shopping', 'recurring', 'bill')),
    recurring_id        uuid        REFERENCES recurring_expenses (id) ON DELETE SET NULL,
    bill_id             uuid        REFERENCES bills (id) ON DELETE SET NULL,
    installment_group_id uuid,
    installment_no      smallint,
    installments_total  smallint,
    settlement_id       uuid        REFERENCES settlements (id) ON DELETE SET NULL,
    version             integer     NOT NULL DEFAULT 1,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    -- gasto pessoal não tem divisão nem acerto e é sempre pago por quem o criou
    CHECK ((scope = 'household' AND visibility = 'household' AND split IS NOT NULL)
        OR (scope = 'personal' AND visibility IN ('private', 'view') AND split IS NULL
            AND settlement_id IS NULL AND paid_by = created_by)),
    CHECK ((installment_group_id IS NULL AND installment_no IS NULL AND installments_total IS NULL)
        OR (installment_group_id IS NOT NULL AND installment_no IS NOT NULL AND installments_total IS NOT NULL
            AND installment_no >= 1 AND installment_no <= installments_total AND installments_total <= 48))
);
CREATE UNIQUE INDEX expenses_installment_unique ON expenses (installment_group_id, installment_no)
    WHERE installment_group_id IS NOT NULL;
-- gerar o gasto fixo de novo no mesmo dia não duplica
CREATE UNIQUE INDEX expenses_recurring_unique ON expenses (recurring_id, spent_on) WHERE recurring_id IS NOT NULL;
CREATE INDEX expenses_household_date_idx ON expenses (household_id, spent_on DESC, id DESC);
CREATE INDEX expenses_open_idx ON expenses (household_id) WHERE settlement_id IS NULL AND scope = 'household';
CREATE INDEX expenses_owner_idx ON expenses (created_by) WHERE scope = 'personal';

ALTER TABLE bills ADD CONSTRAINT bills_expense_fk FOREIGN KEY (expense_id) REFERENCES expenses (id) ON DELETE SET NULL;

CREATE TABLE budgets (
    household_id uuid   NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    category_id  uuid   NOT NULL REFERENCES finance_categories (id),
    limit_cents  bigint NOT NULL CHECK (limit_cents > 0 AND limit_cents <= 1000000000),
    PRIMARY KEY (household_id, category_id)
);

-- um aviso por nível por mês, mesmo com dois celulares lançando gastos ao mesmo tempo
CREATE TABLE budget_alerts (
    household_id uuid        NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    category_id  uuid        NOT NULL REFERENCES finance_categories (id),
    month        date        NOT NULL,
    level        smallint    NOT NULL CHECK (level IN (80, 100)),
    sent_at      timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (household_id, category_id, month, level)
);

-- RLS: por casal em todas. A cláusula de sistema (app.system = 'on') só onde os jobs e os ganchos de mudança de casal
-- atravessam casais: gastos, gastos fixos, contas, acertos (acerto de saída), categorias e renda (acompanham a pessoa)
-- e a divisão padrão (volta para igual quando alguém sai).
DO $$
DECLARE
    t text;
BEGIN
    FOREACH t IN ARRAY ARRAY['budgets', 'budget_alerts'] LOOP
        EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
        EXECUTE format($p$CREATE POLICY %I ON %I
            USING (household_id = nullif(current_setting('app.household_id', true), '')::uuid)
            WITH CHECK (household_id = nullif(current_setting('app.household_id', true), '')::uuid)$p$, t || '_tenant', t);
    END LOOP;
    FOREACH t IN ARRAY ARRAY['finance_categories', 'finance_settings', 'finance_incomes', 'settlements',
                             'recurring_expenses', 'bills', 'expenses'] LOOP
        EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
        EXECUTE format($p$CREATE POLICY %I ON %I
            USING (household_id = nullif(current_setting('app.household_id', true), '')::uuid
                   OR current_setting('app.system', true) = 'on')
            WITH CHECK (household_id = nullif(current_setting('app.household_id', true), '')::uuid
                        OR current_setting('app.system', true) = 'on')$p$, t || '_tenant', t);
    END LOOP;
END $$;

-- os jobs de Finanças só olham os casais com o módulo ligado
ALTER POLICY household_modules_tenant ON household_modules
    USING (household_id = nullif(current_setting('app.household_id', true), '')::uuid
           OR current_setting('app.system', true) = 'on')
    WITH CHECK (household_id = nullif(current_setting('app.household_id', true), '')::uuid);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER POLICY household_modules_tenant ON household_modules
    USING (household_id = nullif(current_setting('app.household_id', true), '')::uuid)
    WITH CHECK (household_id = nullif(current_setting('app.household_id', true), '')::uuid);
ALTER TABLE bills DROP CONSTRAINT bills_expense_fk;
DROP TABLE budget_alerts;
DROP TABLE budgets;
DROP TABLE expenses;
DROP TABLE bills;
DROP TABLE recurring_expenses;
DROP TABLE settlements;
DROP TABLE finance_incomes;
DROP TABLE finance_settings;
DROP TABLE finance_categories;
-- +goose StatementEnd
