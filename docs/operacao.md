# Operação do beta

Como ligar e usar o que a [spec da Fase 5](spec-fase-5-operacao.md) entrega. Tudo é opcional e vem desligado.

## Variáveis de ambiente

| Variável | Para quê |
| --- | --- |
| `SIGNUP_MODE` | `open` (padrão) ou `invite`: cadastro só com código. O `deploy/` usa `invite` |
| `ADMIN_ENABLED` | `true` liga o painel em `/admin/v1`. Desligado, tudo ali responde 404 |
| `SENTRY_DSN`, `APP_ENV`, `APP_RELEASE` | Relatório de erros do backend. No frontend: `VITE_SENTRY_DSN`, `VITE_APP_ENV`, `VITE_APP_RELEASE` (na hora do build) |
| `METRICS_TOKEN` | `/metrics` na porta principal, com `Authorization: Bearer <token>` |
| `METRICS_ADDR` | `/metrics` num endereço interno separado (ex.: `127.0.0.1:9090`). Sem um dos dois, a rota não existe |
| `OPS_DB_PASSWORD` | Senha do papel `casal_ops` (Grafana), criado por `make db-role` e por `deploy/db-init.sh` |

O Caddy do `deploy/` não expõe o `/metrics`; para o Grafana Cloud ler, rode um agente (Grafana Alloy) na mesma máquina, apontado para `METRICS_ADDR`.

## Cadastro por convite

```
go run ./cmd/admin signup-code create --note "amiga da Carla" --uses 1 --days 14
go run ./cmd/admin signup-code list
go run ./cmd/admin signup-code revoke <id>
```

No servidor: `docker compose run --rm --entrypoint admin api signup-code create ...`. O código aparece uma vez. Também dá para criar pelo painel.

## Painel de suporte

1. Crie as duas contas: `go run ./cmd/admin admin create voce@email.com`. Ele mostra a senha e o segredo do 2FA **uma única vez**; cadastre o segredo (ou o endereço `otpauth://`) no aplicativo autenticador.
2. Ligue `ADMIN_ENABLED=true` e abra `/admin`.
3. Login com e-mail, senha e o código de 6 dígitos. A sessão dura 8 h e só vale em `/admin`.

O painel mostra metadados e contagens, nunca o conteúdo dos casais. Cada leitura de ficha e cada ação grava uma linha em **Auditoria** (a tabela só aceita inserção).

## Grafana

Conecte a fonte PostgreSQL com o usuário `casal_ops` (ele só lê o schema `ops`):

| View | Pergunta |
| --- | --- |
| `ops.daily_activation` | Quantos cadastros, casais e parceiros por dia |
| `ops.weekly_active` | Pessoas e casais ativos por semana |
| `ops.module_adoption` | Casais com cada módulo ligado |
| `ops.module_usage` | Casais que criaram algo, por módulo e semana |
| `ops.retention` | Coorte semanal × semanas seguintes com atividade |
| `ops.push_coverage` | Casais com ao menos um aparelho de aviso |

Célula com menos de 3 casais (ou pessoas) vem **nula** de propósito: no começo do beta muitos valores aparecerão vazios. As regras de alerta estão em `deploy/alerts.yml`; o alerta de "app fora do ar" é do monitor externo em `/healthz`.

## Rodando tudo local

1. `make db-up db-role migrate-up` (o `.env` precisa de `OPS_DB_PASSWORD`) e, no `.env`, `ADMIN_ENABLED=true`.
2. Painel: crie a conta (`go run ./cmd/admin admin create voce@email.com`), rode `make run` e `npm run dev` em `frontend/` e abra http://localhost:5173/admin.
3. Grafana: `make grafana-up` e abra http://localhost:3000 (sem login; só escuta em 127.0.0.1). A fonte de dados `Zoog ops` e o dashboard `Zoog · beta` já vêm prontos, em `deploy/grafana/`. `make grafana-down` desliga.
4. Sentry: não há instância local. Use o DSN de um projeto no sentry.io ou no GlitchTip em `SENTRY_DSN` (backend) e `VITE_SENTRY_DSN` (frontend, em `frontend/.env.local`).

## Desvios da spec

- Resposta do código inválido é `400` com `code: signup_code_invalid` (o restante da API usa 400 para `Invalid`; o front traduz pelo `code`).
- Além de `sessions.last_used_at`, existe `user_activity` (um dia por pessoa): sozinho, o "último uso" apaga o histórico e não permite contar ativos por semana nem retenção. Também é limpa aos 13 meses.
- Criar conta de administração é só por comando, como a spec pede; a senha inicial é aleatória.
- Pessoas sem casal não aparecem na busca do painel (que parte dos casais).
- Bloquear uma conta não cancela lembretes já agendados para ela.
