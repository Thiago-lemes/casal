# Casal — Implementação do Frontend: Finanças (Fase 2)

2 de novembro de 2026

Como o frontend vai consumir o módulo Finanças: ordem das entregas, telas, regras de interface, dados e testes. O backend está **pronto** (F0 a F6 da [spec-fase-2-financas.md](spec-fase-2-financas.md)). Este documento complementa o [frontend-implementacao.md](frontend-implementacao.md) (convenções e estado atual do app), o [produto.md](produto.md) (o quê e por quê) e o [arquitetura.md](arquitetura.md) (o backend). As convenções daquele documento valem aqui sem exceção: pasta por feature, `api.ts` + `hooks.ts`, features não se importam, só papéis de cor, textos por i18n em `pt-BR` e `en`, erro traduzido pelo `code`.

**Saída da fase:** o casal fecha um mês de acerto de contas pelo app.

**Situação:** FF0 a FF6 implementadas. O estado do módulo (`/v1/modules`) vive em `shared/modules`, porque Casa, Hoje, Compras e Perfil também precisam dele e as features não se importam. As telas que juntam Finanças com outra feature ficam em `app/` (`ShoppingRoute`, `ProfileRoute`) ou em telas de composição (`HomePage`, `TodayPage`).

## Princípios que esta fase acrescenta

1. **Dinheiro é inteiro em centavos, do começo ao fim.** O JSON só tem centavos (`amount_cents`). Nunca guarde, some ou compare valores como `number` decimal. Converta para texto só na hora de mostrar. Entrada do usuário ("1.234,56") vira centavos por uma função pura com teste.
2. **O servidor é a autoridade em tudo que é conta.** O cliente não calcula saldo, divisão nem parcelas para gravar: ele mostra o que o servidor devolveu. Pode **pré-visualizar** (por exemplo "3 parcelas de R$ 100,00"), mas o valor que vale é o da resposta.
3. **Privacidade é regra de produto.** Gasto pessoal e renda são privados entre os parceiros. A tela nunca pede nem espera dado do parceiro que o servidor não devolve; ausência de dado não é erro.
4. **O módulo é opcional.** Finanças só aparece quando o casal o liga. Desligar esconde e não apaga.

## Navegação

Finanças vive dentro de **Casa** (o hub de módulos), para a barra inferior não passar de cinco itens no celular.

| Onde | O que entra |
| --- | --- |
| Casa | Cartão **Finanças**: com o módulo desligado, mostra "Ligar Finanças" (explica o que é e que dá para desligar sem perder nada); ligado, abre `/finance` |
| `/finance` | Página com abas: **Gastos · Saldo · Contas · Resumo**. Configurações (divisão, renda, categorias, fixos, orçamento) abrem de um botão de engrenagem na própria página |
| Hoje | Bloco **Contas de hoje** (abertas que vencem hoje ou já venceram), só com o módulo ligado |
| Compras | No modo mercado, depois de marcar a compra, o botão **Registrar gasto** abre o formulário de gasto já com a categoria Mercado |
| Perfil → Sair do casal | Antes de confirmar, oferece **Baixar cópia dos dados** (`/finance/export?format=csv`) |

O estado do módulo vem de `GET /v1/modules`. A página `/finance` e o cartão em Casa consultam esse estado; sem módulo, `/finance` mostra o mesmo convite do cartão. Rotas de Finanças com o módulo desligado respondem `403 module_disabled`: trate como "módulo desligado" (invalide `['modules']`), não como falha genérica.

## Ordem das entregas

Cada entrega termina com `npm run typecheck`, `npm test`, `npm run build` e `npm run detect` verdes e a tela rodando de ponta a ponta no F5.

| # | Entrega | Rotas | Tamanho |
| --- | --- | --- | --- |
| FF0 | Base: módulo ligado/desligado, utilitários de dinheiro e data, categorias | `/v1/modules`, `/v1/finance/categories` | Pequeno |
| FF1 | Gastos: lista do mês, criar, editar, apagar, pessoais, parcelas | `/v1/finance/expenses`, `/expense-groups` | Grande |
| FF2 | Saldo e acerto de contas; divisão padrão, renda e consentimento | `/balance`, `/settlements`, `/settings`, `/income` | Grande (a mais delicada) |
| FF3 | Gastos fixos | `/recurring` | Pequeno |
| FF4 | Contas a pagar e o bloco em Hoje | `/bills` | Médio |
| FF5 | Orçamento e resumo do mês | `/budgets`, `/summary` | Médio |
| FF6 | Exportação, integração com Compras e com Sair do casal | `/export`, `/expenses/from-shopping` | Pequeno |

FF1 e FF2 já entregam a saída da fase (registrar e acertar). O resto alivia o trabalho manual.

## Estrutura de pastas

```
src/features/finance/
  api.ts            tipos do JSON e uma função por rota (via api())
  hooks.ts          queryKeys + useQuery/useMutation por área
  money.ts          formatBRL, parseMoney, splitLabel (testado)
  dates.ts          datas "AAAA-MM-DD" sem passar por fuso (testado)
  permissions.ts    expenseActions(expense, meId) (testado)
  FinancePage.tsx   abas
  expenses/         ExpenseList, ExpenseRow, ExpenseComposer, InstallmentFields, SplitEditor, GroupDialog
  balance/          BalanceCard, SettleDialog, SettlementHistory
  settings/         DivisionSettings, IncomeField, ConsentDialog, CategoriesManager
  recurring/        RecurringList, RecurringForm
  bills/            BillList, BillForm, PayDialog, TodayBills
  summary/          MonthSummary, BudgetBar, BudgetEditor
shared/i18n/        namespace "finance" em pt-BR e en
```

Namespace i18n `finance`, registrado em `shared/i18n/resources.ts`. Nomes das categorias padrão vêm de `finance:category.<key>` (`housing`, `groceries`, `transport`, `leisure`, `health`, `bills`, `other`); as criadas pelo casal não têm `key` e mostram o `name`.

## Utilitários (FF0)

**`money.ts`**

| Função | Regra |
| --- | --- |
| `formatBRL(cents)` | `Intl.NumberFormat('pt-BR', {style:'currency', currency:'BRL'})` sobre `cents / 100`. Só exibição. Respeita o idioma ativo para o separador, mas a moeda é sempre BRL |
| `parseMoney(text)` | Aceita `1.234,56`, `1234,56`, `1234.56`, `12`, `0,5`. Devolve **centavos inteiros** ou `null` se inválido. Trabalha com o texto (separa parte inteira e decimais), nunca com `parseFloat × 100` (erro de ponto flutuante). Mais de 2 casas decimais é inválido |
| `MAX_CENTS` | `1_000_000_000` (R$ 10.000.000,00). Acima disso o campo marca erro antes de enviar; o servidor também recusa (`amount_invalid`) |
| `splitLabel(points)` | `6250` → `62,5%`. Pontos-base: 1% = 100 |

**`dates.ts`**: datas do servidor são `"AAAA-MM-DD"` (sem fuso). **Não** use `new Date("2026-11-15")` (é UTC e pode mostrar o dia anterior). Monte a data com `new Date(y, m-1, d)` para exibir, e mande de volta a string. "Hoje" vem do fuso do perfil (`useMe().timezone`), como na tela Hoje. Mês é `"AAAA-MM"`.

**Categorias (FF0)**: `GET /finance/categories` (`?archived=true` inclui as arquivadas). Em formulários de **gasto novo**, mostre só as não arquivadas; ao **editar** um gasto antigo cuja categoria foi arquivada, mantenha-a visível e selecionada. Gerenciador: criar, renomear, arquivar e desarquivar (nunca apagar). `409 category_duplicate` mostra "Já existe uma categoria com esse nome".

## FF1 — Gastos

### Tela

**Aba Gastos** de `/finance`: seletor de mês (anterior/próximo) e alternância **Do casal · Meus**. Lista paginada (`GET /finance/expenses?month=&scope=&limit=50`; a resposta traz `next`, que vai em `before` para a página seguinte; botão "Ver mais"). Cada linha: descrição (ou nome da categoria), categoria, quem pagou (avatar), valor, data, e selos:

- `Parcela 3/10` quando há `installment`;
- `Acertado` quando `settled` (linha em tom neutro, sem ações);
- `Fixo` / `Conta` conforme `source` (`recurring`, `bill`), `Mercado` para `shopping`.

Botão fixo **Novo gasto** abre o `ExpenseComposer` (mesmo padrão de `Dialog` das outras telas).

### ExpenseComposer

| Campo | Regra |
| --- | --- |
| Tipo | **Do casal** (padrão) ou **Pessoal**. Troca os campos abaixo |
| Valor | `parseMoney`; obrigatório; teto `MAX_CENTS` |
| Categoria | Obrigatória; só as não arquivadas |
| Data | Padrão hoje (fuso do perfil) |
| Descrição | Até 140 caracteres, com contador |
| Quem pagou | Só em gasto do casal; padrão eu. **Esconder** em pessoal (é sempre eu) |
| Divisão | Só em gasto do casal. Padrão "Divisão padrão do casal" (não envia `split`). Opções: **Igual**, **Eu pago tudo / o outro deve tudo** e **Personalizar** (`SplitEditor`: um controle de porcentagem que mantém a soma em 100%; envia pontos-base, soma 10000). Presente para o outro = 0/100 |
| Visibilidade | Só em pessoal: **Só eu** (`private`, padrão) ou **Meu parceiro pode ver** (`view`). Reaproveite o `VisibilityPicker` ou um `ChoiceGroup` com dois textos próprios; o `household` não existe aqui |
| Parcelar | Alternância "Foi parcelado". Mostra número de parcelas (2 a 48), data da primeira e a escolha **Informar o total** (padrão) ou **Informar o valor da parcela** (quem pagou juros). Mostra a pré-visualização ("3 × R$ 100,00 = R$ 300,00") calculada no cliente **só como dica** |

Regras:
- Parcelar com `installment_amount_cents` **não** envia `amount_cents` (o servidor usa `count × valor`). Enviando os dois, têm de bater, senão `400 installments_invalid`.
- Primeira parcela no passado é permitido (compra em andamento); avise que as parcelas já vencidas entram no próximo acerto.
- Resposta de parcelamento: `{expenses: [...]}`; gasto único: o próprio gasto. Se vier `split_fallback: true`, mostre um aviso não bloqueante: "A divisão proporcional não está disponível agora; dividimos igualmente".
- Erros: `split_invalid` (soma errada ou pessoa de fora), `installments_invalid`, `amount_invalid`, `category_archived` (recarregue as categorias).

### Editar e apagar

- Abrir uma linha mostra o detalhe e as ações que `expenseActions` permitir.
- `PUT /finance/expenses/{id}` **exige `version`**, a que a pessoa viu. `409 expense_conflict` → "Outra pessoa mudou este gasto. Veja a versão atual" e recarregue (sem perder o que a pessoa digitou: ofereça aplicar de novo). O escopo e a origem não mudam depois de criados.
- `409 expense_settled` → o gasto entrou num acerto; recarregue e mostre-o travado. Para corrigir, a pessoa lança um **gasto de ajuste** novo (diga isso no texto da mensagem).
- `DELETE` devolve `204`.

### Regras de permissão na interface (`expenseActions`)

O backend é a autoridade; a tela espelha para esconder botão que falharia. Use o campo `can_edit` do próprio gasto (já considera dono e acerto) e `created_by` para o resto.

| Ação | Aparece quando |
| --- | --- |
| Editar / Apagar | `can_edit` |
| Mudar visibilidade | pessoal, `can_edit` (é o dono) |
| Ver como somente leitura | `scope = personal` e `created_by` é o parceiro (aparece em "Meus" por estar em `view`): selo "Compartilhado pelo parceiro", sem ações |

### Compra parcelada (GroupDialog)

Tocar numa parcela abre a compra inteira: `GET /finance/expense-groups/{group_id}` → `{group_id, expenses, settled}` (parcelas em ordem, quais já foram acertadas).

- **Editar compra** (descrição, categoria, quem pagou, divisão): `PUT` do grupo; vale só nas parcelas **abertas**. Valor e número de parcelas **não mudam**: o texto diz "Para mudar o valor ou o número de parcelas, apague a compra e registre de novo".
- **Apagar compra**: `DELETE` do grupo devolve `{deleted, remaining}`. Confirmação: "Apagar as N parcelas abertas? As X já acertadas ficam." Se `deleted = 0` e há acertadas, o servidor responde `409 expense_settled`.
- Cada parcela também pode ser editada ou apagada **sozinha** enquanto aberta (é um gasto comum).

### Dados (`hooks.ts`)

- `queryKey`: `['finance','expenses', {month, scope}]` (com `useInfiniteQuery` se preferir a paginação por `before`), `['finance','group', id]`.
- Toda mutação de gasto invalida `['finance','expenses']`, `['finance','balance']`, `['finance','summary']` e `['finance','group']`: um gasto novo muda saldo, resumo e orçamento.
- Criar pode ser otimista na lista; **editar com `version` não**: espera o servidor.

### Testes

- `parseMoney` e `formatBRL`: tabela de entradas, inclusive `0,1 + 0,2` (sem erro de ponto flutuante), vazio, negativo, mais de 2 casas.
- `expenseActions` / `can_edit`: dono, parceiro em `view`, gasto acertado.
- `ExpenseComposer`: pessoal esconde "Quem pagou" e "Divisão"; parcelar com valor da parcela não envia `amount_cents`; `SplitEditor` mantém a soma em 10000.
- Conflito: `409 expense_conflict` mostra a mensagem e recarrega.

## FF2 — Saldo, acerto e divisão

Esta é a entrega que fecha o mês e a que não pode errar.

### Aba Saldo

`GET /finance/balance?through=` (padrão: hoje; parcelas futuras ainda não contam). Resposta: `{through, from_user, to_user, amount_cents, paid_cents, expenses}`.

- **Quem deve a quem**, em destaque, na primeira pessoa: "Você deve R$ 150,00 à Bia" / "A Bia deve R$ 150,00 a você" / "Tudo em dia" (quando `amount_cents = 0`, sem `from_user`).
- Quanto cada um pagou pelo casal (`paid_cents`), em barra simples.
- Lista dos gastos que compõem o saldo (`expenses`, só os do casal ainda não acertados).
- Casal de uma pessoa só: `400 single_member` → explica que o acerto precisa de duas pessoas e mostra o convite.
- Opcional: seletor "Acertar até" (`through`). Por padrão, hoje.

### Fluxo de acertar (`SettleDialog`)

1. Mostra o valor e o sentido ("Bia → Ana: R$ 150,00") e o texto fixo: **"O app só registra. O pagamento acontece por fora (Pix, dinheiro...)."** Não há botão de pagar.
2. Confirmar envia `POST /finance/settlements` com `{through_on: balance.through, amount_cents: balance.amount_cents}` — **exatamente o valor que a pessoa viu**.
3. `409 settlement_stale` → "O saldo mudou desde que você abriu esta tela" e **recarrega o saldo**, voltando ao passo 1 com o valor novo. Nunca reenvie sozinho com o valor novo: a pessoa precisa ver e confirmar.
4. `400 nothing_to_settle` → o saldo já estava zerado; recarregue.
5. Sucesso (`201`): invalide `['finance']` inteiro; o saldo vai a zero e os gastos aparecem como `Acertado`. A outra pessoa recebe um push.

O botão de acertar só aparece com `amount_cents > 0`. Qualquer um dos dois pode registrar.

### Histórico

`GET /finance/settlements` → `[{id, through_on, from_user, to_user, amount_cents, kind, created_at}]`. `kind = leave` é o acerto gerado quando alguém saiu do casal com saldo (só registro): rotule "Acerto na saída do casal".

### Divisão padrão (`DivisionSettings`)

`GET /finance/settings` → `{split_mode, fixed_split?, effective_split, proportional: {available, split?, consented_by, income_set}}`.

| Modo | Tela |
| --- | --- |
| **Igual** (padrão) | Sem campos |
| **Combinada** (`fixed`) | `SplitEditor` com a porcentagem de cada um (soma 100%) |
| **Proporcional à renda** | Só selecionável quando `proportional.available`; mostra a divisão resultante (`proportional.split`) |

`PUT /finance/settings` com `{split_mode, fixed_split?}`. `400 proportional_unavailable` se pedir proporcional sem as duas rendas e as duas concordâncias.

Mostre sempre o `effective_split`: "Hoje os novos gastos são divididos 50% / 50%". Mudar a divisão **não reescreve gastos antigos**; cada gasto guarda a sua. Diga isso no texto de ajuda.

### Renda e consentimento (privacidade, leia com atenção)

A renda é o dado mais sensível do módulo.

- `GET/PUT/DELETE /finance/income`: a renda mensal **da própria pessoa** (`{monthly_income_cents}`, `null` se não informou). **Não existe rota para a renda do parceiro** e a tela jamais deve sugerir que ela é visível. `PUT` exige valor entre 1 centavo e `MAX_CENTS` (`400 income_invalid`).
- O campo de renda é **opcional** e explica: "Só você vê este valor. Ele é usado apenas para calcular a divisão proporcional, se vocês dois quiserem."
- `proportional.income_set` diz **quem informou** a renda (não o valor). Use só para explicar o que falta: "Falta a Bia informar a renda dela" — sem mostrar nada além disso.
- **Antes de pedir o consentimento, mostre um diálogo claro** (`ConsentDialog`), com este conteúdo em essência: *"Na divisão proporcional, cada um paga a parte de acordo com a renda. Como vocês dois veem o percentual de cada gasto, quem sabe a própria renda consegue calcular a renda do outro. Só ative se os dois concordarem. Qualquer um pode retirar a concordância quando quiser."* O botão é "Concordo".
- `POST /finance/settings/proportional/consent` concorda; `DELETE` retira. **Retirar a concordância, apagar a própria renda ou sair do casal faz o casal voltar para Igual.** Quando isso acontece pelo outro, a pessoa recebe um push "A divisão padrão mudou" (sem valores). Ao reabrir a tela, a divisão mostrada já é a nova.
- Mudar a própria renda enquanto o casal está em proporcional também avisa o parceiro (sem valores): avise a pessoa de que o parceiro será notificado, antes de salvar.

### Dados e testes

- `queryKey`: `['finance','balance', through]`, `['finance','settlements']`, `['finance','settings']`, `['finance','income']`. Mutação de acerto invalida `['finance']`.
- Testar: texto do saldo nos três casos (devo, me devem, em dia); `SettleDialog` envia exatamente o valor e o `through` exibidos; em `settlement_stale` recarrega e **não** reenvia; `ConsentDialog` aparece antes de qualquer chamada de consentimento; a tela da renda nunca renderiza valor de outra pessoa (teste com resposta simulada contendo só `income_set`).

## FF3 — Gastos fixos

Em configurações: lista (`GET /finance/recurring`) com valor, dia do mês, quem paga, categoria e estado. Ações: criar, editar, **pausar/retomar**, apagar.

- Campos de gasto (valor, categoria, descrição, pagador, divisão, pessoal/casal) mais `day_of_month` (1 a 31; "em meses mais curtos cai no último dia"), `start_on` (não pode ser de antes do mês atual) e `end_on` opcional.
- Se o dia do mês atual já passou, o gasto **aparece na hora** na lista de gastos; invalide `['finance','expenses']` ao criar.
- Editar vale **dos próximos meses em diante**; o que já foi gerado não muda (diga no texto). Editar um gasto gerado não altera o fixo.
- Apagar um gasto gerado não o recria. Pessoais: o parceiro nunca os vê (`404 recurring_not_found`).
- Pausar mostra o selo "Pausado" e some do próximo mês até retomar.

## FF4 — Contas a pagar

**Aba Contas:** `GET /finance/bills?status=open` (padrão; vencidas primeiro, destacadas em cor de alerta) e `?status=paid` na aba "Pagas". **Nova conta**: `title`, `amount_cents` (opcional: "A conta de luz só tem valor depois"), `due_on`, `responsible_id` (padrão eu), `category_id` (padrão Contas fixas), `notify_both`, `repeat` (`none` ou `monthly`).

- Lembretes são automáticos (3 dias antes e no dia, às 09:00 na hora de quem recebe). A tela só explica isso; não há como configurar. Se a resposta trouxer **`sync_pending: true`**, a conta foi gravada e os lembretes serão refeitos em até uma hora: mostre um aviso discreto, não um erro.
- Mudar o vencimento ou o responsável refaz os lembretes (o servidor cuida).
- **Pagar** (`PayDialog`): `POST /finance/bills/{id}/pay` com `{amount_cents?, paid_on?, paid_by?}`. Se a conta não tem valor, o campo valor é obrigatório (`amount_invalid`). Cria o **gasto do casal** com a divisão padrão e liga à conta. A resposta é `{bill, expense, next_bill?}`; conta mensal traz `next_bill`, com vencimento no mês seguinte (dia 31 vira o último dia do mês curto). Invalide contas, gastos, saldo e resumo.
- `409 bill_already_paid` → alguém pagou antes; recarregue.
- Conta paga não se edita; apagar uma conta paga mantém o gasto.
- **Hoje:** `GET /finance/bills?due=today` traz as abertas que vencem hoje **e as vencidas**. O bloco "Contas de hoje" só aparece com o módulo ligado e lista com o botão de pagar. Como Hoje é uma tela de composição, ela usa o hook de `features/finance` (a tela pode compor várias features; as features não se importam).

## FF5 — Orçamento e resumo

**Aba Resumo:** `GET /finance/summary?month=&scope=household|mine` → `{total_cents, previous_total_cents, categories: [{category_id, total_cents, previous_cents, limit_cents?, used_percent?}], paid_by}`.

- Total do mês, comparação com o mês anterior (diferença em valor e em %; sem seta alarmista: tom neutro), barras por categoria e quanto cada um pagou (`paid_by`, só em `household`).
- `scope = mine` soma **apenas os meus gastos pessoais** (nunca os do parceiro, mesmo os `view`).
- **Orçamento**: `GET/PUT /finance/budgets` (`{category_id, limit_cents}`; o limite vale todo mês), `DELETE /finance/budgets/{category_id}`. A barra de cada categoria usa `used_percent`: neutra até 79%, atenção de 80% a 99%, alerta a partir de 100% (use os papéis de cor, e não só a cor: texto "85% do limite"). O push de 80% e de 100% sai do servidor, uma vez por nível por mês.
- Gasto pessoal não conta no orçamento. O orçamento é só do casal (não há orçamento pessoal nesta fase).
- Parcelas contam **no mês de cada uma**: o resumo de um mês futuro já mostra as parcelas que caem nele.

## FF6 — Exportação e saída do casal

- `GET /finance/export` (JSON) ou `?format=csv`: baixa os gastos que a pessoa pode ver, os acertos e as categorias (a renda, só a própria). No CSV, a coluna `tipo` distingue `gasto` de `acerto`. Use o link de download direto (`<a href download>`), não `fetch` + blob, para o navegador cuidar do arquivo.
- **Sair do casal**: no diálogo de confirmação do Perfil, antes do botão de sair, ofereça "Baixar cópia dos dados". É a promessa do produto a quem sai.
- Texto da confirmação, com o módulo ligado: "Se houver saldo em aberto, ele será registrado como um acerto de saída. Seus gastos pessoais e sua renda vão com você."
- **Compras → Finanças**: no modo mercado, ao concluir a compra, botão **Registrar gasto**. Abre o formulário simplificado (valor, quem pagou, descrição) e chama `POST /finance/expenses/from-shopping` (a categoria é Mercado). O backend de Finanças não lê a lista; é o cliente que leva o valor que a pessoa digita. Só aparece com o módulo ligado.

## Estados de primeira classe

| Situação | Comportamento |
| --- | --- |
| Carregando | `Skeleton` |
| Módulo desligado | Convite para ligar (nunca erro) |
| Sem gastos no mês | `EmptyState` explicando o próximo passo: "Registre o primeiro gasto do mês" |
| Sem saldo | "Tudo em dia" |
| Erro | Mensagem traduzida pelo `code` + "Tentar de novo" |
| Casal de uma pessoa | Gastos funcionam; saldo e acerto explicam que precisam de duas pessoas |

## Contrato do backend (resumo das respostas)

Tudo sob `/v1`, JSON, cookie de sessão. Valores em centavos, datas `AAAA-MM-DD`, meses `AAAA-MM`. Todas as rotas `/v1/finance/*` exigem login, casal e o módulo ligado.

**Módulos.** `GET /modules` → `[{name:"finance", enabled, enabled_at?}]`. `PUT /modules/finance` com `{"enabled": bool}` → o mesmo objeto. Módulo desconhecido: `404 module_not_found`.

**Tipos**

```ts
type Category = { id: string; name: string; key?: string; archived: boolean }

type Expense = {
  id: string; created_by: string; updated_by?: string; paid_by: string
  scope: 'household' | 'personal'; visibility: 'household' | 'private' | 'view'
  amount_cents: number; category_id: string; spent_on: string; description: string
  split?: Record<string, number>            // só no casal; pontos-base, soma 10000
  source: 'manual' | 'shopping' | 'recurring' | 'bill'
  recurring_id?: string; bill_id?: string
  installment?: { group_id: string; no: number; total: number }
  settlement_id?: string; settled: boolean
  can_edit: boolean; version: number; created_at: string; updated_at: string
}

type ExpensePage = { expenses: Expense[]; next?: string }
type Group = { group_id: string; expenses: Expense[]; settled: number }

type Balance = {
  through: string; from_user?: string; to_user?: string; amount_cents: number
  paid_cents: Record<string, number>; expenses: Expense[]
}
type Settlement = {
  id: string; through_on: string; from_user: string; to_user: string
  amount_cents: number; kind: 'manual' | 'leave'; created_by: string; created_at: string
}

type SettingsView = {
  split_mode: 'equal' | 'fixed' | 'proportional'
  fixed_split?: Record<string, number>
  effective_split: Record<string, number>
  proportional: {
    available: boolean; split?: Record<string, number>
    consented_by: string[]; income_set: Record<string, boolean>   // nunca valores
  }
}
type Income = { monthly_income_cents: number | null }

type Recurring = {
  id: string; created_by: string; paid_by: string; amount_cents: number; category_id: string
  description: string; scope: 'household' | 'personal'; visibility: string
  split?: Record<string, number>; day_of_month: number
  start_on: string; end_on?: string; active: boolean; can_edit: boolean
}

type Bill = {
  id: string; created_by: string; title: string; amount_cents?: number; due_on: string
  responsible_id: string; category_id: string; notify_both: boolean; repeat: 'none' | 'monthly'
  paid_at?: string; paid_by?: string; expense_id?: string; sync_pending?: boolean
}
type PayResult = { bill: Bill; expense: Expense; next_bill?: Bill; split_fallback?: boolean }

type Budget = { category_id: string; limit_cents: number }
type Summary = {
  month: string; scope: 'household' | 'mine'; total_cents: number; previous_total_cents: number
  categories: { category_id: string; total_cents: number; previous_cents: number; limit_cents?: number; used_percent?: number }[]
  paid_by?: Record<string, number>
}
type Export = { categories: Category[]; expenses: Expense[]; settlements: Settlement[]; my_monthly_income_cents?: number }
```

**Rotas**

| Rota | Resposta |
| --- | --- |
| `GET/POST /finance/categories`, `PUT .../{id}` `{name}`, `POST .../{id}/archive` e `/unarchive` | `Category[]` / `201 Category` / `Category` |
| `POST /finance/expenses` | `201`: gasto único = `Expense` (+ `split_fallback?`); com `installments` = `{expenses, split_fallback?}` |
| `GET /finance/expenses?month=&scope=household\|mine&category=&paid_by=&limit=&before=` | `ExpensePage` |
| `GET/PUT/DELETE /finance/expenses/{id}` | `Expense` / `Expense` (exige `version`) / `204` |
| `POST /finance/expenses/from-shopping` | `201 Expense` |
| `GET/PUT/DELETE /finance/expense-groups/{id}` | `Group` / `Group` / `{deleted, remaining}` |
| `POST/GET /finance/recurring`, `PUT/DELETE .../{id}`, `POST .../{id}/pause` e `/resume` | `201 Recurring` / `Recurring[]` / `Recurring` / `204` / `Recurring` |
| `GET/PUT /finance/settings` | `SettingsView` |
| `POST/DELETE /finance/settings/proportional/consent` | `SettingsView` |
| `GET/PUT /finance/income`, `DELETE` | `Income` / `Income` / `204` |
| `GET /finance/balance?through=` | `Balance` |
| `POST /finance/settlements` `{through_on, amount_cents}`, `GET` | `201 Settlement` / `Settlement[]` |
| `POST/GET /finance/bills` (`?status=open\|paid&due_before=&due=today`), `GET/PUT/DELETE .../{id}`, `POST .../{id}/pay` | `201 Bill` / `Bill[]` / `Bill` / `204` / `PayResult` |
| `GET/PUT /finance/budgets`, `DELETE .../{category_id}` | `Budget[]` / `Budget` / `204` |
| `GET /finance/summary?month=&scope=` | `Summary` |
| `GET /finance/export` (`?format=csv`) | `Export` / arquivo CSV |

**Erros específicos** (traduza pelo `code`; todos precisam de texto em `pt-BR` e `en`):

| `code` | Status | Tela |
| --- | --- | --- |
| `module_disabled` | 403 | Módulo desligado: mostrar o convite para ligar |
| `module_not_found` | 404 | Nome de módulo inválido (bug do cliente) |
| `expense_not_found`, `category_not_found`, `bill_not_found`, `recurring_not_found` | 404 | "Mudou ou não está mais disponível": recarregar a lista |
| `expense_forbidden` | 403 | Vê, mas não altera (gasto pessoal do parceiro em `view`) |
| `expense_conflict` | 409 | Alguém gravou antes: recarregar e reaplicar |
| `expense_settled` | 409 | Já acertado: recarregar; corrigir com gasto de ajuste |
| `category_archived` | 400 | Recarregar categorias |
| `category_duplicate` | 409 | Nome já usado |
| `split_invalid` | 400 | Divisão não soma 100% ou inclui quem não é do casal |
| `installments_invalid` | 400 | Parcelas fora de 2 a 48, data ou total incoerente |
| `amount_invalid` | 400 | Valor fora de 1 centavo a R$ 10.000.000,00 |
| `income_invalid` | 400 | Renda inválida |
| `proportional_unavailable` | 400 | Faltam rendas ou concordâncias |
| `settlement_stale` | 409 | O saldo mudou: recarregar e confirmar de novo |
| `nothing_to_settle` | 400 | Saldo zerado |
| `single_member` | 400 | Casal de uma pessoa |
| `bill_already_paid` | 409 | Conta já paga |

## O que cada pessoa vê (resumo de privacidade para a interface)

| Dado | Dono | Parceiro |
| --- | --- | --- |
| Gasto do casal | vê e edita (até acertar) | vê e edita (até acertar) |
| Gasto pessoal `private` | vê, edita, apaga | **nada** (404; não aparece em lista, resumo, exportação, saldo) |
| Gasto pessoal `view` | vê, edita, apaga | lê em "Meus", sem ações |
| Renda | lê, muda, apaga a própria | **nada**; só `income_set` (se informou) e a divisão resultante |
| Divisão resultante (proporcional) | vê | vê (e por isso o consentimento é explícito) |
| Gasto fixo pessoal | vê e edita | nada (privado) ou lê (`view`) |

A tela não precisa de caso especial para "dado ausente": se o servidor não devolve, não existe.

## Pronto quando

- O casal liga Finanças em Casa, registra gastos (inclusive pessoais e parcelados), vê o saldo, registra o acerto e o saldo volta a zero, com os gastos travados.
- Em `settlement_stale` a tela recarrega e pede nova confirmação, sem reenviar sozinha.
- Nenhuma tela mostra, sugere ou pede a renda do parceiro, e gasto pessoal `private` do parceiro não aparece em lugar nenhum.
- Desligar o módulo esconde tudo sem perder dados; religar traz de volta.
- Valores nunca passam por `parseFloat`; `0,1 + 0,2` nos testes dá exatamente 30 centavos.
- Funciona no celular (barra inferior) e no desktop (trilho lateral), claro e escuro, `pt-BR` e `en`.
- `npm run typecheck`, `npm test`, `npm run build` e `npm run detect` passam.

## Perguntas em aberto

| # | Pergunta | Recomendação |
| --- | --- | --- |
| 1 | A lista de gastos usa paginação infinita ou "Ver mais"? | "Ver mais" (mais simples de testar; o cursor `next` serve aos dois) |
| 2 | O seletor "Acertar até" fica visível ou escondido num "Avançado"? | Escondido: o padrão (hoje) serve a quase todo mundo |
| 3 | O resumo ganha gráfico? | Barras simples por categoria nesta fase; gráficos de tendência ficam para depois |
| 4 | Aviso de `split_fallback` bloqueia ou só informa? | Só informa: o gasto já foi gravado |
