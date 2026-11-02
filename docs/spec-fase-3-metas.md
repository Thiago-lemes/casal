# Spec — Fase 3: módulo Metas (backend)

2 de novembro de 2026

Define o que construir no backend para o módulo Metas: escopo, modelo de dados, API, regras de cálculo, integrações, testes e a ordem das entregas. Parte do [produto.md](produto.md) (seção "Módulo Metas"), do que a [Fase 2](spec-fase-2-financas.md) já entregou e das regras do [arquitetura.md](arquitetura.md). O que o produto deixou em aberto está na última seção, com uma recomendação para cada ponto.

## Onde isto se encaixa

| Fase | O que é | Situação |
| --- | --- | --- |
| 1 | Base: rotina, tarefas, anotações, lembretes, compras | Backend pronto |
| 2 | Finanças | Backend pronto no branch `backend-financas` |
| **3** | **Metas: financeira, hábito, do casal e projeto** | **Backend pronto no branch `backend-financas`; frontend em [frontend-metas.md](frontend-metas.md)** |
| 4 | Marmita | Spec em [spec-fase-4-marmita.md](spec-fase-4-marmita.md) |
| 5 | Abrir para 5 a 10 casais e definir planos | A especificar |

**Saída da Fase 3 (do produto):** o casal acompanha pelo app ao menos uma meta de cada tipo por um mês.

## Escopo

**Dentro:**

- Módulo `goals` ativável por casal, usando a base de módulos da Fase 2 (`module.Require`, `OnEnable`).
- Quatro tipos de meta: **financeira**, **hábito**, **do casal** e **projeto**.
- Meta pessoal ou do casal; a pessoal é privada, salvo escolha de compartilhar (leitura).
- Prazo opcional e os estados ativa, pausada, concluída e arquivada.
- Progresso calculado pelo servidor, com histórico de marcações e de aportes.
- Meta financeira lendo os gastos de Finanças (veja Meta financeira).
- Etapas de projeto que viram tarefas, com o progresso seguindo as tarefas concluídas.
- Lembrete opcional de hábito, no horário escolhido.
- Aviso quando o parceiro conclui ou marca algo, sem ranking nem cobrança.
- Exportação das metas que a pessoa pode ver e saída/entrada de casal.

**Fora (e por quê):**

| Fora | Motivo | Quando |
| --- | --- | --- |
| Ranking, pontos, sequência com perda, comparação entre os dois | O produto diz "sem ranking nem cobrança" | Fora |
| Meta que mexe em dinheiro de verdade (débito, investimento) | O app registra, não movimenta dinheiro | Fora |
| Marcação de hábito sugerida pela rotina | Pergunta em aberto do produto; começa manual | Depois, se o uso pedir |
| Fotos e anexos de progresso | O app ainda não tem armazenamento de arquivos | Depois dos uploads |
| Metas de outras pessoas fora do casal | Não há conceito de grupo | Fora |
| Modelos prontos de meta | O catálogo começa vazio, como o de receitas | Depois |

## Premissas já decididas

Vêm do produto e das fases anteriores; esta spec não as reabre.

- Toda meta é **pessoal ou do casal**. A pessoal só o dono vê, a menos que escolha compartilhar. Como o RLS não separa os dois parceiros, isso vive só no código e é testado com dois parceiros no mesmo casal.
- Metas têm prazo opcional e podem ser **concluídas, pausadas ou arquivadas**.
- O histórico mostra o progresso, **sem ranking nem cobrança** entre os dois: nenhuma rota compara as duas pessoas, e nenhum aviso diz que o outro está atrasado.
- Hábitos geram um lembrete opcional no horário que a pessoa escolher.
- Etapas de projeto viram tarefas; o progresso acompanha as tarefas concluídas.
- A meta financeira lê os gastos de Finanças. Metas não duplicam dinheiro: o valor "guardado" é um registro de aportes, e o app **não** move dinheiro.
- Valores em centavos inteiros (`bigint`), mesmo teto de Finanças (R$ 10.000.000,00).
- Features não se importam: interfaces pequenas declaradas em quem usa, ligadas só em `internal/app`. Erros com `code` estável. RLS por casal na mesma migração. Isolamento provado removendo o filtro.

## Pré-requisitos (entrega G0)

Três mudanças pequenas em features existentes, como a F0 de Finanças.

| # | Mudança | Por quê |
| --- | --- | --- |
| 1 | `household_modules` aceita `'goals'` (migração troca o `CHECK`) e `PUT /v1/modules/goals` passa a existir | O módulo é opcional por casal |
| 2 | `task` ganha criação por processo interno com `ref` opcional, cancelamento por `ref` e leitura do estado por `ref` (coluna `tasks.ref`, com índice único parcial por casal, como `reminders.ref`) | As etapas de projeto são tarefas, e a meta precisa saber se foram concluídas sem importar `task` |
| 3 | `task.Complete` e `task.Cancel` chamam um gancho opcional `AfterChange(ctx, task)` (fora da transação) | O progresso do projeto se atualiza quando a tarefa muda, sem a meta fazer varredura |

Uma tarefa com `ref` é uma tarefa comum: aparece em Hoje e em Tarefas, pode ser concluída, recusada ou reatribuída. O `ref` só liga de volta à etapa (`goal:<goal_id>:step:<step_id>`).

## Modelo de dados

Toda tabela leva `household_id` e política de RLS na própria migração. As que o job atravessa levam a cláusula de sistema (`app.system = 'on'`). Tabelas de meta pessoal têm `owner_id`; a separação entre os parceiros é só no código.

### G1 — metas

`goals`

| Coluna | Regra |
| --- | --- |
| id, household_id, created_by | |
| owner_id | dono. Em meta do casal, é nulo |
| scope | `household` ou `personal` |
| visibility | só em meta pessoal: `private` (padrão) ou `view` (o parceiro lê, sem editar). Em meta do casal, é sempre `household` |
| kind | `financial`, `habit`, `shared` ou `project` |
| title | 1 a 120 caracteres |
| notes | até 1000 caracteres, opcional |
| status | `active` (padrão), `paused`, `done` ou `archived` |
| due_on | `date`, opcional |
| completed_at, completed_by | preenchidos ao concluir |
| version | edição a dois: gravar com versão velha dá `409` |
| created_at, updated_at | |

Restrições: `scope = 'personal'` exige `owner_id = created_by`; `scope = 'household'` exige `owner_id IS NULL` e `visibility = 'household'`. O `kind` não muda depois de criada (o modelo de progresso de cada tipo é diferente); para trocar, cria-se outra.

Cada tipo guarda os seus parâmetros em uma tabela própria, em vez de colunas soltas em `goals`, para as restrições de cada um serem checadas pelo banco.

`goal_financial(goal_id PK, target_cents, source, category_id, baseline_on)`

| Coluna | Regra |
| --- | --- |
| target_cents | valor a juntar, maior que zero |
| source | `contributions` (aportes que a pessoa registra) ou `budget_savings` (veja Meta financeira) |
| category_id | só em `budget_savings`: a categoria de Finanças acompanhada |
| baseline_on | primeiro dia a considerar |

`goal_contributions(id, goal_id, household_id, created_by, amount_cents (pode ser negativo para retirada, diferente de zero), made_on, note, created_at)`: o que já foi guardado. Retirada existe porque a reserva também é gasta. A soma nunca fica abaixo de zero (`409 contribution_negative`).

`goal_habit(goal_id PK, target_count, period, weekdays, remind_min, remind_enabled)`

| Coluna | Regra |
| --- | --- |
| target_count | quantas vezes por período, de 1 a 31 |
| period | `week` ou `month` |
| weekdays | máscara de dias, opcional; vazia significa "qualquer dia" |
| remind_min, remind_enabled | horário local do lembrete (minuto do dia) e se está ligado |

`goal_checkins(id, goal_id, household_id, user_id, day date, created_at)`, único por `(goal_id, user_id, day)`: uma marcação por pessoa por dia. Em hábito, só o dono marca. Em meta do casal, cada um marca a sua (veja Meta do casal).

`goal_shared(goal_id PK, target_count, period, requires_both)`: a meta do casal ("um jantar fora por mês"). `requires_both` define se a ocorrência conta só quando os dois marcam (`true`) ou quando qualquer um marca (`false`, padrão).

`goal_steps(id, goal_id, household_id, position, title (1 a 200), assignee_id, due_on, done_at, task_id, canceled_at)`: as etapas de um projeto. `task_id` liga à tarefa criada (veja Projeto). `position` ordena.

### G4 — lembretes de hábito

Sem tabela nova: `goal_habit.remind_min` e o `ref` do lembrete (`goal:<id>:<yyyy-mm-dd>`) bastam. Os lembretes são recalculados por job (veja Jobs).

## API

Tudo sob `/v1/goals`, exige login, casal e o módulo ligado (`403 module_disabled`). Corpos em JSON. Valores em centavos; datas `AAAA-MM-DD`; meses `AAAA-MM`.

### G1 — metas (todos os tipos)

| Rota | O que faz |
| --- | --- |
| `POST /v1/goals` | Cria. O corpo traz `kind`, `scope`, `title`, `due_on` e o bloco do tipo (`financial`, `habit`, `shared` ou `project`). Projeto aceita `steps` já na criação |
| `GET /v1/goals` | Lista com `?scope=household\|mine&kind=&status=`. `mine` traz as minhas pessoais; as pessoais do parceiro só aparecem se `view`, e nunca em `household`. Cada item traz o **progresso já calculado** |
| `GET/PUT/DELETE /v1/goals/{id}` | Lê (com o progresso e o histórico recente); substitui título, notas, prazo e visibilidade (com `version`); apaga |
| `POST /v1/goals/{id}/pause`, `/resume`, `/complete`, `/archive`, `/unarchive` | Muda o estado. Concluir grava `completed_at` e `completed_by`. Reabrir uma concluída vai por `/resume` |

### G2 — meta financeira

| Rota | O que faz |
| --- | --- |
| `POST /v1/goals/{id}/contributions` | Registra um aporte (valor positivo) ou retirada (negativo), com `made_on` e `note` |
| `GET /v1/goals/{id}/contributions` | Histórico, mais recente primeiro, paginado por `before` |
| `DELETE /v1/goals/{id}/contributions/{cid}` | Apaga um aporte (só quem o criou, ou o dono da meta) |

### G3 — hábito e meta do casal

| Rota | O que faz |
| --- | --- |
| `POST /v1/goals/{id}/checkins` com `{"day"}` | Marca o dia (padrão: hoje no fuso de quem marca). Idempotente: marcar duas vezes o mesmo dia não duplica. Não aceita data futura nem mais antiga que 31 dias |
| `DELETE /v1/goals/{id}/checkins/{day}` | Desfaz a **própria** marcação |
| `GET /v1/goals/{id}/history?from=&to=` | Marcações por dia e o resumo por período |

### G5 — projeto

| Rota | O que faz |
| --- | --- |
| `POST /v1/goals/{id}/steps` | Acrescenta uma etapa (título, responsável, prazo opcional) |
| `PUT /v1/goals/{id}/steps/{sid}` | Muda título, responsável, prazo e posição |
| `DELETE /v1/goals/{id}/steps/{sid}` | Remove a etapa e **cancela** a tarefa ligada |
| `POST /v1/goals/{id}/steps/{sid}/done` e `/undone` | Conclui e reabre, para etapa **sem tarefa** (veja Projeto) |
| `POST /v1/goals/{id}/steps/{sid}/task` e `DELETE` | Transforma a etapa em tarefa e desliga a tarefa (a etapa continua) |

### G6 — exportação

| Rota | O que faz |
| --- | --- |
| `GET /v1/goals/export` | As metas que a pessoa pode ver, com aportes, marcações e etapas, em JSON (ou CSV com `?format=csv`) |

## Regras e cálculos

### Progresso (um cálculo por tipo, só no servidor)

O cliente nunca soma nada: toda lista e leitura traz `progress` com `done`, `total`, `percent` (0 a 100, inteiro, arredondado para baixo) e, quando faz sentido, `on_track`.

| Tipo | `done` / `total` | Quando está concluída |
| --- | --- | --- |
| Financeira | soma dos aportes (ou a economia calculada) / `target_cents` | A pessoa conclui à mão; o app **sugere** quando passa de 100% |
| Hábito | marcações no período corrente / `target_count` | Hábito não conclui sozinho: é contínuo. A pessoa conclui ou arquiva quando não vale mais |
| Do casal | ocorrências válidas no período / `target_count` | Igual ao hábito |
| Projeto | etapas concluídas / etapas ativas (não canceladas) | A pessoa conclui à mão; o app **sugere** quando chega a 100% |

Concluir nunca é automático: o produto trata a meta como algo que a pessoa decide. O app avisa ("todas as etapas estão prontas") e deixa a pessoa concluir.

`percent` pode passar de 100 nos hábitos (fez mais que o combinado); a interface limita a barra, o dado fica.

### Meta financeira

Duas formas de medir, escolhidas na criação e fixas:

1. **`contributions` (padrão).** O valor guardado é a soma dos aportes registrados à mão. É a forma que não depende de nada e funciona desde o primeiro dia.
2. **`budget_savings`.** O valor guardado é o que **sobrou do orçamento** da categoria desde `baseline_on`: para cada mês fechado ou corrente, `max(0, limite − gasto do casal na categoria)`. Lê `budgets` e `expenses` do casal por uma interface (`Spending`, declarada em Metas, implementada pelo `app` em cima de `finance`). Exige o módulo Finanças ligado (`409 module_required`), uma categoria com limite definido e meta **do casal** (gasto pessoal não entra em totais do casal, e a meta pessoal não pode ler o gasto do casal em nome de um só).

Regras comuns:

- **Ritmo.** Com prazo, o servidor calcula `required_per_month`: `(target − guardado) ÷ meses inteiros que faltam`, arredondado para cima, mínimo de um mês. `on_track` é verdadeiro quando o guardado já passa da fração do prazo decorrido. Sem prazo, não há ritmo.
- **Mês corrente em `budget_savings`** conta só o que sobrou **até hoje**, e pode cair se um gasto entrar depois; por isso a resposta marca o mês corrente como `provisional`.
- **Aporte não é gasto.** Registrar um aporte **não cria** gasto em Finanças e não mexe no saldo do acerto. Quem quer ver o dinheiro "saindo" registra o gasto lá. A interface diz isso.
- **Aporte numa meta do casal** aparece aos dois com quem registrou; numa pessoal, só ao dono (e ao parceiro, se `view`, sem o histórico de aportes: só o total).

### Hábito

- O período é a **semana civil de segunda a domingo** ou o mês civil, no fuso de quem é o dono. A semana não atravessa meses para efeito de `period = 'week'`.
- `weekdays` vazio: qualquer dia vale. Com dias marcados, só marcações nesses dias contam para o progresso (marcar fora deles é permitido, aparece no histórico e não conta; assim não se perde o registro de uma sessão extra).
- **Marcação é manual** (decisão desta spec; veja Perguntas em aberto). Uma por dia.
- Pausar o hábito suspende lembretes e **não zera nada**: o histórico continua. Semanas e meses em que esteve pausado não entram como "falhas" em lugar nenhum; o produto não cobra.
- Não existe "sequência" (streak) que se perde. O histórico mostra, por período, quantas marcações houve e quanto era o combinado, em fatos e sem julgamento.

### Meta do casal

- `requires_both = false`: uma ocorrência por dia basta, de quem marcar; o progresso mostra quem marcou.
- `requires_both = true`: o dia só conta como uma ocorrência quando **os dois** marcam. Um marcar e o outro não aparece como "falta a outra pessoa", sem nome de culpado e sem aviso insistente.
- O progresso é do casal; **não há** ranking de quem marcou mais. A resposta traz as marcações por dia e por pessoa, mas a interface não as soma por pessoa.
- Quando uma pessoa marca, a outra recebe **um** push neutro ("Ana marcou: Jantar fora"), no máximo uma vez por meta por dia, e nunca para quem acabou de marcar.

### Projeto e etapas como tarefas

- Uma etapa pode ter **tarefa** ou ser só um item de checklist. Criar a tarefa da etapa chama `Tasks.Create` com `ref = goal:<goal_id>:step:<step_id>`, `assignee_id` da etapa (nulo: do casal) e `due_at` do `due_on` da etapa às 09:00 no fuso do responsável.
- **A etapa com tarefa não tem `done_at` próprio**: está concluída quando a tarefa está concluída. A etapa guarda `task_id`, e o estado é lido de `Tasks.StateByRef`. Isso evita dois lugares dizendo coisas diferentes.
- Etapa **sem** tarefa conclui por `/done` e `/undone`.
- Tarefa recusada (`declined`) deixa a etapa **aberta** e sem responsável, igual a uma tarefa recusada em qualquer lugar do app: o casal decide.
- Tarefa cancelada por fora (pela tela de Tarefas) deixa a etapa como **sem tarefa** e aberta; a etapa não some.
- Reatribuir a tarefa pela tela de Tarefas muda o responsável da etapa (lido da tarefa, não gravado de novo).
- O `AfterChange` da tarefa (G0) recalcula o progresso do projeto e, se foi a última etapa, manda um push neutro "Todas as etapas de <meta> estão prontas" aos dois (meta do casal) ou ao dono (pessoal). Não conclui a meta sozinho.
- **Meta pessoal com etapa em tarefa:** a tarefa é do dono (`assignee_id = dono`); o parceiro não vê a tarefa na tela de Tarefas se a meta é `private`. Como as tarefas hoje são sempre do casal, **meta pessoal não gera tarefa nesta fase** (veja Perguntas em aberto); suas etapas são só checklist.
- Apagar a meta cancela as tarefas ligadas (as concluídas ficam como histórico, só o `ref` é solto).

### Estados

| De → para | Regra |
| --- | --- |
| ativa → pausada | Suspende lembretes e geração de aviso; mantém tudo |
| pausada → ativa | Retoma o lembrete a partir de hoje |
| ativa ou pausada → concluída | Qualquer um que pode editar; grava quem e quando. Cancela lembretes futuros |
| concluída → ativa | Permitido (reabrir); limpa `completed_*` |
| qualquer → arquivada | Some das listas padrão (`?status=archived` as traz) |

Meta concluída ou arquivada **não aceita** novos aportes, marcações ou etapas (`409 goal_closed`).

### Edição a dois

`version` na meta, como nos gastos. Aportes e marcações são **acréscimos** e não dependem da versão. Editar o prazo ou o título com versão velha dá `409 goal_conflict`.

## Privacidade

- Meta pessoal `private` não aparece para o parceiro em lista, histórico, exportação, aviso, ou contagem de qualquer tipo. O parceiro também não deduz a existência dela (404, não 403).
- `view`: o parceiro **lê** título, prazo e **progresso**, e não vê notas, aportes individuais nem marcações dia a dia; vê só o total. Nunca edita.
- Só o dono muda a visibilidade, edita, marca, aporta e apaga uma meta pessoal.
- Nenhum aviso ao parceiro revela meta pessoal, nem `view`: avisos existem só para metas **do casal**.
- Meta financeira lendo Finanças nunca expõe gasto pessoal: só gastos do casal.
- A exportação traz apenas o que a pessoa pode ver.
- Como o RLS não separa os dois parceiros, todo SQL de leitura de meta filtra por `scope = 'household' OR owner_id = quem pergunta OR visibility = 'view'`, e o teste usa dois parceiros no mesmo casal (regra do projeto).

## Integrações entre features

Declaradas em Metas e ligadas no `app`, como em Finanças. Nenhuma feature importa outra.

| Interface (declarada em Metas) | Implementada por | Usada para |
| --- | --- | --- |
| `Tasks` (`Create` com `ref`, `CancelByRef`, `StateByRef`) | `task.Service` | Etapas de projeto |
| `Reminders` (`CreateWithRef`, `CancelByRef`) | `reminder.Service` | Lembrete de hábito |
| `People` (`Members` com fuso) | `household.Service` | Os dois e a hora local de cada um |
| `Notifier` | o mesmo notificador das tarefas | Avisos neutros de meta do casal |
| `Spending` (`BudgetSavings(household, category, from, to)`) | adaptador em `app` sobre `finance` | Meta financeira por orçamento |
| `Modules` | `module.Service` | O middleware e a checagem de Finanças ligado |

**Consistência.** Metas, aportes, marcações e etapas vivem no mesmo banco e usam transações comuns. Tarefas e lembretes são de outras features e ficam **fora** da transação: grava-se a meta, e depois cria-se a tarefa ou o lembrete; se falhar, a resposta é `200` com `sync_pending` e o job de reconciliação refaz por **recálculo** (como em Finanças).

**Rede fora de transação.** Push só depois do commit.

**Desligar Finanças** não apaga as metas `budget_savings`: o progresso delas passa a `unavailable` (campo no progresso) até o módulo voltar, e a interface explica.

## Jobs

| Job | Frequência | O que faz |
| --- | --- | --- |
| Lembretes de hábito | Hora em hora | Garante o lembrete do dia (no horário local do dono) para hábitos ativos com lembrete, **só se o dia ainda não foi marcado**; cancela o do dia quando a marcação chega |
| Reconciliação | Hora em hora | Refaz tarefas e lembretes de metas que ficaram sem eles (`sync_pending`) |

Ambos usam o escopo de sistema só nas tabelas `goals`, `goal_habit`, `goal_steps` e `goal_checkins`, e só olham casais com o módulo ligado.

O lembrete de hábito não é um por período, é um por **dia** que ainda importa: nos dias fora de `weekdays`, nenhum; depois de atingida a meta do período, nenhum até o próximo período (não se insiste em quem já cumpriu).

## Mudança de casal

Metas registram `HandleLeave` e `HandleJoin` no `household` (precisam da cláusula de sistema no RLS, como em Finanças).

**Ao sair:**

- Metas **do casal** ficam com quem permanece (continuam com os aportes e as marcações de ambos). Marcações da pessoa que saiu ficam, mas a meta passa a exigir só quem sobrou (`requires_both` vira `false` em meta do casal, para a meta não travar).
- Metas `budget_savings` do casal continuam com quem fica.
- Etapas atribuídas à pessoa que saiu passam para quem ficou; as tarefas ligadas já seguem a regra de tarefas (passam a quem ficou).
- As metas **pessoais** da pessoa (hábitos, financeiras, projetos, com aportes, marcações e etapas) a acompanham, e as `view` viram `private`.
- Lembretes de hábito da pessoa que saiu acompanham a regra de lembretes pessoais (cancelados no casal antigo e refeitos pelo job no novo).

**Ao criar ou entrar em um casal:** as metas pessoais deixadas para trás vêm junto, sempre `private`.

**Nunca apagar um casal vazio** (regra do projeto): o `CASCADE` levaria as metas pessoais de quem saiu.

O produto promete uma cópia dos dados compartilhados a quem sai: a exportação (G6) cumpre isso.

## Segurança e isolamento

- `household_id` em toda tabela e em todo `WHERE`; RLS na mesma migração. Cláusula de sistema só em `goals`, `goal_habit`, `goal_steps`, `goal_checkins` e nos movimentos de saída do casal.
- `module.Require("goals")` em todas as rotas do módulo.
- Privacidade entre os parceiros **só no código**, com `visibleTo` único em todo SQL de leitura de meta, e teste com dois parceiros no mesmo casal, provado removendo o filtro (regra do projeto).
- Ids de outro casal respondem `404`; categoria e tarefa referenciadas por id são conferidas contra o casal.
- Aportes e marcações herdam a visibilidade da meta: a consulta junta com `goals` e aplica `visibleTo`, nunca consulta a tabela filha sozinha.
- Valores com teto; listas com teto e paginação; no máximo 200 etapas por projeto; no máximo 100 metas ativas por casal.
- Marcação de hábito limitada a 31 dias para trás e proibida no futuro (evita o histórico "fabricado" e o erro de fuso).

## Erros (códigos estáveis)

| `code` | Status | Quando |
| --- | --- | --- |
| `module_disabled` | 403 | Rota de Metas com o módulo desligado |
| `module_required` | 409 | Meta `budget_savings` com Finanças desligado |
| `goal_not_found`, `step_not_found`, `contribution_not_found` | 404 | Id errado, de outro casal, ou meta pessoal que a pessoa não pode ver |
| `goal_forbidden` | 403 | Vê a meta (`view`) mas não pode alterá-la |
| `goal_conflict` | 409 | Versão velha ao editar |
| `goal_closed` | 409 | Aporte, marcação ou etapa em meta concluída ou arquivada |
| `goal_kind_mismatch` | 400 | Aporte em hábito, marcação em financeira etc. |
| `goal_limit` | 409 | Passou do teto de metas ativas ou de etapas |
| `contribution_negative` | 409 | Retirada maior que o guardado |
| `checkin_date_invalid` | 400 | Marcação no futuro ou além de 31 dias |
| `goal_invalid` | 400 | Dados inválidos (título, período, alvo, prazo no passado) |
| `amount_invalid` | 400 | Valor zero ou acima do teto |

## Testes

Cada entrega traz os seus, no padrão do projeto: unitários do service com fake; integração com o banco; isolamento entre casais provado removendo o filtro; e a rodada com o RLS ligado, com o papel sem `BYPASSRLS`.

| Entrega | Casos que não podem faltar |
| --- | --- |
| G0 | `goals` liga e desliga sem perder dados; tarefa criada e cancelada por `ref` e idempotente; `StateByRef` devolve aberta, concluída e inexistente; o gancho `AfterChange` roda fora da transação e não derruba a conclusão se falhar |
| G1 | **Privacidade com dois parceiros**: pessoal `private` não aparece em lista, leitura por id (404), exportação, aviso nem contagem; `view` lê título e progresso, sem notas nem histórico, e não edita; só o dono edita e apaga; versão velha dá `409`; meta concluída ou arquivada recusa novos registros; `kind` não muda |
| G2 | Soma dos aportes exata em centavos; retirada acima do guardado recusada; `required_per_month` com arredondamento para cima e mínimo de um mês; apagar aporte recalcula; aporte **não** cria gasto em Finanças; `budget_savings` soma só gasto do casal (pessoal nunca entra), conta o mês corrente como `provisional`, exige Finanças ligado e fica `unavailable` se desligado |
| G3 | Marcação idempotente por dia; futuro e além de 31 dias recusados; fuso de quem marca define o dia; semana de segunda a domingo e virada de mês; marcação fora de `weekdays` fica no histórico e não conta; pausar não zera; `requires_both` só conta com os dois; push neutro no máximo uma vez por dia por meta, nunca para quem marcou; **nenhuma resposta ordena ou compara as pessoas** |
| G4 | Lembrete do dia só quando falta marcar; marcar cancela o do dia; fora de `weekdays` e depois de cumprida a meta do período, nenhum; pausar cancela; horário no fuso do dono; rodar o job duas vezes não duplica |
| G5 | Etapa com tarefa fica concluída quando a tarefa conclui, e só então; tarefa recusada deixa a etapa aberta; tarefa cancelada por fora solta a etapa; apagar etapa cancela a tarefa; progresso do projeto = concluídas ÷ ativas; push "todas prontas" uma vez, sem concluir a meta; meta pessoal não cria tarefa; criar tarefa fora do ar devolve `sync_pending` e o job refaz |
| G6 | A exportação não traz o que a pessoa não pode ver; sair do casal leva as pessoais e deixa as do casal, `requires_both` vira `false`, etapas passam para quem ficou, `view` vira `private`; entrar em outro casal traz as pessoais privadas |
| Todas | Teste ponta a ponta no `app`: um casal cria uma meta de cada tipo, registra aporte, marcações e etapas, e confere progresso, avisos e privacidade |

## Entregas e ordem

Cada entrega termina com os testes verdes nos dois papéis do banco e é útil sozinha.

| # | Entrega | Depende de | Tamanho |
| --- | --- | --- | --- |
| G0 | Módulo `goals` e os pré-requisitos em tarefas | Fase 1, F0 | Pequeno |
| G1 | Metas (CRUD, estados, privacidade, progresso por tipo, `version`) | G0 | Médio a grande |
| G2 | Meta financeira: aportes, ritmo e `budget_savings` | G1, Finanças | Médio |
| G3 | Hábito e meta do casal: marcações, períodos, histórico, avisos neutros | G1 | Médio |
| G4 | Lembretes de hábito e o job | G3 | Pequeno |
| G5 | Projeto: etapas e tarefas ligadas | G0, G1 | Médio |
| G6 | Exportação e saída/entrada de casal | G1 a G5 | Médio |

A ordem põe cedo o que dá valor: com G1 e G3 o casal já acompanha hábitos e a meta do casal, que são os tipos de uso diário. G2 e G5 dependem de outras features (Finanças e Tarefas), por isso vêm depois. G6 fecha a promessa de privacidade do produto.

## Critérios de pronto da fase

- O casal cria uma meta de cada tipo e vê o progresso calculado pelo servidor, por API e pelo teste ponta a ponta.
- Meta pessoal `private` não vaza em nenhuma rota, aviso ou exportação, provado com dois parceiros no mesmo casal e removendo o filtro.
- Nenhuma resposta ou aviso compara, ordena ou cobra as duas pessoas.
- A etapa de projeto e a sua tarefa nunca discordam: o estado vem de um só lugar.
- Desligar e religar o módulo não perde dados; sem o módulo, nenhuma rota, lembrete ou aviso de Metas existe para o casal.
- Isolamento entre casais provado em todas as tabelas, nos dois papéis do banco.
- Documentação: README (rotas), `arquitetura.md` (seção de Metas: progresso, privacidade e a ligação com Tarefas e Finanças) e o doc de implementação do frontend.
- Uso real: o casal acompanha ao menos uma meta de cada tipo por um mês (a saída do produto).

## Riscos

| Risco | Efeito | Como reduzir |
| --- | --- | --- |
| Meta virar cobrança entre os dois | O produto promete "sem ranking nem cobrança"; quebrar isso afasta o casal | Nenhuma rota compara pessoas, avisos neutros e limitados, sem sequência que se perde, e teste que procura ordenação/comparação |
| Vazamento de meta pessoal | Quebra a promessa de privacidade (uma meta pode ser delicada: saúde, presente, separação de dinheiro) | `visibleTo` único, filhos sempre juntados à meta, 404 em vez de 403, teste com dois parceiros |
| Etapa e tarefa mostrando estados diferentes | Confiança perdida ("fiz, mas a meta não andou") | A etapa com tarefa não guarda estado próprio; lê da tarefa |
| Progresso financeiro inventado | Dinheiro "guardado" que não existe | Aporte é registro manual declarado como tal; `budget_savings` é calculado só de gastos do casal e marca o mês corrente como provisório |
| Lembrete de hábito insistente | A pessoa desliga todos os avisos do app | Um por dia que importa, nenhum se já marcou ou cumpriu o período, pausa respeitada |
| Hábito e fuso | Marcação cai no dia errado perto da meia-noite | O dia é o do fuso de quem marca; limites de data testados; histórico mostra o dia gravado |
| Acoplamento com Tarefas e Finanças | Uma mudança lá quebra Metas | Interfaces pequenas declaradas em Metas, adaptadores no `app`, testes de contrato nos adaptadores |
| Meta do casal travar quando um sai | A que sobrou fica com uma meta impossível | `requires_both` vira `false` na saída |

## Perguntas em aberto

O produto deixou estas sem resposta. A recomendação é o padrão que a spec já assume; muda-se antes de começar a entrega indicada.

| # | Pergunta | Recomendação | Afeta |
| --- | --- | --- | --- |
| 1 | Metas de hábito: marcação manual basta, ou o app sugere pela rotina? (do produto) | **Manual** nesta fase. Sugerir pela rotina exige casar bloco de rotina com hábito; fica para depois do uso real | G3 |
| 2 | A meta financeira lê gastos como? | Dois modos: aportes à mão (padrão) e `budget_savings` (sobra do orçamento da categoria). Não há "guardar" automático | G2 |
| 3 | Meta pessoal pode ter etapas virando tarefas? | **Não** nesta fase: tarefas são do casal e aparecem aos dois. Etapas de meta pessoal são checklist. Se o uso pedir, cria-se um tipo de tarefa pessoal antes | G5 |
| 4 | O `kind` pode mudar depois de criada? | Não: cada tipo tem seu modelo de progresso | G1 |
| 5 | Concluir é automático ao chegar a 100%? | Não: o app sugere, a pessoa conclui | G1 |
| 6 | Existe sequência (streak)? | Não: o produto diz "sem cobrança". O histórico mostra fatos por período | G3 |
| 7 | Meta do casal conta com um ou com os dois? | Configurável por meta (`requires_both`); padrão: um basta | G3 |
| 8 | Pode marcar um dia passado? | Sim, até 31 dias atrás; nunca o futuro | G3 |
| 9 | Aporte registra gasto em Finanças? | Não: aporte é registro da meta; evita mexer no saldo do acerto sem querer | G2 |
| 10 | O parceiro vê o histórico de aportes de uma meta pessoal `view`? | Não: só o total e o progresso | G1 |
| 11 | Meta tem lembrete além de hábito? | Não nesta fase. Prazo de projeto vira prazo de tarefa, que já avisa | G5 |
| 12 | Quantas metas ativas por casal? | 100 e 200 etapas por projeto, só como teto de segurança | G1 |
