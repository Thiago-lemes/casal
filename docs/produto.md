# App do Casal — Documento de Produto

Oct 24, 2026 · @thiago

## Visão e propósito

O app organiza a vida a dois em um só lugar: quem está onde, o que precisa ser feito, o que vão comer e para onde vai o dinheiro.

Ele nasceu de uma necessidade real do casal: cozinhar para a semana inteira, congelar as marmitas e não se perder na rotina. A partir daí, o produto cresce para a organização da casa como um todo.

- **Visão:** ser o painel da casa para quem vive junto.
- **Propósito:** reduzir o esforço mental de coordenar a vida a dois, para sobrar tempo para o que importa.
- **Promessa ao usuário:** abrir o app e saber, em poucos segundos, o que o dia do casal exige.
- **Nome:** provisório, a definir (ver perguntas em aberto).

## Problema e oportunidade

Casais que moram juntos coordenam a vida em conversas soltas, anotações e memória, e o resultado é esquecimento, retrabalho e atrito.

**Dores principais**

- Cada um sabe da própria agenda, mas ninguém tem a visão do casal: quem chega tarde, quem treina, quando dá para fazer algo juntos.
- Combinados viram mensagens perdidas em conversas ("não esquece de tirar a carne do freezer").
- Cozinhar para a semana exige planejar receitas, comprar na quantidade certa e lembrar o que está congelado.
- As tarefas da casa e os gastos compartilhados geram a sensação de que um faz mais que o outro.

**Oportunidade**

Existem apps de casal para agenda e tarefas, apps de finanças para casais e apps de marmita ou freezer, mas cada um resolve só uma parte e quase todos são feitos para outros países. Falta um produto que una essas partes e fale a língua e a realidade do casal brasileiro. O diferencial está na ligação entre elas: o cardápio sabe da agenda, a lista de compras alimenta as finanças e o lembrete nasce do que foi planejado.

## Público-alvo e personas

O público inicial são casais que moram juntos e têm rotinas diferentes, a começar pelo próprio casal criador do produto.

**Público**

- **Primário:** casais que dividem a casa, trabalham fora ou em horários distintos e querem se organizar melhor.
- **Nicho de entrada:** casais que fazem refeições para a semana e congelam marmitas.
- **Secundário (futuro):** famílias com filhos e moradores que dividem casa, caso o produto se expanda.

**Personas**

| Persona | Perfil | O que quer |
| --- | --- | --- |
| Casal com horários cruzados | Um trabalha de manhã, outro à tarde ou em turnos | Saber quem está onde e achar momentos livres em comum |
| Casal de meal prep | Cozinha no fim de semana e congela porções | Planejar receitas, comprar certo e não perder comida no freezer |
| Casal que está organizando a casa | Começou a morar junto ou mudou de fase | Dividir tarefas e gastos de forma justa e visível |
| Casal que usa a agenda do celular | Já tem calendário, mas só com compromissos | Ter lembretes entre os dois e uma visão da casa, não só de eventos |

**Fora do foco no início:** relacionamentos à distância, grupos com mais de duas pessoas e uso profissional.

## Princípios do produto

Seis princípios guiam as decisões de produto, e quando duas funcionalidades competem, vence a que respeita mais esses princípios.

1. **O casal é a unidade, mas a pessoa também.** Tudo vive dentro do casal, e cada pessoa tem seu espaço individual dentro dele.
2. **Simples de dois.** Poucos toques para o que se repete todo dia; nada que exija manual.
3. **Útil no primeiro dia.** O app entrega valor antes de o casal preencher muita coisa.
4. **Módulos opcionais.** Cada casal liga só o que usa; o que não usa não atrapalha.
5. **Tudo conversa.** Cardápio, compras, agenda e finanças se alimentam, sem digitar duas vezes.
6. **Transparência com privacidade.** O casal compartilha o que quer, e o que é pessoal pode continuar pessoal.

**O que o app não é:** uma rede social, um app de relacionamento com jogos e desafios, nem um banco.

## Conceito central: o casal e os módulos

O produto gira em torno do **casal** (a casa), e cada funcionalidade vive em um módulo que o casal pode ligar ou desligar.

**O casal**

- É formado por duas pessoas, cada uma com sua conta, ligadas por um convite.
- Concentra tudo o que é compartilhado: rotina, lembretes, compras, tarefas, receitas, freezer e gastos.
- Cada pessoa mantém também o que é só seu, como blocos pessoais, anotações, metas e gastos individuais.
- **Uso sem casal (ideia futura):** servir a quem mora sozinho ou não tem parceiro, sem exigir vínculo. Fica fora do início: hoje toda pessoa usa o app dentro de um casal.

**Os módulos**

| Módulo | O que resolve | Quando entra |
| --- | --- | --- |
| Base | Rotina, lembretes, anotações, compras e tarefas da casa | Sempre ativo |
| Marmita | Receitas, cardápio, dia de cozinhar e freezer | Opcional, fase 4 |
| Finanças | Gastos compartilhados e individuais, acerto de contas | Opcional, fase 2 |
| Metas | Metas financeiras, de hábitos, do casal e projetos com etapas | Opcional, fase 3 |

**Como os módulos se ligam**

- A lista de compras é única: o cardápio adiciona ingredientes a ela, e o casal também adiciona itens avulsos.
- O dia de cozinhar aparece na rotina como um bloco do casal.
- O lembrete de descongelar nasce do cardápio e chega ao responsável.
- A compra marcada na lista pode virar um gasto no módulo de finanças.

## Módulo Base: conta, vínculo e tela de hoje

O primeiro contato precisa levar o casal de zero ao primeiro valor em poucos minutos: criar a conta, convidar o parceiro e ver o dia de ambos.

**Conta e vínculo do casal**

- Cada pessoa cria sua conta com nome e uma foto opcional.
- A própria pessoa indica onde está (cidade) e o app deduz o fuso horário; ela pode atualizar ao viajar. O app nunca rastreia a localização.
- A primeira pessoa cria o casal e gera um convite compartilhável por link.
- A segunda pessoa aceita o convite e passa a fazer parte do mesmo casal.
- Cada conta pertence a um único casal por vez.
- Qualquer um dos dois pode sair do casal; os dados compartilhados continuam com quem permanecer, e os dados pessoais de quem sai o acompanham.
- O vínculo é obrigatório: toda pessoa usa o app dentro de um casal.
- Enquanto o convite não for aceito, o casal tem uma pessoa só e o app funciona normalmente, para o primeiro uso já ser útil.

**Apresentação inicial (onboarding)**

- Perguntas rápidas: quais módulos querem usar e quais os dias e horários mais comuns de cada um.
- Orientação para ativar os avisos do celular, que são a base dos lembretes.
- Meta: o casal ver sua primeira semana montada em menos de cinco minutos.

**Tela Hoje**

É a tela inicial e responde a uma pergunta: o que o nosso dia pede?

- Quem está onde e em que horário, para cada um.
- Lembretes que chegaram para a pessoa e ainda não foram resolvidos.
- Tarefas da casa do dia e itens pendentes na lista de compras.
- Com o módulo Marmita ativo: a refeição do dia e o que precisa ser descongelado.
- Com o módulo Finanças ativo: contas que vencem hoje.

## Módulo Base: rotina semanal

A rotina é o coração do produto: cada pessoa descreve sua semana típica em blocos, e o app mostra os dois lado a lado.

**Blocos de rotina**

- Cada bloco tem título, dia(s) da semana, horário de início e fim, categoria e dono (eu, parceiro ou casal).
- Categorias iniciais: trabalho, estudo, treino, deslocamento, compromisso, tempo em casa e outros.
- Blocos podem se repetir toda semana, em dias alternados ou valer só uma vez.
- É possível abrir uma exceção em um dia específico (folga, viagem, mudança de horário) sem alterar a rotina fixa.
- Cada bloco pode ter um local opcional (casa, trabalho, academia, faculdade) para responder a "quem está onde".

**Visões**

| Visão | O que mostra |
| --- | --- |
| Dia | Linha do tempo do dia, com os blocos dos dois lado a lado |
| Semana | Grade da semana, com cores por pessoa e blocos do casal em destaque |
| Horários livres | Janelas em que os dois estão livres ao mesmo tempo, úteis para cozinhar, passear ou resolver pendências |
| Quem está onde agora | Resumo instantâneo do que cada um está fazendo neste momento |

**Regras**

- Cada um edita apenas os próprios blocos; blocos do casal podem ser editados pelos dois.
- Cada bloco tem uma visibilidade escolhida por quem o criou: **compartilhado** (o parceiro vê os detalhes), **só ocupado** (o parceiro vê apenas o horário como "ocupado") ou **privado** (o parceiro não vê nada). O mesmo vale para anotações e metas pessoais: o dono decide se o parceiro vê.
- Blocos que se sobrepõem geram um aviso, mas não são impedidos.
- Uma semana de exceção pode ser copiada da semana típica e ajustada.

## Módulo Base: lembretes entre o casal

Um lembrete é um recado com hora marcada que uma pessoa deixa para a outra (ou para si), e ele só termina quando alguém o resolve.

**Criação**

- Campos: texto, destinatário (eu, parceiro ou os dois), data e hora, e uma repetição opcional.
- Atalhos rápidos: "daqui a 1 hora", "hoje à noite", "amanhã de manhã".
- Um lembrete pode ter um local ou um item ligado a ele (por exemplo, uma marmita do freezer).

**Ciclo de vida**

1. **Criado:** o destinatário é avisado de que recebeu um lembrete.
2. **Disparado:** no horário marcado, o destinatário recebe o aviso.
3. **Resolvido:** o destinatário marca como feito, e quem criou é avisado.
4. **Adiado:** o destinatário pede para ser lembrado de novo mais tarde (10 minutos, 1 hora, amanhã).
5. **Não resolvido:** se passar do horário sem resposta, o lembrete continua visível na tela Hoje.

**Lembretes automáticos**

- Gerados pelos outros módulos, como "descongelar a marmita de amanhã" ou "conta de luz vence amanhã".
- Cada tipo de lembrete automático pode ser ligado ou desligado em Configurações.

**Regras**

- Quem recebe sempre vê quem criou o lembrete.
- Lembretes de um para o outro não podem ser apagados pelo destinatário sem resolver, para não sumirem sem aviso.
- Há um limite de avisos por hora para evitar excesso de notificações.

## Módulo Base: anotações

As anotações guardam o que não cabe em lembrete nem em tarefa: ideias, listas, combinados e informações da casa.

- Há dois tipos de nota: **pessoal** (só do dono) e **do casal** (os dois leem e editam).
- Cada nota é texto livre ou checklist, com título e etiquetas livres.
- Uma linha de checklist vira tarefa ou lembrete com um toque (por exemplo, para o parceiro).
- Notas podem ser fixadas no topo e buscadas por texto ou etiqueta.

| Nota | Quem lê | Quem edita |
| --- | --- | --- |
| Pessoal | O dono | O dono |
| Do casal | Os dois | Os dois |

## Módulo Base: lista de compras e tarefas da casa

A lista de compras e as tarefas são os itens do dia a dia da casa, e as duas existem para o casal enxergar o que falta e quem cuida de quê.

**Lista de compras**

- Uma única lista compartilhada, atualizada para os dois em tempo real.
- Cada item tem nome, quantidade, unidade opcional e categoria (hortifrúti, mercado, limpeza, açougue e outras).
- Itens são marcados ao serem comprados; os comprados ficam visíveis por pouco tempo e depois saem da lista.
- Itens podem vir de três origens: adicionados à mão, vindos do cardápio (módulo Marmita) ou recorrentes (itens que o casal sempre compra).
- Quando o mesmo ingrediente aparece em mais de uma receita, as quantidades são somadas.
- Modo mercado: tela simplificada, com letras grandes e itens agrupados por categoria.

**Tarefas da casa**

- Cada tarefa tem título, responsável (eu, parceiro ou rodízio), prazo opcional e repetição opcional.
- Tarefas recorrentes (lavar o banheiro, tirar o lixo) reaparecem sozinhas depois de concluídas.
- O rodízio alterna o responsável a cada ciclo, para dividir de forma justa.
- Um histórico simples mostra quem concluiu o quê, sem ranking nem cobrança.
- Uma tarefa pode virar um lembrete para o parceiro com um toque.
- Quem recebe uma tarefa pode recusá-la, mas precisa escrever uma justificativa. Quem criou é avisado e pode reatribuir, assumir ou cancelar a tarefa.

| Elemento | Quem pode criar | Quem pode concluir |
| --- | --- | --- |
| Item de compra | Qualquer um dos dois | Qualquer um dos dois |
| Tarefa atribuída a mim | Qualquer um dos dois | Eu |
| Tarefa atribuída ao parceiro | Qualquer um dos dois | O parceiro (o criador pode confirmar); o parceiro pode recusar com justificativa |
| Tarefa do casal | Qualquer um dos dois | Qualquer um dos dois |

## Módulo Marmita: receitas, cardápio, dia de cozinhar e freezer

O módulo Marmita acompanha o ciclo inteiro de cozinhar para a semana e congelar: planejar, comprar, cozinhar, guardar, descongelar e repor.

**1. Receitas**

- Catálogo próprio do casal, com nome, rendimento (número de porções), ingredientes com quantidade, modo de preparo, tempo de preparo e foto opcional.
- Cada receita indica se congela bem e por quanto tempo pode ficar congelada, para o app sugerir a validade.
- Etiquetas livres (por exemplo, "leve", "para o trabalho", "rápido") ajudam a filtrar na hora de montar o cardápio.
- Receitas podem ser favoritadas e duplicadas para criar variações.

**2. Cardápio da semana**

- Grade com os dias e as refeições (café, almoço, jantar, lanche), em que o casal arrasta receitas ou marmitas já congeladas.
- Cada refeição define para quantas pessoas é feita, e o app multiplica as porções.
- O casal pode copiar uma semana anterior e ajustá-la.
- Uma refeição pode vir de duas fontes: **cozinhar na hora** ou **tirar do freezer**.

**3. Lista de compras automática**

- Ao fechar o cardápio, o app soma os ingredientes de todas as receitas a cozinhar e envia para a lista de compras da casa.
- Ingredientes que o casal já tem em estoque (opcional) são descontados.
- Se o cardápio muda, a lista se ajusta e avisa o que foi acrescentado ou retirado.

**4. Dia de cozinhar**

- Aparece na rotina como um bloco do casal, sugerido nos horários livres em comum.
- Gera um roteiro com a ordem de preparo, juntando etapas de várias receitas (por exemplo, "deixar o feijão de molho" antes de tudo).
- Permite dividir as etapas entre os dois, com cada um vendo o que é seu.
- Ao concluir cada receita, o app pergunta quantas porções saíram e o que vai para o freezer.

**5. Freezer**

- Cada item guardado tem receita, quantidade de porções, data de congelamento e validade sugerida.
- Visão "o que temos" agrupada por receita, com contagem de porções restantes.
- Alerta de itens perto do vencimento, para serem consumidos primeiro.
- Ao consumir uma marmita, o casal toca para dar baixa e o estoque se atualiza.
- Ajuste manual de quantidade para corrigir diferenças (uma porção que caiu, um erro de contagem).

**6. Descongelar e repor**

- Na noite anterior a uma refeição "do freezer", o app envia um lembrete a quem for o responsável por descongelar.
- Quando o estoque de uma receita favorita fica baixo, o app sugere incluí-la no próximo dia de cozinhar.
- Um resumo mostra o que foi mais consumido e o que sobrou no freezer, para ajudar a decidir o próximo cardápio.

**Regras do módulo**

- O módulo é opcional: sem ele, a lista de compras e a rotina funcionam normalmente.
- O app nunca afirma que um alimento é seguro para consumo; a validade é uma sugestão e a decisão é do casal.
- Receitas e estoque são do casal; ambos podem editar.

## Módulo Finanças

O módulo Finanças dá ao casal clareza sobre quanto gasta, quem pagou o quê e se estão no caminho das metas, sem exigir ligação com bancos.

**Gastos**

- Registro manual de um gasto em poucos toques: valor, categoria, data, quem pagou e se é do casal ou pessoal.
- Categorias padrão (moradia, mercado, transporte, lazer, saúde, contas fixas e outras), editáveis pelo casal.
- Gastos do casal ficam visíveis aos dois; gastos pessoais só para quem os registrou, salvo escolha em contrário.
- Gastos recorrentes (aluguel, internet, assinaturas) são cadastrados uma vez e aparecem todo mês.
- Gastos parcelados: a pessoa informa o número de parcelas e a data da primeira, e o app cria uma parcela por mês, já com a divisão de cada uma.

**Acerto de contas**

- O app calcula, a cada período, quanto cada um gastou pelo casal e quanto um deve ao outro.
- A divisão padrão pode ser igual, um percentual combinado ou proporcional à renda de cada um. A renda é informada de forma opcional, guardada de forma privada (só quem a informa a vê), e a divisão proporcional só vale com a concordância dos dois.
- O acerto é registrado como um "pagamento entre nós", e o saldo volta a zero.

**Orçamento e metas**

- Limite mensal por categoria, com aviso ao se aproximar dele.
- Metas financeiras (viagem, reserva, compra grande) com valor, prazo e acompanhamento do quanto já foi guardado; elas são um tipo do módulo Metas.

**Contas a pagar**

- Cadastro de contas com vencimento, valor e responsável pelo pagamento.
- Lembrete antes do vencimento, que pode ir para quem paga ou para os dois.

**Ligação com os outros módulos**

- Ao concluir uma compra na lista, o casal pode transformá-la em um gasto de mercado com um toque.
- O resumo mostra quanto do mercado foi gasto em ingredientes do cardápio.

**Fora do escopo inicial:** ligação direta com contas bancárias, pagamentos dentro do app, investimentos e declaração de imposto. O módulo registra e organiza, mas não movimenta dinheiro.

## Módulo Metas

O módulo Metas reúne o que a pessoa ou o casal quer alcançar, com acompanhamento simples do progresso.

**Tipos de meta**

| Tipo | Exemplo | Como mede o progresso |
| --- | --- | --- |
| Financeira | Juntar para a viagem | Valor guardado em relação ao valor e ao prazo (liga com Finanças) |
| Hábito | Treinar 3 vezes por semana | Marcação a cada ocorrência, com contagem da semana |
| Do casal | Fazer um jantar fora por mês | Os dois acompanham e marcam |
| Projeto | Organizar o quarto novo | Etapas, que podem virar tarefas |

**Regras**

- Toda meta é pessoal ou do casal; a pessoal só o dono vê, a menos que escolha compartilhar.
- Metas têm prazo opcional e podem ser concluídas, pausadas ou arquivadas.
- Etapas de um projeto viram tarefas, e o progresso da meta acompanha as tarefas concluídas.
- Hábitos geram um lembrete opcional no horário que a pessoa escolher.
- O histórico mostra o progresso, sem ranking nem cobrança entre os dois.

## Jornadas principais do casal

Quatro jornadas mostram como o produto entra na vida do casal, e cada uma deve funcionar de ponta a ponta antes do lançamento.

**Jornada 1: começar a usar**

1. Uma pessoa cria a conta e o casal.
2. Escolhe os módulos que quer usar.
3. Monta sua semana típica com poucos blocos.
4. Convida o parceiro por link.
5. O parceiro entra, monta a própria semana e os dois veem os horários livres em comum.

**Jornada 2: deixar um lembrete para o outro**

1. Ela abre Lembretes e escreve "comprar pão".
2. Escolhe o parceiro como destinatário e define "hoje às 17h".
3. Ele recebe o aviso no horário e toca em "feito" ao comprar.
4. Ela é avisada de que foi resolvido.

**Jornada 3: semana de marmitas**

1. Na quinta, o casal escolhe as receitas da semana seguinte.
2. O app gera a lista de compras e o casal compra no sábado.
3. No domingo, o dia de cozinhar aparece na rotina; os dois seguem o roteiro dividido.
4. Ao terminar, registram as porções que foram para o freezer.
5. Durante a semana, recebem o lembrete de descongelar na noite anterior e dão baixa ao consumir.
6. Na semana seguinte, o app mostra o que sobrou e sugere o novo cardápio.

**Jornada 4: acertar as contas do mês**

1. Durante o mês, cada um registra os gastos do casal.
2. No fechamento, o app mostra quanto cada um gastou e quanto um deve ao outro.
3. Quem deve faz o pagamento por fora do app e registra o acerto.
4. O saldo zera e o mês seguinte começa.

## Regras de negócio

As regras abaixo definem o comportamento esperado do produto e servem de base para os critérios de aceite de cada funcionalidade.

| Área | Regra |
| --- | --- |
| Casal | Um casal tem exatamente duas pessoas; cada pessoa pertence a um casal por vez; enquanto o convite não é aceito, o casal tem uma pessoa |
| Convite | O link de convite vale por um prazo (sugestão: 7 dias), pode ser reenviado e é invalidado depois de aceito |
| Saída | Quem sai do casal perde acesso aos dados compartilhados; o parceiro que fica mantém tudo |
| Permissões | Qualquer um dos dois edita o que é do casal; o que é pessoal só o dono edita |
| Privacidade | Cada item pessoal tem uma visibilidade escolhida pelo dono (compartilhado, só "ocupado" ou privado); o padrão é o mais restrito |
| Módulos | Desativar um módulo o esconde, mas não apaga os dados; ao reativar, tudo volta |
| Fuso horário | Cada pessoa tem o seu, deduzido da cidade que ela informa; lembretes disparam no fuso do destinatário |
| Lembretes | Um lembrete só deixa de aparecer quando resolvido ou apagado por quem criou |
| Lista de compras | Itens iguais são somados; itens comprados saem da lista após um período |
| Cardápio | Mudar uma refeição já planejada atualiza a lista de compras e avisa o que mudou |
| Freezer | A baixa de uma marmita nunca deixa o estoque negativo; correções são manuais |
| Validade | A validade é uma sugestão baseada na receita e pode ser alterada pelo casal |
| Finanças | Gastos pessoais não entram no acerto de contas do casal |
| Moeda | Real brasileiro (BRL) no início; outras moedas ficam para depois |
| Idioma | Português do Brasil no início |
| Exclusão de conta | A pessoa pode apagar sua conta; os dados do casal permanecem com o parceiro, e os dados pessoais são removidos |

## Telas e navegação

O app é pensado para o celular, com cinco áreas principais sempre à mão e os módulos opcionais aparecendo só quando ativados.

**Navegação principal**

| Área | Para que serve | Módulo |
| --- | --- | --- |
| Hoje | Resumo do dia do casal | Base |
| Semana | Rotina dos dois, horários livres e quem está onde | Base |
| Lembretes | Recados entre os dois e lembretes automáticos | Base |
| Casa | Compras, tarefas, anotações, receitas, cardápio, freezer, finanças e metas | Base + módulos |
| Perfil | Conta, casal, módulos, avisos e privacidade | Base |

**Telas por área**

| Área | Telas |
| --- | --- |
| Hoje | Resumo do dia, lembretes pendentes, refeição do dia (Marmita), contas do dia (Finanças) |
| Semana | Visão semanal, visão do dia, horários livres, criar ou editar bloco, exceções |
| Lembretes | Lista (recebidos e enviados), criar lembrete, detalhe e adiar |
| Casa | Lista de compras e modo mercado, tarefas, receitas, cardápio da semana, dia de cozinhar, freezer, gastos, acerto de contas, metas |
| Perfil | Dados pessoais, convite e dados do casal, ativar módulos, preferências de aviso, privacidade, ajuda |

**Diretrizes de experiência**

- Toda ação frequente (criar lembrete, adicionar item, marcar como feito) em no máximo três toques.
- Cor e nome identificam de quem é cada coisa, para ninguém precisar ler detalhes.
- Estados vazios explicam o próximo passo, em vez de mostrar telas em branco.
- Funciona bem com uma mão e com letras legíveis, inclusive no mercado e na cozinha.
- Texto em português simples, sem termos técnicos.

## Notificações

Os avisos no celular são a parte mais importante da experiência, então o produto deve enviá-los só quando fazem diferença e permitir que cada um controle o que recebe.

| Aviso | Quem recebe | Quando |
| --- | --- | --- |
| Lembrete recebido | Destinatário | Ao ser criado, se for para daqui a pouco, ou no horário marcado |
| Lembrete resolvido | Quem criou | Quando o destinatário marca como feito |
| Convite aceito | Quem convidou | Quando o parceiro entra |
| Tarefa atribuída | Responsável | Ao ser atribuída |
| Descongelar marmita | Responsável pela refeição | Na noite anterior, em horário definido pelo casal |
| Marmita perto do vencimento | Os dois | Alguns dias antes da validade sugerida |
| Dia de cozinhar | Os dois | Na véspera e algumas horas antes |
| Conta a vencer | Responsável pelo pagamento | Dias antes e no dia do vencimento |
| Limite de categoria | Os dois | Ao se aproximar ou ultrapassar o limite |
| Resumo da semana | Os dois | Domingo à noite, com o que vem pela frente |

**Controles**

- Cada tipo de aviso pode ser ligado ou desligado por pessoa.
- Horário de silêncio configurável (por exemplo, durante o sono), exceto para lembretes marcados como importantes.
- Avisos agrupados: se vários chegarem juntos, viram um só resumo.

**Alternativa**

Quando o celular não permite avisos, os lembretes continuam aparecendo na tela Hoje e o app orienta a pessoa a ativá-los, com um passo a passo para cada tipo de aparelho.

## Privacidade e confiança

O app guarda a rotina, a casa e o dinheiro de duas pessoas, então a confiança é requisito do produto e não um extra.

**Compromissos**

- Os dados do casal só são vistos pelas duas pessoas do casal.
- O que é pessoal fica pessoal, e o parceiro vê apenas o que a pessoa escolher compartilhar.
- O app não vende dados nem usa dados do casal para publicidade.
- Cada pessoa pode exportar seus dados e apagar sua conta quando quiser.
- Linguagem clara sobre o que é guardado e por quê, sem textos jurídicos longos na tela principal.

**Pontos de atenção**

| Tema | Cuidado de produto |
| --- | --- |
| Dados financeiros | Registro manual no início, sem acesso a contas bancárias; avisos claros sobre o que é guardado |
| Localização | Opcional e informada pela própria pessoa ("estou na academia"), nunca rastreada automaticamente |
| Fim do relacionamento | Fluxo respeitoso para sair do casal, com opção de levar cópia dos dados compartilhados |
| Convite | Link com prazo e uso único, para ninguém entrar no casal por engano |
| Segurança de alimentos | A validade é uma sugestão, e o app deixa claro que a decisão final é de quem consome |
| Legislação | Atender à lei brasileira de proteção de dados (LGPD): consentimento, acesso, correção e exclusão |

**Situações delicadas a prever**

- Um dos dois quer sair e o outro ainda precisa dos dados compartilhados.
- Um relacionamento termina e as contas continuam ligadas.
- Uma pessoa usa a visibilidade do app para controlar a outra: o produto deve favorecer escolhas explícitas de compartilhamento, sem recursos de vigilância.

## Modelo de negócio como SaaS

O plano é usar o app primeiro em casa, validar com outros casais e só então cobrar por assinatura, com um plano gratuito que prove o valor antes do pagamento.

**Hipótese de planos (a validar)**

| Plano | Para quem | O que inclui |
| --- | --- | --- |
| Gratuito | Casal que está começando | Rotina, lembretes, lista de compras e tarefas, com limites de uso |
| Casal (pago) | Casal que usa no dia a dia | Sem limites, módulo Marmita completo, histórico longo e resumos semanais |
| Casal Plus (pago) | Casal que quer controle financeiro | Tudo do plano Casal, mais o módulo Finanças com metas e orçamento |

Os valores e os limites exatos ficam em aberto até haver testes com casais reais.

**Princípios de cobrança**

- Cobrança por casal, não por pessoa: uma assinatura vale para os dois.
- O gratuito precisa ser realmente útil, porque o convite do parceiro é o principal canal de crescimento.
- O casal nunca perde acesso aos próprios dados ao cancelar: eles ficam disponíveis para consulta e exportação.
- Opção de pagamento mensal e anual, com desconto no anual.

**Crescimento**

- Boca a boca entre casais, reforçado pelo convite ao parceiro.
- Conteúdo para quem faz marmita e meal prep: comunidades, grupos e criadores desse nicho.
- Receitas e modelos de cardápio prontos que o casal pode importar.

## Concorrência e diferenciais

Na pesquisa de 24/10/2026 não apareceu nenhum app que una rotina do casal, marmita com freezer e finanças em português; os concorrentes cobrem uma parte cada.

| Concorrente | O que faz | Limite em relação a este produto |
| --- | --- | --- |
| [Cupla](https://apps.apple.com/app/cupla/id1557764033) | Agenda compartilhada, tarefas e lembretes para casais | Sem cardápio, freezer nem finanças |
| [Kyupid](https://apps.apple.com/us/app/-/id1554721944) | Tarefas, lista de compras e rotinas para casais | Sem cardápio, freezer nem finanças |
| [Honeydue](https://techcrunch.com/2017/08/07/honeydue-is-a-money-management-app-for-couples/?ncid=rss) | Finanças para casais | Só finanças; foco no mercado americano |
| [Noh](https://apps.apple.com/us/app/noh-conta-conjunta-do-casal/id1609125328) | Conta conjunta digital para casais no Brasil | É um serviço bancário, sem rotina nem casa |
| [Plan to Eat](https://learn.plantoeat.com/doc/app-freezer-cooking) | Planejador de refeições com freezer e lembrete para descongelar | Sem agenda do casal nem finanças; em inglês |
| [NoWaste](https://apps.apple.com/app/id926211004) | Estoque de freezer, geladeira e despensa, com planejador | Sem agenda do casal; foco no estoque |

**Diferenciais pretendidos**

1. **Tudo conversa:** o cardápio conhece a agenda do casal, a compra vira gasto e o lembrete nasce do que foi planejado.
2. **Marmita e freezer como recurso central**, em um app de casal, algo que os apps de casal não oferecem.
3. **Feito para o Brasil:** idioma, ingredientes, medidas e, no futuro, formas de pagamento e bancos locais.
4. **Módulos opcionais:** quem não faz marmita usa a base sem peso extra.

**Riscos competitivos**

- Os apps de casal já bem estabelecidos podem adicionar listas de compras ou cardápio.
- A parte de rotina e lembretes é a mais disputada; o produto precisa se destacar pela integração, não por ela isolada.
- Os dados desta seção vêm das páginas das lojas de aplicativos e de artigos; vale revisitá-los antes de decisões de investimento.

## Roadmap por fases

O produto avança em cinco fases, e cada fase só começa quando a anterior cumpre seu critério de saída.

1. **Fase 0: validar o essencial.** Provar que os lembretes chegam de forma confiável no celular dos dois, incluindo iPhone. *Saída:* um lembrete criado por um chega ao outro no horário, de forma repetida, durante uma semana.
2. **Fase 1: base para o casal.** Conta, vínculo do casal, rotina semanal com horários livres, tarefas (inclusive para o outro), anotações, lembretes e lista de compras. A ordem de construção é rotina, tarefas e anotações primeiro. *Saída:* o casal criador usa a rotina, as tarefas e os lembretes todos os dias por duas semanas.
3. **Fase 2: módulo Finanças.** Gastos, acerto de contas, contas a pagar e orçamento (spec em `docs/spec-fase-2-financas.md`). As metas financeiras ficam no módulo Metas. *Saída:* o casal fecha um mês de acerto de contas pelo app.
4. **Fase 3: módulo Metas.** Metas financeiras (que leem os gastos da Fase 2), de hábitos, do casal e projetos com etapas. *Saída:* o casal acompanha pelo app ao menos uma meta de cada tipo por um mês.
5. **Fase 4: módulo Marmita.** Receitas, cardápio, lista automática, dia de cozinhar e freezer (spec em `docs/spec-fase-4-marmita.md`). Fica por último por decisão do casal. *Saída:* o casal faz pelo menos duas semanas de marmitas usando o app do início ao fim.
6. **Fase 5: abrir para outros casais.** Convite de teste para 5 a 10 casais, ajustes e definição dos planos pagos. *Saída:* a maioria dos casais de teste continua usando após quatro semanas.

**O que fica de fora até a Fase 5**

- Ligação com bancos, pagamentos dentro do app e importação automática de receitas.
- Famílias com filhos e grupos de mais de duas pessoas.
- Outros idiomas e moedas.
- Integração com agendas externas, a avaliar depois do uso real.

## Métricas de sucesso

A principal medida de sucesso é o casal voltar ao app todas as semanas por escolha própria, e as demais métricas ajudam a entender por quê.

| Métrica | O que mede | Sinal de sucesso (a calibrar) |
| --- | --- | --- |
| Casais ativos por semana | Casais em que os dois abriram o app | Crescimento constante e retenção após 4 semanas |
| Convite aceito | Parceiros que entram depois do convite | Maioria dos convites aceitos em poucos dias |
| Tempo até o primeiro valor | Do cadastro à primeira semana montada | Menos de 5 minutos |
| Lembretes resolvidos | Lembretes marcados como feitos em relação aos criados | Alta taxa de resolução no mesmo dia |
| Uso do módulo Marmita | Casais que fazem pelo menos um ciclo completo | Maioria dos que ativam o módulo |
| Desperdício no freezer | Marmitas vencidas sem consumo | Queda ao longo das semanas |
| Módulos ativados | Quantos módulos cada casal usa | Mais de um por casal |
| Conversão para o plano pago | Casais que passam a pagar | A definir após os testes |

As metas numéricas serão definidas depois das primeiras semanas de uso real, para não nascerem de suposições.

## Riscos e perguntas em aberto

Os maiores riscos são o uso real dos avisos no celular e o tamanho do público que faz marmita, e as perguntas abaixo precisam de resposta do casal antes de desenhar as telas.

**Riscos**

| Risco | Impacto | Como reduzir |
| --- | --- | --- |
| Os lembretes não chegam de forma confiável, principalmente no iPhone | Alto: lembretes são a base do produto | Validar na Fase 0 antes de construir o resto |
| O público de marmita é pequeno | Médio: limita o crescimento | Manter a base útil para qualquer casal |
| O casal desiste por ter de preencher muita coisa | Alto | Onboarding curto e valor logo no primeiro dia |
| Concorrentes de casal ganham cardápio ou compras | Médio | Apostar na integração e no mercado brasileiro |
| Dados financeiros e de convivência geram desconforto | Médio | Privacidade explícita e registro manual no início |
| Escopo grande demais para o tempo disponível | Alto | Seguir as fases e entregar uma por vez |

**Perguntas em aberto**

- [ ] Qual será o nome do produto?
- [ ] Quais celulares o casal usa hoje, para planejar os avisos?
- [ ] Quais refeições o casal quer planejar (almoço, jantar, lanche)?
- [x] A divisão de gastos será igual ou proporcional à renda? Decidido: os dois. O padrão é igual; o casal pode combinar um percentual fixo ou usar a divisão proporcional, que exige a concordância dos dois. A renda é guardada, mas só a própria pessoa a vê.
- [ ] Quem entra no teste da Fase 5 (amigos, colegas, comunidade de meal prep)?
- [ ] Uma nota pessoal pode ser compartilhada com o parceiro só para leitura ou também para edição?
- [ ] Metas de hábito: marcação manual a cada ocorrência basta, ou o app deve sugerir a marcação pela rotina?
- [x] Haverá receitas prontas no início ou o catálogo começa vazio? Decidido: começa vazio, e o casal cadastra as próprias receitas.
- [ ] Qual o limite do plano gratuito?
- [ ] Quanto tempo por semana o casal pode dedicar ao desenvolvimento?
