# Spec — Fase 2: módulo Finanças (backend)

1 de novembro de 2026

Define o que construir no backend para o módulo Finanças: escopo, modelo de dados, API, regras de cálculo, integrações, testes e a ordem das entregas. Parte do [produto.md](produto.md) (seção "Módulo Finanças") e das regras do [arquitetura.md](arquitetura.md). O que o produto deixou em aberto está na última seção, com uma recomendação para cada ponto.

## Onde isto se encaixa

A ordem das fases mudou: **a Marmita ficou por último**. A Fase 1 (base) está pronta no backend.

| Fase | O que é | Situação |
| --- | --- | --- |
| 0 | Validar que os lembretes chegam, inclusive no iPhone | Validação feita com o casal, fora do código (não verificada aqui) |
| 1 | Base: rotina, tarefas, anotações, lembretes, compras | Backend pronto; frontend em andamento |
| **2** | **Finanças: gastos, acerto de contas, contas a pagar, orçamento** | **Esta spec** |
| 3 | Metas | A especificar |
| 4 | Marmita | Spec em [spec-fase-4-marmita.md](spec-fase-4-marmita.md) |
| 5 | Abrir para 5 a 10 casais e definir planos | A especificar |

**Saída da Fase 2 (do produto):** o casal fecha um mês de acerto de contas pelo app.

## Escopo

**Dentro:**

- Módulos ativáveis por casal (Finanças é opcional) e as três mudanças pequenas nas features existentes de que ela precisa.
- Gastos em reais, registrados à mão, do casal ou pessoais, com categoria, quem pagou e como dividir.
- **Gasto parcelado:** a pessoa informa que a compra foi parcelada e o app cria as parcelas, uma por mês, já com a divisão de cada uma.
- Categorias padrão, editáveis pelo casal.
- Gastos fixos que aparecem sozinhos todo mês (aluguel, internet, assinaturas).
- **Renda de cada pessoa**, guardada de forma privada, para a divisão proporcional.
- Acerto de contas: saldo entre os dois, divisão igual, combinada ou proporcional à renda, e o registro do "pagamento entre nós" que zera o saldo.
- Contas a pagar, com vencimento, responsável e lembrete.
- Orçamento mensal por categoria, com aviso ao se aproximar do limite.
- Resumo do mês e exportação dos dados.

**Fora (e por quê):**

| Fora | Motivo | Quando |
| --- | --- | --- |
| Metas financeiras (viagem, reserva, compra grande) | O produto as colocou no módulo Metas, como um tipo de meta; ficam na Fase 3 e leem os dados daqui | Fase 3 |
| Ligação com contas bancárias, pagamentos dentro do app, investimentos, imposto | O produto exclui: o módulo registra e organiza, mas não movimenta dinheiro | Fora |
| Importar extrato bancário | Depende de bancos | Fora |
| Comprovantes e fotos | Exige armazenamento de arquivos, que o app ainda não tem | Depois dos uploads |
| Outras moedas | O produto fixa o real no início | Depois |
| Orçamento pessoal | O produto fala em limite por categoria do casal | Pergunta em aberto |

## Premissas já decididas

Vêm do produto e das decisões anteriores; esta spec não as reabre.

- Moeda: real brasileiro. Valores em **centavos inteiros** (`bigint`), nunca em ponto flutuante.
- Gastos do casal são visíveis aos dois. Gastos pessoais são só de quem os registrou, **salvo escolha em contrário** da própria pessoa (visibilidade `view`).
- **Gastos pessoais nunca entram no acerto de contas** nem nos totais do casal.
- A divisão padrão é igual; o casal pode combinar um percentual fixo ou usar a divisão **proporcional à renda**. A renda é guardada, mas é um dado **só de quem a informou** (veja Privacidade da renda).
- O acerto é um registro de "pagamento entre nós" feito **por fora do app**; o saldo volta a zero. O app não move dinheiro.
- Vínculo obrigatório; um casal de uma pessoa só funciona (sem acerto, que precisa de duas).
- Features não se importam; interfaces pequenas declaradas em quem usa; erros com `code` estável; RLS por casal na mesma migração; isolamento provado removendo o filtro.

## Pré-requisitos (entrega F0)

Finanças é o primeiro módulo opcional, então traz a base que os próximos (Metas, Marmita) também usam.

| # | Mudança | Por quê |
| --- | --- | --- |
| 1 | **Módulos ativáveis**: pacote `internal/module`, tabela `household_modules`, `GET /v1/modules`, `PUT /v1/modules/{name}` e o middleware `module.Require("finance")` | O produto diz que cada casal liga só o que usa; desativar esconde e **não apaga** |
| 2 | `household.Member` ganha `Timezone` (JOIN somente leitura em `users`) | O lembrete de conta a pagar sai na hora local de quem paga |
| 3 | O `reminder` ganha criação por processo interno com um `ref` opcional, e cancelamento por `ref` | Lembretes de conta a pagar nascem do módulo e mudam quando a conta muda |

A pergunta "quais módulos querem usar", do onboarding do produto, usa `PUT /v1/modules`.

## Modelo de dados

Toda tabela leva `household_id` e política de RLS na própria migração. As que os jobs atravessam levam a cláusula de sistema (`app.system = 'on'`), como `shopping_items`. Todo valor monetário é `amount_cents bigint` com `CHECK (amount_cents > 0)`, e toda tabela com valor tem `currency text NOT NULL DEFAULT 'BRL' CHECK (currency = 'BRL')`, só para a moeda poder crescer sem migração dolorosa.

### F0 — módulos

`household_modules(household_id, module text, enabled_at, PK (household_id, module))`, com `module IN ('finance')` por enquanto. A ausência da linha significa desligado.

### F1 — categorias e gastos

`finance_categories`

| Coluna | Regra |
| --- | --- |
| id, household_id | |
| name | 1 a 40 caracteres, único por casal sem diferenciar maiúsculas |
| key | `housing`, `groceries`, `transport`, `leisure`, `health`, `bills`, `other` nas padrão; nulo nas criadas pelo casal. A interface traduz pelo `key` |
| archived_at | arquivar em vez de apagar: categorias em uso continuam nos gastos antigos |

As categorias padrão (moradia, mercado, transporte, lazer, saúde, contas fixas, outras) são criadas **na primeira vez que o módulo é ligado**. Não há outro dado de partida.

`expenses`

| Coluna | Regra |
| --- | --- |
| id, household_id, created_by, updated_by | |
| paid_by | quem pagou; membro do casal. Em gasto pessoal, é sempre `created_by` |
| scope | `household` ou `personal` |
| visibility | só em gasto pessoal: `private` (padrão) ou `view` (o parceiro lê, sem editar). Em gasto do casal, é sempre `household` |
| amount_cents | maior que zero |
| category_id | categoria do casal (pode estar arquivada em gastos antigos, mas não aceita novos) |
| spent_on | `date`, o dia do gasto |
| description | até 140 caracteres, opcional |
| split | `jsonb` `{"<user_id>": pontos-base}`, soma **10000** (1% = 100 pontos-base; 62,5% = 6250), só em gasto do casal. **Gravado em cada gasto na hora de criar** (veja Divisão) |
| source | `manual`, `shopping`, `recurring`, `bill` |
| recurring_id, bill_id | origem, quando houver |
| installment_group_id, installment_no, installments_total | só em parcela: o grupo da compra, o número desta parcela e o total (por exemplo 3 de 10). Os três juntos ou nenhum; `1 <= installment_no <= installments_total <= 48`; único por `(installment_group_id, installment_no)` |
| settlement_id | preenchido quando o gasto entra num acerto: a partir daí está **travado** |
| version | edição a dois: gravar com versão velha dá `409` |
| created_at, updated_at | |

Restrições: gasto pessoal não tem `split` nem `settlement_id` (e pode ser parcelado como qualquer outro); gasto gerado de um gasto fixo é único por `(recurring_id, spent_on)` (isso torna a geração idempotente).

Uma compra parcelada vira **uma linha de gasto por parcela**, todas com o mesmo `installment_group_id`. Cada parcela é um gasto comum: entra no mês dela, no orçamento daquele mês e no acerto quando chega a hora, e pode ser acertada sem as outras. Não existe uma "linha da compra inteira".

`finance_settings(household_id PK, split_mode, fixed_split jsonb, proportional_consent jsonb, updated_at)` guarda a divisão padrão do casal:

| Coluna | Regra |
| --- | --- |
| split_mode | `equal` (padrão, também vale sem linha), `fixed` ou `proportional` |
| fixed_split | `{"<user_id>": pontos-base}`, soma 10000; usado em `fixed` |
| proportional_consent | lista com os ids de quem **concordou** com a divisão proporcional; precisa ter os dois |

`finance_incomes(user_id PK, household_id, monthly_income_cents bigint, updated_at)`: a renda mensal de cada pessoa, opcional. É **privada**: só o dono a lê e a altera, e **nenhuma rota devolve a renda do parceiro** (veja Privacidade da renda).

### F2 — gastos fixos

`recurring_expenses(id, household_id, created_by, paid_by, amount_cents, category_id, description, scope, visibility, split (pontos-base), day_of_month, start_on, end_on, active)`. `day_of_month` vai de 1 a 31; em mês mais curto vale o último dia. `start_on` não pode ser anterior ao primeiro dia do mês atual (não há retroatividade além do mês corrente).

### F3 — acerto

`settlements(id, household_id, through_on date, from_user, to_user, amount_cents, kind, created_by, created_at)` com `kind` em `manual` (as pessoas acertaram) ou `leave` (gerado quando alguém sai do casal com saldo em aberto, só como registro). `amount_cents` maior que zero, `from_user <> to_user`.

Um acerto **fecha** os gastos do casal com `spent_on <= through_on` ainda abertos: grava o `settlement_id` neles.

### F4 — contas a pagar

`bills(id, household_id, created_by, title, amount_cents (opcional), due_on, responsible_id, category_id, notify_both, repeat, paid_at, paid_by, expense_id, reminder_refs)` com `repeat` em `none` ou `monthly`. `amount_cents` é opcional porque a conta de luz só tem o valor depois que chega. `reminder_refs` guarda os lembretes criados, para cancelar e refazer.

### F5 — orçamento

`budgets(household_id, category_id, limit_cents, PK (household_id, category_id))`: o limite vale todo mês.

`budget_alerts(household_id, category_id, month date, level, sent_at, PK (household_id, category_id, month, level))` com `level` em `80` e `100`: garante um aviso por nível por mês, mesmo com dois celulares lançando gastos ao mesmo tempo.

## API

Tudo sob `/v1/finance`, exige login, casal e o módulo ligado (`403 module_disabled`). Corpos em JSON. Valores sempre em centavos; datas `AAAA-MM-DD`; meses `AAAA-MM`.

### F0

| Rota | O que faz |
| --- | --- |
| `GET /v1/modules` | Módulos do casal e se estão ligados |
| `PUT /v1/modules/{name}` com `{"enabled": true}` | Liga ou desliga (qualquer um dos dois). Ao ligar pela primeira vez, cria as categorias padrão |

### F1 — categorias e gastos

| Rota | O que faz |
| --- | --- |
| `GET/POST /v1/finance/categories` | Lista (com `?archived=true` inclui as arquivadas) e cria |
| `PUT /v1/finance/categories/{id}` | Renomeia |
| `POST /v1/finance/categories/{id}/archive` e `/unarchive` | Arquiva e desarquiva |
| `POST /v1/finance/expenses` | Registra. Em gasto do casal, `split` é opcional (sem ele vale a divisão do casal). Para **parcelar**, envia-se `installments`: `{"count": 10, "first_on": "AAAA-MM-DD", "installment_amount_cents": 12990}`. Sem `installment_amount_cents`, o `amount_cents` é o **total** e o app o reparte; com ele, o total é `count × valor` (para quem pagou juros). `first_on` pode ser passado, para registrar uma compra que já tem parcelas vencidas. A resposta traz todas as parcelas |
| `GET /v1/finance/expense-groups/{group_id}` | A compra parcelada: todas as parcelas e quais já foram acertadas |
| `PUT /v1/finance/expense-groups/{group_id}` | Muda descrição, categoria, quem pagou e divisão nas parcelas **ainda não acertadas**. Para mudar valor ou número de parcelas, apaga-se o grupo e cria-se de novo |
| `DELETE /v1/finance/expense-groups/{group_id}` | Apaga as parcelas ainda não acertadas e devolve quantas ficaram (as acertadas são imutáveis) |
| `GET /v1/finance/expenses` | Lista com `?month=&scope=household\|mine&category=&paid_by=`, mais recentes primeiro, paginada por `before`. `mine` traz meus gastos pessoais; os pessoais do parceiro só aparecem se estiverem como `view`, e nunca em `household` |
| `GET/PUT/DELETE /v1/finance/expenses/{id}` | Lê; substitui (com `version`); apaga. Gasto já acertado dá `409 expense_settled` |
| `POST /v1/finance/expenses/from-shopping` | Registra um gasto de mercado a partir da lista de compras: `amount_cents`, `paid_by`, `description` e `source = 'shopping'`. A categoria é a de mercado |

### F2 — gastos fixos

| Rota | O que faz |
| --- | --- |
| `POST/GET /v1/finance/recurring` | Cria e lista |
| `PUT/DELETE /v1/finance/recurring/{id}` | Muda (vale dos próximos meses em diante; os já gerados não mudam) e apaga |
| `POST /v1/finance/recurring/{id}/pause` e `/resume` | Pausa e retoma |

### F3 — acerto de contas

| Rota | O que faz |
| --- | --- |
| `GET/PUT /v1/finance/settings` | Divisão padrão do casal: `split_mode` e, em `fixed`, o `fixed_split`. Em `proportional`, devolve a **divisão resultante** (pontos-base de cada um) e quem já concordou, **nunca as rendas** |
| `GET/PUT/DELETE /v1/finance/income` | A renda mensal **da própria pessoa** (`monthly_income_cents`). Só ela lê, muda ou apaga a sua |
| `POST /v1/finance/settings/proportional/consent` e `DELETE` | A pessoa concorda (ou retira a concordância) com a divisão proporcional. Retirar volta o casal para `equal` |
| `GET /v1/finance/balance` | O saldo em aberto: quem deve a quem e quanto, os gastos que o compõem e o total que cada um gastou pelo casal. `?through=AAAA-MM-DD` limita o período; sem ele vale **hoje**, então parcelas futuras ainda não contam |
| `POST /v1/finance/settlements` | Registra o "pagamento entre nós": `through_on` e `amount_cents`. O valor precisa ser **exatamente** o saldo calculado agora, senão `409 settlement_stale` |
| `GET /v1/finance/settlements` | Histórico de acertos |

### F4 — contas a pagar

| Rota | O que faz |
| --- | --- |
| `POST/GET /v1/finance/bills` | Cria; lista com `?status=open\|paid&due_before=`. `due=today` serve a tela Hoje |
| `GET/PUT/DELETE /v1/finance/bills/{id}` | Lê; muda; apaga |
| `POST /v1/finance/bills/{id}/pay` com `{"amount_cents", "paid_on"}` | Marca como paga e **cria o gasto** do casal na mesma transação; se a conta é mensal, cria a próxima |

### F5 — orçamento e resumo

| Rota | O que faz |
| --- | --- |
| `GET/PUT /v1/finance/budgets` e `DELETE .../{category}` | Limite mensal por categoria |
| `GET /v1/finance/summary?month=AAAA-MM` | Total do mês por categoria, quanto cada um pagou, uso de cada orçamento e comparação com o mês anterior. `?scope=household\|mine` |

### F6 — exportação

| Rota | O que faz |
| --- | --- |
| `GET /v1/finance/export` | Os gastos que a pessoa pode ver e os acertos, em JSON (ou CSV com `?format=csv`). É a "cópia dos dados compartilhados" que o produto promete a quem sai do casal |

## Regras e cálculos

### Divisão e os centavos

Cada gasto do casal carrega seu próprio `split`, **gravado na criação** a partir da divisão do casal, e pode ser trocado só naquele gasto (um presente para a outra pessoa pode ser 0/100). Mudar a divisão do casal depois **não reescreve** os gastos antigos.

Em centavos inteiros, a parte de cada um é `floor(valor × pontos-base / 10000)`. Os centavos que sobram vão um a um para quem tem a maior parte fracionária perdida, e no empate para o menor `user_id`. A soma das partes **sempre é igual ao valor**; isso tem teste de propriedade.

**Qual divisão vale ao criar o gasto**, nesta ordem: a `split` enviada no gasto; senão a do casal, conforme `split_mode`:

- `equal`: 5000 para cada um;
- `fixed`: o `fixed_split` combinado;
- `proportional`: calculada a partir das rendas (veja abaixo), **só se estiver disponível**; se não estiver, o gasto cai em `equal` e a resposta traz o aviso `split_fallback`.

### Divisão proporcional à renda

A parte de cada pessoa é `renda dela ÷ soma das duas rendas`, em pontos-base, com os arredondamentos distribuídos pela mesma regra dos centavos para somar exatamente 10000. Só está **disponível** quando as três condições valem juntas:

1. as duas pessoas informaram a renda (e as duas são maiores que zero);
2. as duas **concordaram** com a divisão proporcional (`proportional_consent` com os dois ids);
3. o casal tem duas pessoas.

Comportamento:

- A divisão é recalculada a cada **novo** gasto; os já gravados não mudam (cada gasto guarda o seu `split`).
- Quando uma renda muda e o modo é `proportional`, o parceiro recebe um aviso: "A divisão padrão mudou" (sem valores). Isso evita que um dos dois mexa na divisão sem o outro saber.
- Retirar a concordância, apagar a própria renda ou sair do casal faz o casal voltar para `equal`.
- A renda é **mensal e em reais**, um valor só; não há histórico de rendas.

### Parcelamento

Quem registra informa que a compra foi parcelada em `count` vezes (2 a 48) e a data da primeira parcela. O app cria as parcelas numa única transação (ou todas, ou nenhuma):

- **Valor:** sem `installment_amount_cents`, o total se reparte em `count` partes; cada parcela vale `floor(total / count)` e os centavos que sobram vão um a um para as **primeiras** parcelas, então a soma é exatamente o total. Com `installment_amount_cents`, cada parcela vale esse valor e o total é `count × valor` (o caso de quem pagou juros).
- **Datas:** a parcela `n` cai no mesmo dia do mês da primeira, `n - 1` meses depois. Dia 31 vira o último dia do mês curto, a mesma regra das tarefas e das contas.
- **Divisão e quem pagou:** valem para todas as parcelas e são gravadas em cada uma, como em qualquer gasto do casal.
- **Primeira parcela no passado:** é permitido, para registrar uma compra que já está em andamento. As parcelas já vencidas entram como gastos abertos e vão ao próximo acerto.
- **Efeito nos cálculos:** cada parcela conta no **mês dela** (orçamento e resumo) e entra no saldo só quando `spent_on` chega a `hoje` ou ao `through` pedido; parcelas futuras ficam de fora até lá.
- **Edição:** cada parcela é um gasto e pode ser editada ou apagada sozinha enquanto aberta; apagar ou mudar um grupo age nas parcelas abertas e deixa as acertadas como estão.

O `PUT` de grupo não muda valor nem número de parcelas: para isso, apaga-se o grupo e registra-se de novo. Isso evita reescrever o histórico de uma compra já parcialmente acertada.

### Saldo

Para cada gasto do casal **aberto** (sem `settlement_id`): quem pagou "adianta" o valor e cada um deve a sua parte. O saldo líquido de uma pessoa é `o que pagou − a soma das suas partes`. Com duas pessoas, quem tem saldo negativo deve ao outro o valor absoluto. Gastos pessoais não entram. Sem gastos abertos, ou saldo zero, não há o que acertar (`400 nothing_to_settle`). Casal de uma pessoa só não acerta (`400 single_member`).

### Acerto

`POST /settlements` recalcula o saldo **dentro de uma transação**, com os gastos abertos travados (`FOR UPDATE`), e compara com o `amount_cents` enviado. Se diferir, `409 settlement_stale` e nada muda (alguém lançou um gasto no meio). Se bater, grava o acerto e carimba os gastos. Dois acertos ao mesmo tempo: um passa, o outro dá `settlement_stale`, pois o saldo mudou.

Gasto acertado **não se edita nem se apaga**. Para corrigir, lança-se um gasto novo (um ajuste) no período seguinte. Isso mantém o histórico confiável para os dois.

### Gastos fixos

Um job (a cada hora) gera os gastos do mês: para cada gasto fixo ativo, cria o gasto do dia `day_of_month` que já chegou, **no fuso de quem pagou**. É idempotente: a unicidade `(recurring_id, spent_on)` impede duplicar, e rodar duas vezes não muda nada. Um gasto gerado é um gasto comum: pode ser editado (um mês em que a internet veio mais cara) sem afetar o fixo.

### Contas a pagar

- Criar a conta cria os lembretes (feature `reminder`), em horário local de quem paga: `3 dias antes` e `no dia`, às 09:00. Com `notify_both`, também para a outra pessoa. Os `ref` ficam em `reminder_refs`.
- Mudar o vencimento ou o responsável cancela e recria os lembretes. Pagar ou apagar a conta cancela os pendentes.
- Pagar grava `paid_at`, cria o gasto do casal (`source = 'bill'`, quem pagou, categoria da conta, divisão padrão) e liga `expense_id`, tudo na mesma transação. Conta mensal cria a próxima, com o vencimento no mês seguinte (dia 31 vira o último dia do mês curto, como nas tarefas).
- Conta vencida e não paga continua aberta e aparece primeiro na lista.

### Orçamento

O uso de uma categoria no mês é a soma dos gastos **do casal** daquela categoria. Gastos pessoais não contam. Ao registrar ou mudar um gasto, **depois** de gravar (fora da transação, como o resto das notificações), confere-se se o uso cruzou 80% ou 100% do limite; se sim e o nível ainda não foi avisado naquele mês, grava-se em `budget_alerts` (a chave primária garante um aviso só) e envia-se o push para os dois.

### Privacidade

- Gasto pessoal `private` não aparece para o parceiro em lista, resumo, exportação, orçamento ou saldo. `view` aparece só para leitura e só em `scope=mine` do dono e na lista pessoal do parceiro; nunca nos totais do casal.
- Só o dono muda a visibilidade de um gasto pessoal e só ele o apaga ou edita.
- A exportação traz apenas o que a pessoa pode ver.

### Privacidade da renda

A renda é o dado mais sensível do módulo, e o produto sempre disse que ela é "informada de forma opcional". Como ela passa a ser guardada, valem estas regras:

- **Só o dono lê a própria renda.** Nenhuma rota, resumo, exportação ou log devolve a renda do parceiro. A tabela `finance_incomes` é lida só por `user_id = quem pergunta`.
- **A divisão resultante é visível aos dois**, porque cada um precisa ver como o gasto foi dividido. Isso tem uma consequência que a interface deve dizer com clareza antes de pedir a concordância: quem conhece a própria renda e o percentual consegue **calcular a renda do outro**. Por isso a divisão proporcional só vale com a **concordância expressa dos dois**, e qualquer um pode retirá-la quando quiser.
- A renda não entra na exportação do parceiro; entra só na da própria pessoa.
- A renda **acompanha quem sai** do casal e é apagada com a conta (o produto promete apagar os dados pessoais).
- A renda não aparece em mensagens de erro nem em avisos: o aviso de mudança diz só que a divisão mudou.
- Como o RLS não separa os dois parceiros, essa trava existe **só no código** e tem teste com dois parceiros no mesmo casal (regra do projeto).

## Integrações entre features

Declaradas em Finanças e ligadas no `app`, como `routine.People`. Nenhuma importa outra.

| Interface (declarada em Finanças) | Implementada por | Usada para |
| --- | --- | --- |
| `Reminders` (`Create` com `ref`, `Cancel` por `ref`) | `reminder.Service` | Lembretes de conta a pagar |
| `People` (`Members` com fuso) | `household.Service` | Quem são os dois e a hora local de cada um |
| `Notifier` | o mesmo notificador das tarefas | Aviso de orçamento e de acerto registrado |
| `Modules` | `module.Service` | O middleware e a criação das categorias padrão |

A integração com a **lista de compras** não precisa de interface: a tela, depois de marcar a compra, chama `from-shopping` com o valor total. O backend de Finanças não lê a lista.

**Consistência.** Gasto, saldo, acerto, orçamento e conta a pagar vivem no mesmo banco e usam transações comuns. Só os lembretes (outra feature) ficam de fora: gravam-se depois da transação principal e, se falharem, a resposta é `200` com `sync_pending` e um job de reconciliação a cada hora refaz o que faltou, por recálculo e não por incremento.

**Rede fora de transação.** O push de orçamento e de acerto sai depois do commit.

## Jobs

| Job | Frequência | O que faz |
| --- | --- | --- |
| Gastos fixos | Hora em hora | Gera os gastos do dia que chegou; idempotente |
| Reconciliação de lembretes | Hora em hora | Refaz lembretes de contas abertas que ficaram sem eles |

Ambos usam o escopo de sistema só nas tabelas `recurring_expenses`, `expenses` e `bills`.

## Mudança de casal

Finanças registra `HandleLeave` e `HandleJoin` no `household`, pois tem dados pessoais e um saldo.

**Ao sair:**

- Se há saldo em aberto, cria-se um acerto `kind = 'leave'` com o valor devido e carimba os gastos abertos, **só como registro** (o app não cobra ninguém). Quem fica vê de quem era a dívida. O carimbo inclui as **parcelas futuras** do casal, para a dívida que ficou não crescer em silêncio depois da saída.
- Gastos do casal e gastos fixos ficam com quem permanece; responsabilidades de contas a pagar e `paid_by` de gastos fixos da pessoa que saiu passam para quem ficou.
- Os gastos pessoais da pessoa (inclusive as parcelas pessoais e os fixos pessoais) a acompanham, e os `view` viram `private`.
- **A renda da pessoa a acompanha** (a linha de `finance_incomes` muda de casal). Se o casal estava em `proportional`, volta para `equal` e quem fica é avisado de que a divisão mudou.
- Lembretes de contas cuja responsável saiu são cancelados e refeitos para quem ficou.

**Ao criar ou entrar em um casal:** os gastos pessoais deixados para trás vêm junto, sempre `private`.

O produto promete uma cópia dos dados compartilhados a quem sai: a exportação (F6) cumpre isso, e a interface deve oferecê-la antes de sair.

## Segurança e isolamento

- `household_id` em toda tabela e em todo `WHERE`; RLS na mesma migração. A cláusula de sistema só em `recurring_expenses`, `expenses`, `bills` e nos movimentos da saída do casal.
- `module.Require("finance")` em todas as rotas do módulo.
- A privacidade entre os dois parceiros (gastos pessoais) existe **só no código**: o RLS não separa os dois do mesmo casal. Todo SQL de leitura de gasto filtra por `scope = 'household' OR created_by = quem pergunta OR visibility = 'view'`, e o teste usa dois parceiros no mesmo casal (regra do projeto).
- Ids de outro casal respondem `404`; categoria e conta referenciadas por id são conferidas contra o casal.
- Valores com teto (por exemplo R$ 10.000.000,00) para evitar estouro e erro de digitação absurdo; listas com teto e paginação.
- A renda tem as regras próprias da seção Privacidade da renda: leitura só pelo dono, nunca em resposta que traga dados do casal, e teste com dois parceiros no mesmo casal.

## Erros (códigos estáveis)

| `code` | Status | Quando |
| --- | --- | --- |
| `module_disabled` | 403 | Rota de Finanças com o módulo desligado |
| `expense_not_found`, `category_not_found`, `bill_not_found`, `recurring_not_found` | 404 | Id errado, de outro casal, ou gasto pessoal que a pessoa não pode ver |
| `expense_forbidden` | 403 | Vê o gasto mas não pode alterá-lo (pessoal do parceiro em `view`) |
| `expense_conflict` | 409 | Versão velha ao editar |
| `expense_settled` | 409 | Editar ou apagar gasto que já entrou num acerto |
| `category_archived` | 400 | Gasto novo numa categoria arquivada |
| `category_duplicate` | 409 | Nome de categoria já usado |
| `split_invalid` | 400 | Pontos-base que não somam 10000, ou de quem não é do casal |
| `installments_invalid` | 400 | Menos de 2 ou mais de 48 parcelas, data inválida, ou `installment_amount_cents` junto de um total incoerente |
| `income_invalid` | 400 | Renda zero, negativa ou acima do teto |
| `proportional_unavailable` | 400 | Pedir divisão proporcional sem as duas rendas ou sem a concordância dos dois |
| `settlement_stale` | 409 | O saldo mudou desde que a pessoa o viu |
| `nothing_to_settle` | 400 | Saldo zero ou sem gastos abertos |
| `single_member` | 400 | Acerto num casal de uma pessoa |
| `bill_already_paid` | 409 | Pagar duas vezes |
| `amount_invalid` | 400 | Valor zero, negativo ou acima do teto |

## Testes

Cada entrega traz os seus, no padrão do projeto: unitários do service com fake; integração com o banco; isolamento entre casais provado removendo o filtro; e a rodada com o RLS ligado, com o papel sem `BYPASSRLS`.

| Entrega | Casos que não podem faltar |
| --- | --- |
| F0 | Módulo desligado bloqueia e **não apaga**; religar traz tudo de volta; categorias padrão criadas uma vez só, mesmo com duas chamadas simultâneas; `Members` devolve o fuso; lembrete criado e cancelado por `ref` |
| F1 | **Privacidade com dois parceiros**: pessoal `private` não aparece em lista, resumo, exportação nem saldo; `view` aparece só para leitura; só o dono edita e apaga; versão velha dá `409`; categoria arquivada não aceita gasto novo; valor em centavos sem arredondar; `from-shopping` usa a categoria de mercado ; **parcelamento:** a soma das parcelas é exatamente o total (propriedade, com muitos totais e contagens), os centavos extras vão às primeiras parcelas, `installment_amount_cents` dá total `count × valor`, dia 31 cai no último dia do mês curto, primeira parcela no passado cria parcelas vencidas, criar é tudo ou nada (se uma parcela falhar, nenhuma é gravada), cada parcela conta no mês dela no resumo e o grupo apaga só as abertas |
| F2 | Gerar duas vezes não duplica; dia 31 em fevereiro cai no último dia; pausar para a geração; editar um gasto gerado não muda o fixo; fuso de quem pagou define o dia |
| F3 | **Propriedade: a soma das partes é sempre o valor** (muitos valores e percentuais aleatórios); saldo simétrico entre os dois; divisão do gasto não muda quando a do casal muda; **dois acertos simultâneos: um passa, o outro `settlement_stale`**; gasto acertado não edita; acerto com valor errado não grava nada; pessoal nunca entra no saldo ; **renda e divisão proporcional:** a renda do parceiro nunca aparece em nenhuma resposta (settings, balance, resumo, exportação), provado com dois parceiros no mesmo casal; proporcional só vale com as duas rendas e as duas concordâncias, senão o gasto cai em `equal` com `split_fallback`; retirar a concordância volta para `equal`; mudar a renda avisa o parceiro sem dizer valores; mudar a renda não reescreve gastos antigos; as partes em pontos-base somam 10000 mesmo com rendas que dão dízima; parcelas futuras não entram no saldo até a data chegar |
| F4 | Pagar cria o gasto na mesma transação (se o gasto falhar, a conta continua aberta); pagar duas vezes é recusado; conta mensal cria a próxima (dia 31 vira o fim do mês curto); mudar vencimento refaz os lembretes; lembrete no fuso de quem paga |
| F5 | Alerta de 80% e de 100% sai uma vez por mês, **mesmo com dois gastos simultâneos**; pessoal não conta no orçamento; mudar de mês zera o uso |
| F6 | A exportação não traz o que a pessoa não pode ver; sair do casal com saldo cria o acerto `leave`; gastos pessoais vão com quem sai e chegam privados; responsabilidades passam para quem ficou ; a renda vai só na exportação do dono, e acompanha quem sai; o acerto `leave` carimba também as parcelas futuras; saindo, o casal em `proportional` volta para `equal` |
| Todas | Teste ponta a ponta no `app`: registrar gastos, ver o saldo, acertar e conferir que o saldo zerou e os gastos travaram |

## Entregas e ordem

Cada entrega termina com os testes verdes nos dois papéis do banco e é útil sozinha.

| # | Entrega | Depende de | Tamanho |
| --- | --- | --- | --- |
| F0 | Módulos ativáveis e os três pré-requisitos | Fase 1 | Pequeno |
| F1 | Categorias e gastos (casal e pessoais, divisão gravada, **parcelamento**) | F0 | Médio a grande |
| F2 | Gastos fixos e o job de geração | F1 | Pequeno |
| F3 | Acerto de contas (saldo e "pagamento entre nós") e **renda com divisão proporcional** | F1 | Grande (a mais delicada: centavos, concorrência e privacidade da renda) |
| F4 | Contas a pagar e lembretes | F1, F0 | Médio |
| F5 | Orçamento, alertas e resumo do mês | F1 | Pequeno |
| F6 | Exportação e saída do casal | F1 a F4 | Médio |

A ordem põe cedo o que dá valor: com F1 e F3 o casal já registra gastos e acerta o mês, que é a saída da fase. F2, F4 e F5 aliviam o trabalho manual. F6 fecha a promessa de privacidade do produto.

## Critérios de pronto da fase

- O casal registra gastos, vê o saldo, registra o acerto e o saldo volta a zero, por API e pelo teste ponta a ponta.
- A soma das partes de qualquer divisão, e a soma das parcelas de qualquer compra parcelada, é exatamente o valor (testes de propriedade).
- A renda de uma pessoa nunca aparece para a outra, provado com dois parceiros no mesmo casal.
- Gasto pessoal `private` não vaza em nenhuma rota, provado com dois parceiros no mesmo casal.
- Desligar e religar o módulo não perde dados; sem o módulo, nenhuma rota, lembrete ou aviso de Finanças existe para o casal.
- Isolamento entre casais provado em todas as tabelas, nos dois papéis do banco.
- Documentação: README (rotas), `arquitetura.md` (privacidade entre parceiros em dinheiro, saldo e acerto) e o contrato para o frontend em `docs/frontend-implementacao.md`.
- Uso real: o casal fecha um mês de acerto pelo app (a saída do produto).

## Riscos

| Risco | Efeito | Como reduzir |
| --- | --- | --- |
| Erro de centavos no saldo | Um deve ao outro um valor errado, o que **mina a confiança no app inteiro** | Só inteiros; regra de distribuição dos centavos única e determinística; teste de propriedade; o acerto exige o valor exato e confere dentro de transação |
| Vazamento de gasto pessoal | Quebra a promessa de privacidade e pode causar conflito real | Filtro em todo SQL de leitura, teste com dois parceiros, e a regra no `CLAUDE.md` |
| Duas pessoas editando ao mesmo tempo | Saldo mudando enquanto acertam | `version` nos gastos, acerto recalculado em transação e `settlement_stale` |
| Histórico adulterado depois do acerto | Discussão sobre "quem mudou o quê" | Gasto acertado é imutável; correção é um gasto novo |
| Conta a pagar esquecida porque o push não chegou | Multa e juros | Lembretes na tela Hoje, lembrete no dia e 3 dias antes, e a conta vencida segue no topo |
| Dado financeiro gera desconforto | A pessoa não usa | Registro manual, nada de bancos, renda opcional e privada, privacidade explícita |
| **Renda deduzida pelo parceiro** | Quem sabe a própria renda e o percentual calcula a do outro; pode gerar conflito | A divisão proporcional só vale com a concordância expressa dos dois, a interface avisa isso antes de pedir, qualquer um retira quando quiser, e mudar a renda avisa o outro |
| Vazamento da renda | Dado mais sensível do módulo exposto | Tabela lida só pelo dono, nenhuma rota devolve a renda do parceiro, teste com dois parceiros, fora da exportação do parceiro e dos logs |
| Parcelas mal repartidas ou fora de ordem | Total diferente do pago, ou parcela no mês errado | Centavos extras às primeiras parcelas, soma exata testada por propriedade, datas com a regra do último dia do mês, criação em transação única |

## Perguntas em aberto

O produto deixou estas sem resposta. A recomendação é o padrão que a spec já assume; muda-se antes de começar a entrega indicada.

| # | Pergunta | Recomendação | Afeta |
| --- | --- | --- | --- |
| 1 | ~~A divisão é igual ou proporcional à renda?~~ | **Decidido: a renda é guardada**, privada de cada pessoa. A divisão é `equal` (padrão), `fixed` ou `proportional`, e a proporcional exige a concordância dos dois (veja Privacidade da renda) | F3 |
| 2 | O gasto pessoal é visível ao parceiro por padrão? | Não: `private` por padrão; a pessoa escolhe `view` gasto a gasto | F1 |
| 3 | Os dois podem editar e apagar um gasto do casal? | Sim, até ele ser acertado, registrando `updated_by`. Sem histórico completo de alterações nesta fase | F1 |
| 4 | O acerto pode ser parcial (pagar só parte da dívida)? | Não. O acerto é do saldo inteiro, como o produto descreve ("o saldo volta a zero") | F3 |
| 5 | Existe orçamento pessoal? | Não nesta fase | F5 |
| 6 | Os níveis do alerta de orçamento (80% e 100%) são fixos? | Sim, fixos | F5 |
| 7 | A compra da lista de compras vira gasto sozinha? | Não: um toque na tela chama `from-shopping`, com o valor que a pessoa informa | F1 |
| 8 | O gasto tem hora? | Não: só a data. O mês é o mês civil | F1 |
| 9 | ~~O gasto parcelado entra na fase?~~ | **Decidido: entra.** A pessoa informa o número de parcelas e a data da primeira, e o app cria uma linha de gasto por parcela, já com a divisão | F1 |
| 10 | A renda tem histórico (renda que muda ao longo dos meses)? | Não: um valor mensal só. Os gastos antigos guardam a divisão que valia na época, então o histórico de divisões já está preservado | F3 |
| 11 | O aviso de "a divisão mudou" precisa de confirmação do parceiro? | Não: só aviso. A concordância já foi dada à divisão proporcional como mecanismo, e qualquer um pode retirá-la | F3 |
| 12 | Parcelas com juros: a pessoa informa o total ou o valor da parcela? | Os dois caminhos: total (o app reparte) ou valor da parcela (o total é `count × valor`) | F1 |
| 13 | Dá para mudar o valor ou o número de parcelas de uma compra já criada? | Não pelo grupo: apaga-se (as parcelas abertas) e registra-se de novo. Parcelas já acertadas nunca mudam | F1 |
