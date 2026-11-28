# Estatísticas

As Estatísticas mantêm os contadores de vendas de cada evento (pedidos, ingressos e valor) para o organizador. Elas leem o evento `TicketIssued` que o Checkout publica no Redis Streams quando um pedido chega a `CONFIRMED` ([ADR-001](../adr/0001-SAGA-HIBRIDA.md), [ADR-006](../adr/0006-REDIS-STREAMS.md)). É o segundo consumidor do mesmo stream, ao lado da Notificação: cada um no seu consumer group e no seu ritmo, então uma queda das Estatísticas não atrasa o e-mail nem a compra.

Código em [`services/stats/`](../../services/stats/), escrito em Go. Contrato da rota em [`stats.openapi.yaml`](../contracts/stats.openapi.yaml).

## Como lê os eventos

| | |
|---|---|
| Stream | `tickets.issued` ([EVENTOS.md](../contracts/EVENTOS.md)) |
| Consumer group | `stats` |
| Nome do consumidor | O hostname do container, diferente em cada réplica |
| DLQ | `tickets.issued.dead` |

1. Ao subir, cria o grupo com `XGROUP CREATE tickets.issued stats 0 MKSTREAM`. O `0` faz o grupo começar do início do stream, então vendas publicadas antes de o serviço existir também são contadas.
2. Lê as mensagens novas com `XREADGROUP ... BLOCK 2000 STREAMS tickets.issued >`, até 10 por vez.
3. Para cada uma, grava a venda no Postgres e só então confirma com `XACK`.
4. A cada volta do laço, procura no grupo inteiro mensagens pendentes há pelo menos 5 segundos (`XPENDING ... IDLE 5000`) e as pega para si (`XCLAIM`). Assim uma mensagem que falhou é tentada de novo, e as mensagens de uma réplica que caiu no meio são terminadas por outra.

### Tentativas e DLQ

O número da tentativa é a contagem de entregas que o próprio Redis guarda: a leitura com `>` é a primeira, e cada `XCLAIM` soma uma. O `XPENDING` devolve essa contagem, que o `XAUTOCLAIM` não devolve; por isso o serviço usa os dois comandos separados. O `XCLAIM` repete o tempo mínimo de espera, então duas réplicas nunca pegam a mesma mensagem.

| Situação | O que acontece |
|---|---|
| Falhou na 1ª ou na 2ª tentativa | Fica pendente e volta depois de 5 s |
| Falhou na 3ª tentativa | Vai para `tickets.issued.dead` e a original recebe `XACK`, numa só transação (`MULTI`/`EXEC`), para nunca estar nos dois lugares nem em nenhum |
| Já foi entregue 3 vezes sem terminar (o processo caiu ao tratá-la) | Vai para a DLQ sem ser tratada de novo, porque derrubaria o processo outra vez |

Cada mensagem da DLQ tem os campos `payload` (o original), `source_id` (ID no `tickets.issued`), `group` (`stats`, já que a Notificação usa a mesma DLQ), `attempts` e `error` (o motivo da última falha).

Um evento é recusado quando o JSON é inválido, o `type` não é `TicketIssued`, a `version` não é 1, falta `order_id` ou `event.id`, a lista de assentos está vazia ou o valor é negativo. Campos desconhecidos são ignorados, como o contrato pede.

### Postgres ou Redis fora

Antes de cada leitura, o consumidor confere se o Postgres responde. Se não responde, ele não lê nem pega nenhuma mensagem e tenta de novo com espera crescente, de 0,5 s até 10 s. Sem isso, cada mensagem falharia três vezes durante a queda e iria para a DLQ sem ter nada de errado. No máximo as mensagens que já estavam sendo tratadas quando o banco caiu gastam uma tentativa. Com o Redis fora, a espera é a mesma, e as mensagens continuam no stream (com AOF, [REDIS.md](../infra/REDIS.md#persistência)).

**Desligamento:** no `SIGTERM`, para de aceitar requisições, termina as mensagens que já leu e só então sai.

## Tabelas

No schema `stats`, criadas pelo próprio serviço ao subir ([POSTGRES.md](../infra/POSTGRES.md)):

| Tabela | Chave | Conteúdo |
|---|---|---|
| `processed_orders` | `order_id` | Pedidos já contados, com o evento e o momento |
| `event_sales` | `event_id` | Nome do evento, pedidos, ingressos vendidos, valor em centavos e a última atualização |

**Pedido repetido não conta duas vezes.** O Outbox do Checkout pode publicar o mesmo evento duas vezes (ADR-001, regra 5). Numa só transação, o serviço insere o `order_id` em `processed_orders` com `ON CONFLICT DO NOTHING` e, só se a linha entrou, soma em `event_sales`. Um evento repetido não insere nada, não soma nada e recebe `XACK` normalmente. Duas cópias chegando ao mesmo tempo são serializadas pela chave primária: a segunda espera a primeira terminar e então não insere.

## Rotas

| Rota | Quem chama | Resposta |
|---|---|---|
| `GET /api/stats/events/{event_id}` | Organizador, pelo Traefik com `buyer-auth@file` | `200` com os contadores; `403` sem o papel `organizer`; `404` se o evento não vendeu nada |
| `GET /health` | Healthcheck do Docker (o binário chama a si mesmo com `-healthcheck`) | `200 {"status":"ok"}` |
| `GET /metrics` | Prometheus (job `stats`) | Métricas do runtime Go e do processo |

O Traefik só encaminha `/api/stats`; `/health` e `/metrics` ficam só na rede interna. O papel vem do `X-User-Roles` que o Guard define, e o Traefik apaga o valor que o cliente mande ([GUARD.md](../infra/GUARD.md#por-que-apagar-os-cabeçalhos-de-identidade-antes)).

```json
{"event_id":"show-2026-12-01","event_name":"Show de Abertura","orders":2,"tickets_sold":4,"revenue_cents":80000,"currency":"BRL","updated_at":"2026-11-26T22:03:35.875958Z"}
```

Qualquer organizador vê qualquer evento: o `TicketIssued` não diz quem é o dono do evento, e o Inventário ainda não guarda isso.

## Variáveis de ambiente

| Variável | Padrão | Valor no Compose |
|---|---|---|
| `PORT` | `8080` | (padrão) |
| `DATABASE_URL` | obrigatória | `postgres://stats_svc:${STATS_DB_PASSWORD}@postgres:5432/ticketrush` |
| `REDIS_ADDR` | `redis:6379` | `redis:6379` |

Nenhuma variável nova no `.env`: a `STATS_DB_PASSWORD` já existe.

## Logs

```json
{"level":"INFO","msg":"sale recorded","message_id":"1795730615875-0","order_id":"8","event_id":"show-2026-12-01","tickets":2,"amount_cents":40000}
{"level":"INFO","msg":"repeated order ignored","message_id":"1795730615801-0","order_id":"7","event_id":"show-2026-12-01"}
{"level":"WARN","msg":"message failed, will retry","message_id":"1795730181563-0","attempt":1,"error":"invalid TicketIssued payload: ..."}
{"level":"ERROR","msg":"message sent to dead-letter stream","message_id":"1795730181563-0","dead_stream":"tickets.issued.dead","attempts":3,"error":"invalid TicketIssued payload: ..."}
{"level":"WARN","msg":"consumer paused","error":"failed to connect to `user=stats_svc database=ticketrush`: ...","retry_in":"1s"}
```

## Como testar

Testes automáticos, com um Redis em memória ([miniredis](https://github.com/alicebob/miniredis)) para o consumidor e um armazenamento falso para a rota:

```bash
cd services/stats && go test -race ./...
```

Eles cobrem: mensagem processada sai dos pendentes; falha passageira é tentada de novo; falha permanente vai para a DLQ depois de exatamente 3 tentativas, com os campos certos, sem travar as outras; pendente de uma réplica morta é terminada por outra; mensagem que derrubou o processo vai para a DLQ sem ser tratada; nada é lido com o banco fora; o desligamento termina o que já foi lido; organizador `200`, comprador `403`, sem `X-User-Id` `401`, evento sem vendas `404`.

Os testes do Postgres (pedido repetido não conta duas vezes, 10 cópias simultâneas contam uma, pedidos diferentes somam) rodam contra o banco do Compose e são pulados sem a variável `STATS_TEST_DATABASE_URL`. O Postgres não tem porta na máquina, então os testes rodam num container na rede `data`:

```bash
set -a; . ./.env; set +a
docker compose up -d postgres
docker run --rm --network ticket-rush_data -v "$PWD/services/stats:/src" -w /src \
  -e STATS_TEST_DATABASE_URL="postgres://stats_svc:${STATS_DB_PASSWORD}@postgres:5432/ticketrush" \
  golang:1.27.4-alpine go test -v -run 'Repeated|Concurrent|Different|ReportedAsMissing' ./...
```

Cada teste usa IDs próprios, então pode rodar num banco que já tem dados.

No Compose, de ponta a ponta:

```bash
docker compose up -d --build stats
docker compose exec redis redis-cli XADD tickets.issued '*' payload '{"type":"TicketIssued","version":1,"order_id":"7","event":{"id":"show-2026-12-01","name":"Show de Abertura"},"seats":[{"seat_id":"A-12"},{"seat_id":"A-13"}],"amount_cents":40000,"currency":"BRL"}'
```

Com um token de `organizer1` e outro de `buyer1` ([KEYCLOAK.md](../infra/KEYCLOAK.md#obter-um-token)):

```bash
curl -H "Authorization: Bearer $ORGANIZER_TOKEN" http://localhost:8180/api/stats/events/show-2026-12-01   # 200
curl -H "Authorization: Bearer $BUYER_TOKEN" http://localhost:8180/api/stats/events/show-2026-12-01       # 403
```

Publicar o mesmo `XADD` de novo não muda os números (`repeated order ignored` no log). Para ver a DLQ, publicar `payload 'not json'` e, uns 10 segundos depois, `docker compose exec redis redis-cli XRANGE tickets.issued.dead - +`.

Resultado conferido no Compose em 26/11/2026: contadores certos com um pedido repetido, `403` para comprador mesmo mandando `X-User-Roles: organizer`, `404` para `/metrics` pela entrada pública, evento quebrado na DLQ depois de 3 tentativas, e com o Postgres parado por 25 s nenhuma mensagem foi para a DLQ e a venda foi gravada quando ele voltou.
