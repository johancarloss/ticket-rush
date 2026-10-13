# Changelog

Mudanças notáveis do projeto, agrupadas por versão. Cada versão corresponde a um marco concluído, e o texto da versão indica a qual Projeto da disciplina ela pertence. Formato baseado em [Keep a Changelog](https://keepachangelog.com/pt-BR/1.1.0/).

## [Unreleased]

## [0.3.0] - 2026-10-13

Infraestrutura e entrada do sistema rodando, parte do setup do Projeto 02. Os serviços da aplicação entram nas próximas versões.

### Funcionalidades
- Guard em Go respondendo ao `forwardAuth` do Traefik: confere os tokens do Keycloak (RS256, assinatura, emissor, audiência e validade) e repassa `X-User-Id` e `X-User-Roles` ao serviço. Duas réplicas; se as duas caírem, a compra é bloqueada.
- Keycloak com o realm `ticket-rush` importado de um arquivo versionado: papéis `buyer` e `organizer`, client `ticket-rush-web` e usuários de teste.

### Infraestrutura
- Docker Compose com PostgreSQL, Redis, Traefik, Keycloak e Guard, todos com healthcheck.
- PostgreSQL com um schema e um usuário por serviço, cada usuário restrito ao próprio schema, e um database separado para o Keycloak.
- Redis com AOF ligado.
- Redes separadas: `edge` para o que passa pelo Traefik e `data` para bancos.
- Rota temporária `/api/whoami` para testar o Guard de ponta a ponta.

### Segurança
- Traefik apaga os cabeçalhos `X-User-Id` e `X-User-Roles` mandados pelo cliente antes de consultar o Guard.
- Guard compilado com Go 1.27.2, que corrige 9 vulnerabilidades da biblioteca padrão encontradas pelo `govulncheck`.

### CI
- Pipeline em todo PR e push: testes, `go vet`, detector de corrida e `govulncheck` do Guard; validação do Compose; `gitleaks` no histórico inteiro; links da documentação.
- Actions fixadas por hash de commit e gitleaks com checksum conferido.
- Dependabot para Go, Docker e Actions, com 14 dias de espera para versões novas.

### Documentação
- `docs/infra/`: como executar, PostgreSQL, Redis, Traefik, Keycloak, Guard e CI.
- `docs/architecture/EVOLUCAO.md` com a arquitetura versão por versão.
- Guia de criação de serviço em `docs/guides/NOVO-SERVICO.md`.
- Nomes dos documentos em maiúsculas.

## [0.2.0] - 2026-10-13

Documentação de arquitetura do Projeto 02. O setup (Docker Compose e CI) entra na próxima versão.

### Documentação
- Diagrama C4 nível 1 (contexto) e nível 2 (containers).
- ADRs 001 a 007: SAGA híbrida, Traefik com Guard, stack, controle do hold, schema por serviço, Redis Streams e Keycloak.
- Contrato do evento `TicketIssued`.
- Lista de padrões arquiteturais com o problema que cada um resolve e o ADR correspondente.
- Guia de contribuição com o fluxo de branches, commits e pull requests do grupo.
- README com arquitetura, responsabilidades e índice da documentação.

## [0.1.0] - 2026-10-09

Projeto 01: definição do grupo e tema.

### Documentação
- README inicial com descrição, equipe e tecnologias previstas.
- Proposta do projeto.
