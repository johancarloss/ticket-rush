# Padrões arquiteturais

Padrões da seção 5 do enunciado aplicados no Ticket Rush, com o problema que cada um resolve e onde a decisão está registrada.

## Padrões

| Área | Padrão | Problema que resolve no projeto | Onde |
|---|---|---|---|
| Confiabilidade | SAGA com orquestração | A compra passa por Inventário e Pagamento, com bancos separados; se o pagamento falhar depois da reserva, o assento precisa ser liberado sem uma transação única | [ADR-001](adr/0001-SAGA-HIBRIDA.md) |
| Confiabilidade | Coreografia | Depois da compra, Notificação e Estatísticas reagem ao evento `TicketIssued` sem o Checkout saber quem são | [ADR-001](adr/0001-SAGA-HIBRIDA.md), [ADR-006](adr/0006-REDIS-STREAMS.md) |
| Confiabilidade | Transactional Outbox | O Checkout pode cair entre gravar CONFIRMED e publicar o evento; o evento é gravado na mesma transação e publicado depois | [ADR-001](adr/0001-SAGA-HIBRIDA.md) |
| Confiabilidade | Retry e DLQ | Um consumidor que falha não pode perder a mensagem nem travar as outras; após 3 falhas ela vai para um stream de mensagens mortas | [ADR-006](adr/0006-REDIS-STREAMS.md) |
| Desempenho | Queues / PubSub / Messaging Fanout | Um evento, vários consumidores, cada um no seu ritmo (consumer groups do Redis Streams) | [ADR-006](adr/0006-REDIS-STREAMS.md) |
| Desempenho | Cache-Aside | Milhares de compradores consultando o mapa de assentos ao mesmo tempo | [ADR-004](adr/0004-CONTROLE-DO-HOLD.md) |
| Desempenho | Rate Limit / Throttling | Bots de cambistas martelando as rotas de compra; limite por conta no Guard | [ADR-002](adr/0002-TRAEFIK-GUARD.md) |
| Desempenho | Database-per-service (isolamento lógico) | Serviços acoplados pelo banco; cada serviço tem schema e usuário próprios | [ADR-005](adr/0005-SCHEMA-POR-SERVICO.md) |
| Segurança | API Gateway | Ponto único de entrada que autentica e limita antes de chegar nos serviços | [ADR-002](adr/0002-TRAEFIK-GUARD.md) |
| Segurança | OAuth 2.0, OpenID Connect e JWT | Saber quem é o comprador sem que todo serviço possa criar tokens | [ADR-007](adr/0007-KEYCLOAK.md) |
| Deployment | Blue-Green e Canary | Publicar versões novas durante a venda sem derrubar o sistema, usando a divisão de tráfego por peso do Traefik | [ADR-002](adr/0002-TRAEFIK-GUARD.md) (detalhes no setup do Projeto 02/03) |

## Previstos, ainda sem ADR

| Área | Padrão | Uso previsto |
|---|---|---|
| Confiabilidade | Circuit Breaker | No Checkout, para a chamada síncrona ao Inventário e ao Pagamento: se o outro lado não responde, falhar rápido em vez de prender conexões ([C4 nível 2](architecture/C4-2-CONTAINERS.md)) |
| Segurança | Observabilidade de segurança | Métricas, logs e traces de todos os serviços no Prometheus, Grafana e Jaeger, com alertas (por exemplo, pedidos aguardando estorno) |
| Segurança | DevSecOps | SAST, análise de dependências e de imagens no pipeline de CI |

## Conceitos usados como justificativa

Itens da seção 5 que não são padrões a implementar, mas aparecem nas decisões:

- **Sync vs Async:** síncrono quando alguém espera a resposta para decidir (Checkout → Inventário), assíncrono quando é só avisar que algo aconteceu (eventos pós-compra). Ver o [C4 nível 2](architecture/C4-2-CONTAINERS.md).
- **Strong vs Eventual Consistency:** a reserva do assento é forte (`UPDATE` condicional no Postgres); a leitura do mapa é eventual (cache de cerca de 2 segundos); os passos da SAGA são eventualmente consistentes entre si. Ver [ADR-001](adr/0001-SAGA-HIBRIDA.md) e [ADR-004](adr/0004-CONTROLE-DO-HOLD.md).
- **Monolito vs Microsserviços:** a separação em serviços segue a regra de que cada serviço é dono de um dado e só ele escreve nesse dado.
- **Stateless e Stateful:** Guard e serviços HTTP sem estado em memória (podem ter réplicas); o estado fica no Postgres e no Redis.
