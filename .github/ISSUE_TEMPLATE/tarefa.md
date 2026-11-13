---
name: Tarefa
about: Uma tarefa do projeto, do tamanho de um pull request
title: "[Área] "
labels: []
---

**Responsável:** @usuario · **Parte de:** #épico
**Fase N, até DD/MM**

## Contexto
Por que esta tarefa existe. Cite o ADR, o C4 ou o contrato de onde ela vem.

## O que fazer
- [ ] Passo concreto
- [ ] Passo concreto

## Fora do escopo
O que fica para outra tarefa, para esta não crescer.

## Contrato
Rotas, payloads ou tabelas que outros serviços vão usar. Apague a seção se não houver.

## Depende de
- #N (também marcar em *Relationships → Blocked by* e pôr o label `bloqueada`)

## Integra com (dá para começar com mock do contrato)
- #N

## Libera
- #N

## Pronto quando
- [ ] Critério que dá para testar

## Como testar
Comando ou passo a passo.

## Documentar ao fechar
- [ ] Arquivo do repositório a atualizar

---
Como pegar, fazer e fechar uma tarefa: [docs/tarefas/README.md](https://github.com/johancarloss/ticket-rush/blob/develop/docs/tarefas/README.md#ciclo-de-uma-tarefa)
