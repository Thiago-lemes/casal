# Spec — Fase 4: módulo Marmita (backend)

1 de novembro de 2026

Define o que construir no backend para o módulo Marmita: escopo, modelo de dados, API, regras, integrações, testes e a ordem das entregas. Parte do [produto.md](produto.md) (seção "Módulo Marmita") e das regras do [arquitetura.md](arquitetura.md). O que o produto deixou em aberto está na última seção, com uma recomendação para cada ponto.

## Onde isto se encaixa

O produto tem seis marcos, de 0 a 5. A Fase 1 (base) está pronta no backend.

| Fase | O que é | Situação |
| --- | --- | --- |
| 0 | Validar que os lembretes chegam, inclusive no iPhone | Validação feita com o casal, fora do código (não verificada aqui) |
| 1 | Base: rotina, tarefas, anotações, lembretes, compras | Backend pronto; frontend em andamento |
| 2 | Finanças | Spec em [spec-fase-2-financas.md](spec-fase-2-financas.md) |
| 3 | Metas | A especificar |
| **4** | **Marmita: receitas, cardápio, lista automática, dia de cozinhar, freezer** | **Esta spec** |
| 5 | Abrir para 5 a 10 casais e definir planos | A especificar |

**Saída da Fase 4 (do produto):** o casal faz pelo menos duas semanas de marmitas usando o app do início ao fim.

## Escopo

**Dentro:**

- Módulos ativáveis por casal (a Marmita é opcional; sem ela, nada muda para quem não a usa).
- Receitas do casal, com ingredientes, etapas, rendimento e se congela, **cadastradas por eles: o app não traz receitas prontas nem de exemplo**.
- Cardápio da semana, com refeição a cozinhar na hora ou a tirar do freezer.
- Lista de compras automática a partir do cardápio, que se ajusta quando o cardápio muda.
- Dia de cozinhar: aparece na rotina, tem roteiro unificado das receitas, etapas divididas entre os dois e termina mandando porções ao freezer.
- Freezer: estoque por receita, validade sugerida, baixa, ajuste manual e alertas.
- Lembrete de descongelar na véspera, alerta de vencimento, sugestão de repor e resumo semanal.

**Fora (e por quê):**

| Fora | Motivo | Quando |
| --- | --- | --- |
| Foto da receita | Exige armazenamento de arquivos, que o app ainda não tem | Depois, com uma entrega própria de uploads |
| Estoque de despensa ("descontar o que já tenho") | É um módulo à parte, com a mesma complexidade do freezer; o produto marca como opcional | Fase 4b, se o casal pedir |
| Importar receitas de sites | O produto exclui até a Fase 5 | Fora |
| Catálogo de receitas prontas ou de exemplo | Decidido: o casal começa do zero e cadastra as próprias | Fora |
| Dizer que um alimento é seguro | O produto proíbe: a validade é só uma sugestão | Nunca |

## Premissas já decididas

Vêm do produto e das decisões anteriores; esta spec não as reabre.

- Receitas, cardápio e freezer são **do casal**: os dois leem e editam. Não há itens pessoais neste módulo.
- Vínculo obrigatório: tudo pertence a um casal. Um casal de uma pessoa só funciona.
- Features não se importam. A Marmita usa compras, rotina e lembretes por **interfaces pequenas declaradas nela**, ligadas no `internal/app`.
- Erros com `code` estável; RLS por casal na mesma migração que cria cada tabela; teste de isolamento provado removendo o filtro.
- O estoque nunca fica negativo. A validade é sugestão e o casal pode alterá-la.

## Pré-requisitos (entrega M0)

A Marmita é o último módulo, então quase tudo de que ela precisa já foi entregue pelas fases anteriores:

| Pré-requisito | Origem |
| --- | --- |
| Módulos ativáveis (`internal/module`, `GET/PUT /v1/modules`, `module.Require`) | **Fase 2 (Finanças), entrega F0.** A Marmita só acrescenta `'meal'` à lista de módulos e usa `module.Require("meal")` |
| `household.Member` com `Timezone` | Fase 2, F0 |
| `reminder` com criação por processo interno e `ref` para cancelar | Fase 2, F0 |
| `shopping.Service` com `AddFromMenu` e `ReduceFromMenu`, que gravam `source = 'menu'` (a coluna e o valor já existem) | **Resta para a M0 da Marmita**: a lista automática precisa distinguir o que veio do cardápio |

Se esta fase for antecipada antes da Finanças, os três primeiros itens voltam para a M0 daqui.

## Modelo de dados

Toda tabela leva `household_id` e política de RLS na própria migração. As que os jobs atravessam levam a cláusula de sistema (`app.system = 'on'`), como `shopping_items`. Datas são `date`; horários de relógio, minutos desde a meia-noite, como na rotina.

### M0 — módulos (a tabela vem da Fase 2)

`household_modules(household_id, module text, enabled_at, PK (household_id, module))`, criada na Fase 2. A Marmita só inclui `'meal'` no `CHECK` de `module`. A ausência da linha significa desligado.

### M1 — receitas

`recipes`

| Coluna | Tipo | Regra |
| --- | --- | --- |
| id, household_id, created_by | uuid | |
| name | text | 1 a 120 caracteres |
| servings | integer | rendimento, 1 a 100 |
| prep_minutes | integer | opcional, até 1440 |
| freezes | boolean | se congela bem |
| freezer_days | integer | validade sugerida em dias; obrigatório se `freezes`, até 365 |
| favorite | boolean | |
| tags | jsonb | até 10, minúsculas, como nas notas |
| version | integer | edição a dois: gravar com versão velha dá `409` |
| created_at, updated_at | timestamptz | |

`recipe_ingredients(recipe_id, household_id, position, name, quantity numeric(10,3), unit, category)` com `category` igual à da lista de compras (`produce`, `grocery`, `cleaning`, `butcher`, `other`). Quantidade e unidade são opcionais ("sal a gosto"); unidade exige quantidade.

`recipe_steps(recipe_id, household_id, position, text, minutes, kind)` com `kind` em `prep_ahead` (precisa começar antes, como deixar o feijão de molho), `cook` ou `finish`. `minutes` até 1440.

Ingredientes e etapas são lidos e gravados junto com a receita, numa transação só.

### M2 — freezer

`freezer_batches`

| Coluna | Regra |
| --- | --- |
| id, household_id, recipe_id (`ON DELETE SET NULL`) | |
| name | cópia do nome da receita na hora de congelar (a receita pode ser apagada) |
| portions_total, portions_left | `0 <= portions_left <= portions_total` por `CHECK` |
| frozen_on, expires_on | `expires_on` é a sugestão (`frozen_on + freezer_days`) e pode ser alterada |
| expiry_notified_at | evita avisar duas vezes |
| created_by, created_at | |

`freezer_movements(id, household_id, batch_id, delta, reason, by, at)` com `reason` em `frozen`, `consumed`, `adjusted`. É o histórico, e a base do resumo.

### M3 e M4 — cardápio e lista automática

`meal_settings(household_id PK, meals jsonb, defrost_time_min, expiry_alert_days, low_stock_portions)` com padrões: refeições `["lunch","dinner"]`, descongelar às 20:00, avisar vencimento 3 dias antes, estoque baixo abaixo de 2 porções.

`meal_entries`

| Coluna | Regra |
| --- | --- |
| id, household_id, created_by | |
| on_date, meal | `meal` em `breakfast`, `lunch`, `dinner`, `snack` |
| source | `cook` (cozinhar na hora) ou `freezer` (tirar do freezer) |
| recipe_id | obrigatório, `ON DELETE RESTRICT` (não se apaga receita em uso no cardápio) |
| servings | porções desta refeição, 1 a 100 |
| responsible_id | quem descongela; obrigatório se `source = 'freezer'` |
| reminder_ref | para achar e cancelar o lembrete de descongelar |
| note | opcional |

Pode haver mais de uma entrada por dia e refeição (um prato e uma sobremesa).

`meal_weeks(household_id, week_start date, closed_at, PK (household_id, week_start))`: `week_start` é sempre uma segunda-feira. Cardápio "fechado" é o que alimenta a lista de compras.

`menu_shopping_lines(household_id, week_start, name_key, unit, category, quantity)`: quanto o cardápio **já enviou** à lista, por ingrediente. É a memória que permite calcular a diferença quando o cardápio muda.

### M5 — dia de cozinhar

`cook_sessions(id, household_id, on_date, start_min, end_min, status, routine_block_id, created_by)` com `status` em `planned`, `cooking`, `done`.

`cook_session_recipes(session_id, recipe_id, planned_portions, actual_portions, to_freezer)`.

`cook_session_steps(session_id, recipe_id, step_position, assignee_id, done_at)`: quem faz cada etapa e o que já foi feito. As etapas em si vêm da receita; esta tabela guarda só a divisão e o progresso.

## API

Tudo sob `/v1/meal`, exige login, casal e o módulo ligado (`403 module_disabled`). Corpos em JSON. `PUT` de receita exige `version`.

### M0 — módulos (as rotas vêm da Fase 2)

| Rota | O que faz |
| --- | --- |
| `GET /v1/modules` | Módulos do casal e se estão ligados |
| `PUT /v1/modules/{name}` com `{"enabled": true}` | Liga ou desliga (qualquer um dos dois) |

### M1 — receitas

| Rota | O que faz |
| --- | --- |
| `POST/GET /v1/meal/recipes` | Cria; lista com `?q=&tag=&favorite=true`. A lista não traz ingredientes nem etapas |
| `GET/PUT/DELETE /v1/meal/recipes/{id}` | Lê com tudo; substitui; apaga. Apagar receita em uso no cardápio dá `409 recipe_in_use` |
| `POST /v1/meal/recipes/{id}/duplicate` | Cópia com nome "… (cópia)", para criar variações |
| `POST /v1/meal/recipes/{id}/favorite` e `DELETE` | Favorita e desfavorita |

### M2 — freezer

| Rota | O que faz |
| --- | --- |
| `POST /v1/meal/freezer/batches` | Congela porções de uma receita (`recipe_id`, `portions`, `frozen_on`); a validade sai da receita se não vier |
| `GET /v1/meal/freezer` | Estoque agrupado por receita, com as porções restantes e a validade mais próxima; `?expiring_in=3` filtra os que vencem em até N dias |
| `GET /v1/meal/freezer/batches/{id}` | Um lote com o histórico |
| `POST /v1/meal/freezer/batches/{id}/consume` com `{"portions": 1}` | Baixa. Estoque insuficiente dá `409 freezer_empty` e nada muda |
| `POST /v1/meal/freezer/batches/{id}/adjust` com `{"portions_left": 3, "reason": "..."}` | Correção manual (porção que caiu, erro de contagem) |
| `PUT /v1/meal/freezer/batches/{id}` | Muda a validade |

### M3 — cardápio

| Rota | O que faz |
| --- | --- |
| `GET /v1/meal/plan?week=AAAA-MM-DD` | A semana: entradas, se está fechada e as refeições configuradas |
| `POST /v1/meal/plan/entries` | Adiciona uma refeição |
| `PUT/DELETE /v1/meal/plan/entries/{id}` | Muda ou remove; num cardápio fechado, a lista se ajusta |
| `POST /v1/meal/plan/{week}/copy-from/{other_week}` | Copia uma semana anterior |
| `GET/PUT /v1/meal/settings` | Refeições, hora do aviso de descongelar, dias do alerta de vencimento, estoque baixo |

### M4 — lista automática

| Rota | O que faz |
| --- | --- |
| `POST /v1/meal/plan/{week}/close` | Fecha o cardápio e envia os ingredientes à lista. Responde com o que foi acrescentado |
| `POST /v1/meal/plan/{week}/reopen` | Reabre (tira da lista o que ainda não foi comprado) |
| `GET /v1/meal/plan/{week}/shopping-diff` | O que a lista tem a mais ou a menos que o cardápio, sem alterar nada |

### M5 — dia de cozinhar

| Rota | O que faz |
| --- | --- |
| `GET /v1/meal/cook-days/suggest?week=` | Janelas em que os dois estão livres e cabem o tempo estimado (usa os horários livres da rotina) |
| `POST /v1/meal/cook-days` | Marca o dia (`on_date`, `start`, `end`, receitas e porções). Cria o bloco do casal na rotina |
| `GET/PUT/DELETE /v1/meal/cook-days/{id}` | Lê; muda (o bloco da rotina acompanha); cancela (o bloco some) |
| `GET /v1/meal/cook-days/{id}/script` | O roteiro unificado, em ordem, com as etapas de cada pessoa |
| `PUT /v1/meal/cook-days/{id}/steps/{recipe}/{pos}` | Atribui uma etapa a uma pessoa |
| `POST /v1/meal/cook-days/{id}/steps/{recipe}/{pos}/done` e `DELETE` | Marca e desmarca a etapa |
| `POST /v1/meal/cook-days/{id}/recipes/{recipe}/finish` com `{"portions": 8, "to_freezer": 6}` | Termina a receita e manda porções ao freezer |

### M6 — repor e resumo

| Rota | O que faz |
| --- | --- |
| `GET /v1/meal/suggestions` | Receitas favoritas com estoque baixo, para o próximo dia de cozinhar |
| `GET /v1/meal/summary?weeks=4` | O mais consumido, o que sobrou, o que venceu sem consumo |

## Regras e algoritmos

### Porções

Uma entrada com `servings = S` de uma receita com rendimento `R` usa os ingredientes multiplicados por `S / R`, arredondado a 3 casas. Receita sem ingrediente quantificado entra na lista sem quantidade ("sal a gosto").

### Soma de ingredientes

Na lista automática, ingredientes iguais somam. A chave é o nome sem diferença de maiúsculas e espaços, mais a **unidade normalizada**, mais a categoria, a mesma regra da lista de compras. A normalização converte só o que é exato:

| Família | Unidade base | Equivalentes |
| --- | --- | --- |
| Massa | `g` | `kg` (1000 g) |
| Volume | `ml` | `l` (1000 ml) |
| Contagem | `un` | `un`, vazio |

Qualquer outra unidade (`colher`, `xícara`, `pitada`) **não converte e não soma com outra unidade**: vira linha separada. Depois da soma, o valor volta à unidade maior quando passa de 1000 (1500 g viram 1,5 kg). A tabela fica num lugar só, com teste próprio.

### Lista que acompanha o cardápio

Calcula-se o **desejado**: os ingredientes de todas as entradas `source = 'cook'` da semana, somados. O **enviado** é o que está em `menu_shopping_lines`. A diferença decide:

- falta enviar: `AddFromMenu` (soma na lista, como qualquer item igual);
- sobrou: `ReduceFromMenu` reduz a quantidade do item **aberto**; se zerar, remove;
- o item já foi **comprado**: nada é desfeito, e a diferença aparece na resposta como "já comprado" para a pessoa decidir.

Depois, `menu_shopping_lines` passa a refletir o desejado. A operação é **idempotente**: rodar de novo sem mudança não altera nada. Isso importa porque a lista é outra feature e não dá para ter uma transação só (veja Integrações).

Entradas `source = 'freezer'` **não** entram na lista, pois já foram cozinhadas.

### Validade e baixa

- A validade sugerida é `frozen_on + freezer_days`. Receita que não congela não aceita lote (`400`).
- A baixa é uma única instrução condicional (`UPDATE ... SET portions_left = portions_left - n WHERE portions_left >= n`), então dois celulares dando baixa na última porção ao mesmo tempo resultam em um sucesso e um `freezer_empty`.
- Toda mudança grava uma linha em `freezer_movements`.
- A API nunca usa as palavras "seguro" ou "próprio para consumo". O campo é `expires_on` e a documentação diz "validade sugerida".

### Roteiro do dia de cozinhar

O roteiro junta as etapas de todas as receitas da sessão numa ordem determinística:

1. todas as etapas `prep_ahead`, da mais longa para a mais curta (o feijão de molho vem antes de tudo);
2. as etapas `cook`, receita por receita, começando pela receita de maior tempo total;
3. as etapas `finish` no fim.

Cada etapa traz receita, posição, texto, minutos e quem a faz. O horário estimado de cada uma sai da soma dos minutos, a partir do início da sessão. A divisão inicial é sugerida alternando as receitas entre as duas pessoas, e qualquer um muda.

### Dia de cozinhar na rotina

Criar a sessão cria um bloco **único e do casal** na rotina (`visibility = household`), no horário da sessão. Mudar ou cancelar a sessão muda ou apaga esse bloco. O vínculo é `routine_block_id`; se alguém apagar o bloco direto na rotina, a sessão continua e o vínculo é refeito na próxima edição.

### Lembretes e avisos

| Aviso | Quem recebe | Quando | Como |
| --- | --- | --- | --- |
| Descongelar | O `responsible_id` da entrada | Na véspera, na hora de `defrost_time` **no fuso dele** | Um lembrete (feature `reminder`), criado ao salvar a entrada, guardado em `reminder_ref` e cancelado se a entrada mudar ou sumir |
| Véspera do dia de cozinhar | Os dois | Dia anterior, 18:00 de cada um | Dois lembretes |
| Horas antes do dia de cozinhar | Os dois | 3 horas antes do início | Dois lembretes |
| Marmita perto do vencimento | Os dois | Uma vez, `expiry_alert_days` antes | Job diário, push direto; marca `expiry_notified_at` |

Se a hora calculada já passou ao criar a entrada, o lembrete sai na hora, desde que a refeição ainda não tenha acontecido.

## Integrações entre features

A Marmita declara o que precisa e o `app` liga, como já é feito para `routine.People`. Nenhuma importa outra.

| Interface (declarada na Marmita) | Implementada por | Usada para |
| --- | --- | --- |
| `Shopping` (`AddFromMenu`, `ReduceFromMenu`) | `shopping.Service` | Lista automática |
| `Routine` (`CreateHouseholdBlock`, `UpdateBlock`, `DeleteBlock`, `FreeWindows`) | `routine.Service` | Dia de cozinhar e sugestão de horário |
| `Reminders` (`Create`, `Cancel`) | `reminder.Service` | Descongelar e dia de cozinhar |
| `People` (`Members` com fuso) | `household.Service` | Hora local de cada pessoa |
| `Notifier` | o mesmo notificador das tarefas | Alerta de vencimento |

**Consistência entre features.** Não há transação que abranja a Marmita e a lista de compras. Por isso a regra é: gravar primeiro o que é da Marmita (a entrada, a semana fechada), depois sincronizar as outras features; e toda sincronização é **idempotente e recalcula a partir do estado**, nunca por incremento. Se a segunda parte falhar, a resposta é `200` com um aviso `sync_pending`, e um job de reconciliação a cada hora repete o que ficou pendente. O mesmo vale para lembretes e blocos da rotina.

**Rede fora de transação.** O envio de push do alerta de vencimento segue a regra do projeto: o job reserva o lote, envia e só depois conclui (`expiry_notified_at`).

## Mudança de casal

A Marmita registra `HandleLeave` no `household`. Tudo é do casal, então só há três ajustes quando alguém sai:

- entradas do cardápio e etapas de sessão em que a pessoa era responsável passam para quem ficou;
- lembretes de descongelar e de dia de cozinhar que eram para a pessoa que saiu são cancelados;
- o módulo continua ligado para quem fica.

Não há `HandleJoin`: nada aqui é pessoal.

## Segurança e isolamento

- `household_id` em toda tabela e em todo `WHERE`; RLS na mesma migração. O escopo de sistema só nas tabelas que jobs atravessam: `freezer_batches` (alerta de vencimento) e `meal_entries` (reconciliação).
- `module.Require("meal")` em todas as rotas do módulo.
- Ids de outro casal respondem `404`. Entradas e lotes de outro casal não podem ser referenciados (`recipe_id` e `batch_id` conferidos contra o casal).
- Limites de tamanho em tudo que o usuário digita; listas com teto.

## Erros (códigos estáveis)

| `code` | Status | Quando |
| --- | --- | --- |
| `module_disabled` | 403 | Rota da Marmita com o módulo desligado |
| `recipe_not_found`, `meal_entry_not_found`, `batch_not_found`, `cook_day_not_found` | 404 | Id errado ou de outro casal |
| `recipe_conflict` | 409 | Versão velha ao editar receita |
| `recipe_in_use` | 409 | Apagar receita que está no cardápio |
| `freezer_empty` | 409 | Baixa maior que o estoque |
| `recipe_not_freezable` | 400 | Congelar uma receita que não congela |
| `week_not_monday` | 400 | `week` que não é segunda-feira |

## Testes

Cada entrega traz os seus, no padrão do projeto: unitários do service com fake; integração com o banco; teste de isolamento entre casais provado removendo o filtro de propósito; e a rodada com o RLS ligado.

| Entrega | Casos que não podem faltar |
| --- | --- |
| M0 | Módulo `meal` desligado bloqueia e **não apaga**; religar traz tudo de volta; `AddFromMenu` grava `source = 'menu'` e soma com item igual aberto; `ReduceFromMenu` nunca mexe em item já comprado |
| M1 | Ingredientes e etapas gravados juntos; versão velha dá `409`; duplicar copia tudo; apagar receita em uso é recusado; isolamento entre casais |
| M2 | **Duas baixas simultâneas na última porção** (um sucesso, um `freezer_empty`); estoque nunca negativo; histórico confere com o saldo; receita que não congela não gera lote |
| M3 | Entrada `freezer` exige responsável; copiar semana; mudar entrada recria o lembrete e cancela o antigo; lembrete no fuso do responsável |
| M4 | Soma com conversão (500 g + 1 kg = 1,5 kg); unidades sem conversão ficam separadas; cardápio que muda ajusta a lista; **item já comprado não é desfeito**; fechar duas vezes não duplica (idempotência); entradas do freezer não vão à lista |
| M5 | Roteiro determinístico (prep_ahead primeiro, mais longo antes); sessão cria e apaga o bloco da rotina; terminar receita manda porções ao freezer na mesma transação; divisão de etapas |
| M6 | Alerta de vencimento sai uma vez só, mesmo com o job rodando duas vezes; sugestões só de favoritas com estoque baixo; resumo confere com os movimentos |
| Todas | Sair do casal reatribui responsáveis e cancela lembretes; teste ponta a ponta no `app` do ciclo planejar, comprar, cozinhar, guardar, descongelar |

## Entregas e ordem

Cada entrega termina com `make check` verde (ou os testes nos dois papéis) e é útil sozinha.

| # | Entrega | Depende de | Tamanho |
| --- | --- | --- | --- |
| M0 | Compras com origem `menu` e o módulo `meal` | Fase 2 (F0) | Pequeno |
| M1 | Receitas | M0 | Médio |
| M2 | Freezer | M1 | Médio |
| M3 | Cardápio e lembrete de descongelar | M1, M2 | Médio |
| M4 | Lista de compras automática | M3 | Médio (a mais delicada: idempotência e conversão) |
| M5 | Dia de cozinhar, roteiro e fim da receita no freezer | M1, M2, M4 | Grande |
| M6 | Repor, resumo e alertas de vencimento | M2, M5 | Pequeno |

A ordem leva o casal a usar cedo o que já dá valor: com M1 e M2 ele já cataloga receitas e controla o freezer, mesmo sem cardápio. O frontend pode acompanhar cada entrega.

## Critérios de pronto da fase

- O ciclo inteiro funciona por API e pelo teste ponta a ponta: planejar a semana, fechar o cardápio, lista pronta, marcar o dia de cozinhar, seguir o roteiro, guardar no freezer, receber o lembrete de descongelar e dar baixa.
- Desligar e religar o módulo não perde dados.
- Sem o módulo ligado, nenhuma rota, lembrete ou aviso da Marmita existe para o casal.
- Isolamento entre casais provado em todas as tabelas, nos dois papéis do banco.
- Documentação: README (rotas), `arquitetura.md` (integrações e reconciliação) e o contrato para o frontend em `docs/frontend-implementacao.md`.
- Uso real: o casal faz duas semanas de marmitas pelo app (a saída do produto).

## Riscos

| Risco | Efeito | Como reduzir |
| --- | --- | --- |
| Lista e cardápio saírem de sincronia | Comprar ingrediente a mais ou a menos | Sincronização idempotente por recálculo, aviso `sync_pending`, job de reconciliação e a rota `shopping-diff` para a pessoa conferir |
| Conversão de unidades errada | Quantidade errada na lista | Converter só massa e volume exatos; o resto não soma; tabela única com teste |
| Lembretes de descongelar não chegarem | A marmita não sai do freezer a tempo | Reaproveitar o agendador já validado na Fase 0; a tela Hoje mostra o que descongelar mesmo sem push |
| Escopo grande | Atrasa a fase | Seguir M0 a M6; M0 a M2 já entregam valor sem o cardápio |
| Tratar a validade como garantia | Risco real de saúde | Texto e nomes só dizem "sugerida"; sem frases de segurança na API |

## Perguntas em aberto

O produto deixou estas sem resposta. A recomendação é o padrão que a spec já assume; muda-se antes de começar a entrega indicada.

| # | Pergunta | Recomendação | Afeta |
| --- | --- | --- | --- |
| 1 | Quais refeições o casal planeja (café, almoço, jantar, lanche)? | Almoço e jantar por padrão, configurável em `meal_settings` | M3 |
| 2 | ~~O catálogo de receitas começa vazio ou com receitas prontas?~~ | **Decidido: começa vazio, sem exemplos; o casal alimenta.** Sem dados de partida em nenhuma tabela da fase (só as configurações padrão de `meal_settings`, criadas ao primeiro uso) | M1 |
| 3 | O roteiro do dia de cozinhar deve ser mais esperto (paralelizar o que dá)? | Começar com a ordem determinística descrita aqui; evoluir só se o uso real pedir | M5 |
| 4 | O estoque de despensa entra na fase? | Não. Fica como Fase 4b | M4 |
| 5 | A hora do aviso de descongelar é do casal ou de cada um? | Do casal, na hora local de quem descongela | M3 |
| 6 | Quem pode apagar receita e cardápio: os dois ou só quem criou? | Os dois, já que são do casal (diferente de notas e blocos) | M1 |
| 7 | Fotos de receita ficam para depois dos uploads? | Sim | Fora |
