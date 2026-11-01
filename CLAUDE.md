# Casal (backend Go + Postgres)

Leia `docs/arquitetura.md` antes de mexer: pastas por feature, regras de dependência, erros, isolamento por casal.

## Comandos
- `make check` = lint + testes de integração (dono e `casal_app`). Precisa do Docker e de `APP_DB_PASSWORD` no `.env`.
- `make test` = só unitários. `make lint` = vet + gofmt + staticcheck.

## Regras que não são óbvias
- O app conecta como `casal_app` (RLS ligado). Testes de RLS só valem com esse papel; o dono do banco ignora o RLS.
- Toda tabela com `household_id` ganha política de RLS na mesma migração. Consulta sem `household_id` precisa de justificativa.
- Features não se importam: usam `identity.From(ctx)` e interfaces pequenas declaradas em quem consome. Só `internal/app` as liga.
- Erros de negócio são `apperr.New(kind, msg)`; o handler só chama `httpx.Fail`. Nunca devolva `err.Error()` de erro inesperado.
- Rede nunca dentro de transação (o agendador reserva com `Claim`, envia, e conclui com `MarkSent`).
- Testes de integração rodam em paralelo no mesmo banco: use `testutil.NewUser`/`NewHousehold` (dados únicos, com limpeza).
- Mudou isolamento? Prove com o teste: remova o filtro de propósito e veja o teste falhar.

## Ambiente
- Windows: o `autocrlf` do git é `true` na máquina, mas `.gitattributes` fixa LF. Não commite CRLF.
- Faça commit só dos arquivos que você mudou (evite `git add -A`): a pasta pode ter trabalho de outra frente (por exemplo `frontend/`).
