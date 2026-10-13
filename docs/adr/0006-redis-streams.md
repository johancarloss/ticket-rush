# ADR-006: Redis Streams para os eventos e Redis com três papéis

- **Status:** aceita
- **Data:** 13/10/2026

## Contexto

Depois que uma compra é confirmada, Notificação e Estatísticas precisam saber dela, mas o comprador não deve esperar por eles (ADR-001). Para isso o Checkout publica o evento `IngressoEmitido` e os dois o consomem de forma assíncrona. É preciso escolher a ferramenta de mensageria.

O Redis já faz parte do sistema como cache do mapa de assentos (ADR-004) e para os contadores do Guard (ADR-002).

## Decisão

Usar **Redis Streams** para os eventos.

- O Checkout publica com `XADD tickets.issued`.
- Notificação e Estatísticas têm cada um o seu **consumer group**. Cada grupo recebe todas as mensagens e lê no próprio ritmo; se um ficar para trás, não atrasa o outro.
- Depois de processar, o consumidor confirma com `XACK`. Uma mensagem lida e não confirmada (o consumidor caiu no meio) continua pendente e é reprocessada.
- **DLQ:** se a mesma mensagem falhar 3 vezes, o consumidor a move para o stream `tickets.issued.dead` e confirma a original. A contagem de tentativas vem do próprio Redis (`XPENDING` informa quantas vezes a mensagem foi entregue).
- Os consumidores ignoram um pedido que já processaram, porque o Outbox (ADR-001) pode publicar o mesmo evento duas vezes. Cada um guarda os IDs processados no próprio schema (ADR-005):
  - **Notificação:** antes de enviar, insere o ID em `notification.sent_emails`, que tem o ID do pedido como chave primária. Se o ID já existe, o banco recusa a inserção e o e-mail não é enviado de novo.
  - **Estatísticas:** insere o ID em `stats.processed_orders` e incrementa os contadores na mesma transação; um evento repetido falha na inserção e não conta duas vezes.

Assim, o Redis passa a ter três papéis: cache de leitura, contadores do Guard e streams de eventos.

## Alternativas consideradas

- **RabbitMQ.** Tem DLQ, retry e roteamento prontos, feitos para isso. Seria mais uma peça para subir, configurar e monitorar.
- **Kafka.** Padrão de mercado para streaming e guarda o histórico, mas é pesado demais para o volume do projeto e para a VPS.
- **Redis Pub/Sub (sem Streams).** Mais simples, mas não guarda a mensagem: se o consumidor estiver fora do ar no momento da publicação, ele perde o evento.

## Consequências

**Positivas**
- Nenhuma peça nova além das que já existem.
- Consumer groups e confirmação de leitura dão entrega garantida para quem estiver fora do ar por um tempo.
- A DLQ fica em código do grupo, então o mecanismo é visível e explicável.

**Negativas e riscos aceitos**
- **A DLQ e o retry são implementados pelo grupo**, não vêm prontos.
- **Durabilidade depende da configuração de persistência do Redis.** Com AOF ligado, as mensagens sobrevivem a uma reinicialização.
- **O Redis vira ponto único de falha para três funções.** Se ele cair:
  - o mapa de assentos passa a ler direto do Postgres, mais lento, mas funcionando;
  - o Guard bloqueia as compras (fail-closed, ADR-002);
  - os eventos param de ser publicados, mas não se perdem, porque continuam na tabela `outbox` do Checkout e saem quando o Redis voltar.

## Como validar

- Derrubar a Notificação, fazer 100 compras e subir de novo: os 100 e-mails saem.
- Fazer a Notificação falhar sempre para um pedido específico: a mensagem vai para `tickets.issued.dead` depois de 3 tentativas, e as outras seguem normalmente.
- Derrubar o Redis durante a carga: medir a latência do mapa de assentos, confirmar que a compra é bloqueada e que nenhum evento se perde depois que ele volta.
