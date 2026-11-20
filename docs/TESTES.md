# Plano de testes

O que vamos testar no Ticket Rush, como, com qual métrica e qual resultado conta como sucesso. Cada teste vem de uma promessa feita na proposta ou num ADR, e a origem aparece na tabela. Os resultados entram neste mesmo arquivo, na seção [Resultados](#resultados), à medida que cada teste roda.

## Critérios de sucesso

São os da [proposta](PROPOSTA.md#critérios-de-sucesso), com a métrica que decide cada um. Os testes das próximas seções existem para medir estes seis números.

| # | Critério | Como é medido | Teste |
|---|---|---|---|
| 1 | **Zero assentos vendidos duas vezes** com 2.000 bots disputando 500 assentos | Consulta no banco depois da carga: nenhum assento com dois pedidos CONFIRMED | [C2](#carga) |
| 2 | **Nenhum assento preso:** com o Pagamento caindo durante a carga, toda reserva termina vendida ou liberada | Consulta no banco depois do teste e do tempo de expiração: nenhum pedido em RESERVED, PAYING ou AWAITING_REFUND | [R2](#resiliência), [R3](#resiliência) |
| 3 | **Nenhuma mensagem perdida:** toda mensagem é processada ou termina na DLQ | Compras confirmadas = e-mails enviados + mensagens em `tickets.issued.dead`; o mesmo para as Estatísticas | [R5](#resiliência), [R6](#resiliência) |
| 4 | **p95 abaixo de 200 ms** na consulta ao mapa de assentos com 1.000 usuários simultâneos | `http_req_duration` p(95) no k6 e `traefik_router_request_duration_seconds` no Prometheus | [C4](#carga) |
| 5 | **Bots contidos:** pelo menos 95% das requisições acima do limite recusadas, e menos de 1% de erro para os fãs | Respostas 429 dos cambistas sobre o excesso, e erros (5xx, timeout) dos fãs sobre o total deles | [C3](#carga) |
| 6 | **Deploy sem queda:** versão nova publicada durante a carga com taxa de erro abaixo de 1% | Taxa de 5xx no Traefik durante a troca de versão | [C5](#carga) |

## Ambiente

| Item | Como é |
|---|---|
| Sistema medido | A VPS do projeto: 4 vCPUs ARM (aarch64) e 24 GB de RAM, com o Compose completo. A VPS também roda outros projetos; antes de cada teste de carga, anotar o uso de CPU e memória da máquina (`docker stats --no-stream`) para saber o ruído de fundo |
| Origem da carga | Uma máquina separada da VPS, para a CPU dos bots não competir com o sistema medido (proposta e ADR-003). A máquina usada e a rede entre as duas são anotadas em cada resultado, porque a latência da rede entra nos números do k6 |
| Versão testada | O commit da `develop` (ou a tag) anotado em cada resultado |
| Dados | Um evento com 500 assentos criado pelo seed do Inventário (#23) e as contas de carga `loadbuyer0001`–`loadbuyer2000` e `loadscalper01`–`05` ([bots/README.md](../bots/README.md#contas-de-carga-testusers)) |
| Antes de cada rodada | Banco do Inventário e do Checkout zerados (evento novo), contadores do Prometheus anotados, tokens gerados de novo (eles duram 5 minutos e os bots renovam) |
| Ferramentas | k6 para latência e vazão; bots em Go (#51) para o fluxo completo e a conferência no banco; Prometheus, Grafana e Jaeger para o que acontece por dentro ([OBSERVABILIDADE.md](infra/OBSERVABILIDADE.md)) |

## Níveis de teste

| Nível | O que cobre | Onde roda | Responsável |
|---|---|---|---|
| Unidade | Regras de cada serviço sem rede: transições da SAGA, validação de token, contagem do rate limit, cálculo do limite de 4 assentos | `go test` / `npm test` em cada PR, pelo CI ([CI.md](infra/CI.md)) | Dono de cada serviço |
| Integração | Cada serviço contra o Postgres e o Redis de verdade, sem mock do banco: o `UPDATE` condicional, a outbox, o consumer group, a idempotência do Pagamento | Testes do serviço com os containers de pé | Dono de cada serviço |
| Contrato | Cada lado de uma chamada HTTP ou de um evento segue o arquivo em `docs/contracts/` | Revisão obrigatória de quem consome no PR do contrato; testes do consumidor contra exemplos do contrato | Quem publica e quem consome |
| Ponta a ponta | Uma compra inteira pelo Traefik: login, reserva, pagamento, evento, e-mail e estatística | Script da #53, contra o Compose | Bruno |
| Carga | Os cenários C1 a C5 abaixo | Máquina de carga contra a VPS | Johan, com o dono do serviço envolvido |
| Resiliência | Os cenários R1 a R10 abaixo, com falha injetada durante a carga | VPS, com `docker kill`/`stop` e a simulação do Pagamento | Dono do serviço que cai |
| Segurança | Os cenários S1 a S7 abaixo | VPS e CI | Johan |

### Testes que todo serviço precisa ter

Além dos testes da própria lógica, cada serviço cobre os casos que a arquitetura promete, porque são eles que viram critério de sucesso:

| Serviço | Casos obrigatórios | Issue |
|---|---|---|
| Inventário | Cada linha da tabela de operações do ADR-004 contra um Postgres real; hold vencido conta como livre; pedido com vários assentos segura todos ou nenhum; repetir uma chamada que deu certo não muda nada | #24 |
| Checkout | Todas as transições do diagrama do ADR-001 e as proibidas; dois `pay` simultâneos no mesmo pedido, só um passa; 5º assento recusado; CONFIRMED sem linha na outbox é impossível | #33, #34, #35, #36 |
| Pagamento | 10 cobranças simultâneas com a mesma `Idempotency-Key` geram 1 transação; estorno repetido devolve uma vez | #29 |
| Notificação e Estatísticas | Evento repetido não gera segundo e-mail nem segunda contagem; 3 falhas mandam para a DLQ sem travar as outras | #41, #42 |
| Guard | Os 14 tokens inválidos, a contagem por conta e o fail-open sem Redis ([GUARD.md](infra/GUARD.md#testes)) | feito |

## Carga

| # | Cenário | Como | Métrica | Sucesso | Origem | Issue |
|---|---|---|---|---|---|---|
| C1 | 1.000 bots no mesmo assento ao mesmo tempo | Bots disparam a reserva do mesmo assento juntos | Reservas com sucesso; consulta no banco | Exatamente 1 | ADR-004 | #54 |
| C2 | 2.000 bots disputando 500 assentos | Cada fã escolhe assentos livres do mapa, reserva e paga | Assentos vendidos, pedidos por estado final, assentos com dois donos (banco); p95 da reserva | Zero assentos com dois donos; todo pedido em estado final | Proposta (1), ADR-004 | #54 |
| C3 | Cambistas misturados aos fãs | 5 contas de cambista a 50 req/s cada, junto com os fãs do C2 | 429 sobre o excesso dos cambistas; erros dos fãs; assentos por conta de cambista | ≥ 95% do excesso recusado; < 1% de erro para os fãs; nenhum cambista com mais de 4 assentos no evento | Proposta (5), ADR-002 | #54 |
| C4 | 1.000 usuários consultando o mapa de assentos | k6 com 1.000 usuários virtuais lendo o mapa, sem login | p95 da consulta; acertos do cache | p95 < 200 ms | Proposta (4), ADR-004 | #54 |
| C5 | Deploy durante a carga | Canary 90/10 e depois troca total de uma versão nova do Checkout ou do Inventário com o C2 rodando | Taxa de 5xx no Traefik durante a troca | < 1% | Proposta (6) | #49 |

O C2 também é a carga de fundo dos testes de resiliência: as falhas da próxima seção são injetadas com ele rodando.

## Resiliência

| # | Falha injetada | Como | Resultado esperado | Métrica | Origem | Responsável | Issue |
|---|---|---|---|---|---|---|---|
| R1 | Checkout cai no meio da carga | `docker kill` numa réplica e subir de novo | Todo pedido termina CONFIRMED ou CANCELLED; nenhuma cobrança dupla no Pagamento | Pedidos por estado depois do job de recuperação; cobranças por pedido | ADR-001 | Bruno | #55 |
| R2 | Pagamento recusando 20% | Simulação do Pagamento (#30) | Nenhum assento preso; recusados liberados na hora | Assentos em hold sem pedido ativo | ADR-001 | Diego | #56 |
| R3 | Pagamento fora com estornos pendentes | Pagamento no modo "cobra e responde erro", depois `docker stop` e `start` | Todo pedido em AWAITING_REFUND termina CANCELLED, com um estorno só | Tempo até zerar AWAITING_REFUND; alerta do Grafana disparando | ADR-001, Proposta (2) | Diego | #56 |
| R4 | Pagamento lento | Latência de 5 s na simulação | O Circuit Breaker abre e o Checkout responde rápido; pedidos ficam em PAYING e o job termina depois | p95 do Checkout com o circuito aberto | C4 nível 2, ADR-008 (#38) | Bruno | #38 |
| R5 | Notificação fora durante as compras | `docker stop` na Notificação, 100 compras, `start` | Os 100 e-mails saem quando ela volta; a compra não é afetada | E-mails enviados; atraso entre `occurred_at` e o envio | ADR-006, Proposta (3) | Diego | #56 |
| R6 | Mensagem que sempre falha | Fazer a Notificação falhar sempre para um pedido | Vai para `tickets.issued.dead` depois de 3 tentativas; as outras seguem | Tamanho da DLQ; mensagens processadas | ADR-006, Proposta (3) | Diego | #41 |
| R7 | Redis fora durante a carga | `docker stop redis` com o C2 rodando, depois `start` | Ver a [divergência abaixo](#divergência-com-o-adr-006). Mapa continua respondendo direto do Postgres; nenhum evento se perde depois que ele volta | p95 do mapa sem cache; eventos na outbox antes e depois | ADR-006, C4 nível 2 | Luís | #57 |
| R8 | Uma réplica do Guard cai | `docker kill` numa réplica com carga | Sem erro perceptível | Taxa de 5xx no Traefik | ADR-002 | Johan | #58 |
| R9 | As duas réplicas do Guard caem | `docker kill` nas duas | Compra bloqueada (fail-closed); mapa de assentos continua funcionando | Códigos por rota no Traefik | ADR-002 | Johan | #58 |
| R10 | Keycloak cai | `docker stop keycloak` com carga | Compradores já logados continuam comprando (o Guard tem as chaves em memória); novos logins falham | Decisões do Guard; erros no login | ADR-007 | Johan | #58 |

### Divergência com o ADR-006

O ADR-006 diz que, com o Redis fora, "a compra é bloqueada". Isso deixou de ser verdade com o rate limit (#71): o Guard passou a deixar a requisição passar quando não consegue contar (fail-open, justificativa no [GUARD.md](infra/GUARD.md#se-o-redis-cair-deixa-passar-fail-open)), e o Inventário e o Checkout guardam o que importa no Postgres. Com o Redis fora, então, a compra deve **continuar**: o mapa fica mais lento sem o cache, o `TicketIssued` espera na outbox e sai quando o Redis voltar, e o limite por conta fica desligado nesse intervalo, com o limite de 4 assentos do Checkout segurando o cambista.

O R7 mede esse comportamento. Se o grupo concordar com ele, a decisão vira um ADR que substitui esse trecho do ADR-006 (a regra do repositório é não editar ADR aceito).

## Segurança

| # | Cenário | Resultado esperado | Origem | Situação |
|---|---|---|---|---|
| S1 | Tokens inválidos: sem token, malformado, vencido, adulterado, `alg: none`, HS256 com a chave pública, outro realm, outra audiência | 401 em todos | ADR-002, ADR-007 | Coberto pelos testes do Guard e verificado na v0.3 ([GUARD.md](infra/GUARD.md#resultados-verificados-v03)) |
| S2 | `X-User-Id` e `X-User-Roles` forjados pelo cliente | O serviço recebe só os valores do Guard | ADR-002 | Verificado na v0.3 |
| S3 | Console de administração e realm `master` pela entrada pública, incluindo variações de caminho | 403 | Issue #3 | Verificado em 19/11 ([KEYCLOAK.md](infra/KEYCLOAK.md#console-de-administração-fechado-na-entrada-pública)) |
| S4 | Um serviço lendo o schema de outro (`checkout_svc` fazendo `SELECT` em `inventory.seats`) | Negado pelo Postgres | ADR-005 | Verificado no setup ([POSTGRES.md](infra/POSTGRES.md)); repetir com os serviços reais |
| S5 | Cambista acima do limite por conta | 429 | ADR-002 | Verificado em 19/11 sem carga ([GUARD.md](infra/GUARD.md#resultados-verificados-v04)); repetir no C3 |
| S6 | Segredos no histórico e dependências vulneráveis | CI reprova | DevSecOps | `gitleaks`, `govulncheck` e `npm audit` em todo PR; análise de código e de imagens na #48 |
| S7 | Rotas internas (`/internal/...` do Inventário, Pagamento) pela entrada pública | 404 (sem rota no Traefik) | C4 nível 2 | Verificar quando os serviços existirem |

## Como registrar um resultado

Cada teste que roda ganha uma entrada na seção abaixo, com:

1. Data, commit testado e máquina de carga.
2. Configuração usada (número de bots, ritmo, duração, limite do Guard, taxa de falha do Pagamento).
3. Os números da métrica da tabela, com a consulta usada (PromQL ou SQL) para alguém conseguir repetir.
4. Print do painel do Grafana durante o teste, em `docs/resultados/` (criada no primeiro resultado com imagem).
5. Passou ou não passou no critério e, se não passou, o que mudou depois (calibragem, correção, novo ADR).

Resultado que não passou fica registrado também. Ele conta como lição aprendida, e o professor avalia "insights e lições aprendidas" (seção 4 do enunciado).

## Resultados

| Data | Teste | Resultado | Detalhe |
|---|---|---|---|
| 13/10/2026 | S1, S2, R8, R9 (sem carga) | Passou | 14 tokens inválidos recusados; identidade forjada substituída; 10 de 10 com uma réplica do Guard fora; compra bloqueada com as duas fora ([GUARD.md](infra/GUARD.md#resultados-verificados-v03)) |
| 19/11/2026 | S3 | Passou | Console e realm `master` com 403 na entrada pública, inclusive 7 variações de caminho ([KEYCLOAK.md](infra/KEYCLOAK.md#console-de-administração-fechado-na-entrada-pública)) |
| 19/11/2026 | S5, prévia do C3 (uma conta, sem carga de fundo) | Passou | Bot a 49 req/s por 6 s: 11,4 liberadas por segundo, 230 de 240 excedentes recusadas (95,8%). Pico de 2× na virada do segundo, preço da janela fixa ([GUARD.md](infra/GUARD.md#resultados-verificados-v04)) |
| 19/11/2026 | Prévia do R7 no Guard | Comportamento esperado | Com o Redis parado, 5 de 5 requisições passaram, cerca de 60 ms mais lentas |
| 19/11/2026 | Custo do Guard (ADR-002) | Medido | Decisão do Guard em 0,7 ms dentro de 1,3 ms de autenticação no Traefik; p95 de 0,5 ms ([OBSERVABILIDADE.md](infra/OBSERVABILIDADE.md#primeiro-resultado-19112026)) |
| 19/11/2026 | Preparação da carga | Pronto | 2.005 contas de carga criadas e logadas na VPS ([bots/README.md](../bots/README.md#resultados-na-vps-19112026)) |
