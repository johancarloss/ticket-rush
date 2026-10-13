# ADR-003: Stack tecnológica

- **Status:** aceita
- **Data:** 13/10/2026

## Contexto

O sistema tem seis serviços com papéis diferentes: Inventário e Checkout concentram as regras que não podem falhar (nenhum assento vendido duas vezes, nenhum dinheiro cobrado sem assento), enquanto Pagamento, Notificação e Estatísticas são periféricos e podem cair sem afetar a compra. Tudo roda numa VPS própria, então o conjunto precisa caber numa máquina só.

## Decisão

### Linguagens: TypeScript no núcleo, Go na periferia

| Serviço | Linguagem | Responsável |
|---|---|---|
| Inventário | TypeScript (Node.js) | Bruno ou Luis |
| Checkout | TypeScript (Node.js) | Luis ou Bruno |
| Pagamento (simulado), Notificação, Estatísticas | Go | Diego |
| Guard, bots de carga | Go | Johan |

Inventário e Checkout, que concentram as regras críticas, ficam juntos em TypeScript. Go fica com os serviços menores e isolados, e também com os bots, que precisam disparar milhares de clientes concorrentes, e com o Guard, que precisa responder rápido a cada requisição.

Como TypeScript e Go não compartilham tipos, os contratos entre serviços são escritos antes do código: OpenAPI para as rotas HTTP e um JSON de exemplo para cada evento.

O framework de cada serviço fica a critério do responsável, desde que exponha `/health` e as métricas para o Prometheus.

### Dados e mensageria

| Tecnologia | Uso | Motivo |
|---|---|---|
| PostgreSQL | Estado de cada serviço (um schema por serviço) | Transações e `UPDATE` condicional, que são a base da SAGA (ADR-001) e da disputa por assento |
| Redis | Cache do mapa de assentos, contadores do Guard, Redis Streams para eventos | Dados em memória com expiração; Streams têm grupos de consumidores e confirmação de leitura |

### Plataforma

| Tecnologia | Uso | Motivo |
|---|---|---|
| Traefik | Entrada, HTTPS, roteamento, Blue-Green e Canary | ADR-002 |
| Keycloak | Login e emissão de tokens (OAuth2 / OIDC) | Autenticação pronta e padrão, sem escrever código de login |
| Docker Compose | Subir o sistema inteiro com um comando | Exigência do projeto e suficiente para uma VPS |
| GitHub Actions | CI/CD | Integrado ao repositório |
| Prometheus, Grafana, Jaeger | Métricas, painéis e rastreamento entre serviços | Medir os critérios de sucesso e seguir uma compra por todos os serviços |
| k6 | Bots de carga (fãs e cambistas) | Scripts de carga com métricas de latência prontas; roda de outra máquina para não disputar CPU com o sistema |

## Consequências

**Positivas**
- Um erro num serviço periférico não afeta a compra.
- O sistema demonstra na prática que microsserviços podem usar linguagens diferentes, desde que os contratos sejam claros.

**Negativas**
- Dois ecossistemas para manter: dois jeitos de testar, de fazer build e de configurar o CI.
- Sem tipos compartilhados, uma mudança de contrato precisa ser feita à mão nos dois lados; os contratos escritos antes do código reduzem esse risco.
