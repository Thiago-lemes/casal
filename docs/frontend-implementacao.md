# Casal — Implementação do Frontend (React + Vite + TypeScript)

1 de novembro de 2026

Este documento diz como o frontend vai crescer: o que já existe, as convenções a seguir, a ordem das entregas e o que cada tela precisa do backend. Ele complementa o [produto.md](produto.md) (o quê e por quê) e o [arquitetura.md](arquitetura.md) (o backend).

## Estado atual

PWA em React 19, Vite e TypeScript, dentro de `frontend/`. Hoje tem login e cadastro, criação ou entrada em um casal, a tela **Hoje** com lembretes (criar, resolver, adiar, cancelar), as páginas **Tarefas** (F1), **Anotações** (F2), **Semana** com a rotina (F3), **Lembretes** (F4) e **Compras** (F5), **Finanças** (FF0 a FF6, módulo opcional), o **Perfil** (nome, cidade e fuso, sair do casal), a ativação dos avisos e as preferências de tema e idioma (no menu do avatar).

| Tema | Como está hoje |
| --- | --- |
| Rotas | `react-router-dom` 7. `RequireAuth` e `RequireHousehold` protegem as telas; sem casal, aparece o `SetupPage` |
| Dados do servidor | `@tanstack/react-query`. Sem retry em erro 4xx; consulta de novo ao voltar o foco |
| HTTP | `shared/api/client.ts`: um só `api()`, cookie de sessão na mesma origem, erros viram `ApiError` com `code` |
| Estilo | CSS Modules + tokens em `design-system/tokens` (camada crua `--c-*` e papéis `--color-*`); tema claro e escuro |
| Componentes | `design-system/`: Avatar, Button, Card, ChoiceGroup, Dialog, EmptyState, Skeleton, Tabs, TextArea, TextField. `shared/ui/VisibilityPicker` (anotações e rotina). O Radix Dropdown é usado direto no `UserMenu` |
| Idiomas | `i18next`, um arquivo por namespace e idioma (`pt-BR` e `en`): `common`, `auth`, `household`, `today`, `push`, `tasks`, `profile`, `notes`, `routine`, `reminders`, `shopping`, `finance` |
| Testes | Vitest + Testing Library (jsdom) |
| Desenvolvimento | `F5` no VS Code sobe banco, API e Vite; o Vite repassa `/v1` para a API em `:8080` |
| Navegação | Hoje · Semana · Tarefas · Casa. Anotações, Lembretes, Compras e Finanças (quando ligado) ficam em **Casa** (hub de módulos); o Perfil abre pelo menu do avatar |

## Convenções (todas as telas novas seguem)

1. **Uma pasta por funcionalidade** em `src/features/<nome>/`, com `api.ts`, `hooks.ts`, os componentes e os `*.module.css`. O padrão é o de `features/reminders`.
2. **`api.ts`** declara os tipos do JSON do backend (nomes em `snake_case`, como chegam) e um objeto com uma função por rota, sempre via `api()`. Nada de `fetch` solto.
3. **`hooks.ts`** declara a `queryKey` da feature e dois hooks: um `useQuery` e um `use<Feature>Actions` com os `useMutation`. A ação invalida a chave da lista.
4. **Features não se importam**, como no backend. Quem compõe é a tela (`today/`, `home/`), que pode usar vários hooks. Código usado por mais de uma feature vai para `shared/`.
5. **Estilo só com papéis de cor** (`--color-*`) e com os tokens de espaço e tamanho. Alvo de toque mínimo `--tap` (44 px). Sem cor escrita à mão.
6. **Textos sempre por i18n**, em `pt-BR` e `en` ao mesmo tempo. Um namespace por feature, registrado em `shared/i18n/resources.ts`. Nunca mostre a frase em português que vem do servidor: use `errorMessage(t, err)`, que traduz o `code`.
7. **Erro, carregando e vazio** são estados de primeira classe: `Skeleton` ao carregar, `EmptyState` explicando o próximo passo (regra do produto), mensagem de erro com "Tentar de novo".
8. **Teste** da lógica pura (por exemplo `stats.ts`) e dos componentes com regra de permissão. Cada tela tem ao menos um teste de renderização do estado vazio e de uma ação.

## Navegação alvo

O produto prevê cinco áreas: **Hoje, Semana, Lembretes, Casa e Perfil**. Hoje o `Shell` tem três itens. A evolução proposta:

| Entrega | Barra de navegação |
| --- | --- |
| Agora | Hoje · Rotina (em breve) · Casa |
| Com tarefas | Hoje · Rotina (em breve) · **Tarefas** · Casa |
| Com a rotina | Hoje · **Semana** · Tarefas · Casa |
| Depois | Anotações e metas entram dentro de **Casa**, para a barra não passar de cinco itens no celular |

A área **Casa** vira o hub dos módulos (compras, anotações, e depois cardápio, freezer, finanças e metas). Os módulos opcionais só aparecem quando o casal os ativa.

## Ordem das entregas

A ordem segue o roadmap do produto e o que o backend já tem. Cada entrega termina com `npm run typecheck`, `npm test` e `npm run build` verdes, e a tela rodando de ponta a ponta no F5.

| # | Entrega | Backend | Situação do backend |
| --- | --- | --- | --- |
| F1 | Tarefas | `/v1/tasks` | Pronto. **Frontend feito** (inclui repetição, rodízio e o bloco "Tarefas de hoje" em Hoje) |
| F2 | Anotações | `/v1/notes` | Pronto. **Frontend feito** |
| F3 | Rotina semanal | `/v1/routine/*` | Pronto. **Frontend feito** (dia, semana, horários livres, quem está onde, blocos e exceções) |
| F4 | Lembretes em tela própria e perfil | `/v1/reminders`, `PATCH /v1/me`, `POST /v1/household/leave` | Pronto. **Frontend feito** |
| F5 | Lista de compras | `/v1/shopping/*` | Pronto. **Frontend feito** (inclui modo mercado) |
| F6 | Finanças (fase 2), Metas (fase 3), Marmita (fase 4) | Finanças pronto; Metas e Marmita a criar | Finanças: backend pronto e **frontend feito** (FF0 a FF6), veja [frontend-financas.md](frontend-financas.md). Specs em `docs/spec-fase-2-financas.md` e `docs/spec-fase-4-marmita.md` |

Da F1 à F5 o backend está pronto: veja o **Contrato do backend** mais abaixo, e o frontend também. Falta só rodar tudo junto contra o backend real (as telas foram testadas com respostas simuladas e com testes automatizados).

## F1 — Tarefas

### O que o backend oferece

| Rota | Quem pode |
| --- | --- |
| `POST /v1/tasks` com `title`, `assignee_id`, `shared`, `due_at` | Qualquer um do casal. Sem responsável e sem `shared`, é para quem criou |
| `GET /v1/tasks?status=open\|done\|declined` | Qualquer um do casal. **Um status por chamada**; não há listagem de canceladas |
| `POST /v1/tasks/{id}/complete` | O responsável; qualquer um se a tarefa for do casal; o criador também |
| `POST /v1/tasks/{id}/decline` com `reason` | Só o responsável, e não a própria tarefa; justificativa obrigatória (até 500 caracteres) |
| `POST /v1/tasks/{id}/assign` com `assignee_id` ou `shared` | Só o criador; reabre a tarefa e apaga a justificativa |
| `DELETE /v1/tasks/{id}` | Só o criador |

Formato da tarefa: `id`, `created_by`, `created_by_name`, `assignee_id` (nulo = tarefa do casal), `title`, `due_at?`, `status` (`open`, `done`, `declined`, `canceled`), `decline_reason?`, `completed_by?`, `completed_at?`, `created_at`.

Limites: título de 1 a 200 caracteres; lista com até 200 itens.

### Telas e componentes

**Página `/tasks`** com três abas: **Abertas**, **Recusadas** e **Concluídas**. Cada aba é uma consulta (`status=`), carregada só quando a aba é aberta.

- **Abertas:** tarefas com prazo primeiro, depois as sem prazo. Cada linha mostra título, responsável (avatar e nome, ou "do casal"), prazo e o botão de concluir.
- **Recusadas:** só o que exige decisão do criador. Mostra a justificativa em destaque e as ações **Assumir**, **Passar para outra pessoa** e **Cancelar**. Para quem não é o criador, a aba é só leitura.
- **Concluídas:** mais recentes primeiro, com quem concluiu e quando.
- **Criar tarefa:** botão fixo, abre um diálogo (mesmo padrão do `Composer`). Campos: título, para quem (Eu, o nome do parceiro ou **Do casal**) e prazo opcional.
- **Recusar:** diálogo com campo de justificativa obrigatório e contador de caracteres. O botão só habilita com texto.
- **Tela Hoje:** ganha um bloco "Tarefas de hoje" com as tarefas abertas que vencem hoje ou estão sem prazo e são minhas ou do casal. Fica para depois da página própria.

Componentes novos:

| Componente | Onde | Observação |
| --- | --- | --- |
| `Dialog` | `design-system/Dialog` | Extrair do `Composer` (Radix Dialog) para reutilizar em criar e recusar |
| `Tabs` | `design-system/Tabs` | Acessível (`role="tablist"`), navegável pelo teclado |
| `TaskRow` | `features/tasks` | A linha; recebe a tarefa e o usuário e decide quais ações mostrar |
| `TaskComposer` | `features/tasks` | Diálogo de criação |
| `DeclineDialog` | `features/tasks` | Diálogo de recusa |
| `AssigneeSelect` | `features/tasks` | Eu, parceiro ou do casal; usado em criar e reatribuir |

### Regras de permissão na interface

A tela **espelha** as regras do backend para esconder o botão que falharia. O backend continua sendo a autoridade: se a regra mudar lá, a tela só perde um botão, nunca abre uma brecha. Uma função pura `taskActions(task, meId)` concentra isso e tem teste próprio.

| Ação | Aparece quando |
| --- | --- |
| Concluir | `status = open` e (`assignee_id` nulo, ou eu sou o responsável, ou eu sou o criador) |
| Recusar | `status = open` e eu sou o responsável e **não** sou o criador |
| Assumir e Passar para outra pessoa | eu sou o criador e `status` é `open` ou `declined` |
| Cancelar | eu sou o criador e `status` é `open` ou `declined` |

### Dados

```ts
// features/tasks/api.ts
export type TaskStatus = 'open' | 'done' | 'declined' | 'canceled'
export type Task = {
  id: string
  created_by: string
  created_by_name: string
  assignee_id: string | null
  title: string
  due_at?: string
  status: TaskStatus
  decline_reason?: string
  completed_by?: string
  completed_at?: string
  created_at: string
}
```

- `queryKey`: `['tasks', status]`, para cada aba ter o seu cache. Toda ação invalida `['tasks']` inteira, porque uma tarefa muda de aba.
- **Concluir** pode ser otimista (a linha some da aba Abertas e volta se falhar). **Recusar, reatribuir e cancelar** esperam o servidor, porque dependem de regra e justificativa.
- A tela precisa do parceiro para `AssigneeSelect`: vem de `useHousehold().data.members`, como na tela Hoje.
- `refetchInterval` de 60 s na aba Abertas, como nos lembretes. Atualização em tempo real fica fora desta fase.

### Textos e erros

Namespace novo `tasks` em `pt-BR` e `en`. O backend agora envia `code` em todo erro (veja o **Contrato do backend**): "tarefa não encontrada" (id errado, outro casal, sem permissão) chega como `404 task_not_found`. A interface mostra uma mensagem única ("Essa tarefa mudou ou não está mais disponível. Atualize a lista") e recarrega a lista. Falta de justificativa na recusa e prazo inválido chegam como `400 invalid`.

### Testes

- `taskActions`: uma linha da tabela de permissões por caso, incluindo criador igual ao responsável e tarefa do casal.
- `TaskRow`: mostra só os botões certos para cada combinação.
- `DeclineDialog`: botão desabilitado sem justificativa; envia o texto sem espaços nas pontas.
- Página: estado vazio por aba, e concluir tira a linha de Abertas.
- `api.ts`: o corpo e a rota de cada chamada, no mesmo estilo de `client.test.ts`.

### Pronto quando

- Criar para mim, para o parceiro e do casal funciona; o parceiro recebe o push de "Nova tarefa para você" (o backend já envia).
- O parceiro recusa com justificativa e o criador a vê na aba Recusadas, com as três ações.
- Nenhum botão aparece para quem não tem permissão.
- Funciona no celular (barra inferior) e no desktop (trilho lateral), claro e escuro, `pt-BR` e `en`.
- `npm run typecheck`, `npm test`, `npm run build` e `npm run detect` passam.

## Itens transversais

**Fuso horário.** `GET /v1/me` já devolve `timezone` e a tela Hoje o usa. O fluxo decidido no produto é a pessoa informar a cidade e o app deduzir o fuso: o backend já aceita `PATCH /v1/me` com `timezone` (nome IANA), e falta a lista de cidades com fuso no frontend. Entra na F4, mas **antes da rotina (F3)**, porque a rotina depende do fuso certo.

**Privacidade por item.** Blocos de rotina, anotações e metas pessoais terão visibilidade escolhida pelo dono (compartilhado, só "ocupado" ou privado). Criar um componente único `VisibilityPicker` em `shared/` e reutilizá-lo nas três telas, com o padrão no valor mais restrito.

**Repetição da rotina.** O backend segue o modelo do iCalendar (regra de repetição mais exceções por data). No frontend, a tela cria a regra a partir de opções simples (todo dia, dias da semana, alternados); nunca pede ao usuário para escrever a regra.

**Offline e PWA.** Hoje o service worker só faz o pré-cache dos arquivos do app. Decisão proposta: dados continuam exigindo rede; sem rede, as telas mostram o erro `network` com "Tentar de novo". Cache de leitura dos dados e fila de ações offline ficam para uma fase própria, depois das telas principais.

**Acessibilidade.** Foco visível em tudo; diálogos devolvem o foco ao botão que os abriu (Radix cuida); abas e menus navegáveis pelo teclado; alvo de toque de 44 px; contraste conferido nos dois temas; `aria-live` quando uma ação tira ou move uma linha.

**Desempenho.** Cada página em `React.lazy` com divisão por rota, para o carregamento inicial não crescer a cada módulo novo. Listas até 200 itens dispensam virtualização.

**Qualidade de design.** Antes de fechar cada tela, rodar `npm run detect` e, se quiser uma revisão de hierarquia, espaçamento e estados, a skill `impeccable`.

## Contrato do backend (Fase 1 completa)

O backend da Fase 1 está pronto: tarefas, anotações, rotina, lista de compras, perfil e saída do casal. Tudo sob `/v1`, JSON, cookie de sessão. O que o frontend precisa saber de cada parte:

### Erros

Toda resposta de erro é `{"error": "frase em português", "code": "..."}`. **Traduza pelo `code`**, nunca mostre a frase. O `code` é o do tipo do erro (`invalid` 400, `unauthorized` 401, `forbidden` 403, `not_found` 404, `conflict` 409, `upstream` 502, `internal` 500; `429` não traz `code`, derive do status) ou um específico:

| `code` | Quando |
| --- | --- |
| `no_household` (403) | A pessoa não tem casal: mostrar a tela de criar ou entrar |
| `already_in_household`, `household_full`, `invite_invalid` | Convites e casal |
| `email_taken`, `invalid_credentials` | Cadastro e login |
| `task_not_found`, `reminder_not_found` | Id errado, outro casal ou sem permissão (indistinguíveis de propósito) |
| `note_not_found` / `note_forbidden` / `note_conflict` | Não vê a nota / vê mas não pode alterar (403) / alguém gravou antes: recarregue (409) |
| `routine_not_found` / `routine_forbidden` / `routine_conflict` | O mesmo, para blocos da rotina |
| `shopping_not_found`, `shopping_duplicate` | Item inexistente / já existe um igual aberto (ao renomear ou desfazer a compra) |

### Perfil

`PATCH /v1/me` com `name` e/ou `timezone` (campo ausente fica como está). O fuso é um nome IANA válido (`America/Sao_Paulo`); a lista de cidades fica no frontend. `Local` e nomes inventados dão `400`. Os horários da rotina e as janelas livres usam o fuso do perfil.

### Tarefas (novidades)

`POST /v1/tasks` aceita `repeat` (`none`, `daily`, `weekly`, `monthly`) e `rotation` (rodízio: exige `repeat` e responsável; não vale em tarefa do casal). A tarefa devolvida traz `repeat` e `rotation`. Ao **concluir** uma tarefa que se repete, a próxima já nasce aberta (prazo um ciclo adiante, ou o próximo futuro se estava atrasada; no rodízio, com a outra pessoa como responsável, que recebe o aviso). Basta recarregar a lista; a resposta de `complete` é só a tarefa concluída. Recusar e cancelar não criam a próxima. Passar uma tarefa com rodízio para "do casal" desliga o rodízio.

### Anotações

`POST /v1/notes` com `title`, `kind` (`text` ou `checklist`), `body` (só texto), `items` (`[{text, done}]`, só checklist), `tags`, `visibility` e `pinned`. `visibility`: `private` (padrão), `view` (o parceiro lê) ou `household` (os dois editam). `GET /v1/notes?q=&tag=` lista só o que a pessoa vê, fixadas primeiro; a busca olha título, texto e itens. `PUT /v1/notes/{id}` substitui o conteúdo e **exige `version`** (a que a pessoa viu): `409 note_conflict` se mudou. `kind` não muda depois de criada; `visibility` só o dono muda (vazio mantém). `DELETE` só o dono. Para virar tarefa ou lembrete, o frontend chama a API de tarefas ou de lembretes com o texto da linha.

### Rotina

Bloco: `title`, `category` (`work`, `study`, `workout`, `commute`, `appointment`, `home`, `other`), `location`, `visibility` (`private` o parceiro não vê; `busy` vê só o horário; `view` vê os detalhes; `household` bloco do casal, os dois editam), `recurrence` (`once` ou `weekly`), `weekdays` (1 segunda a 7 domingo), `interval_weeks` (1 a 4; 2 é semana sim, semana não), `start_date` (para `once`, o dia; para `weekly`, a âncora da regra), `end_date` opcional, `start` e `end` em `"HH:MM"` (`end` aceita `"24:00"`; o fim deve ser depois do início, sem cruzar a meia-noite), `version`.

| Rota | O que faz |
| --- | --- |
| `POST/GET /v1/routine/blocks`, `GET/PUT/DELETE .../{id}` | CRUD. `PUT` exige `version` (`409 routine_conflict`). Só o dono apaga. `POST` e `PUT` devolvem `overlaps`: ids de outros blocos **seus** que se sobrepõem (aviso, nunca impede) |
| `PUT /v1/routine/blocks/{id}/exceptions/{AAAA-MM-DD}` | Exceção num dia: `{"skip": true}` (folga) ou `{"start","end"}` (outro horário). Só em dia em que o bloco acontece. `DELETE` na mesma rota desfaz |
| `GET /v1/routine/occurrences?from=&to=` | O que acontece, dos dois, entre os dias (até 62), já em instantes UTC (`start_at`, `end_at`). Serve o dia e a semana |
| `GET /v1/routine/free?date=&min=30&from_hour=7&to_hour=22` | Janelas em que os dois estão livres, no dia do fuso de quem pergunta. Só conta o que a pessoa vê |
| `GET /v1/routine/now` | Quem está onde agora: uma entrada por pessoa, com `now` nulo se não há nada marcado (ou nada visível) |

Um bloco do parceiro marcado como `busy` chega com `busy: true`, `title`, `category` e `location` vazios: mostre "ocupado". Blocos `private` do parceiro **não aparecem em lugar nenhum**, nem nos cálculos.

### Lista de compras

Item: `name`, `quantity` (opcional), `unit`, `category` (`produce`, `grocery`, `cleaning`, `butcher`, `other`), `recurring`, `bought_at`. `POST /v1/shopping/items` soma a quantidade num item aberto igual (mesmo nome sem diferenciar maiúsculas, unidade e categoria): responde `200` com `merged: true` em vez de `201`. `GET` devolve os abertos e os comprados nas últimas 24 horas (abertos primeiro). `PUT /{id}` substitui o item; `POST /{id}/bought` e `/unbought` marcam e desfazem; `DELETE` apaga. `POST /v1/shopping/restock` traz de volta os itens recorrentes já comprados (`{"restocked": n}`).

### Sair do casal

`POST /v1/household/leave` (`204`). Quem sai fica **sem casal**: as rotas do casal passam a responder `403 no_household`, e a pessoa pode criar outro casal ou aceitar um convite. O que era compartilhado fica com quem permanece (notas e blocos do casal, tarefas abertas, lista de compras); o que era pessoal acompanha quem sai e chega ao próximo casal **sempre privado**. Lembretes pendentes entre os dois são cancelados. Convites que a pessoa criou e ninguém usou deixam de valer. Peça confirmação na tela: não há como desfazer.

## Perguntas em aberto

- ~~O criador deve ver na aba Recusadas só as tarefas que ele criou, ou todas do casal?~~ Decidido na F1: mostra todas; as ações só aparecem para o criador.
- ~~A página de tarefas entra na barra principal já agora?~~ Decidido na F1: entrou na barra (Hoje · Rotina em breve · Tarefas · Casa; com o avatar são cinco itens no celular).
- ~~Tarefa recorrente e rodízio: mesma tela ou segunda entrega?~~ Decidido: **na mesma tela de criar tarefa**. O rodízio só aparece quando a tarefa se repete, tem responsável e há duas pessoas.
- ~~Lista de cidades com fuso: curta ou completa do `Intl`?~~ Decidido: **lista curta embutida** em `features/profile/cities.ts` (capitais brasileiras e algumas cidades do exterior), com teste que valida cada fuso contra o `Intl`. Um fuso que não está na lista continua selecionável.

## Decisões tomadas na implementação

- **Erros.** `ApiError.code` é o `code` do servidor; `errorMessage` traduz por ele e, sem texto para um `code` específico, cai no tipo geral pelo status. Todos os `code` do contrato têm texto em `pt-BR` e `en`.
- **Anotações.** O filtro por busca e por etiqueta roda no navegador sobre a lista que o backend já devolve filtrada pela visibilidade (o `GET` aceita `q` e `tag`, mas não é preciso). Atalhos do cartão (fixar, marcar item) mandam `visibility: ""` para nunca mudar a visibilidade.
- **Conflito de versão** (`note_conflict`, `routine_conflict`): o diálogo explica e oferece "Ver a versão atual", que recarrega o rascunho já com a versão nova.
- **Rotina.** Datas e horas sempre pelo fuso do perfil (nunca o do aparelho). Semana de segunda a domingo, como o `weekdays` do backend. Blocos "ocupado" do parceiro aparecem sem título e sem exceção.
- **Compras.** Marcar e desmarcar são otimistas. "Repor recorrentes" avisa quantos voltaram. O modo mercado esconde cadastro e edição.
- **Perfil.** Sair do casal pede confirmação e lista as consequências (o que fica, o que acompanha, o que é cancelado).
- **Fora desta entrega:** transformar uma linha de anotação em tarefa ou lembrete, e tela de metas.
