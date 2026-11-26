# Notificação

A Notificação envia ao comprador o e-mail de confirmação da compra. Ela não recebe chamadas de ninguém: lê o evento `TicketIssued` que o Checkout publica no Redis Streams quando um pedido chega a `CONFIRMED` ([ADR-001](../adr/0001-SAGA-HIBRIDA.md), [ADR-006](../adr/0006-REDIS-STREAMS.md)). Assim, o comprador vê a compra confirmada sem esperar pelo e-mail, e a Notificação pode cair sem afetar nenhuma compra.

Código em [`services/notification/`](../../services/notification/), escrito em Go.

## Estado atual

| Versão | O que faz |
|---|---|
| **Esqueleto (atual)** | Lê o stream pelo consumer group, confere o formato do evento, registra o pedido no log e confirma com `XACK` |
| [#41](https://github.com/johancarloss/ticket-rush/issues/41) | Envia o e-mail por SMTP, ignora pedido já notificado, reprocessa pendentes de réplicas que caíram e manda para a DLQ depois de 3 falhas |
| [#43](https://github.com/johancarloss/ticket-rush/issues/43) | Métricas do consumidor e o trace da compra continuando na Notificação |

## Como lê os eventos

| | |
|---|---|
| Stream | `tickets.issued` ([EVENTOS.md](../contracts/EVENTOS.md)) |
| Consumer group | `notification` |
| Nome do consumidor | O hostname do container, diferente em cada réplica |

1. Ao subir, cria o grupo com `XGROUP CREATE tickets.issued notification 0 MKSTREAM`. O `0` faz o grupo começar do início do stream, então eventos publicados antes de a Notificação existir também são processados. Se o grupo já existe (o caso normal), segue em frente.
2. Antes de ler mensagens novas, relê as que **este consumidor** recebeu e não confirmou, caso tenha caído no meio de uma. Cada uma é tentada uma vez por subida, para uma mensagem que sempre falha não travar as seguintes.
3. Depois, lê as mensagens novas com `XREADGROUP ... BLOCK 2000 STREAMS tickets.issued >`, até 10 por vez.
4. Só confirma com `XACK` depois de processar. Uma mensagem que falhou continua na lista de pendentes do grupo.

O evento é recusado (fica pendente, com o motivo no log) quando o JSON é inválido, o `type` não é `TicketIssued`, a `version` não é 1, ou falta `order_id`, `buyer_email`, `event.id` ou a lista de assentos. Campos desconhecidos são ignorados, como o contrato pede.

**Limite do esqueleto:** uma mensagem que falhou fica pendente até uma nova subida do mesmo consumidor. Como o nome do consumidor é o hostname, um container recriado tem outro nome e não relê as pendentes do anterior. Pegar as pendentes de outros consumidores (`XAUTOCLAIM`) e a DLQ entram na [#41](https://github.com/johancarloss/ticket-rush/issues/41).

**Redis fora:** o serviço continua de pé e tenta de novo, com espera crescente de 0,5 s até 10 s. Nada se perde, porque as mensagens ficam no stream (com AOF, [REDIS.md](../infra/REDIS.md#persistência)).

**Desligamento:** no `SIGTERM`, termina as mensagens que já leu, para de ler e só então sai.

## Rotas

Nenhuma rota no Traefik. As duas rotas existem só para a rede interna:

| Rota | Quem chama | Resposta |
|---|---|---|
| `GET /health` | Healthcheck do Docker (o binário chama a si mesmo com `-healthcheck`, porque a imagem não tem shell) | `200 {"status":"ok"}` |
| `GET /metrics` | Prometheus (job `notification`) | Métricas do runtime Go e do processo |

## Variáveis de ambiente

| Variável | Padrão | Valor no Compose |
|---|---|---|
| `PORT` | `8080` | (padrão) |
| `REDIS_ADDR` | `redis:6379` | `redis:6379` |

Nenhuma variável nova no `.env`. A senha `NOTIFICATION_DB_PASSWORD` já existe e passa a ser usada na [#41](https://github.com/johancarloss/ticket-rush/issues/41), com a tabela de pedidos notificados.

## Logs

Uma linha JSON por evento. O e-mail do comprador não vai para o log, porque o ID do pedido basta para achar a compra.

```json
{"level":"INFO","msg":"ticket issued received","message_id":"1795730181485-0","event_message_id":"0b7c6f0e-5d1a-4c3e-9f2b-8a1d2e3f4a5b","order_id":"7","event_id":"show-2026-12-01","seats":2,"trace_id":"4bf92f3577b34da6a3ce929d0e0e4736"}
{"level":"ERROR","msg":"message failed, left pending","message_id":"1795730181563-0","error":"invalid TicketIssued payload: invalid character 'o' in literal null (expecting 'u')"}
```

## Como testar

Testes automáticos, com um Redis em memória ([miniredis](https://github.com/alicebob/miniredis)):

```bash
cd services/notification && go test -race ./...
```

Eles cobrem: mensagem processada sai da lista de pendentes; evento publicado antes de o grupo existir é processado; mensagem com falha fica pendente sem travar as outras; pendente de antes de uma queda é reprocessada ao subir; e o consumidor volta sozinho quando o Redis volta.

No Compose:

```bash
docker compose up -d --build notification
docker compose ps notification            # (healthy)
```

Publicar o exemplo do [EVENTOS.md](../contracts/EVENTOS.md#ticketissued) e um evento quebrado:

```bash
docker compose exec redis redis-cli XADD tickets.issued '*' payload '{"message_id":"0b7c6f0e-5d1a-4c3e-9f2b-8a1d2e3f4a5b","type":"TicketIssued","version":1,"occurred_at":"2026-11-20T14:03:12.481Z","trace_id":"4bf92f3577b34da6a3ce929d0e0e4736","traceparent":"00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01","order_id":"7","buyer_id":"42","buyer_email":"comprador@exemplo.com","event":{"id":"show-2026-12-01","name":"Show de Abertura"},"seats":[{"seat_id":"A-12","sector":"Pista","row":"A","number":12}],"amount_cents":20000,"currency":"BRL"}'
docker compose exec redis redis-cli XADD tickets.issued '*' payload 'not json'
```

Conferir:

```bash
docker compose logs notification                                      # "ticket issued received" e "message failed, left pending"
docker compose exec redis redis-cli XPENDING tickets.issued notification  # 1 pendente: só o evento quebrado
```
