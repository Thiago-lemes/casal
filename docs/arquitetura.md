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
  task/              tarefas do casal: atribuir, concluir, recusar com justificativa, repetir e rodízio
  note/              anotações (texto ou checklist), pessoais ou do casal, com visibilidade
  routine/           rotina semanal: blocos, repetição, exceções, horários livres, quem está onde
  shopping/          lista de compras do casal (itens iguais somam, comprados saem sozinhos)
  module/            módulos opcionais por casal (Finanças, Metas): liga, desliga e o middleware que barra rota de módulo desligado
  finance/           gastos, divisão e acerto de contas, renda, gastos fixos, contas a pagar, orçamento
  goal/              metas: financeira, hábito, do casal e projeto (progresso calculado no servidor, privacidade entre os parceiros)
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

## Itens pessoais dentro do casal

O RLS isola um casal do outro, mas **não** separa os dois parceiros: ambos têm o mesmo `household_id`.
Itens que podem ser pessoais (anotações hoje; blocos de rotina e metas depois) ganham:

- `owner_id`: quem criou, e único que apaga e que muda a visibilidade;
- `visibility`: `private` (só o dono), `view` (o parceiro lê) ou `household` (os dois leem e editam). O padrão é `private`.

Essa trava vive **só no código**: toda consulta de leitura filtra por dono ou visibilidade (inclusive listagem e busca)
e toda escrita confere a permissão no `WHERE`, além do service. Por isso o teste de integração desses itens precisa
de **dois parceiros no mesmo casal**, além de dois casais. Quem lê mas não pode alterar recebe `403`; quem nem pode ler recebe `404`.

Itens editados pelos dois (notas do casal) levam `version`: gravar com uma versão velha dá `409`, em vez de apagar o que o outro escreveu.

## Mudança de casal (sair, criar, entrar)

Sair do casal move dados entre casais, o que as features não podem fazer sozinhas nem importando umas às outras.
O `household` expõe dois ganchos (`Service.OnLeave` e `Service.OnJoin`), e o `app` registra o de cada feature
(`HandleLeave` e `HandleJoin` no repository). Os ganchos recebem um `db.Handoff` (quem, de qual casal, para qual,
e quem ficou) e rodam **na mesma transação** da mudança: se um falhar, nada muda.

| Feature | Ao sair | Ao criar casal ou entrar em um |
| --- | --- | --- |
| notas, rotina | o que era do casal passa para quem ficou; o que o parceiro lia ou via vira privado e fica com quem saiu | as da pessoa (deixadas para trás) vêm junto, **sempre privadas** |
| tarefas | as abertas e recusadas viram de quem ficou | nada (tarefas são do casal) |
| lembretes | os pendentes entre os dois são cancelados | os da pessoa para ela mesma vêm junto |
| compras | nada (é do casal) | nada |
| finanças | se há saldo em aberto, um acerto `leave` (só registro) trava os gastos abertos, inclusive parcelas futuras; fixos e contas da pessoa passam para quem ficou; a divisão padrão volta para igual; o pessoal "view" vira privado | gastos e fixos pessoais, a renda e as categorias que usavam, **sempre privados** |
| metas | as do casal ficam com quem permanece (a que exigia os dois passa a exigir só quem sobrou; as etapas da pessoa passam para quem ficou); as pessoais ficam com quem saiu e o que o parceiro lia ("view") vira privado | as pessoais deixadas para trás vêm junto, com aportes, marcações e etapas, **sempre privadas** |

Lembretes criados por outro módulo com `ref` (contas a pagar) são do casal, não da pessoa: ao sair, os pendentes dela são
cancelados (quem fica os refaz) e nenhum acompanha a pessoa a outro casal.

Quem sai fica **sem casal** (as rotas do casal respondem `403 no_household`) e pode criar outro ou aceitar um convite.
Os convites que ela criou e ninguém usou são revogados; se o casal ficou vazio, todos.

Os ganchos usam o escopo de sistema (`db.SetScope` com `System: true`), então toda tabela cujo dado pode mudar de casal
precisa da cláusula `OR current_setting('app.system', true) = 'on'` na política de RLS.

**Cuidado:** um casal vazio guarda os dados pessoais de quem saiu até a pessoa criar ou entrar em outro casal.
Como `household_id` tem `ON DELETE CASCADE`, **apagar um casal vazio apaga esses dados**. Nada no código apaga casais;
se algum dia houver limpeza de casais vazios, ela precisa antes migrar ou descartar esses dados de propósito.

## Módulos opcionais

`module.Require(svc, log, "finance")` (ou `"goals"`) barra as rotas de um módulo desligado (`403 module_disabled`). Desligar apaga só a
linha de `household_modules`: os dados ficam e voltam ao religar. O gancho `OnEnable` roda na transação que liga (as categorias
padrão de Finanças; precisa ser idempotente, pois religar também dispara) e `AfterChange` roda depois, fora da transação
(cancela ou refaz os lembretes das contas). Os jobs só olham casais com o módulo ligado.

## Dinheiro (Finanças)

- **Centavos inteiros** (`bigint`), nunca ponto flutuante. A divisão é em pontos-base (soma 10000); a parte de cada um é
  `floor(valor × pontos ÷ 10000)` e os centavos que sobram vão para quem perdeu a maior parte fracionária (empate: menor
  `user_id`). A mesma regra reparte as parcelas (os extras vão para as primeiras) e a divisão proporcional. `finance.Distribute`
  é a única implementação, com teste de propriedade.
- **Cada gasto do casal guarda o seu `split`** na criação. Mudar a divisão do casal não reescreve gastos antigos.
- **Saldo e acerto.** O saldo soma, nos gastos do casal ainda abertos (`settlement_id` nulo, `spent_on <= hoje`), o que cada um
  pagou menos a parte que lhe cabe. Parcelas futuras só entram quando a data chega. O acerto trava os gastos abertos com
  `FOR UPDATE`, recalcula e só grava se o valor enviado for exatamente o saldo (`409 settlement_stale`); dois acertos ao mesmo
  tempo se enfileiram e o segundo já não acha nada em aberto. Gasto acertado é imutável: corrige-se com um gasto novo.
- **Privacidade entre os parceiros vive só no código** (o RLS não separa os dois do mesmo casal). Todo SQL de leitura de gasto
  usa `visibleTo` (`scope = 'household' OR created_by = quem pergunta OR visibility = 'view'`); gasto pessoal nunca entra em
  saldo, resumo do casal ou orçamento. O teste usa dois parceiros no mesmo casal e foi provado removendo o filtro.
- **Renda.** Só o dono lê a própria (`finance_incomes` é consultada por `user_id = quem pergunta`). A divisão proporcional lê as
  duas dentro do service, mas só devolve a **divisão resultante**, e só vale com as duas rendas e a concordância dos dois; senão
  o gasto cai em igual com `split_fallback`. Mudar a renda avisa o parceiro sem valores.
- **Lembretes das contas** vivem na feature `reminder` (fora da transação da conta). Cada um leva um `ref` com o vencimento e a
  pessoa; mudar um dos dois muda os refs, e o sincronismo cancela os velhos e cria os novos. Se falhar, a resposta é `200` com
  `sync_pending` e o job de hora em hora recalcula.
- **Jobs.** Gastos fixos (gera o gasto do dia no fuso de quem paga; idempotente, e apagar um gerado não o recria) e
  reconciliação de lembretes.

## Metas

Módulo opcional (`goals`), com a mesma base de módulos de Finanças. Quatro tipos, cada um com os parâmetros numa tabela própria
(`goal_financial`, `goal_habit`, `goal_shared`; o projeto usa `goal_steps`), para o banco checar as regras de cada um. O `kind` não muda.

- **Progresso é calculado só no servidor** (`progress.go`, funções puras testadas): financeira = aportes (ou a sobra do orçamento) ÷ alvo;
  hábito = marcações do dono no período ÷ alvo; do casal = dias com ocorrência válida ÷ alvo; projeto = etapas concluídas ÷ ativas.
  Concluir nunca é automático: o app só sugere (`suggest_complete`). O período é a semana civil (segunda a domingo) ou o mês civil,
  no fuso do dono (na meta do casal, no de quem lê). Marcação fora de `weekdays` fica no histórico e não conta.
- **Privacidade entre os parceiros vive só no código.** Todo SQL de leitura de meta usa `visibleTo` e toda escrita confere `writableBy`
  no `WHERE`; as tabelas filhas (aportes, marcações, etapas) nunca são lidas sozinhas, sempre juntadas a `goals`. Meta pessoal `private`
  responde `404` ao parceiro (nunca `403`) e não entra em lista, exportação, contagem do teto nem aviso. Meta `view`: o parceiro recebe só
  título, prazo e progresso (`access = "view"`); sem notas, parâmetros, etapas, aportes nem marcações, e nunca edita (`403 goal_forbidden`).
  Avisos existem só para metas do casal. O teste usa dois parceiros no mesmo casal e foi provado removendo o filtro.
- **Nenhuma resposta ou aviso compara, ordena ou cobra as duas pessoas.** Não há sequência (streak), ranking nem total por pessoa; as marcações
  de um dia vêm em ordem de id, sem significado. O push da meta do casal é neutro ("Ana marcou: Jantar fora"), no máximo um por meta por dia
  (`goal_shared.notified_on`), só para marcações de hoje, e nunca para quem marcou.
- **Etapa de projeto e tarefa nunca discordam.** A etapa com tarefa (`want_task`) não tem `done_at` próprio: concluída e responsável são lidos da
  tarefa (`Tasks.StatesByRef`, por `ref = goal:<meta>:step:<etapa>`, coluna `tasks.ref` com índice único parcial por casal). Tarefa recusada
  deixa a etapa aberta e sem responsável; tarefa cancelada por fora solta a etapa (continua, aberta, sem tarefa). O gancho `task.Service.AfterChange`
  (concluir e cancelar, fora da transação, com `recover`) recalcula o projeto e manda o aviso único "todas as etapas estão prontas" (`all_done_notified_at`,
  que rearma quando uma etapa reabre). Meta pessoal não gera tarefa: suas etapas são checklist.
- **Aporte não é gasto.** Não cria nada em Finanças nem mexe no saldo do acerto. A meta `budget_savings` (só do casal) lê Finanças pela interface
  `Spending`, que o `app` liga a `finance.Service.CategoryMonthlySpend`: por mês, `max(0, limite − gasto do casal)`, e o mês corrente é `provisional`.
  Gasto pessoal nunca entra. Com Finanças desligado, o progresso vira `unavailable` e as metas ficam.
- **Tarefas e lembretes ficam fora da transação.** Grava-se a meta; depois cria-se a tarefa ou o lembrete. Se falhar, a resposta é `200` com
  `sync_pending` e os jobs de hora em hora refazem por recálculo: `ReconcileSteps` (etapas que querem tarefa e não têm) e `ReconcileHabitReminders`
  (um lembrete por **dia** que importa, com `ref = goal:<id>:<AAAA-MM-DD>`; nenhum fora de `weekdays`, depois de marcado o dia, cumprido o período,
  com a meta pausada ou com o horário já passado). Os jobs só olham casais com o módulo ligado.
- **Mudança de casal** (`HandleLeave`/`HandleJoin`): veja a tabela acima. As tabelas de Metas têm a cláusula de sistema no RLS por causa dos ganchos e dos jobs.

## Códigos de erro

`httpx.Fail` responde `{"error": "...", "code": "..."}`. O `code` é o do tipo do erro (`invalid`, `unauthorized`,
`forbidden`, `not_found`, `conflict`, `upstream`, `internal`) ou um específico, declarado com
`apperr.NewCoded(kind, "note_conflict", msg)`. O cliente traduz pelo `code` e nunca mostra a frase.
Use código próprio quando a tela precisa agir diferente (por exemplo, recarregar ao receber `note_conflict`).

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
