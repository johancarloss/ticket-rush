# Como instrumentar um serviço

Passos para um serviço aparecer no Prometheus e no Jaeger. O porquê está no [ADR-010](../adr/0010-OBSERVABILIDADE.md) e a estrutura no [OBSERVABILIDADE.md](../infra/OBSERVABILIDADE.md). O Guard já segue este guia e serve de exemplo em Go ([`services/guard/telemetry.go`](../../services/guard/telemetry.go)).

## O que todo serviço entrega

1. `GET /metrics` no formato do Prometheus, na mesma porta do serviço (`8080`). Não há rota no Traefik para ele, então só a rede interna alcança.
2. As métricas HTTP de entrada: total de requisições e duração, por rota, método e código.
3. As métricas do próprio domínio, listadas na issue do serviço (por exemplo, reservas que ganharam e que perderam no Inventário).
4. Traces enviados para o Jaeger, continuando o `traceparent` que chega na requisição e repassando-o em toda chamada HTTP de saída.

## Convenções

| Item | Convenção |
|---|---|
| Nome do serviço no trace | Variável `OTEL_SERVICE_NAME`, com o nome da pasta (`inventory`, `checkout`, `payment`, `notification`, `stats`) |
| Para onde mandar | Variável `OTEL_EXPORTER_OTLP_ENDPOINT=http://jaeger:4318`. Sem ela, o tracing fica desligado (útil nos testes) |
| Nomes de métrica | Prefixo com o nome do serviço, em inglês e `snake_case`, com a unidade no fim: `inventory_holds_total`, `checkout_saga_step_duration_seconds` |
| Rótulos | Poucos valores possíveis por rótulo (estado, resultado, rota). **Nunca** ID de pedido, de assento ou de usuário: cada valor diferente vira uma série nova no Prometheus |
| Rota no rótulo | O padrão da rota (`/api/orders/:id`), e não o caminho com o ID |
| O que não vira trace | `/health` e `/metrics`, que só geram ruído |

## Compose e Prometheus

No bloco do serviço no `compose.yaml`:

```yaml
    environment:
      OTEL_SERVICE_NAME: inventory
      OTEL_EXPORTER_OTLP_ENDPOINT: http://jaeger:4318
    networks:
      - edge
      - data
      - observability
```

E um job no [`infra/prometheus/prometheus.yml`](../../infra/prometheus/prometheus.yml), copiando o do Guard. O DNS do Docker devolve um endereço por réplica, então cada réplica vira um alvo:

```yaml
  - job_name: inventory
    dns_sd_configs:
      - names: ["inventory"]
        type: A
        port: 8080
        refresh_interval: 10s
```

## TypeScript (Node.js)

Bibliotecas: [`prom-client`](https://github.com/siimon/prom-client) para as métricas e o SDK do OpenTelemetry com as instrumentações automáticas, que já cobrem HTTP de entrada e saída, `pg` e `ioredis`. Fixe as versões exatas no `package.json`, respeitando as duas semanas de espera.

```bash
npm install --save-exact prom-client @opentelemetry/sdk-node @opentelemetry/auto-instrumentations-node @opentelemetry/exporter-trace-otlp-http
```

O tracing precisa ser carregado **antes** do resto do código, para conseguir instrumentar o `http` e o `pg` quando eles forem importados. Um arquivo `src/tracing.ts`:

```ts
import { NodeSDK } from "@opentelemetry/sdk-node";
import { getNodeAutoInstrumentations } from "@opentelemetry/auto-instrumentations-node";
import { OTLPTraceExporter } from "@opentelemetry/exporter-trace-otlp-http";

// Without an endpoint (tests, local run) tracing stays off.
if (process.env.OTEL_EXPORTER_OTLP_ENDPOINT) {
  const sdk = new NodeSDK({
    traceExporter: new OTLPTraceExporter(),
    instrumentations: [getNodeAutoInstrumentations({
      "@opentelemetry/instrumentation-fs": { enabled: false },
      "@opentelemetry/instrumentation-http": {
        ignoreIncomingRequestHook: (req) => req.url === "/health" || req.url === "/metrics",
      },
    })],
  });
  sdk.start();
  process.on("SIGTERM", () => sdk.shutdown());
}
```

E o processo sobe com `node --import ./dist/tracing.js dist/main.js` (no `CMD` do Dockerfile).

Métricas, num `src/metrics.ts`:

```ts
import client from "prom-client";

export const registry = new client.Registry();
client.collectDefaultMetrics({ register: registry });

export const httpDuration = new client.Histogram({
  name: "inventory_http_request_duration_seconds",
  help: "Duration of HTTP requests, by route pattern, method and status.",
  labelNames: ["route", "method", "status"],
  buckets: [0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5],
  registers: [registry],
});

export const holds = new client.Counter({
  name: "inventory_holds_total",
  help: "Hold attempts, by result (won or unavailable).",
  labelNames: ["result"],
  registers: [registry],
});
```

A rota `GET /metrics` responde `await registry.metrics()` com o `Content-Type` de `registry.contentType`. A duração entra num middleware que, ao fim da resposta, observa o tempo com o padrão da rota (no Express, `req.route?.path`; no Fastify, `request.routeOptions.url`).

## Go

O Guard é o exemplo completo ([`telemetry.go`](../../services/guard/telemetry.go)): um `prometheus.Registry` com as métricas do serviço, o `promhttp.HandlerFor` em `GET /metrics`, o exportador OTLP HTTP ligado só quando `OTEL_EXPORTER_OTLP_ENDPOINT` existe, e o `otelhttp.NewHandler` em volta do roteador. As versões usadas estão no [`go.mod`](../../services/guard/go.mod).

Para chamadas HTTP de saída (o Checkout é TypeScript, mas um serviço Go que chame outro), o cliente precisa repassar o trace:

```go
client := &http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)}
```

## Eventos pelo Redis

O `traceparent` vai dentro do evento, porque o Redis Streams não tem cabeçalhos ([EVENTOS.md](../contracts/EVENTOS.md)).

**Quem publica** (Checkout, ao gravar na outbox) grava o contexto atual:

```ts
import { context, propagation } from "@opentelemetry/api";

const carrier: Record<string, string> = {};
propagation.inject(context.active(), carrier);
event.traceparent = carrier.traceparent;
```

**Quem consome** (Notificação e Estatísticas, em Go) continua o trace a partir do campo:

```go
carrier := propagation.MapCarrier{"traceparent": event.Traceparent}
ctx := otel.GetTextMapPropagator().Extract(context.Background(), carrier)
ctx, span := otel.Tracer("notification").Start(ctx, "process TicketIssued")
defer span.End()
```

## Como conferir

1. `docker compose up -d --build <serviço>`.
2. Em `http://127.0.0.1:8183/targets`, o job do serviço aparece com uma linha por réplica, todas `UP`.
3. Faça uma chamada que passe pelo serviço e procure pelo `OTEL_SERVICE_NAME` em `http://127.0.0.1:8184/`. O trace precisa começar no `traefik` e ter o trecho do serviço dentro dele, e não um trace separado.
