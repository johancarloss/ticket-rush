# Observabilidade

Prometheus guarda as métricas, Jaeger guarda os traces e Grafana mostra os painéis. A decisão e as alternativas estão no [ADR-010](../adr/0010-OBSERVABILIDADE.md). Como um serviço passa a aparecer aqui está no [guia de instrumentação](../guides/INSTRUMENTACAO.md).

```
                  métricas (pull, a cada 5 s)
Prometheus ◄──────────────────────────────── Traefik :8082, Guard :8080/metrics, cada serviço /metrics
    │
    ▼                       traces (OTLP HTTP, porta 4318)
 Grafana ◄──── Jaeger ◄──────────────────────── Traefik, Guard, cada serviço
```

Todos ficam na rede `observability`. Um serviço que exporta métricas ou traces entra nessa rede além das suas.

## Endereços

Os três só abrem da própria máquina (`127.0.0.1`). Para ver do seu computador, use um túnel SSH: `ssh -L 8182:127.0.0.1:8182 -L 8184:127.0.0.1:8184 <vps>`.

| Endereço | O quê |
|---|---|
| `http://127.0.0.1:8182/` | Grafana. Login com `GRAFANA_ADMIN_USER` e `GRAFANA_ADMIN_PASSWORD` do `.env`. Painéis na pasta **Ticket Rush** |
| `http://127.0.0.1:8183/targets` | Prometheus, com o estado de cada alvo coletado |
| `http://127.0.0.1:8184/` | Jaeger, para procurar traces por serviço |

## O que já é coletado

| Origem | Métricas principais | Traces |
|---|---|---|
| Traefik | `traefik_router_requests_total` e `traefik_router_request_duration_seconds` por rota, método e código | Começa o trace de cada requisição; trechos `AuthRequest` (consulta ao Guard) e `ReverseProxy` (encaminhamento ao serviço) |
| Guard | `guard_decisions_total{result}` (liberado ou o motivo da recusa, incluindo `rate_limited`), `guard_verify_duration_seconds` e `guard_rate_limit_errors_total` (liberadas sem contar, com o Redis fora) | Trecho `GET /verify`, dentro do trace aberto pelo Traefik |

Os rótulos de `result` no Guard são os mesmos motivos dos logs ([GUARD.md](GUARD.md#rotas)): `allowed`, `rate_limited`, `missing_token`, `malformed_token`, `bad_algorithm`, `bad_signature`, `unknown_key`, `wrong_issuer`, `wrong_audience`, `token_expired` e `invalid_token`.

O Traefik também manda traces das próprias consultas à API do Docker (o que ele faz para descobrir os containers). Para procurar compras no Jaeger, filtre pela operação `GET` ou `POST` ou pelo serviço de destino.

## Painéis

Ficam em [`infra/grafana/dashboards/`](../../infra/grafana/dashboards/), como JSON. O Grafana carrega os arquivos na subida e não deixa editar pela interface, para o repositório continuar sendo a fonte. Para mudar um painel: editar no Grafana, exportar o JSON (*Share → Export*), substituir o arquivo e abrir o PR.

| Painel | Mostra |
|---|---|
| Entrada (Traefik e Guard) | Requisições por rota e código, decisões do Guard, latência do Guard e das rotas |

Os painéis da venda e da SAGA, e o alerta de estorno parado, são a issue #47.

## Primeiro resultado (19/11/2026)

Numa requisição autenticada a `/api/whoami`, o Jaeger mostrou um trace com quatro trechos: a requisição no Traefik (2,4 ms no total), a autenticação (1,3 ms), dentro dela a decisão do Guard (0,7 ms) e o encaminhamento ao serviço (0,6 ms). O p95 da decisão no Guard, medido no Prometheus, ficou em 0,5 ms. É o custo do salto extra que o ADR-002 aceitou e prometeu medir.

## Rodar uma segunda cópia na mesma máquina

O Traefik descobre containers de **todos** os projetos Docker da máquina. Se uma segunda cópia do Ticket Rush subir com outro nome de projeto (`docker compose -p outro ...`), as rotas com o mesmo nome (`whoami`, `keycloak`) ficam com servidores dos dois projetos, e metade das requisições vai para uma rede que o Traefik não alcança (504). Para testar em paralelo, a cópia precisa de um `--providers.docker.constraints` com o nome do próprio projeto e da rede `<projeto>_edge` no lugar de `ticket-rush_edge`, num arquivo de override fora do repositório.
