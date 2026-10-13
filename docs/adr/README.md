# Architecture Decision Records

Cada arquivo registra uma decisão de arquitetura: o contexto, o que foi decidido, as alternativas e as consequências. Uma decisão aceita não é editada para mudar de ideia; uma decisão nova que a substitui ganha um ADR próprio.

| ADR | Decisão | Status |
|---|---|---|
| [001](0001-saga-hibrida.md) | SAGA híbrida para a compra de ingressos | aceita |
| [002](0002-traefik-guard.md) | Entrada pelo Traefik com um Guard próprio em Go | aceita |
| [003](0003-stack.md) | Stack tecnológica | aceita |
| [004](0004-controle-do-hold.md) | Controle do hold com UPDATE condicional no Postgres | aceita |
| [005](0005-schema-por-servico.md) | Um Postgres com um schema por serviço | aceita |
| [006](0006-redis-streams.md) | Redis Streams para os eventos e Redis com três papéis | aceita |
| [007](0007-keycloak.md) | Keycloak como servidor de identidade | aceita |
