-- +goose Up
-- +goose StatementBegin

-- O agendador reserva o lembrete por um prazo (claimed_until) antes de enviar o aviso,
-- sem segurar transação aberta durante a chamada de rede. Se o processo cair,
-- a reserva expira e outro tenta de novo. attempts limita as repetições.
ALTER TABLE reminders
    ADD COLUMN claimed_until timestamptz,
    ADD COLUMN attempts      integer NOT NULL DEFAULT 0;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE reminders
    DROP COLUMN attempts,
    DROP COLUMN claimed_until;
-- +goose StatementEnd
