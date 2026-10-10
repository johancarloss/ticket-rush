# Ticket Rush

Projeto final de Engenharia de Sistemas Distribuídos (UFPB, 2026.2), Prof. Raoni Kulesza.

Sistema de venda de ingressos com reserva de assentos para aberturas de venda de alta demanda. Milhares de compradores disputam poucos assentos ao mesmo tempo, a compra passa por vários serviços que podem falhar no meio do caminho, e bots de cambistas tentam reservar em massa. O objetivo é que nenhum assento seja vendido duas vezes, que nenhuma reserva fique presa quando um serviço cai, e que os bots sejam contidos sem prejudicar os compradores reais.

A proposta completa está em [`docs/proposta.md`](docs/proposta.md).

## Status

| Entrega | Data | Situação |
|---|---|---|
| Projeto 01: definição do grupo e tema | 09/10/2026 | Proposta enviada |
| Projeto 02: documentação inicial (C4, ADRs, stack, setup) | 06/11/2026 | Pendente |
| Projeto 03: documentação final, código e testes | 11/12/2026 | Pendente |

## Equipe

| Nome | Matrícula | E-mail |
|---|---|---|
| Bruno Almeida dos Santos | 20230013099 | almeidabruno1506@gmail.com |
| Diego de Carvalho Dias | 20230089696 | diegoddias007@gmail.com |
| Johan Carlos Pereira Fernandes | 20230126771 | johancarlos62@gmail.com |
| Luis Antonio Feliciano da Silva | 20230012752 | luis.feliciano@academico.ufpb.br |

## Tecnologias previstas

TypeScript (Node.js) e Go, PostgreSQL, Redis, Docker Compose, GitHub Actions, Prometheus, Grafana, Jaeger e k6.

## Como executar

Instruções serão adicionadas a partir do Projeto 02, junto com o setup do Docker Compose.

## Ferramentas de IA utilizadas

Esta seção é atualizada ao longo do projeto.

| Ferramenta | Onde atuou | Como foi orientada | Avaliação |
|---|---|---|---|
| Claude (Claude Code) | Leitura do enunciado, discussão do tema e redação da proposta do Projeto 01 | O grupo forneceu o PDF do enunciado e discutiu as opções de tema, padrões e tecnologias; as decisões foram tomadas pelo grupo a partir das alternativas apresentadas | A comparação entre temas e a lista de padrões foram úteis. Detalhes técnicos que ele trouxe cedo demais (como o mecanismo de bloqueio do assento) foram adiados para os ADRs do Projeto 02 |
