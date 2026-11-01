# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Stack

React + Vite + TypeScript, entregue como PWA instalável. A pasta `frontend/` é servida pelo backend Go (em produção, o `dist` embutido no binário). Estilo com CSS Modules e tokens em variáveis CSS, primitivos acessíveis do Radix, TanStack Query para dados do servidor e react-i18next para os idiomas pt-BR e en.

## Users

Uma pessoa de um casal que mora junto, com rotina diferente da do parceiro. Abre o app no celular, muitas vezes em pé ou em trânsito, várias vezes ao dia, para saber em segundos o que o dia pede e deixar um recado ao outro. Também usa o desktop para planejar (rotina, cardápio, finanças), então telas largas pesam de verdade, embora o celular venha primeiro. Cada pessoa tem sua conta; as duas contas se ligam por convite a um mesmo casal.

## Product Purpose

Organiza a vida a dois em um só lugar: quem está onde, o que precisa ser feito, o que vão comer e para onde vai o dinheiro. A promessa é abrir o app e saber, em poucos segundos, o que o dia do casal exige. Sucesso é o casal voltar toda semana por escolha própria. Hoje o backend entrega conta, casal, convite, lembretes e push; rotina, compras/tarefas, Marmita e Finanças vêm em módulos seguintes, opcionais.

## Positioning

O produto une o que os concorrentes resolvem em separado (agenda e tarefas, finanças, marmita e freezer) e fala a realidade do casal brasileiro. O diferencial é a ligação entre as partes: o cardápio sabe da agenda, a lista de compras alimenta as finanças e o lembrete nasce do que foi planejado.

## Operating Context

- Lembretes entre o casal têm ciclo próprio: criado, disparado, resolvido, adiado ou não resolvido. Quem recebe sempre vê quem criou, e não pode apagar sem resolver.
- Os avisos chegam por Web Push, inclusive com o app fechado; no iPhone isso só funciona com o app instalado na tela inicial, então a instalação faz parte do onboarding.
- Enquanto o convite não é aceito, o app funciona em modo individual.
- Cada conta pertence a um único casal por vez, de exatamente duas pessoas.
- Sessão por cookie na mesma origem; o fuso horário vem do perfil de cada pessoa.

## Capabilities and Constraints

- Idiomas: pt-BR (padrão) e inglês desde o início. Todo texto passa por chaves de tradução; o backend deve devolver códigos de erro estáveis, não frases.
- O nome do app é indefinido; deve existir em um único lugar de configuração para ser trocado depois.
- Módulos são opcionais e ligáveis por casal; a navegação precisa aceitar módulos que ainda não existem sem parecer vazia.
- O app não é rede social, não tem jogos nem ranking entre o casal, e não é um banco.
- O app nunca afirma que um alimento é seguro para consumo (módulo Marmita).

## Brand Commitments

Preferência declarada pelo usuário: tons de laranja, tema escuro e claro, experiência fluida. Nome provisório "Casal". Sem logo ainda; os ícones atuais em `web/` são provisórios.

## Evidence on Hand

- `docs/produto.md`: visão, personas, módulos, jornadas, regras e telas previstas.
- API real: autenticação, casal, convite, lembretes e push (rotas em `internal/*/handler.go`).
- Não há conteúdo real de receitas, rotinas ou finanças; nenhum depoimento ou número de uso. Nada disso deve ser inventado na interface.

## Product Principles

1. O casal é a unidade; cada pessoa também tem seu espaço.
2. Simples de dois: poucos toques para o que se repete todo dia.
3. Útil no primeiro dia, antes de o casal preencher muita coisa.
4. Módulos opcionais: o que não se usa não atrapalha.
5. Transparência com privacidade: compartilha-se o que se quer, o pessoal pode continuar pessoal.

## Accessibility & Inclusion

WCAG AA como piso nos dois temas: contraste, foco visível, navegação por teclado, leitor de tela em diálogos e alvos de toque de pelo menos 44 px. Respeitar `prefers-reduced-motion` e `prefers-color-scheme`. Interface completa em pt-BR e inglês, com datas e horas formatadas pelo fuso do perfil.
