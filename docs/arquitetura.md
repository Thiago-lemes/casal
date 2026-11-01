# Arquitetura

Monólito em Go com Postgres. Uma pasta por funcionalidade, sem framework: a biblioteca padrão
(`net/http`, `database/sql`, `log/slog`) e três dependências externas.

## Mapa de pastas

```
cmd/api/             ponto de entrada: lê a configuração e chama app.Run
cmd/vapid/           gera as chaves VAPID do Web Push
internal/
  app/               monta tudo: rotas, middlewares, jobs. Único lugar que conhece todas as features
  config/            variáveis de ambiente -> Config

  auth/              contas, senhas, sessões, cookie
  household/         casal (2 pessoas) e convites
  reminder/          lembretes e o agendador que os dispara
  push/              Web Push (assinaturas e envio)

  apperr/            classifica erros de negócio (inválido, não achou, conflito...) sem HTTP
  identity/          "quem está chamando": Principal{UserID, HouseholdID} no contexto
  token/             gera e faz hash de tokens (sessão, convite)
  httpx/             utilidades HTTP: JSON, Fail, Bind, middlewares, rate limit, headers
  db/                abrir o pool, transação com escopo de casal (RLS), erro de chave duplicada
  testutil/          ajudantes dos testes de integração
migrations/          SQL versionado (goose)
web/                 página de teste do PWA
```

## Dentro de cada feature

| Arquivo | Responsabilidade | Conhece HTTP? | Conhece SQL? |
| --- | --- | --- | --- |
| `handler.go` | lê a requisição, chama o service, escreve a resposta | sim | não |
| `service.go` | regras de negócio e validação | não | não |
| `repository.go` | consultas ao banco | não | sim |
| `middleware.go` | (quando existe) filtros de requisição da feature | sim | não |
| `worker.go` | (quando existe) processo em segundo plano | não | não |

O fluxo é sempre `handler -> service -> repository`, nunca ao contrário.

## Regras de dependência

1. **Uma feature não importa outra.** Quando precisa de algo de outra, declara uma interface pequena
   do que precisa (`reminder.Members`, `reminder.Notifier`) e o `app` liga as duas.
   O que é compartilhado vive em pacotes neutros: `identity`, `apperr`, `token`, `httpx`, `db`.
2. **Só `internal/app` conhece todas as features.**
3. **Services e repositories não importam `httpx`.** Erros de negócio são `apperr.Error`;
   só `httpx.Fail` os traduz em status HTTP.
4. **Interfaces ficam em quem as usa**, pequenas, e os testes trocam a implementação por um fake.

## Erros

Cada feature declara seus erros com uma classificação:

```go
var ErrNotFound = apperr.New(apperr.NotFound, "lembrete não encontrado")
```

O handler só faz `httpx.Fail(w, r, log, err)`. Erros do cliente mostram a mensagem; erros inesperados
viram `500 erro interno`, sem detalhes, e vão para o log com o `request_id`.

## Isolamento entre casais (duas travas)

1. **No código:** o `household_id` vem da sessão (`identity.Principal`), nunca do corpo ou da URL;
   toda consulta de uma feature do casal filtra por ele; recursos de outro casal respondem `404`.
2. **No banco (RLS):** o app conecta como `casal_app`, que não ignora o RLS. Cada operação roda em
   `db.InTx`, que informa o casal ao Postgres; as políticas só deixam passar as linhas dele.
   O agendador usa o escopo `System`, pois atravessa todos os casais.

**Regra de revisão:** se uma consulta não tem `household_id`, ela precisa de justificativa.
Toda tabela nova com `household_id` ganha política de RLS na mesma migração que a cria.

## Agendador de lembretes

1. `Claim` reserva os vencidos por 1 minuto (`claimed_until`) numa transação curta, com `SKIP LOCKED`.
2. O aviso é enviado **fora** de qualquer transação, com timeout de 10 s.
3. `MarkSent` conclui; só vale se o lembrete ainda estiver `scheduled` (se a pessoa resolveu, adiou ou
   cancelou no meio, essa ação prevalece).
4. Falhou? A reserva expira e outra rodada tenta de novo, até 3 vezes.

Entrega **pelo menos uma vez**: se o processo cair entre enviar e concluir, o aviso pode chegar duas vezes,
mas não se perde.

## Segurança em resumo

- Senhas: PBKDF2-SHA256, 600 mil iterações, com limite de hashes simultâneos.
- Sessões: token aleatório no cookie (`HttpOnly`, `SameSite=Lax`, `Secure` por padrão); só o hash fica no banco.
- Limite de tentativas em login (IP e e-mail) e cadastro (IP).
- Headers de segurança, timeouts do servidor e por requisição.
- Push só para hosts conhecidos (contra SSRF).
- Sessões vencidas e convites antigos são apagados a cada hora.

## Como adicionar uma funcionalidade

1. Crie `internal/<feature>/` com `repository.go`, `service.go` e `handler.go`.
2. Se tem `household_id`: migração com a tabela **e** a política de RLS; leia o casal de `identity.From`.
3. Declare os erros com `apperr`.
4. Ligue em `internal/app/app.go` (uma linha para o service, uma para as rotas).
5. Teste: unitário do service com fake; integração do isolamento (um casal não vê o outro).
