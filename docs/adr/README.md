# Architecture Decision Records

Cada arquivo registra uma decisão de arquitetura: o contexto, o que foi decidido, as alternativas e as consequências. Uma decisão aceita não é editada para mudar de ideia; uma decisão nova que a substitui ganha um ADR próprio.

| ADR | Decisão | Status |
|---|---|---|
| [001](0001-SAGA-HIBRIDA.md) | SAGA híbrida para a compra de ingressos | aceita |
| [002](0002-TRAEFIK-GUARD.md) | Entrada pelo Traefik com um Guard próprio em Go | aceita |
| [003](0003-STACK.md) | Stack tecnológica | aceita |
| [004](0004-CONTROLE-DO-HOLD.md) | Controle do hold com UPDATE condicional no Postgres | aceita |
| [005](0005-SCHEMA-POR-SERVICO.md) | Um Postgres com um schema por serviço | aceita |
| [006](0006-REDIS-STREAMS.md) | Redis Streams para os eventos e Redis com três papéis | aceita |
| [007](0007-KEYCLOAK.md) | Keycloak como servidor de identidade | aceita |
| [010](0010-OBSERVABILIDADE.md) | Observabilidade com Prometheus, Jaeger e Grafana, instrumentados por OpenTelemetry | aceita |
| [011](0011-KIT-DE-AVALIACAO.md) | Compatibilidade com o kit de avaliação da disciplina | aceita |

Os números 008 (Circuit Breaker, issue #38) e 009 (Blue-Green e Canary, issue #49) estão reservados para as decisões que entram com essas issues.
