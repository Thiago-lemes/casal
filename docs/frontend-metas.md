# Casal — Implementação do Frontend: Metas (Fase 3)

2 de novembro de 2026

Como o frontend vai consumir o módulo Metas: ordem das entregas, telas, regras de interface, contrato da API e testes. O backend está **pronto** (G0 a G6 da [spec-fase-3-metas.md](spec-fase-3-metas.md)). Este documento complementa o [frontend-implementacao.md](frontend-implementacao.md) (convenções e estado atual do app), o [frontend-financas.md](frontend-financas.md) (o módulo anterior, cujo padrão se repete aqui), o [produto.md](produto.md) (o quê e por quê) e o [arquitetura.md](arquitetura.md) (o backend). As convenções valem sem exceção: pasta por feature, `api.ts` + `hooks.ts`, features não se importam, só papéis de cor, textos por i18n em `pt-BR` e `en`, erro traduzido pelo `code`.

**Saída da fase:** o casal acompanha pelo app ao menos uma meta de cada tipo por um mês.

## Princípios que esta fase acrescenta

1. **O servidor calcula todo progresso.** O cliente nunca soma aportes, marcações nem etapas: mostra `progress`. Pode limitar a barra a 100% (`percent` passa de 100 em hábito e meta do casal; o dado fica).
2. **Meta é decisão da pessoa, não do app.** Chegar a 100% nunca conclui nada: quando `progress.suggest_complete` é verdadeiro, a tela **sugere** ("Tudo pronto. Concluir a meta?") e a pessoa confirma com `POST /complete`.
3. **Sem ranking, sem cobrança, sem comparação.** Nenhuma tela ordena, soma por pessoa, compara ou destaca "quem fez mais / quem está atrasado". Não existe sequência (streak) que se perde. Semana ou mês sem marcação é um número (`0 de 3`), nunca uma falha em vermelho. A API não devolve nada para isso e a interface não deve inventar. Textos neutros: "Ana marcou", nunca "Beto ainda não marcou".
4. **Privacidade é regra de produto.** Meta pessoal é do dono; o parceiro só a enxerga se o dono escolheu `view`, e então lê apenas título, prazo e progresso. Ausência de dado não é erro: se o servidor não devolve, não existe (`notes`, `habit`, `recent`, `steps`... vêm vazios para quem só enxerga).
5. **Dinheiro é inteiro em centavos** (`*_cents`), com as mesmas funções de `features/finance/money.ts` (`formatBRL`, `parseMoney`). Como features não se importam, **mova `money.ts` e `dates.ts` para `shared/`** (o import muda, o conteúdo não) antes de começar.
6. **O módulo é opcional.** Metas só aparece quando o casal o liga (`goals`, em `shared/modules`). Desligar esconde e não apaga.

## Navegação

Metas vive dentro de **Casa**, como Finanças.

| Onde | O que entra |
| --- | --- |
| Casa | Cartão **Metas**: desligado, "Ligar Metas" (explica e diz que dá para desligar sem perder nada); ligado, abre `/goals` com um resumo (quantas ativas) |
| `/goals` | Lista em abas **Do casal · Minhas**, com filtro por tipo e por estado (ativas é o padrão; concluídas e arquivadas num filtro) |
| `/goals/:id` | Detalhe da meta; o conteúdo muda por tipo (veja as seções FM1 a FM5) |
| Hoje | Bloco **Hábitos de hoje** (hábitos ativos do próprio usuário ainda sem marcação hoje, com botão **Marcar**) e **Metas do casal de hoje** (marcar a ocorrência). Só com o módulo ligado |
| Perfil → Sair do casal | Antes de confirmar, **Baixar cópia dos dados** também inclui `/goals/export?format=csv` |

Rotas de Metas com o módulo desligado respondem `403 module_disabled`: trate como "módulo desligado" (`isModuleDisabled` de `shared/modules`, invalide `['modules']`), não como falha genérica.

## Ordem das entregas

Cada entrega termina com `npm run typecheck`, `npm test`, `npm run build` e `npm run detect` verdes e a tela rodando de ponta a ponta no F5.

| # | Entrega | Rotas | Tamanho |
| --- | --- | --- | --- |
| FM0 | Base: módulo, tipos do JSON, lista e criação (assistente por tipo), estados | `/v1/goals`, `/pause` `/resume` `/complete` `/archive` `/unarchive` | Médio |
| FM1 | Hábito: marcar, desfazer, semana/mês, histórico, lembrete | `/checkins`, `/history` | Médio |
| FM2 | Meta do casal: marcar a dois, `requires_both`, avisos | `/checkins`, `/history` | Pequeno |
| FM3 | Meta financeira: aportes, ritmo, economia do orçamento | `/contributions` | Médio |
| FM4 | Projeto: etapas, tarefas ligadas, "tudo pronto" | `/steps` | Grande |
| FM5 | Hoje, Casa e exportação / sair do casal | `/goals/export` | Pequeno |

FM0 + FM1 + FM2 já entregam o uso diário (hábito e meta do casal). FM3 e FM4 dependem de Finanças e Tarefas.

## Estrutura de pastas

```
src/features/goals/
  api.ts            tipos do JSON e uma função por rota (via api())
  hooks.ts          queryKeys + useQuery/useMutation por área
  weekdays.ts       máscara de dias <-> lista de dias (testado)
  permissions.ts    goalActions(goal) (testado)
  GoalsPage.tsx     lista
  GoalPage.tsx      detalhe (escolhe o painel pelo kind)
  create/           GoalWizard, HabitFields, SharedFields, FinancialFields, ProjectFields
  habit/            HabitPanel, WeekdayPicker, ReminderField, HistoryList
  shared/           SharedPanel (marcar, "falta a outra pessoa")
  financial/        FinancialPanel, ContributionDialog, ContributionList, PaceNote
  project/          ProjectPanel, StepRow, StepEditor
  GoalProgress.tsx  barra + texto por unidade (cents, count, steps)
shared/i18n/        namespace "goals" em pt-BR e en
```

Namespace i18n `goals`, registrado em `shared/i18n/resources.ts`. As telas que juntam Metas com outra feature (bloco em Hoje, cartão em Casa) ficam em telas de composição ou em `app/`, como em Finanças.

## Contrato do backend

Datas `AAAA-MM-DD` (nunca passe por `new Date("2026-11-15")`; veja `dates.ts` de Finanças), meses `AAAA-MM`, valores em centavos. "Hoje" vem do fuso do perfil.

### Meta (`Goal`)

```ts
type Kind = 'financial' | 'habit' | 'shared' | 'project'
type Status = 'active' | 'paused' | 'done' | 'archived'

type Goal = {
  id: string
  created_by: string
  owner_id: string | null            // nulo: meta do casal
  scope: 'household' | 'personal'
  visibility: 'household' | 'private' | 'view'
  kind: Kind
  title: string
  notes?: string                     // ausente para quem só enxerga
  status: Status
  due_on: string | null
  completed_at?: string
  completed_by?: string
  version: number                    // mande de volta no PUT
  access: 'full' | 'view'            // 'view': parceiro lendo uma meta pessoal compartilhada
  can_edit: boolean                  // use isto para mostrar ou esconder ações
  created_at: string
  updated_at: string

  financial?: { target_cents: number; source: 'contributions' | 'budget_savings'; category_id?: string; baseline_on: string }
  habit?: { target_count: number; period: 'week' | 'month'; weekdays: number; remind_min: number; remind_enabled: boolean }
  shared?: { target_count: number; period: 'week' | 'month'; requires_both: boolean }
  steps?: Step[]                     // só no detalhe de um projeto

  progress: Progress
  recent?: { contributions?: Contribution[]; checkins?: DayMarks[] }   // só no detalhe
  sync_pending?: boolean             // veja "sync_pending"
}

type Progress = {
  done: number; total: number; percent: number
  unit: 'cents' | 'count' | 'steps'
  on_track?: boolean                           // financeira com prazo
  required_per_month_cents?: number            // financeira com prazo e ainda sem bater a meta
  provisional?: boolean                        // economia do orçamento: o mês corrente ainda pode mudar
  unavailable?: boolean                        // economia do orçamento com Finanças desligado
  suggest_complete?: boolean
  period?: { start: string; end: string }      // hábito e meta do casal: o período corrente (pontas incluídas)
  marked_today?: boolean                       // quem está lendo já marcou hoje
  pending_days?: string[]                      // meta do casal que exige os dois: dias com uma só marcação
}
```

- `weekdays` é uma **máscara**: bit 0 = segunda, 1 = terça, ... 6 = domingo; `0` significa qualquer dia. `weekdays.ts` converte de e para a lista de dias do seletor (`[1,3]` ↔ `0b0000101`).
- `remind_min` é o minuto do dia (09:00 = 540) **no fuso do dono**.
- `percent` é inteiro, arredondado para baixo.

### Rotas

| Rota | Corpo / resposta |
| --- | --- |
| `POST /v1/goals` | Corpo: `{kind, scope, visibility?, title, notes?, due_on?}` mais **um** bloco do tipo (veja abaixo). `201` com a `Goal` |
| `GET /v1/goals?scope=&kind=&status=` | `scope`: `household` (do casal), `mine` (minhas pessoais) ou vazio (**tudo que a pessoa vê**, inclusive as pessoais do parceiro com `view`). `status`: vazio = não arquivadas; `all`; ou um estado. Lista de `Goal` **sem** `steps` e `recent` |
| `GET /v1/goals/{id}` | A `Goal` completa. `404 goal_not_found` se não existe, é de outro casal ou é pessoal privada do parceiro |
| `PUT /v1/goals/{id}` | `{version, title, notes?, due_on?, visibility?}` e, se quiser, um bloco do tipo com o que pode mudar (veja abaixo). `409 goal_conflict` se a versão está velha: recarregar e pedir de novo |
| `DELETE /v1/goals/{id}` | `204`. Cancela as tarefas ligadas |
| `POST /v1/goals/{id}/pause\|resume\|complete\|archive\|unarchive` | A `Goal`. Idempotente no estado em que já está. `resume` também **reabre** uma concluída |
| `POST /v1/goals/{id}/contributions` | `{amount_cents, made_on?, note?}` (negativo = retirada). `201 {contribution, goal}` |
| `GET /v1/goals/{id}/contributions?before=&limit=` | `{items: Contribution[], next_before?}`, mais recente primeiro. `next_before` ausente = acabou |
| `DELETE /v1/goals/{id}/contributions/{cid}` | A `Goal` recalculada |
| `POST /v1/goals/{id}/checkins` | `{day?}` (corpo opcional; padrão hoje). `200 {created, goal}` |
| `DELETE /v1/goals/{id}/checkins/{AAAA-MM-DD}` | Desfaz a **própria** marcação (idempotente). A `Goal` |
| `GET /v1/goals/{id}/history?from=&to=` | `{from, to, days: DayMarks[], periods: PeriodSummary[]}`; padrão: últimos 90 dias; máximo 366 |
| `POST /v1/goals/{id}/steps` | `{title, assignee_id?, due_on?, task?}`. `201` com a `Goal` (detalhe) |
| `PUT /v1/goals/{id}/steps/{sid}` | `{title?, assignee_id?, due_on?, position?}`; `""` limpa responsável e prazo. A `Goal` |
| `DELETE /v1/goals/{id}/steps/{sid}` | Remove a etapa e cancela a tarefa. A `Goal` |
| `POST /v1/goals/{id}/steps/{sid}/done\|undone` | Só etapa **sem tarefa**. A `Goal` |
| `POST /v1/goals/{id}/steps/{sid}/task`, `DELETE` | Transforma em tarefa / desliga a tarefa (a etapa continua). A `Goal` |
| `GET /v1/goals/export?format=csv` | Cópia das metas visíveis (JSON, ou CSV com `Content-Disposition: attachment`) |

**Blocos do tipo, na criação**

```ts
financial: { target_cents: number; source?: 'contributions' | 'budget_savings'; category_id?: string; baseline_on?: string }
habit:     { target_count: number; period: 'week' | 'month'; weekdays?: number; remind_enabled?: boolean; remind_min?: number }
shared:    { target_count: number; period: 'week' | 'month'; requires_both?: boolean }
steps:     { title: string; assignee_id?: string; due_on?: string; task?: boolean }[]   // projeto
```

Regras que o assistente de criação precisa conhecer (o servidor recusa com `goal_invalid`; a tela evita):

| Tipo | Escopo permitido | Observações |
| --- | --- | --- |
| `habit` | só **pessoal** (só o dono marca) | semanal: 1 a 7 vezes; mensal: 1 a 31 |
| `shared` | só **do casal** | `requires_both`: o dia só conta quando os dois marcam (padrão: um basta) |
| `financial` | pessoal ou do casal | `budget_savings` só do casal, com Finanças ligado (`409 module_required`) e uma categoria que tenha limite de orçamento |
| `project` | pessoal ou do casal | `task: true` nas etapas só no projeto do casal; etapas de projeto pessoal só podem ser do próprio dono |

**Na edição (`PUT`)**, o `kind`, `period`, `source` e `category_id` não mudam. Blocos aceitos: `financial: {target_cents}`, `habit: {target_count, weekdays, remind_enabled, remind_min?}`, `shared: {target_count, requires_both}`. Só o dono muda `visibility` (`private` ↔ `view`); meta do casal não tem visibilidade. Prazo no passado é recusado (a menos que não tenha mudado). O teto é de **100 metas ativas ou pausadas** visíveis para quem cria (`409 goal_limit`) e 200 etapas por projeto.

### Outros tipos

```ts
type Step = {
  id: string; position: number; title: string
  assignee_id: string | null       // com tarefa, é o responsável DA TAREFA (nulo se recusada)
  due_on?: string
  done: boolean; done_at?: string  // com tarefa, vem DA TAREFA
  has_task: boolean; task_id?: string
  task_status?: 'open' | 'done' | 'declined'
}
type Contribution = { id: string; amount_cents: number; made_on: string; note?: string; created_by: string; created_by_name: string; created_at: string }
type DayMarks = { day: string; by: string[]; counted: boolean }       // by: ids, em ordem sem significado
type PeriodSummary = { start: string; end: string; count: number; target: number }
```

### Códigos de erro

| `code` | Status | O que a tela faz |
| --- | --- | --- |
| `module_disabled` | 403 | Trata como módulo desligado; invalida `['modules']` |
| `module_required` | 409 | "A economia do orçamento precisa do módulo Finanças ligado" |
| `goal_not_found`, `step_not_found`, `contribution_not_found` | 404 | Volta à lista; recarrega. Nunca diz "é privada" |
| `goal_forbidden` | 403 | Esconder as ações (`can_edit`); se aparecer, mensagem "só o dono altera" |
| `goal_conflict` | 409 | Recarrega e pede para repetir a edição |
| `goal_closed` | 409 | "Reabra a meta para registrar" (concluída ou arquivada) |
| `goal_kind_mismatch` | 400 | Bug de interface: o painel certo não oferece a ação |
| `goal_limit` | 409 | "Você chegou ao limite de 100 metas ativas (ou 200 etapas)" |
| `contribution_negative` | 409 | "A retirada é maior que o valor guardado" (também ao apagar um aporte que sustenta uma retirada) |
| `checkin_date_invalid` | 400 | "Marque até hoje e no máximo 31 dias atrás" |
| `goal_invalid` | 400 | Campo a campo no formulário (título, período, alvo, prazo no passado...) |
| `amount_invalid` | 400 | Valor fora de 1 centavo a R$ 10.000.000,00 (e diferente de zero no aporte) |
| `step_has_task` | 409 | A etapa tem tarefa: conclua ou reatribua pela tarefa |

## FM0 — Base, lista e criação

- **Lista:** cada cartão mostra título, tipo (ícone + texto), `GoalProgress` e o prazo. Pausada fica esmaecida com o selo "Pausada"; `suggest_complete` mostra "Pronta para concluir". A lista **não** destaca, ordena nem compara pessoas. Quem só enxerga (`access === 'view'`) vê o cartão com o selo "Compartilhada por <nome>" e sem ações.
- **Assistente de criação:** passo 1 tipo (os quatro com uma frase cada), passo 2 de quem é (**Minha** ou **Do casal**; hábito só aceita "Minha", meta do casal só aceita "Do casal": a opção que não vale some, não fica desabilitada sem explicação), passo 3 os campos do tipo, passo 4 (só pessoal) **Quem pode ver**: "Só eu" (padrão) ou "Meu par pode ver o progresso" (`view`) com a frase do que fica visível.
- **Estados:** menu da meta com Pausar / Retomar / Concluir / Arquivar / Desarquivar conforme `status` e `can_edit`. Concluída mostra quem concluiu e quando; "Reabrir" chama `/resume`. Apagar pede confirmação e avisa que cancela as tarefas ligadas.
- **Edição a dois:** o formulário envia `version`. Em `goal_conflict`, recarrega e mostra o que mudou antes de repetir (não reenvie sozinho).
- **`goalActions(goal)`**: `{edit, pause, resume, complete, archive, unarchive, delete, contribute, mark, addStep}` a partir de `can_edit`, `status` e `kind`. Concluída ou arquivada: só reabrir/desarquivar e apagar.

## FM1 — Hábito

Painel com o período corrente (`progress.period`), a barra `done` de `total` e uma linha de dias.

- **Marcar hoje** (botão grande) chama `POST /checkins` sem corpo; é idempotente. `progress.marked_today` decide o rótulo ("Marcado hoje" com **Desfazer**). Marcar outro dia: seletor de dia dos últimos 31 dias; o servidor recusa o futuro e além de 31 dias.
- **Dias combinados (`weekdays`)**: marcar fora deles é permitido; o dia aparece no histórico com `counted: false` ("fora dos dias combinados") e **não** soma. Isso é informação, não erro.
- **Histórico** (`/history`): uma linha por período (`start` a `end`, `count` de `target`) e os dias marcados. Em fatos, sem cor de "fracasso": sem semana vermelha, sem sequência, sem "você perdeu".
- **Pausar** suspende o lembrete e não zera nada. Semana sem marcação durante a pausa não vira "falha" em lugar nenhum.
- **Lembrete** (`ReminderField`): liga/desliga e o horário (`remind_min`), na edição (`PUT` com o bloco `habit`). Explique: "um aviso por dia que ainda importa: não avisamos se você já marcou, se já cumpriu a meta da semana, ou fora dos dias combinados".
- Hábito do parceiro com `view`: só a barra e o período; sem botão, sem histórico (`GET /history` responderia `403`).

## FM2 — Meta do casal

- Cada um marca a sua; o painel mostra **as marcações do dia** (quem marcou, em ordem fixa por id, sem totais por pessoa) e o progresso do casal.
- `requires_both`: se só um marcou o dia, mostre "Falta a outra pessoa" (use `progress.pending_days`) em tom neutro, **sem nome de culpado** e sem notificação extra. O texto da configuração deve dizer: "Conta quando os dois marcam" / "Conta quando qualquer um marca".
- O parceiro recebe **um** push neutro por meta por dia ("Ana marcou: Jantar fora"). A tela não depende dele.
- Quem sai do casal: a meta continua com quem ficou e `requires_both` vira falso sozinho. Se a tela mostra o texto da configuração, ele reflete `shared.requires_both` lido do servidor.

## FM3 — Meta financeira

- **Aportes (`source = contributions`, padrão):** `FinancialPanel` mostra `done` de `total` (BRL), o ritmo e a lista de aportes (mais recente primeiro, "Ver mais" com `next_before`). **Aportar** e **Retirar** abrem o mesmo diálogo (valor positivo; o botão decide o sinal). Aviso fixo no diálogo: "Isto é só um registro da meta. Não cria gasto nem mexe no acerto de contas; para registrar o gasto, use Finanças." Em `contribution_negative`, mensagem clara; sem reenvio automático. Apagar um aporte: só quem o registrou (ou o dono). Numa meta pessoal compartilhada (`view`), o parceiro vê só o total.
- **Ritmo (meta com prazo):** `required_per_month_cents` ("Guardar R$ 333,34 por mês") e `on_track` (neutro: "No ritmo" / "Abaixo do ritmo"; sem vermelho de alarme). Sem prazo, nada disso aparece. Meta batida: some o "por mês" e aparece `suggest_complete`.
- **Economia do orçamento (`budget_savings`):** meta do casal, criada escolhendo uma **categoria com limite** (a lista vem de `/v1/finance/budgets` + categorias; se o casal não tem Finanças ligado, a opção não aparece, com link para ligar). O valor é calculado: `progress.provisional` mostra "O mês atual ainda pode mudar" ao lado do valor. `progress.unavailable` ("Disponível quando Finanças estiver ligado") com o valor escondido. Não há botão de aporte nesta forma de medir. Só gasto do casal conta, nunca o pessoal.
- A tela de Finanças e a de Metas **não** se importam: a lista de categorias com limite para o formulário vem de `shared/` ou de uma tela de composição em `app/`.

## FM4 — Projeto

- Lista de etapas em ordem (`position`), com checkbox, título, responsável e prazo. Reordenar com `PUT {position}` (comece pelos botões subir/descer; arrastar é refinamento).
- **Etapa sem tarefa** é checklist: o checkbox chama `/done` e `/undone`.
- **Etapa com tarefa** (`has_task`): o checkbox fica **desabilitado**, com o texto "Concluída na tarefa". O estado (`done`, `assignee_id`) vem da tarefa: concluir, reatribuir, recusar ou cancelar acontece na tela de **Tarefas**, e a etapa reflete na próxima leitura (invalide `['goals']` ao voltar à tela). `task_status = 'declined'`: "Recusada, sem responsável" (a etapa continua aberta). Tarefa cancelada por fora: a etapa continua, volta a ser checklist (`has_task: false`).
- **Virar tarefa** (`POST .../task`): só em projeto do casal e em etapa aberta; o botão fica fora de projeto pessoal (as tarefas são do casal, e a meta pessoal é privada). O prazo da etapa vira o prazo da tarefa às 09:00. **Desligar a tarefa** (`DELETE .../task`) cancela a tarefa e mantém a etapa (concluída, se a tarefa estava concluída).
- Etapa com tarefa: não edite o responsável por aqui (`step_has_task`); o título e o prazo podem mudar.
- **Todas as etapas prontas:** `progress.suggest_complete` mostra "Todas as etapas estão prontas. Concluir a meta?" (não conclui sozinho). O parceiro recebe um push neutro, uma vez.
- **`sync_pending`:** a meta foi gravada, mas a tarefa de alguma etapa ainda será criada (o servidor refaz em até uma hora). Mostre um aviso discreto "Criando a tarefa..." e não trate como erro. Vale para criação do projeto com `task: true`, `AddStep` e `.../task`.
- Concluída/arquivada: `goal_closed` ao mexer em etapas; a tela já esconde as ações (`goalActions`).

## FM5 — Hoje, Casa e exportação

- **Hoje:** "Hábitos de hoje" lista os hábitos ativos do próprio usuário com `progress.marked_today === false` e `Marcar`; "Metas do casal" lista as do casal ativas com o botão de marcar. Uma consulta `GET /v1/goals?status=active` basta (hábitos e metas do casal; filtre no cliente por `kind`). Nada de contador de "pendentes por pessoa".
- **Casa:** cartão com o número de metas ativas e atalho para criar.
- **Exportação:** `GET /v1/goals/export?format=csv` (CSV com colunas `record, goal_id, kind, scope, status, title, date, user_id, amount_cents, text`; uma linha por meta, aporte, marcação e etapa). Entra em **Perfil → Sair do casal** como "Baixar cópia dos dados". Só vem o que a pessoa pode ver.

## Estados de primeira classe

| Estado | Como aparece |
| --- | --- |
| Módulo desligado | Convite "Ligar Metas" em Casa e em `/goals` |
| Sem metas | Texto curto que explica os quatro tipos e botão **Criar a primeira meta** |
| Carregando | Esqueleto de cartões, não spinner de página |
| Pausada | Cartão esmaecido, selo "Pausada", sem lembrete |
| Concluída / arquivada | Filtro próprio; só reabrir e apagar |
| `access = 'view'` | Só título, prazo e progresso; sem ações, sem histórico |
| Economia do orçamento indisponível | `progress.unavailable`: mensagem para ligar Finanças, valor escondido |
| `sync_pending` | Aviso discreto, sem bloquear |

## O que cada pessoa vê (resumo de privacidade para a interface)

| Dado | Dono | Parceiro |
| --- | --- | --- |
| Meta do casal | vê e edita | vê e edita (aportes e marcações dos dois ficam visíveis aos dois) |
| Meta pessoal `private` | tudo | **nada** (`404`; não aparece em lista, exportação, contagem nem aviso) |
| Meta pessoal `view` | tudo | título, prazo e progresso; **sem** notas, aportes, marcações dia a dia, etapas, nem ações |
| Aporte de meta do casal | quem registrou (e o parceiro) vê quem registrou | vê; só apaga o que registrou |
| Aviso de marcação | só em meta **do casal**, neutro, no máximo um por meta por dia | idem |

## Testes

- **Unidade (Vitest):** `weekdays.ts` (máscara ↔ lista, domingo = bit 6), `goalActions` (todas as combinações de `status`, `kind`, `can_edit`, `scope`), formatação de `GoalProgress` por unidade, `parseMoney`/`formatBRL` (já existentes).
- **Componente (Testing Library + MSW):** cada painel com respostas fixas — hábito marcado/não marcado/fora dos dias; meta do casal com `pending_days`; financeira com e sem prazo, `provisional` e `unavailable`; projeto com etapa de checklist, com tarefa aberta/concluída/recusada, `sync_pending` e `suggest_complete`.
- **Fluxos:** criar cada tipo; marcar duas vezes o mesmo dia (uma só marcação); `goal_conflict` recarrega e não reenvia; `contribution_negative` e `checkin_date_invalid` mostram a mensagem certa; `module_disabled` esconde o módulo.
- **Privacidade:** com `access = 'view'`, nenhum botão de ação, nenhuma pergunta ao histórico; com `404`, a tela não diz que a meta "existe".
- **Neutralidade:** teste que procura nos textos `pt-BR` e `en` do namespace `goals` palavras de cobrança ("atrasad", "falhou", "perdeu", "sequência", "ranking") e falha se achar.
- **Acessibilidade:** a barra é `role="progressbar"` com `aria-valuenow`; "Marcar hoje" é botão com estado anunciado (`aria-pressed`).

## Pronto quando

- O casal liga Metas em Casa e cria uma meta de cada tipo; o progresso mostrado é sempre o do servidor.
- Marcar o mesmo dia duas vezes conta uma; o futuro e além de 31 dias são recusados com mensagem clara.
- Nenhuma tela ordena, soma por pessoa, compara ou cobra; textos neutros nos dois idiomas.
- Meta pessoal privada do parceiro não aparece em lugar nenhum; `view` mostra só o resumo.
- A etapa com tarefa nunca mostra um estado diferente do da tarefa (nunca há checkbox próprio).
- Desligar o módulo esconde tudo sem perder dados; religar traz de volta.
- Funciona no celular (barra inferior) e no desktop (trilho lateral), claro e escuro, `pt-BR` e `en`.
- `npm run typecheck`, `npm test`, `npm run build` e `npm run detect` passam.

## Decisões do backend que a interface herda

Pontos em que a implementação precisou escolher onde a spec deixava aberto; o front deve assumir o que está aqui.

| # | Decisão |
| --- | --- |
| 1 | A **semana** é sempre segunda a domingo, inteira, mesmo quando atravessa dois meses (não é cortada na virada do mês). O mês é o civil |
| 2 | `GET /v1/goals` sem `scope` devolve tudo o que a pessoa vê (do casal, as próprias e as `view` do parceiro); `scope=mine` só as próprias pessoais; `scope=household` só as do casal |
| 3 | Os mutadores (`PUT`, estados, etapas, desfazer marcação, apagar aporte) devolvem a `Goal` recalculada; aporte e marcação devolvem `{contribution, goal}` e `{created, goal}` |
| 4 | Apagar um aporte que sustenta uma retirada responde `409 contribution_negative` (o saldo nunca fica abaixo de zero) |
| 5 | Meta financeira de economia do orçamento: o limite usado é o **limite atual** da categoria em todos os meses contados; o primeiro mês é o de `baseline_on`, mas o gasto do **mês inteiro** conta (inclusive o de antes do `baseline_on`), senão a sobra do primeiro mês seria inflada. `baseline_on` não pode ser de mais de 5 anos atrás |
| 6 | O push da meta do casal só sai para marcação **de hoje**, com a meta ativa; marcar um dia passado não avisa ninguém |
| 7 | O lembrete do hábito só é criado se o horário de hoje ainda não passou (no fuso do dono); se ele muda o horário, o lembrete de hoje é refeito |
| 8 | Etapas de projeto pessoal só podem ser do dono (ou sem responsável) e nunca viram tarefa |
| 9 | `409 step_has_task` é um código a mais, além dos da spec: concluir, reabrir ou reatribuir uma etapa que tem tarefa |
| 10 | Uma tarefa com `ref` é uma tarefa comum: aparece em Hoje e em Tarefas. O JSON de tarefa ganhou o campo `ref` (opcional); a tela de Tarefas pode ignorá-lo |
| 11 | Passou o teto de metas: a contagem é das metas do casal mais as pessoais de quem cria, para não revelar as do parceiro |
| 12 | `PUT` em meta **concluída ou arquivada** responde `409 goal_closed`: reabra antes de editar (a tela esconde "Editar" nesses estados) |
| 13 | Editar título ou prazo de uma etapa que tem tarefa atualiza a tarefa também; quem recebe uma tarefa de etapa é avisado ("Nova tarefa para você"), como em qualquer tarefa atribuída |
| 14 | Reabrir ou desarquivar uma meta com o teto cheio responde `409 goal_limit` (a conta é feita no banco, sob trava) |

## Perguntas em aberto

| # | Pergunta | Recomendação |
| --- | --- | --- |
| 1 | Reordenar etapas por arrastar ou por botões? | Botões nesta fase; arrastar depois |
| 2 | O histórico do hábito vira calendário? | Lista por período nesta fase; um calendário simples (mês) se o uso pedir |
| 3 | Mostrar a meta do parceiro `view` na Hoje? | Não: Hoje é do que a pessoa faz; as do parceiro ficam na lista |
| 4 | Aviso de `sync_pending` bloqueia algo? | Não: só informa; o job refaz |
