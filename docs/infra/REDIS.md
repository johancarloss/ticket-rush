# Redis

Um Redis 7.4 com três papéis: cache do mapa de assentos ([ADR-004](../adr/0004-CONTROLE-DO-HOLD.md)), contadores do Guard ([ADR-002](../adr/0002-TRAEFIK-GUARD.md)) e streams de eventos ([ADR-006](../adr/0006-REDIS-STREAMS.md)).

## Papéis e chaves

| Papel | Quem usa | Chaves | Se o Redis cair |
|---|---|---|---|
| Cache do mapa de assentos | Inventário | `inventory:seatmap:<evento>` com validade de cerca de 2 s | O mapa passa a ser lido do Postgres, mais lento |
| Contadores de requisições por conta | Guard | `guard:rate:<conta>:<segundo>` com validade curta | O Guard bloqueia as compras (fail-closed) |
| Eventos | Checkout publica; Notificação e Estatísticas consomem | Stream `tickets.issued` e DLQ `tickets.issued.dead` | Os eventos esperam na tabela `outbox` do Checkout e saem quando ele voltar |

Os nomes de chave do cache e do Guard são a convenção proposta; cada responsável confirma ao implementar. O formato das mensagens do stream está em [EVENTOS.md](../contracts/EVENTOS.md).

## Persistência

O Redis guarda tudo em memória. Para as mensagens dos streams sobreviverem a uma reinicialização, o container sobe com **AOF** (append-only file) ligado: cada escrita é registrada num arquivo no volume `redis-data` e reaplicada quando ele volta.

```yaml
command: ["redis-server", "--appendonly", "yes"]
```

O cache e os contadores não precisariam disso, porque são descartáveis, mas não atrapalham.

## Inspecionar

```bash
docker compose exec redis redis-cli
```

```
CONFIG GET appendonly                 # appendonly yes
XINFO GROUPS tickets.issued           # consumer groups e mensagens pendentes
XPENDING tickets.issued notification  # mensagens entregues e não confirmadas
XLEN tickets.issued.dead              # quantas mensagens foram para a DLQ
```

O Redis não tem porta publicada na máquina nem senha; ele só é alcançável pela rede interna `data`. Uma senha (`requirepass`) entra quando os serviços começarem a usá-lo.
