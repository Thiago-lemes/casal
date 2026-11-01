# Casal

Backend do app de organização da vida a dois (Go + PostgreSQL): contas, casal com convite,
lembretes entre o casal e avisos por Web Push, mesmo com o app fechado.

- Visão e escopo do produto: [docs/produto.md](docs/produto.md)
- Como o código está organizado e as regras do projeto: [docs/arquitetura.md](docs/arquitetura.md)
- Guia passo a passo da implementação: [docs/guia-implementacao.md](docs/guia-implementacao.md)

## Requisitos

- Go 1.24 ou mais novo
- Docker (para o Postgres)
- [goose](https://github.com/pressly/goose) (`go install github.com/pressly/goose/v3/cmd/goose@latest`)
- `make` (no Windows, via Git Bash ou `winget install ezwinports.make`)

## Rodar em desenvolvimento

```bash
cp .env.example .env        # ajuste as senhas
make db-up                  # sobe o Postgres em Docker
make db-role                # cria o papel limitado casal_app (o app conecta com ele)
make migrate-up             # cria as tabelas
go run ./cmd/vapid          # gera as chaves do push; cole no .env (opcional no começo)
make run                    # http://localhost:8080
```

As chamadas de exemplo estão em [requests.http](requests.http) (extensão REST Client do VS Code).

## Configuração (`.env`)

| Variável | Para quê |
| --- | --- |
| `DATABASE_URL` | conexão do **app**, com o papel `casal_app` |
| `MIGRATE_DATABASE_URL` | conexão das **migrações**, com o dono do banco |
| `APP_DB_PASSWORD` | senha do papel `casal_app` (usada por `make db-role`) |
| `HTTP_ADDR` | endereço do servidor (padrão `:8080`) |
| `COOKIE_SECURE` | `true` (padrão) em produção, atrás de HTTPS; `false` só em desenvolvimento local |
| `TRUST_PROXY` | `true` só se estiver atrás de um proxy seu (usa `X-Forwarded-For`) |
| `SESSION_TTL_HOURS` | duração da sessão (padrão 720 = 30 dias) |
| `VAPID_PUBLIC_KEY`, `VAPID_PRIVATE_KEY`, `VAPID_SUBJECT` | Web Push; sem elas, os avisos só aparecem no log |

## Comandos

| Comando | O que faz |
| --- | --- |
| `make run` | sobe a API |
| `make test` | testes unitários (rápidos, sem banco) |
| `make test-integration` | todos os testes, no Postgres, como dono **e** como `casal_app` (com RLS) |
| `make lint` | `go vet`, `gofmt` e `staticcheck` |
| `make check` | tudo o que o CI roda |
| `make migrate-up` / `migrate-down` / `migrate-status` | migrações |

Os testes de integração criam e apagam os próprios dados no banco `casal_test`; nunca tocam no banco de desenvolvimento.

## API

Tudo sob `/v1`, JSON, autenticação por cookie de sessão.

| Método e rota | O que faz |
| --- | --- |
| `POST /auth/register`, `/auth/login` | cria conta / entra (limitado a poucas tentativas) |
| `POST /auth/logout`, `/auth/logout-all` | sai deste aparelho / de todos |
| `GET /me` | quem está logado |
| `POST /household` | cria o casal |
| `GET /household` | membros do casal |
| `POST /household/invites` | gera convite (vale 7 dias, uso único) |
| `POST /household/invites/accept` | entra no casal com o token |
| `POST /reminders`, `GET /reminders` | cria / lista lembretes |
| `POST /reminders/{id}/resolve`, `/snooze` | quem recebeu resolve ou adia |
| `DELETE /reminders/{id}` | quem criou cancela |
| `GET /push/vapid-key`, `POST/DELETE /push/subscriptions`, `POST /push/test` | Web Push |
| `GET /healthz` | saúde (fora do `/v1`) |

Erros vêm como `{"error": "mensagem"}`; cada resposta traz `X-Request-Id`, o mesmo que aparece nos logs.

## Antes de mexer no código

- `make check` precisa passar.
- Toda tabela com `household_id` precisa de política de RLS na mesma migração.
- Leia as **regras de dependência** em [docs/arquitetura.md](docs/arquitetura.md): uma feature não importa outra.
