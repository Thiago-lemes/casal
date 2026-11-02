# Casal — Implementação do Frontend (React + Vite + TypeScript)

1 de novembro de 2026

Este documento diz como o frontend vai crescer: o que já existe, as convenções a seguir, a ordem das entregas e o que cada tela precisa do backend. Ele complementa o [produto.md](produto.md) (o quê e por quê) e o [arquitetura.md](arquitetura.md) (o backend).

## Estado atual

PWA em React 19, Vite e TypeScript, dentro de `frontend/`. Hoje tem login e cadastro, criação ou entrada em um casal, a tela **Hoje** com lembretes (criar, resolver, adiar, cancelar), a página **Tarefas** (F1), a ativação dos avisos e as preferências de tema e idioma (no menu do avatar).

| Tema | Como está hoje |
| --- | --- |
| Rotas | `react-router-dom` 7. `RequireAuth` e `RequireHousehold` protegem as telas; sem casal, aparece o `SetupPage` |
| Dados do servidor | `@tanstack/react-query`. Sem retry em erro 4xx; consulta de novo ao voltar o foco |
| HTTP | `shared/api/client.ts`: um só `api()`, cookie de sessão na mesma origem, erros viram `ApiError` com `code` |
| Estilo | CSS Modules + tokens em `design-system/tokens` (camada crua `--c-*` e papéis `--color-*`); tema claro e escuro |
| Componentes | `design-system/`: Avatar, Button, Card, ChoiceGroup, Dialog, EmptyState, Skeleton, Tabs, TextArea, TextField. O Radix Dropdown é usado direto no `UserMenu` |
| Idiomas | `i18next`, um arquivo por namespace e idioma (`pt-BR` e `en`): `common`, `auth`, `household`, `today`, `push`, `tasks` |
| Testes | Vitest + Testing Library (jsdom) |
| Desenvolvimento | `F5` no VS Code sobe banco, API e Vite; o Vite repassa `/v1` para a API em `:8080` |
| Navegação | Hoje, Rotina (marcada "em breve", sem rota), Tarefas e Casa; o avatar da conta fica no pé da barra |

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
| F1 | Tarefas | `/v1/tasks` | Pronto. **Frontend feito**; falta o bloco "Tarefas de hoje" na tela Hoje |
| F2 | Anotações | a criar | Falta |
| F3 | Rotina semanal | a criar | Falta (inclui visibilidade e fuso) |
| F4 | Lembretes em tela própria e perfil | `/v1/reminders`, `/v1/me` | Lembretes prontos; edição de perfil e fuso faltam |
| F5 | Lista de compras | a criar | Falta |
| F6 | Marmita, Finanças, Metas | a criar | Fases 2 a 4 do produto |

Só F1 está desbloqueada. Para F2 em diante, o backend vem primeiro e este documento ganha uma seção por entrega.

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

Namespace novo `tasks` em `pt-BR` e `en`. O backend ainda não envia um `code` estável: hoje "tarefa não encontrada" (id errado, outro casal, sem permissão) chega como `404 not_found`. A interface mostra uma mensagem única ("Essa tarefa mudou ou não está mais disponível. Atualize a lista") e recarrega a lista. Se mais tarde o backend enviar códigos próprios (`task_not_allowed`, `reason_required`), troca-se só o texto.

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

**Fuso horário.** `GET /v1/me` já devolve `timezone` e a tela Hoje o usa. Falta o fluxo decidido no produto: a pessoa informa a cidade e o app deduz o fuso. Isso exige um endpoint de edição do perfil (por exemplo `PATCH /v1/me`) e uma lista de cidades com fuso no frontend. Entra na F4, mas **antes da rotina (F3)**, porque a rotina depende do fuso certo.

**Privacidade por item.** Blocos de rotina, anotações e metas pessoais terão visibilidade escolhida pelo dono (compartilhado, só "ocupado" ou privado). Criar um componente único `VisibilityPicker` em `shared/` e reutilizá-lo nas três telas, com o padrão no valor mais restrito.

**Repetição da rotina.** O backend segue o modelo do iCalendar (regra de repetição mais exceções por data). No frontend, a tela cria a regra a partir de opções simples (todo dia, dias da semana, alternados); nunca pede ao usuário para escrever a regra.

**Offline e PWA.** Hoje o service worker só faz o pré-cache dos arquivos do app. Decisão proposta: dados continuam exigindo rede; sem rede, as telas mostram o erro `network` com "Tentar de novo". Cache de leitura dos dados e fila de ações offline ficam para uma fase própria, depois das telas principais.

**Acessibilidade.** Foco visível em tudo; diálogos devolvem o foco ao botão que os abriu (Radix cuida); abas e menus navegáveis pelo teclado; alvo de toque de 44 px; contraste conferido nos dois temas; `aria-live` quando uma ação tira ou move uma linha.

**Desempenho.** Cada página em `React.lazy` com divisão por rota, para o carregamento inicial não crescer a cada módulo novo. Listas até 200 itens dispensam virtualização.

**Qualidade de design.** Antes de fechar cada tela, rodar `npm run detect` e, se quiser uma revisão de hierarquia, espaçamento e estados, a skill `impeccable`.

## Pendências de backend que travam o frontend

| Pendência | Trava |
| --- | --- |
| Anotações (pessoais e do casal, texto e checklist) | F2 |
| Rotina: blocos, repetição, exceções, visibilidade e horários livres | F3 |
| `PATCH /v1/me` para cidade e fuso | F4 e F3 |
| Código de erro estável nas respostas (`code`), em vez de só o status | Mensagens de erro específicas em todas as telas |
| Sair do casal | Tela de Perfil |
| Lista de compras | F5 |

## Perguntas em aberto

- ~~O criador deve ver na aba Recusadas só as tarefas que ele criou, ou todas do casal?~~ Decidido na F1: mostra todas; as ações só aparecem para o criador.
- ~~A página de tarefas entra na barra principal já agora?~~ Decidido na F1: entrou na barra (Hoje · Rotina em breve · Tarefas · Casa; com o avatar são cinco itens no celular).
- Tarefa recorrente e rodízio (previstos no produto) ficam para uma segunda entrega das tarefas. Confirmar.
- Lista de cidades com fuso: usar uma lista curta embutida (cidades brasileiras) ou a lista completa do `Intl`?
