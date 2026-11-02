# Casal

Backend do app de organização da vida a dois (Go + PostgreSQL): contas, casal com convite,
lembretes entre o casal e avisos por Web Push, mesmo com o app fechado.

- Visão e escopo do produto: [docs/produto.md](docs/produto.md)
- Como o código está organizado e as regras do projeto: [docs/arquitetura.md](docs/arquitetura.md)
- Como o frontend vai crescer (convenções, entregas e telas): [docs/frontend-implementacao.md](docs/frontend-implementacao.md)
- Próxima fase do backend, o módulo Finanças: [docs/spec-fase-2-financas.md](docs/spec-fase-2-financas.md)
- Depois de Finanças e Metas, o módulo Marmita: [docs/spec-fase-4-marmita.md](docs/spec-fase-4-marmita.md)
- Guia passo a passo da implementação: [docs/guia-implementacao.md](docs/guia-implementacao.md)

## Requisitos

- Go 1.24 ou mais novo
- Docker (para o Postgres)
- [goose](https://github.com/pressly/goose) (`go install github.com/pressly/goose/v3/cmd/goose@latest`)
- `make` (no Windows, via Git Bash ou `winget install ezwinports.make`)

## Rodar em desenvolvimento

### Pelo VS Code (F5)

Uma vez, na raiz do projeto:

```bash
cp .env.example .env              # ajuste as senhas
go run ./cmd/vapid                # gera as chaves do push; cole no .env (opcional no começo)
```

Depois, **F5** com a configuração **Casal: Backend + Frontend**. Ela, nesta ordem:

1. sobe o Postgres (Docker), garante o papel `casal_app` e aplica as migrações (`scripts/dev-db.mjs`);
2. compila e roda a API com o depurador, lendo o `.env` (`http://localhost:8080`);
3. sobe o Vite e, quando o backend já responde, abre o Edge com depurador (`http://localhost:5173`).

Breakpoints funcionam no Go e no TypeScript. Parar a depuração encerra tudo. O Vite repassa `/v1` para a API,
então o cookie de sessão funciona sem CORS.

Também há configurações separadas: **Backend (Go)**, **Frontend (Edge)** e **Frontend (Chrome)**.
Se o F5 não começar pelo composto, escolha-o uma vez na lista de execução; o VS Code lembra.

Requisitos extras: Docker aberto, Node 22+, `goose` e `dlv` no PATH
(`go install github.com/go-delve/delve/cmd/dlv@latest`). O front instala as dependências sozinho na primeira vez.
A porta 5173 é fixa: se outro Vite já a ocupa, o F5 falha avisando, em vez de abrir o servidor errado.

### Pelo terminal

```bash
cp .env.example .env        # ajuste as senhas
make db-up                  # sobe o Postgres em Docker
make db-role                # cria o papel limitado casal_app (o app conecta com ele)
make migrate-up             # cria as tabelas
go run ./cmd/vapid          # gera as chaves do push; cole no .env (opcional no começo)
make run                    # API em http://localhost:8080
cd frontend && npm install && npm run dev    # front em http://localhost:5173
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
| `GET /me`, `PATCH /me` | quem está logado / muda nome e fuso (nome IANA, ex.: `America/Sao_Paulo`) |
| `POST /household` | cria o casal |
| `GET /household` | membros do casal |
| `POST /household/invites` | gera convite (vale 7 dias, uso único) |
| `POST /household/invites/accept` | entra no casal com o token |
| `POST /household/leave` | sai do casal: o compartilhado fica com quem permanece, o pessoal acompanha quem sai |
| `POST /reminders`, `GET /reminders` | cria / lista lembretes |
| `POST /reminders/{id}/resolve`, `/snooze` | quem recebeu resolve ou adia |
| `DELETE /reminders/{id}` | quem criou cancela |
| `POST /tasks`, `GET /tasks?status=open\|done\|declined` | cria (para mim, para o parceiro ou do casal; com `repeat` e `rotation`) / lista |
| `POST /tasks/{id}/complete` | o responsável conclui (qualquer um, se for do casal; o criador também) |
| `POST /tasks/{id}/decline` | o responsável recusa, com justificativa obrigatória; o criador é avisado |
| `POST /tasks/{id}/assign`, `DELETE /tasks/{id}` | o criador reatribui (reabre a tarefa) / cancela |
| `POST /notes`, `GET /notes?q=&tag=`, `GET /notes/{id}` | cria / lista e busca / lê (só o que a pessoa pode ver) |
| `PUT /notes/{id}`, `DELETE /notes/{id}` | edita (com `version`; `409` se mudou) / apaga (só o dono) |
| `POST/GET /routine/blocks`, `GET/PUT/DELETE /routine/blocks/{id}` | blocos da rotina semanal (repetição, visibilidade, `version`) |
| `PUT/DELETE /routine/blocks/{id}/exceptions/{data}` | folga ou horário trocado num dia, sem mudar a rotina fixa |
| `GET /routine/occurrences?from=&to=`, `/routine/free?date=`, `/routine/now` | o que acontece, horários livres dos dois e quem está onde agora |
| `POST/GET /shopping/items`, `PUT/DELETE /shopping/items/{id}` | lista de compras do casal (itens iguais somam) |
| `POST /shopping/items/{id}/bought`, `/unbought`, `POST /shopping/restock` | marca / desfaz a compra / repõe os recorrentes |
| `GET /modules`, `PUT /modules/{name}` | módulos opcionais do casal (`finance`, `goals`): liga ou desliga; desligar esconde e não apaga |
| `GET/POST /finance/categories`, `PUT .../{id}`, `POST .../{id}/archive`, `/unarchive` | **Finanças** (exige o módulo ligado; `403 module_disabled`). Categorias: as padrão nascem ao ligar; arquivar em vez de apagar |
| `POST/GET /finance/expenses`, `GET/PUT/DELETE /finance/expenses/{id}` | gastos do casal ou pessoais, em centavos. `installments` parcela a compra (uma linha por parcela). `PUT` exige `version`; gasto acertado não muda |
| `POST /finance/expenses/from-shopping` | gasto de mercado depois de uma compra (categoria Mercado) |
| `GET/PUT/DELETE /finance/expense-groups/{id}` | uma compra parcelada: lê, muda descrição/categoria/pagador/divisão e apaga só as parcelas abertas |
| `POST/GET /finance/recurring`, `PUT/DELETE .../{id}`, `POST .../{id}/pause`, `/resume` | gastos fixos (aluguel, assinaturas): um job gera o gasto do dia, no fuso de quem paga |
| `GET/PUT /finance/settings`, `POST/DELETE /finance/settings/proportional/consent` | divisão padrão (`equal`, `fixed`, `proportional`); a proporcional exige a concordância dos dois |
| `GET/PUT/DELETE /finance/income` | a renda mensal **da própria pessoa** (privada; nenhuma rota devolve a do parceiro) |
| `GET /finance/balance?through=`, `POST/GET /finance/settlements` | saldo em aberto / registra o "pagamento entre nós" (valor exato do saldo, senão `409 settlement_stale`) / histórico |
| `POST/GET /finance/bills`, `GET/PUT/DELETE .../{id}`, `POST .../{id}/pay` | contas a pagar com lembretes (3 dias antes e no dia); pagar cria o gasto na mesma transação |
| `GET/PUT /finance/budgets`, `DELETE .../{categoria}`, `GET /finance/summary?month=&scope=` | orçamento mensal por categoria (aviso em 80% e 100%) e resumo do mês |
| `GET /finance/export?format=csv` | cópia dos gastos que a pessoa pode ver e dos acertos (JSON ou CSV) |
| `POST/GET /goals`, `GET/PUT/DELETE /goals/{id}` | **Metas** (exige o módulo `goals`; `403 module_disabled`). Quatro tipos: `financial`, `habit`, `shared` (do casal) e `project`. Pessoal ou do casal; a pessoal é privada (`404` ao parceiro) ou `view` (o parceiro lê só título, prazo e progresso). Toda resposta traz `progress` já calculado. `PUT` exige `version` |
| `POST /goals/{id}/pause`, `/resume`, `/complete`, `/archive`, `/unarchive` | estados da meta; concluir nunca é automático (o app sugere com `suggest_complete`) |
| `POST/GET /goals/{id}/contributions`, `DELETE .../{cid}` | meta financeira: aporte (ou retirada, negativa) em centavos; **não** é gasto em Finanças. Lista paginada por `before` |
| `POST /goals/{id}/checkins`, `DELETE .../{dia}`, `GET /goals/{id}/history?from=&to=` | hábito e meta do casal: marca o dia (idempotente; até 31 dias atrás, nunca o futuro), desfaz a própria marcação, histórico por dia e por período. Sem ranking nem sequência |
| `POST /goals/{id}/steps`, `PUT/DELETE .../{sid}`, `POST .../{sid}/done`, `/undone`, `POST/DELETE .../{sid}/task` | projeto: etapas (checklist ou ligadas a uma tarefa, cujo estado é o da tarefa) |
| `GET /goals/export?format=csv` | cópia das metas que a pessoa pode ver, com aportes, marcações e etapas (JSON ou CSV) |
| `GET /push/vapid-key`, `POST/DELETE /push/subscriptions`, `POST /push/test` | Web Push |
| `GET /healthz` | saúde (fora do `/v1`) |

Erros vêm como `{"error": "mensagem", "code": "..."}` (o cliente traduz pelo `code`); cada resposta traz `X-Request-Id`, o mesmo que aparece nos logs.

## Antes de mexer no código

- `make check` precisa passar.
- Toda tabela com `household_id` precisa de política de RLS na mesma migração.
- Leia as **regras de dependência** em [docs/arquitetura.md](docs/arquitetura.md): uma feature não importa outra.
