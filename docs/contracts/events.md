# Contratos de eventos

Eventos publicados no Redis Streams (ADR-006). Como os serviços são escritos em TypeScript e Go, sem tipos compartilhados, este arquivo é a referência para quem publica e para quem consome.

## Regras gerais

- Cada mensagem do stream tem um único campo, `payload`, com o JSON do evento.
- Nomes de campos em `snake_case`, datas em ISO 8601 (UTC), valores em centavos (inteiro).
- Mudanças que só acrescentam campos são permitidas sem aviso; consumidores ignoram campos que não conhecem.
- Mudanças que removem ou alteram um campo existente exigem incrementar `event_version` e um ADR.

## `TicketIssued`

Publicado pelo Checkout, via Transactional Outbox, quando um pedido chega a `CONFIRMED` (ADR-001).

| | |
|---|---|
| **Stream** | `tickets.issued` |
| **Publicado por** | Checkout |
| **Consumido por** | Notificação (grupo `notification`), Estatísticas (grupo `stats`) |
| **Chave para ignorar repetição** | `order_id` |
| **DLQ** | `tickets.issued.dead` |

```json
{
  "event_id": "0b7c6f0e-5d1a-4c3e-9f2b-8a1d2e3f4a5b",
  "event_type": "TicketIssued",
  "event_version": 1,
  "occurred_at": "2026-11-20T14:03:12.481Z",
  "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
  "order_id": "7",
  "buyer_id": "42",
  "buyer_email": "comprador@exemplo.com",
  "event": {
    "event_id": "show-2026-12-01",
    "name": "Show de Abertura"
  },
  "seats": [
    { "seat_id": "A-12", "sector": "Pista", "row": "A", "number": 12 },
    { "seat_id": "A-13", "sector": "Pista", "row": "A", "number": 13 }
  ],
  "amount_cents": 40000,
  "currency": "BRL"
}
```

| Campo | Tipo | Descrição |
|---|---|---|
| `event_id` | string (UUID) | Identificador único desta mensagem |
| `event_type` | string | Sempre `TicketIssued` |
| `event_version` | inteiro | Versão do formato; começa em 1 |
| `occurred_at` | string (ISO 8601) | Momento em que o pedido foi confirmado |
| `trace_id` | string | ID do trace distribuído, para o Jaeger ligar a compra ao processamento do evento |
| `order_id` | string | ID do pedido no Checkout; é a chave que os consumidores usam para ignorar repetição |
| `buyer_id` | string | ID do comprador no Keycloak |
| `buyer_email` | string | E-mail para a Notificação, que não tem acesso aos dados de usuário |
| `event.event_id` | string | ID do evento (show) no Inventário |
| `event.name` | string | Nome do evento, para o texto do e-mail |
| `seats` | lista | Assentos vendidos no pedido (até 4, ADR-002) |
| `amount_cents` | inteiro | Valor total cobrado, em centavos |
| `currency` | string | Sempre `BRL` |

O campo `event` dentro do JSON se refere ao **show**; o campo `event_id` no topo se refere à **mensagem**. Os nomes ficam assim porque "evento" é o termo do domínio de ingressos.
