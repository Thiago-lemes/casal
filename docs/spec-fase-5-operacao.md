# Spec — Fase 5: operação do beta (observabilidade, métricas e painel de suporte)

4 de novembro de 2026

Define o que construir para acompanhar a aplicação enquanto ela é aberta, de graça, a outros casais: saúde técnica, métricas de uso, um painel mínimo de suporte e um canal de feedback. Parte do [produto.md](produto.md) (seções "Roadmap por fases", "Métricas de sucesso" e "Privacidade e confiança") e das regras do [arquitetura.md](arquitetura.md). O que ficou em aberto está na última seção, com uma recomendação para cada ponto.

## Onde isto se encaixa

Esta spec cobre a parte **operacional** da Fase 5 do produto ("abrir para 5 a 10 casais"). A outra metade da Fase 5, definir os planos pagos, **fica de fora**: o beta é gratuito.

| Fase | O que é | Situação |
| --- | --- | --- |
| 1 | Base: rotina, tarefas, anotações, lembretes, compras | Backend pronto |
| 2 | Finanças | Spec em [spec-fase-2-financas.md](spec-fase-2-financas.md) |
| 3 | Metas | Spec em [spec-fase-3-metas.md](spec-fase-3-metas.md) |
| 4 | Marmita | Spec em [spec-fase-4-marmita.md](spec-fase-4-marmita.md) |
| **5** | **Abrir para outros casais: operação (esta spec); planos pagos (fora)** | **Esta spec; O1 a O5 implementadas (ver [operacao.md](operacao.md))** |

**Saída desta spec:** quem opera (o casal criador) consegue responder, sem abrir o banco à mão, a três perguntas: o app está funcionando? Os casais de teste estão usando e voltando? A pessoa que reclamou do aviso tem algum problema na conta?

## Quem usa

Duas pessoas: o casal que criou o produto. Não há equipe de suporte nem papéis diferentes. As duas têm o mesmo acesso, e a auditoria serve para tirar dúvida ("quem liberou isso?"), não para controle.

## Escopo

**Dentro:**

- Rastreio de erros no backend e no frontend.
- Monitor de disponibilidade e alertas dos pontos que já deram problema (agendador parado, falha de push).
- Métricas técnicas em formato Prometheus, para o Grafana.
- Métricas de produto (ativação, uso por módulo, retorno) a partir de agregados do banco, para o Grafana.
- Painel de suporte (`/admin`) com login próprio: buscar casal ou usuário, ver o estado da conta, ligar e desligar módulos, bloquear conta, gerenciar convites do beta, ler o feedback.
- Cadastro por convite durante o beta (modo configurável).
- Feedback dentro do app.
- Trilha de auditoria das ações do painel.

**Fora (e por quê):**

| Fora | Motivo | Quando |
| --- | --- | --- |
| Planos, preços, assinatura, pagamento, boleto | O beta é gratuito. A liberação de módulos já é centralizada em `internal/module`; plano será só uma regra que decide quais módulos o casal tem | Quando houver cobrança |
| Ver o conteúdo dos casais (notas, despesas, lembretes, receitas) | Contraria a promessa de privacidade do produto | Nunca por padrão (ver "Privacidade") |
| Entrar na conta de um usuário ("impersonar") | Mesma razão, e o risco é alto | Fora |
| Painel de métricas feito à mão | O Grafana já faz isso | Fora |
| Página pública de status | Poucos casais; um aviso direto resolve | Depois, se crescer |
| Editor de configuração do sistema no painel | Sem cobrança, quase nada precisa mudar em tempo de execução. Segredos ficam em variável de ambiente | Quando houver uma configuração que o justifique |

## Premissas já decididas

Vêm do produto e da conversa de planejamento; esta spec não as reabre.

- O app roda como `casal_app` (RLS ligado). O painel **não** usa o dono do banco.
- Erros de negócio são `apperr`; o handler só chama `httpx.Fail`. Nunca devolver `err.Error()` de erro inesperado.
- Rede nunca dentro de transação.
- Nada que contenha texto digitado pelo casal vai para log, Sentry, métrica ou evento.
- Desligar um módulo esconde e não apaga os dados.
- Nunca apagar um casal vazio nem dados de quem saiu (o `CASCADE` leva os dados pessoais; ver `docs/arquitetura.md`). **Bloquear não é apagar.**

## Pré-requisitos (entrega O0)

| Pré-requisito | Situação hoje |
| --- | --- |
| Log JSON com `request_id`, tratamento de panic e erro 500 sem detalhe | Pronto (`internal/httpx`) |
| `GET /healthz` com teste do banco | Pronto |
| Um lugar para o app rodar fora da máquina local (hospedagem, `Dockerfile`, variáveis) | **Não existe no repositório.** É decisão aberta (ver "Perguntas em aberto") e define como o Sentry, o monitor e o Grafana se ligam |
| Contagem de uso por pessoa (`last_seen`) | Não existe. Entra na O2 |

## Observabilidade

### Erros

- **Sentry** (ou GlitchTip, compatível e hospedável): no backend, captura de panic e de erro inesperado (o mesmo ponto que hoje loga o 500 com `request_id`); no frontend, erros não tratados e falha de carregamento.
- O `request_id` vai como tag, para ligar o evento ao log.
- **Filtro obrigatório antes de enviar:** o envio remove corpo de requisição, cookies, cabeçalhos e query string. Só saem o método, a rota (o padrão `/v1/reminders/{id}`, não o caminho real), o status e o `request_id`. Há teste que prova isso.
- O identificador do usuário no Sentry é o `user_id` (uuid), nunca e-mail nem nome.

### Disponibilidade

- Monitor externo em `/healthz` (UptimeRobot, Better Stack ou parecido), aviso por e-mail e push para as duas pessoas.
- `/healthz` continua respondendo `503` com o banco parado.

### Métricas técnicas

Rota `GET /metrics` em formato Prometheus, **fora do `/v1` e não exposta à internet**: escuta em endereço interno separado (`METRICS_ADDR`) ou exige um token de bearer (`METRICS_TOKEN`). Sem nenhum dos dois configurado, a rota não existe.

| Métrica | Tipo | Rótulos | Para quê |
| --- | --- | --- | --- |
| `http_requests_total` | contador | `route` (padrão da rota), `status` | Taxa de erro |
| `http_request_duration_seconds` | histograma | `route` | Latência |
| `reminder_queue_due` | gauge | | Lembretes vencidos ainda não enviados |
| `reminder_worker_last_tick_timestamp` | gauge | | Prova de que o agendador está vivo |
| `reminder_send_total` | contador | `result` (`ok`, `no_devices`, `error`) | Entrega dos lembretes |
| `push_send_total` | contador | `status` (200, 403, 404, 410, `error`), `host` (serviço de push) | Falha de push por serviço |
| `maintenance_last_success_timestamp` | gauge | `job` | Jobs de manutenção (recorrentes, reconciliação) |
| `db_connections_in_use` | gauge | | Saturação do banco |

**Cardinalidade:** os rótulos são só os da tabela. Nunca `user_id`, `household_id`, e-mail ou caminho real.

### Alertas

| Alerta | Condição | Por quê |
| --- | --- | --- |
| App fora do ar | O monitor externo falha em 2 verificações seguidas | Básico |
| Agendador parado | `time() - reminder_worker_last_tick_timestamp > 120` | O lembrete é a base do produto |
| Lembretes atrasados | `reminder_queue_due > 0` por mais de 5 min | Fila travada |
| Push recusado | `push_send_total{status="403"}` sobe | Chave VAPID trocada ou `sub` inválido (o problema que já aconteceu) |
| Taxa de erro | 5xx acima de 2% por 10 min | Regressão |
| Job de manutenção parado | `maintenance_last_success_timestamp` mais velho que 2 h | Recorrentes e reconciliações |

## Métricas de produto

Duas fontes, sem painel feito à mão.

1. **O que já dá para derivar** das tabelas existentes (`created_at` em usuários, casais, convites aceitos, itens de cada módulo).
2. **O que não dá**: uso real (voltou?) e ações como ligar um módulo. Isso exige o `last_seen` e uma tabela de eventos.

### Última atividade

Coluna `sessions.last_used_at`, atualizada no máximo **uma vez por hora** por sessão (para não escrever a cada requisição). Daí sai "pessoas ativas nos últimos 7 dias".

### Eventos de produto

Tabela `product_events`, **só de nomes e contagens**, sem conteúdo:

| Coluna | Observação |
| --- | --- |
| `id`, `occurred_at` | |
| `household_id` | Pode ser nulo (evento antes do vínculo) |
| `user_id` | Pode ser nulo |
| `name` | Lista fechada (abaixo), checada por `CHECK` |

Sem coluna de texto livre e sem JSON: não há onde gravar conteúdo por engano.

Eventos: `user_registered`, `household_created`, `partner_joined`, `module_enabled`, `module_disabled`, `reminder_created_for_partner`, `push_enabled`, `feedback_sent`.

Escrita do evento fora da transação do negócio e sem falhar a operação (erro só vai para o log). RLS: leva `household_id` e a cláusula de sistema, porque quem grava atravessa casais.

### Acesso do Grafana: o papel `casal_ops`

O Grafana não lê as tabelas. Existe um papel `casal_ops`, **sem acesso às tabelas de negócio**, com `SELECT` apenas em **views** do schema `ops`, que devolvem **agregados**:

| View | O que devolve |
| --- | --- |
| `ops.daily_activation` | Por dia: cadastros, casais criados, parceiros que entraram |
| `ops.weekly_active` | Por semana: pessoas e casais ativos (a partir de `last_used_at`) |
| `ops.module_adoption` | Por módulo: casais com o módulo ligado |
| `ops.module_usage` | Por módulo e semana: casais que criaram algo (contagem, sem conteúdo) |
| `ops.retention` | Coorte semanal de cadastro × semanas seguintes com atividade |
| `ops.push_coverage` | Casais com ao menos um aparelho de push ativo |

Regras:

- As views pertencem ao dono do banco, que ignora o RLS; por isso **devolvem só contagens**, nunca linhas de usuário. **Confirmar na implementação** que as tabelas não usam `FORCE ROW LEVEL SECURITY`; se usarem, as views passam a ser funções `SECURITY DEFINER` com `search_path` fixo.
- Agregados com **menos de 3 casais** em uma célula viram nulos, para um casal de teste não ser reconhecível em um gráfico.
- `casal_ops` é criado pelo mesmo caminho que cria `casal_app` (Makefile), com senha própria no `.env` (`OPS_DB_PASSWORD`).

## Painel de suporte (`/admin`)

### Acesso

- Contas de administração **separadas** das contas dos casais: tabela `admin_users` (e-mail, hash de senha, segredo TOTP, `created_at`). Não há cadastro: as duas contas são criadas por um comando (`go run ./cmd/admin create <email>`).
- **2FA (TOTP) obrigatório** no login.
- Sessão própria (`admin_sessions`), com cookie `admin_session`, `Path=/admin`, `HttpOnly`, `Secure`, `SameSite=Strict`, validade curta (8 h). Uma sessão de casal **não** vale no `/admin` e vice-versa.
- Limite de tentativas de login mais duro que o do app (mesma estrutura do `httpx.Limiter`).
- Rotas em `/admin/v1/...`, ligadas só se `ADMIN_ENABLED=true`. Sem isso, respondem `404`.
- Middleware `requireAdmin` próprio. Toda ação de escrita exige o cabeçalho `X-Requested-With`, além do `SameSite`.

### O que o painel mostra de cada casal

Só metadados: identificador, data de criação, quantas pessoas (e-mail e nome de cada uma, que o suporte precisa para localizar a conta), módulos ligados, `last_used_at`, quantos aparelhos de push, e a contagem de itens por módulo (por exemplo, "12 lembretes, 8 tarefas"). **Nunca o conteúdo.**

### Ações de escrita

Todas passam pelos serviços de domínio, nunca por SQL solto, e todas gravam auditoria.

| Ação | Efeito |
| --- | --- |
| Ligar ou desligar um módulo de um casal | Usa `module.Service` (mesmos ganchos e regras) |
| Bloquear ou desbloquear uma pessoa | Define `users.blocked_at`; apaga as sessões; o login responde `account_blocked`. **Não apaga dado** |
| Criar ou revogar convite de cadastro | Ver "Cadastro por convite" |
| Marcar feedback como lido | |

Não há ação de apagar casal, apagar usuário ou editar conteúdo.

### Auditoria

Tabela `admin_audit`, **somente inserção**: o papel `casal_app` tem `INSERT` e `SELECT`, sem `UPDATE` nem `DELETE`.

| Coluna | Observação |
| --- | --- |
| `id`, `occurred_at` | |
| `admin_id` | Quem fez |
| `action` | Lista fechada (`module_enabled`, `user_blocked`, `invite_created`...) |
| `target_type`, `target_id` | `household`, `user` ou `invite` |
| `before`, `after` | Só valores estruturados (estado do módulo, `blocked_at`), nunca texto do casal |

Leitura de dados pessoais também é auditada: abrir a ficha de um casal ou de uma pessoa grava `household_viewed` ou `user_viewed`.

### Acesso ao conteúdo, em caso de suporte

Por padrão **não existe**. Se um dia for preciso olhar o conteúdo de um casal para resolver um problema, o caminho é pedir ao casal que mostre na tela ou envie o que escreveu. Se essa regra mudar, vira uma decisão de produto própria, com consentimento registrado, e não parte desta spec.

## Cadastro por convite (beta)

Hoje o `POST /v1/auth/register` é aberto (com limite por IP). No beta, o controle de entrada vem de `SIGNUP_MODE`:

- `open`: como hoje.
- `invite`: o cadastro exige um código de convite válido.

Tabela `signup_codes`: `id`, `code_hash` (o código é mostrado uma vez, como o token de convite do casal), `note` (rótulo para vocês: "amiga da Carla"), `expires_at`, `max_uses`, `uses`, `revoked_at`, `created_by` (admin).

O corpo do `register` ganha o campo opcional `signup_code`. Em modo `invite` sem código válido, resposta `422` com o código estável `signup_code_invalid`. A mensagem não diz se o código existe, expirou ou foi revogado.

Isto **não** substitui o convite do parceiro (`household_invites`): são dois fluxos diferentes.

## Feedback dentro do app

- `POST /v1/feedback` (exige login e casal): `{ "message": "...", "screen": "tasks" }`. Texto de 1 a 2000 caracteres; `screen` é opcional, de uma lista fechada.
- Tabela `feedback`: `household_id`, `user_id`, `message`, `screen`, `created_at`, `read_at`. Com RLS por casal, mais a cláusula de sistema para o painel ler.
- Limite de 10 envios por pessoa por dia.
- **É a única exceção ao "nunca ver conteúdo", porque a pessoa escreveu aquilo para nós.** A tela diz isso ("Esta mensagem chega para o time do Zoog").
- O painel lista o feedback com o nome do casal e permite marcar como lido.
- Não vai para Sentry, métrica nem log.

## API

Casal (`/v1`), acréscimos:

| Método e rota | Notas |
| --- | --- |
| `POST /v1/auth/register` | Aceita `signup_code`; em modo `invite` ele é obrigatório |
| `POST /v1/feedback` | Ver acima |

Painel (`/admin/v1`), todas exigem `requireAdmin` (exceto o login):

| Método e rota | Notas |
| --- | --- |
| `POST /admin/v1/auth/login` | E-mail, senha e código TOTP |
| `POST /admin/v1/auth/logout` | |
| `GET /admin/v1/households?q=` | Busca por e-mail ou nome de quem pertence ao casal; devolve metadados |
| `GET /admin/v1/households/{id}` | Ficha com metadados e contagens (grava auditoria) |
| `PUT /admin/v1/households/{id}/modules/{name}` | Corpo `{ "enabled": true }` |
| `GET /admin/v1/users/{id}` | Metadados da pessoa (grava auditoria) |
| `POST /admin/v1/users/{id}/block` e `.../unblock` | |
| `GET`, `POST /admin/v1/signup-codes` e `DELETE .../{id}` | O código só aparece na criação |
| `GET /admin/v1/feedback` e `POST .../{id}/read` | |
| `GET /admin/v1/audit` | Últimas ações |

## Estrutura no código

- Nova feature `internal/ops` (métricas, eventos, ganchos de contagem) e `internal/admin` (contas, sessão, rotas e ações do painel), cada uma com sua pasta, como as demais.
- Features **não se importam**. O `admin` declara pequenas interfaces do que precisa (`Modules`, `Users`, `Households`, `Feedback`, `SignupCodes`) e o `internal/app` as liga.
- O `reminder` e o `push` expõem as contagens de que as métricas precisam por interfaces declaradas em `ops`, também ligadas no `app`.
- Frontend: rotas `/admin` carregadas sob demanda no frontend atual, separadas do restante do app, para quem não é admin não baixar esse código.

## Segurança e isolamento

- O `/admin` é a superfície de maior risco do produto: uma conta de administração comprometida expõe todos os casais. Por isso o 2FA, a sessão curta, o cookie separado, o limite de tentativas e a auditoria.
- O painel não lê tabelas de negócio por SQL próprio. Lê pelos serviços e pelas views de agregados.
- `users.blocked_at` é checado no login **e** a cada requisição autenticada (uma sessão antiga não sobrevive ao bloqueio).
- Isolamento por casal: o `GET /v1/feedback` não existe para o casal; só o painel lê. O teste confirma que um casal não enxerga o feedback de outro e que `casal_ops` não lê nenhuma tabela de negócio.
- Item pessoal dentro do casal (`owner_id` + `visibility`): as contagens por módulo do painel **não** distinguem nem expõem itens pessoais, só o total.

## Erros (códigos estáveis)

| Código | Tipo | Quando |
| --- | --- | --- |
| `signup_code_invalid` | Invalid | Código ausente, errado, expirado, revogado ou esgotado |
| `account_blocked` | Forbidden | Login ou uso por conta bloqueada |
| `admin_login_failed` | Unauthorized | Senha ou TOTP errados (mensagem única) |
| `feedback_rate_limited` | Conflict | Mais de 10 envios no dia |
| `module_not_found` | NotFound | Já existe, no `module` |

## Testes

- **Eventos e métricas sem conteúdo:** teste que cria itens com texto reconhecível ("segredo-de-teste") e prova que o texto não aparece na saída de `/metrics`, nas views `ops`, nos eventos nem no payload enviado ao Sentry.
- **Papel `casal_ops`:** com esse papel, `SELECT` em qualquer tabela de negócio é negado; nas views `ops` funciona. Provar com o papel certo (o dono ignora o RLS).
- **Mínimo de 3 casais:** célula com menos de 3 devolve nulo.
- **Sessões separadas:** cookie do casal no `/admin` dá `401`; cookie do admin no `/v1` dá `401`.
- **2FA:** senha certa sem TOTP não entra; TOTP reaproveitado não entra de novo.
- **Bloqueio:** pessoa bloqueada perde as sessões, não consegue login e uma sessão antiga já aberta falha na requisição seguinte. Os dados continuam lá.
- **Auditoria:** toda ação de escrita grava uma linha; o papel `casal_app` não consegue `UPDATE` nem `DELETE` nela.
- **Cadastro por convite:** modo `invite` sem código, com código revogado, expirado ou esgotado: todos `signup_code_invalid`; modo `open` ignora o campo.
- **Feedback:** isolamento com **dois casais**, remover o filtro de propósito e ver o teste falhar pelo motivo certo; limite diário.
- **Módulo pelo painel:** ligar e desligar passa pelos mesmos ganchos do `module.Service`.
- **Alertas:** testes unitários dos gauges (o `last_tick` avança a cada ciclo do agendador).

Testes de integração usam `testutil.NewUser`/`NewHousehold` e rodam nos dois papéis (dono e `casal_app`), mais `casal_ops` onde indicado.

## Entregas e ordem

Cada entrega termina com `make check` verde e é útil sozinha.

| # | Entrega | Depende de | Tamanho |
| --- | --- | --- | --- |
| O0 | Hospedagem mínima (`Dockerfile`, variáveis, `/healthz` acessível) e monitor de disponibilidade | Decisão de hospedagem | Pequeno |
| O1 | Sentry (backend e frontend) com o filtro de conteúdo, e `/metrics` com os alertas | O0 | Médio |
| O2 | `last_used_at`, `product_events`, schema `ops` com views e papel `casal_ops`, painel no Grafana | O1 | Médio |
| O3 | Cadastro por convite (`signup_codes`, `SIGNUP_MODE`) | O0 | Pequeno |
| O4 | Painel `/admin`: contas, 2FA, sessão, busca e ficha, módulos, bloqueio, auditoria | O3 | Grande |
| O5 | Feedback dentro do app e leitura no painel | O4 | Pequeno |

A ordem protege primeiro o que mais machuca (app fora do ar, lembrete que não sai), depois deixa o beta controlado (O3) e só então constrói o painel, que é a parte mais cara. Com O0 a O3 já dá para abrir para poucos casais.

## Critérios de pronto da fase

- [ ] Um erro 500 provocado aparece no Sentry com `request_id` e **sem** corpo, cookie ou e-mail.
- [ ] Parar o agendador de propósito dispara o alerta em até 2 minutos.
- [ ] Trocar a chave VAPID e enviar um push dispara o alerta de 403.
- [ ] O Grafana mostra ativação, casais ativos por semana e adoção por módulo, lendo só pelo papel `casal_ops`.
- [ ] `casal_ops` não consegue ler nenhuma tabela de negócio.
- [ ] No modo `invite`, só entra quem tem código; no modo `open`, tudo como antes.
- [ ] As duas contas de administração entram com senha e TOTP; sessão de casal não abre o `/admin`.
- [ ] Bloquear uma pessoa a desconecta na requisição seguinte e não apaga nada.
- [ ] Toda ação do painel aparece na auditoria, e a tabela não aceita `UPDATE` nem `DELETE`.
- [ ] Nenhuma tela, log, métrica ou evento do painel mostra o texto de uma nota, despesa, lembrete ou receita.
- [ ] Uma pessoa envia feedback no app e as duas o leem no painel.

## Riscos

| Risco | Impacto | Como reduzir |
| --- | --- | --- |
| Conteúdo do casal vazar por log, Sentry ou métrica | Alto: contraria a promessa de privacidade | Filtro no envio, listas fechadas de eventos e rótulos, teste com texto reconhecível |
| Conta de administração comprometida | Alto: expõe metadados de todos os casais | 2FA, sessão curta e separada, limite de tentativas, auditoria, painel desligado por padrão |
| O painel crescer até virar o produto | Médio: custo de manutenção para duas pessoas | Escopo fechado nesta spec; qualquer ação nova entra como decisão |
| Alerta demais e as duas ignorarem | Médio | Poucos alertas, só dos pontos que já deram problema |
| Cardinalidade de métricas explodir | Médio | Rótulos só da tabela, revisão antes de acrescentar |
| Casal reconhecível em um agregado | Médio | Mínimo de 3 por célula |

## Perguntas em aberto

- [ ] **Onde o app vai rodar** (VPS, Fly.io, Railway, outro)? Define o O0, o Sentry hospedado ou próprio e como o Grafana alcança o banco. Recomendação: uma plataforma simples com Postgres gerenciado e backup automático.
- [ ] **Grafana Cloud (plano gratuito) ou próprio?** Recomendação: o gratuito na nuvem, para não operar mais um serviço; ele lê só as views do `ops` pelo `casal_ops`.
- [ ] **Sentry hospedado ou GlitchTip próprio?** Recomendação: Sentry hospedado no plano gratuito, com o filtro de conteúdo ativo.
- [ ] **Quanto tempo guardar os eventos de produto?** Recomendação: 13 meses, o suficiente para comparar com o mesmo período do ano seguinte.
- [ ] **O beta é por convite desde o primeiro dia ou começa aberto?** Recomendação: `invite`, para controlar o ritmo e conhecer quem entra.
- [ ] **Quem entra no teste?** Continua a pergunta do produto (amigos, colegas, comunidade de meal prep).
- [ ] **O aviso de privacidade no cadastro**: texto e onde aparece. Recomendação: uma linha na tela de cadastro dizendo que o time vê números de uso e as mensagens de feedback, e nunca o conteúdo do casal.
- [ ] **O que acontece com a conta de quem pedir para sair do beta?** Recomendação: exportar os dados (já existe exportação nas Finanças) e remover por um procedimento manual documentado, separado do painel.
