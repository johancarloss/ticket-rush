# ADR-010: Observabilidade com Prometheus, Jaeger e Grafana, instrumentados por OpenTelemetry

- **Status:** aceita
- **Data:** 19/11/2026
- **Responsável pela implementação:** Johan (estrutura e Guard); cada serviço se instrumenta seguindo o [guia de instrumentação](../guides/INSTRUMENTACAO.md)

## Contexto

Os critérios de sucesso do projeto são números: nenhum assento vendido duas vezes, latência da reserva no pico, quantos pedidos terminaram em cada estado, quanto tempo um estorno ficou pendente. Os testes de carga e de resiliência (plano em `docs/TESTES.md`) precisam medir isso enquanto acontece, e não só no relatório final dos bots.

Uma compra também atravessa cinco processos (Traefik, Guard, Checkout, Inventário e Pagamento), e depois segue por eventos para Notificação e Estatísticas. Quando uma compra demora, o log de cada serviço mostra só o seu pedaço; para saber onde o tempo foi gasto é preciso juntar os pedaços da mesma compra.

São dois tipos de pergunta, que pedem dados diferentes:

| Pergunta | Dado | Exemplo |
|---|---|---|
| Como o sistema está se comportando, no agregado | **Métricas**: números somados ao longo do tempo | Reservas por segundo, p95 da latência, pedidos em AWAITING_REFUND |
| O que aconteceu com esta compra | **Traces**: o caminho de uma requisição, com o tempo gasto em cada serviço | O Checkout esperou 800 ms pelo Pagamento nesta compra |

## Decisão

- **Métricas no Prometheus**, por coleta (*pull*): cada serviço expõe `GET /metrics` e o Prometheus lê a cada 5 segundos. Serviços com réplicas são encontrados pelo DNS do Docker, então cada réplica aparece separada.
- **Traces no Jaeger v2**, enviados por cada serviço no protocolo OTLP. O Jaeger v2 é construído sobre o OpenTelemetry Collector e recebe OTLP direto, então não há um coletor separado.
- **Instrumentação com as bibliotecas do OpenTelemetry** nos serviços (Go e TypeScript), com propagação do cabeçalho padrão W3C `traceparent`. O trace começa no Traefik e continua em cada serviço que a requisição atravessa.
- **Grafana** para os painéis, com as fontes de dados e os painéis versionados no repositório (`infra/grafana/`) e carregados na subida.
- Os três painéis ficam publicados só em `127.0.0.1`, como o do Traefik. Nenhum deles passa pelo Traefik.

### Eventos assíncronos

Um trace segue uma requisição HTTP pelo cabeçalho `traceparent`. O `TicketIssued` viaja pelo Redis, sem cabeçalho, então o contrato do evento ganha o campo `traceparent` ([EVENTOS.md](../contracts/EVENTOS.md)). O Checkout grava o valor na outbox, e a Notificação e as Estatísticas continuam o mesmo trace ao consumir o evento. Assim o Jaeger mostra a compra inteira, incluindo o e-mail enviado segundos depois.

## Alternativas consideradas

- **Só logs**, procurando pelo `trace_id` nos logs de cada serviço. Não dá o agregado (p95, taxa por segundo) e exige juntar os pedaços à mão.
- **Envio de métricas por OTLP** (*push*), em vez de o Prometheus coletar. Deixaria métricas e traces no mesmo protocolo, mas exigiria um coletor a mais para converter, e o formato do Prometheus é o que as bibliotecas de cada linguagem já oferecem pronto.
- **Zipkin** no lugar do Jaeger. Equivalente para o que o projeto precisa; o Jaeger recebe OTLP nativamente e o Grafana já tem a fonte de dados dele.
- **Grafana Tempo e Loki** (traces e logs na mesma família do Grafana). Mais peças para operar na VPS, e o volume de dados de um teste de carga cabe na memória do Jaeger.
- **Serviço gerenciado** (Grafana Cloud, Datadog). Tiraria a operação da VPS, mas mandaria os dados para fora e depende de conta e de limite do plano gratuito no dia da apresentação.

## Consequências

**Positivas**
- Cada critério de sucesso do plano de testes vira uma consulta no Prometheus e um painel no Grafana, e os números da apresentação saem dali.
- O salto extra ao Guard (preço aceito no ADR-002) fica medido: no primeiro teste, a consulta ao Guard levou 0,7 ms dentro de 1,3 ms de autenticação no Traefik.
- O padrão é o mesmo nas duas linguagens, então um trace atravessa serviços em Go e TypeScript sem tradução.

**Negativas e riscos aceitos**
- **Os traces ficam em memória.** Reiniciar o Jaeger apaga o histórico. Para os testes isso basta; os resultados que importam são registrados no `docs/TESTES.md`.
- **Mais três containers na VPS**, disputando memória com o sistema durante a carga. O Prometheus guarda 15 dias de dados; os testes rodam a carga de outra máquina para não somar a CPU dos bots.
- **Cada serviço precisa se instrumentar.** Sem isso ele some do trace. O guia de instrumentação e o checklist do esqueleto reduzem esse risco.
- **Dependências novas no Guard** (cliente do Prometheus e SDK do OpenTelemetry). Elas trouxeram indiretamente uma versão do `golang.org/x/net` com vulnerabilidades, encontrada pelo `govulncheck` e atualizada ([GUARD.md](../infra/GUARD.md#dependências-e-supply-chain)).

## Como validar

- Prometheus com todos os serviços como alvo `up` (`http://127.0.0.1:8183/targets`).
- Uma requisição autenticada aparece no Jaeger como um trace só, com os trechos do Traefik, do Guard e do serviço de destino.
- Depois da compra, o trace continua na Notificação e nas Estatísticas pelo `traceparent` do evento.
