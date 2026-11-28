# ADR-011: Compatibilidade com o kit de avaliação da disciplina

- **Status:** aceita
- **Data:** 28/11/2026
- **Responsáveis pela implementação:** Checkout (Bruno), Inventário (Luís), Pagamento (Diego), serviço de teste e Keycloak (Johan)

## Contexto

O professor publicou o [kit de avaliação comparativa](https://github.com/rkulesza/esd-carga-tickets) dos cinco projetos de ingressos da turma. O Ticket Rush é o grupo 09. O kit aplica a mesma massa de dados (5 eventos, 21.450 assentos, 50 mil usuários, 30 mil intenções de compra) e os mesmos seis cenários a todos os projetos, e monta uma tabela comparativa:

| Cenário | O que faz | O que conta |
|---|---|---|
| `carga` | Até 120 compras por segundo, por cerca de 6,5 min | Vazão, p95, taxa de erro |
| `pico` | De 10 para 300 compras por segundo em 5 s | Recusa controlada (429/503) é aceitável; 5xx e timeout não são |
| `corrida` | 500 pessoas disputando 50 assentos | Nenhum assento vendido duas vezes, e os 50 vendidos |
| `idempotencia` | Cada reserva e cada pagamento enviados 5 vezes em paralelo | Uma reserva e uma cobrança por compra |
| `expiracao` | 400 reservas nunca pagas | Assentos liberados depois do tempo da reserva |
| `compensacao` | 400 pagamentos recusados | Assentos liberados e nenhuma cobrança |

As invariantes são conferidas no **estado final** do sistema (`GET /_teste/vendas`), e não nas respostas HTTP. O kit cruza o que o servidor diz que vendeu com o que o cliente recebeu como confirmado.

O kit traz um [contrato sugerido](https://github.com/rkulesza/esd-carga-tickets/blob/main/CONTRATO.md) e um adaptador por grupo, que traduz nomes de rota, campos, códigos e status. Diferenças de **nome** se resolvem no adaptador. Diferenças de **forma** pedem mudar o código do kit, numa cópia versionada pelo grupo. A massa de dados, os cenários e as invariantes não podem mudar.

Seis pontos do kit não batem com o desenho dos ADRs anteriores:

1. O comprador vem no corpo (`usuario_id`, 50 mil usuários diferentes), e a autenticação é, no máximo, um cabeçalho fixo. No Ticket Rush, o comprador vem do token, pelo `X-User-Id` que o Guard define (ADR-002), e o rate limit é por conta.
2. A criação da reserva leva `Idempotency-Key`. No ADR-001, só a cobrança é idempotente.
3. O tempo da reserva vem da massa de dados (20 s). No ADR-004, o hold é fixo em 5 minutos.
4. Cada pagamento diz ao provedor simulado o que fazer (`aprovar`, `recusar` ou `atrasar`). O nosso Pagamento só tem uma taxa de falha global.
5. Os dados chegam num arquivo único, por `POST /_teste/carga`.
6. O kit precisa de três rotas de teste (`reset`, `carga` e `vendas`) e lê o estado de três serviços numa resposta só.

## Decisão

**Adaptar o Ticket Rush ao kit por nomes, sem alterar o código do kit**, e criar um **perfil de teste**, desligado por padrão, que só existe quando a variável `EVALUATION_MODE=true` está ligada. Nada do perfil de teste muda o comportamento de produção.

### 1. Comprador e autenticação

- As rotas de compra continuam passando pelo Traefik e pelo Guard, com token.
- Um client do Keycloak só para o kit, `evaluation-kit`, com conta de serviço (*client credentials*), papel `buyer`, audiência `ticket-rush-api` e token de 1 hora, o tempo de uma rodada inteira do kit. Ele é criado por script antes da rodada (como as contas de carga em [`bots/`](../../bots/README.md)), e não pelo arquivo do realm, para não existir numa instalação normal. O token vai no campo `cabecalhos` do adaptador.
- No perfil de teste, o **Checkout usa o `user_id` do corpo** quando ele vem, em vez do `X-User-Id`. Fora do perfil, o campo é ignorado. Assim, o limite de 4 assentos e a contagem de pedidos valem por usuário do kit, e não para a conta de serviço inteira.
- O rate limit por conta do Guard passa a valer para a conta de serviço inteira, então no perfil de teste ele sobe (`GUARD_RATE_LIMIT_PER_SECOND`). O controle de admissão do pico vem de um limite global de requisições em andamento nas rotas de compra (`inFlightReq` do Traefik, respondendo 429), que vale também em produção (issue própria).

### 2. Reserva idempotente

`POST /api/orders` exige `Idempotency-Key`. A mesma chave do mesmo usuário devolve **o mesmo pedido** (`200`), inclusive com as cópias chegando em paralelo. A garantia vem de uma chave única (`user_id`, `idempotency_key`) na tabela de pedidos, gravada antes de chamar o Inventário, como na regra "grava antes de agir" do ADR-001.

### 3. Pagamento idempotente pela chave do cliente

`POST /api/orders/{id}/pay` exige `Idempotency-Key`. A primeira chamada leva o pedido de `RESERVED` a `PAYING` (o `UPDATE` condicional do ADR-001) e responde `200` (confirmado) ou `402` (recusado). Uma cópia que chega **enquanto o pedido está em `PAYING`** responde `202`, e o kit consulta `GET /api/orders/{id}` até o estado final. Uma cópia que chega **depois** devolve o mesmo resultado da primeira. Assim, nenhuma repetição vira erro nem segunda cobrança, e a regra do ADR-001 continua: só uma chamada passa de `RESERVED` para `PAYING`.

### 4. Tempo da reserva configurável

O Inventário lê o tempo do hold de uma configuração: a variável `HOLD_TTL_SECONDS` (padrão 300, os 5 minutos do ADR-004), sobrescrita pelo `config.ttl_reserva_segundos` da massa de dados quando o kit carrega os dados. O `UPDATE` condicional e a expiração sem timer do ADR-004 não mudam.

### 5. Simulação de pagamento por requisição

No perfil de teste, o corpo do pagamento aceita `simulation` com os valores do próprio kit (`aprovar`, `recusar`, `atrasar`). Os valores ficam em português porque o adaptador traduz nomes, mas não valores. O Checkout repassa ao Pagamento, cuja cobrança ganha o campo opcional `simulation`, que tem prioridade sobre a taxa global (`approve`, `decline`, `delay`). O atraso usa o `atraso_pagamento_ms` da massa de dados (3 s).

### 6. Serviço de teste

Um serviço pequeno em Go, `services/evaluation`, atende `/api/_teste/reset`, `/api/_teste/carga` e `/api/_teste/vendas`. Ele só sobe com o perfil `evaluation` do Compose (`docker compose --profile evaluation up`), exige o cabeçalho `X-Chave-Teste` com o valor do `.env`, e não guarda dado nenhum: chama rotas internas de cada serviço.

| Rota do kit | O que o serviço faz |
|---|---|
| `reset` | Pede a cada serviço para apagar pedidos, holds, cobranças e chaves de idempotência |
| `carga` | Manda locais, setores, assentos, preços e eventos ao Inventário, e o `config` ao Inventário (tempo do hold) e ao Pagamento (atraso). Os 50 mil usuários não são carregados: o comprador vem no corpo |
| `vendas` | Junta os pedidos do evento (Checkout), os assentos vendidos (Inventário) e as cobranças aprovadas (Pagamento) no formato do kit |

Tradução dos estados para o vocabulário do kit no `vendas`:

| Ticket Rush | Kit |
|---|---|
| `RESERVED` com o hold dentro do prazo | `reservada` |
| `RESERVED` com o hold vencido | `expirada` |
| `PAYING`, `AWAITING_REFUND` | `pendente` |
| `CONFIRMED` | `confirmada` |
| `CANCELLED` por recusa do pagamento | `recusada` |
| `CANCELLED` por expiração ou falha na venda | `expirada` / `cancelada` |

Uma cobrança estornada não entra em `pagamentos_aprovados`: o dinheiro voltou.

### 7. Rotas e códigos

| Operação do kit | Rota do Ticket Rush | Sucesso | Recusa |
|---|---|---|---|
| Listar assentos de um setor | `GET /api/events/{event_id}/seats?sector={setor}` | `200`, lista em `seats`, status `available`, `held` ou `sold` | `404` |
| Reservar | `POST /api/orders` | `201` (novo) ou `200` (mesma chave) | `409` assento indisponível; `422` limite de 4 assentos por comprador |
| Pagar | `POST /api/orders/{order_id}/pay` | `200` confirmado; `202` em andamento | `402` recusado; `409` pedido expirado ou cancelado |
| Consultar | `GET /api/orders/{order_id}` | `200` | `404` |

A recusa pelo limite de 4 assentos sai com `422`, que o adaptador conta como conflito. A massa de carga tem 1.395 combinações de usuário e evento acima de 4 assentos, e essas recusas são a regra anti-cambista do ADR-002 funcionando, e não erro.

O adaptador fica em [`kit-avaliacao/grupo_09.json`](../../kit-avaliacao/grupo_09.json).

## Alternativas consideradas

- **Seguir o contrato sugerido ao pé da letra** (rotas `/reservas`, campos em português). Duplicaria a API ou trocaria os nomes de todos os serviços. O kit aceita a tradução pelo adaptador, e é para isso que ele existe.
- **Desligar o Guard no teste.** Mais simples, mas o kit mediria um sistema sem a entrada que faz parte da arquitetura. Com o token de serviço, a requisição de teste percorre o mesmo caminho da de produção, com o custo do Guard incluído nos números.
- **Criar os 50 mil usuários no Keycloak** e gerar um token por usuário. O token vai num cabeçalho fixo do adaptador, então o kit usaria um token só de qualquer jeito.
- **Mudar o código do kit** para traduzir a simulação ou consultar o banco direto. Funciona, mas cada mudança no kit é algo que o professor precisa conferir para a comparação continuar justa. A decisão foi não mudar nada no kit.
- **Rotas de teste dentro de cada serviço, sem um serviço novo.** O `vendas` precisa juntar três serviços numa resposta, e alguém teria de fazer essa junção; um serviço separado também deixa o código de teste fora dos serviços de negócio e fora da produção.

## Consequências

**Positivas**
- O kit testa o sistema inteiro, pela mesma entrada da produção: Traefik, Guard, SAGA, outbox e UPDATE condicional.
- As invariantes do kit são as garantias que os ADRs 001, 004 e 006 já prometem; o kit vira a prova externa delas.
- Nenhuma linha do kit muda, então os resultados entram na comparação sem ressalva.

**Negativas e riscos aceitos**
- **Código que só existe no teste:** o `user_id` no corpo, a simulação por requisição, as rotas internas de reset e carga e o serviço de teste. Tudo fica atrás do `EVALUATION_MODE` e do perfil do Compose; o teste de segurança S7 confere que nada disso responde fora do perfil.
- **O teste mede a conta de serviço, e não 50 mil contas no Guard.** O custo de validar o token está nos números, mas o rate limit por conta não age; a contenção do pico vem do limite global.
- **A recusa pelo limite de 4 assentos reduz as vendas** em comparação com um grupo sem essa regra. É uma escolha do domínio e fica explicada no relatório de resultados.
- **Mais trabalho no caminho crítico:** idempotência na reserva, `202` no pagamento e tempo de hold configurável entram nas tarefas do Checkout e do Inventário.

## Como validar

- O kit no perfil `leve`, depois no `padrao`, com zero invariantes violadas nos seis cenários (`./executar.sh kit-avaliacao/grupo_09.json padrao`).
- Com `EVALUATION_MODE` desligado, `/api/_teste/*` responde `404`, e o `user_id` no corpo de `POST /api/orders` é ignorado.
- Os resultados entram no [plano de testes](../TESTES.md#kit-de-avaliação-da-disciplina).
