# Tarefas do Projeto 03

Mapa das tarefas até a entrega final de **11/12/2026**. O detalhe de cada tarefa (o que fazer, de quem depende, como testar, o que documentar) está na issue; aqui fica a visão de conjunto: quem faz o quê, o que trava o quê e o que já dá para começar.

O status de cada tarefa fica só na issue e no quadro, para não haver dois lugares dizendo coisas diferentes.

| Onde | Para quê |
|---|---|
| [Quadro do projeto](https://github.com/users/johancarloss/projects/1) | Colunas Bloqueada, Pronta, Em andamento, Em revisão e Feita, com o campo Fase |
| [Milestone Projeto 03](https://github.com/johancarloss/ticket-rush/milestone/1) | Quanto falta para a entrega |
| [Minhas tarefas](https://github.com/johancarloss/ticket-rush/issues/assigned/@me) | Só as issues atribuídas a você |
| [Prontas para pegar](https://github.com/johancarloss/ticket-rush/issues?q=is%3Aopen+-label%3Abloqueada+-label%3Aepic+milestone%3A%22Projeto+03%22) | Abertas, sem dependência pendente |

## Fases

| Fase | Até | Objetivo |
|---|---|---|
| 1 | 20/11 | Contratos, esqueletos e o que não depende de ninguém |
| 2 | 27/11 | Compra completa funcionando |
| 3 | 04/12 | Falhas, observabilidade, carga e resiliência |
| 4 | 11/12 | Resultados, documentação final e apresentação |

## Grafo de dependências

Seta cheia: a tarefa só começa quando a anterior fechar. Seta tracejada: integra com a anterior, mas dá para começar usando um mock do contrato. Cor: responsável.

```mermaid
flowchart LR
  subgraph F1 ["Fase 1"]
    INV_1["#21 Contrato Inventário"]
    INV_2["#22 Esqueleto Inventário"]
    PAY_1["#27 Contrato Pagamento"]
    PAY_2["#28 Esqueleto Pagamento"]
    CHK_1["#31 Contrato Checkout"]
    CHK_2["#32 Esqueleto Checkout"]
    NOT_1["#40 Consumidor Notificação"]
    STA_1["#42 Estatísticas"]
    PLT_RL["#44 Rate limit no Guard"]
    PLT_CI["#45 CI TS e Go"]
    PLT_OBS["#46 Prometheus, Grafana, Jaeger"]
    TST_PLAN["#52 Plano de testes"]
    DOC_QA["#59 Perguntas ao professor"]
  end
  subgraph F2 ["Fase 2"]
    INV_3["#23 Eventos e assentos"]
    INV_4["#24 Hold e venda (UPDATE condicional)"]
    PAY_3["#29 Cobrança idempotente"]
    CHK_3["#33 Máquina de estados"]
    CHK_4["#34 Reservar + limite 4"]
    CHK_5["#35 Pagar e compensar"]
    CHK_6["#36 Outbox + TicketIssued"]
    NOT_2["#41 E-mail, retry e DLQ"]
    PLT_USR["#50 Contas de teste"]
    PLT_BOT["#51 Bots Go"]
    TST_E2E["#53 Compra ponta a ponta"]
  end
  subgraph F3 ["Fase 3"]
    INV_5["#25 Cache do mapa"]
    INV_6["#26 Métricas Inventário"]
    PAY_4["#30 Simulação de falhas"]
    CHK_7["#37 Job de recuperação"]
    CHK_8["#38 Circuit Breaker"]
    CHK_9["#39 Métricas Checkout"]
    DGO_OBS["#43 Métricas serviços Go"]
    PLT_DASH["#47 Painéis e alerta"]
    PLT_SEC["#48 DevSecOps"]
    PLT_DEP["#49 Blue-Green e Canary"]
    TST_LOAD["#54 Carga 2.000 × 500"]
    TST_RES_CHK["#55 Resiliência Checkout"]
    TST_RES_PAY["#56 Resiliência Pagamento"]
    TST_RES_INV["#57 Resiliência Inventário"]
    TST_RES_GRD["#58 Resiliência entrada"]
  end
  subgraph F4 ["Fase 4"]
    DOC_ARCH["#60 C4 e ADRs finais"]
    DOC_RES["#61 Resultados e lições"]
    DOC_README["#62 README final"]
    DOC_PRES["#63 Apresentação"]
    DOC_REL["#64 Release v1.0.0"]
  end
  INV_1 --> INV_2
  PLT_CI -.-> INV_2
  INV_2 --> INV_3
  INV_3 --> INV_4
  INV_3 --> INV_5
  INV_2 --> INV_6
  PLT_OBS --> INV_6
  PAY_1 --> PAY_2
  PLT_CI -.-> PAY_2
  PAY_2 --> PAY_3
  PAY_3 --> PAY_4
  INV_1 -.-> CHK_1
  PAY_1 -.-> CHK_1
  CHK_1 --> CHK_2
  PLT_CI -.-> CHK_2
  CHK_2 --> CHK_3
  CHK_3 --> CHK_4
  INV_4 -.-> CHK_4
  CHK_4 --> CHK_5
  PAY_3 -.-> CHK_5
  INV_4 -.-> CHK_5
  CHK_5 --> CHK_6
  CHK_5 --> CHK_7
  PAY_3 -.-> CHK_7
  CHK_5 --> CHK_8
  PAY_4 -.-> CHK_8
  CHK_2 --> CHK_9
  PLT_OBS --> CHK_9
  PLT_CI -.-> NOT_1
  NOT_1 --> NOT_2
  PLT_CI -.-> STA_1
  PAY_2 --> DGO_OBS
  NOT_1 --> DGO_OBS
  STA_1 --> DGO_OBS
  PLT_OBS --> DGO_OBS
  PLT_OBS --> PLT_DASH
  CHK_9 --> PLT_DASH
  PLT_CI --> PLT_SEC
  CHK_2 --> PLT_DEP
  PLT_DASH -.-> PLT_DEP
  PLT_USR --> PLT_BOT
  CHK_1 -.-> PLT_BOT
  CHK_6 --> TST_E2E
  NOT_2 --> TST_E2E
  STA_1 --> TST_E2E
  TST_E2E --> TST_LOAD
  PLT_BOT --> TST_LOAD
  PLT_DASH --> TST_LOAD
  PLT_RL -.-> TST_LOAD
  CHK_7 --> TST_RES_CHK
  TST_LOAD --> TST_RES_CHK
  PAY_4 --> TST_RES_PAY
  CHK_7 --> TST_RES_PAY
  NOT_2 --> TST_RES_PAY
  TST_LOAD --> TST_RES_PAY
  INV_4 --> TST_RES_INV
  INV_5 --> TST_RES_INV
  TST_LOAD -.-> TST_RES_INV
  PLT_RL --> TST_RES_GRD
  TST_LOAD --> TST_RES_GRD
  TST_LOAD --> DOC_ARCH
  TST_LOAD --> DOC_RES
  TST_RES_CHK --> DOC_RES
  TST_RES_PAY --> DOC_RES
  TST_RES_INV --> DOC_RES
  TST_RES_GRD --> DOC_RES
  TST_E2E --> DOC_README
  DOC_RES --> DOC_PRES
  DOC_ARCH --> DOC_REL
  DOC_RES --> DOC_REL
  DOC_README --> DOC_REL
  classDef johan fill:#6f42c1,stroke:#222,color:#fff
  class PLT_RL,PLT_CI,PLT_OBS,PLT_DASH,PLT_SEC,PLT_DEP,PLT_USR,PLT_BOT,TST_PLAN,TST_LOAD,TST_RES_GRD,DOC_QA,DOC_ARCH,DOC_RES,DOC_README,DOC_PRES,DOC_REL johan
  classDef bruno fill:#0b4884,stroke:#222,color:#fff
  class CHK_1,CHK_2,CHK_3,CHK_4,CHK_5,CHK_6,CHK_7,CHK_8,CHK_9,TST_E2E,TST_RES_CHK bruno
  classDef diego fill:#00758f,stroke:#222,color:#fff
  class PAY_1,PAY_2,PAY_3,PAY_4,NOT_1,NOT_2,STA_1,DGO_OBS,TST_RES_PAY diego
  classDef luis fill:#1168bd,stroke:#222,color:#fff
  class INV_1,INV_2,INV_3,INV_4,INV_5,INV_6,TST_RES_INV luis
```

Legenda das cores: roxo Johan, azul-escuro Bruno, verde-azulado Diego, azul Luís.

## Caminho crítico

Se uma destas atrasa, a entrega inteira atrasa. Elas vêm primeiro na fila de cada um:

[#21](https://github.com/johancarloss/ticket-rush/issues/21) Contrato Inventário → [#22](https://github.com/johancarloss/ticket-rush/issues/22) Esqueleto Inventário → [#23](https://github.com/johancarloss/ticket-rush/issues/23) Eventos e assentos → [#24](https://github.com/johancarloss/ticket-rush/issues/24) Hold e venda (UPDATE condicional) → [#34](https://github.com/johancarloss/ticket-rush/issues/34) Reservar + limite 4 → [#35](https://github.com/johancarloss/ticket-rush/issues/35) Pagar e compensar → [#36](https://github.com/johancarloss/ticket-rush/issues/36) Outbox + TicketIssued → [#53](https://github.com/johancarloss/ticket-rush/issues/53) Compra ponta a ponta → [#54](https://github.com/johancarloss/ticket-rush/issues/54) Carga 2.000 × 500 → [#61](https://github.com/johancarloss/ticket-rush/issues/61) Resultados e lições → [#63](https://github.com/johancarloss/ticket-rush/issues/63) Apresentação

## Pode começar agora

Tarefas sem nenhuma dependência:

- [#21](https://github.com/johancarloss/ticket-rush/issues/21) Contrato Inventário (Luís)
- [#27](https://github.com/johancarloss/ticket-rush/issues/27) Contrato Pagamento (Diego)
- [#31](https://github.com/johancarloss/ticket-rush/issues/31) Contrato Checkout (Bruno)
- [#40](https://github.com/johancarloss/ticket-rush/issues/40) Consumidor Notificação (Diego)
- [#42](https://github.com/johancarloss/ticket-rush/issues/42) Estatísticas (Diego)
- [#44](https://github.com/johancarloss/ticket-rush/issues/44) Rate limit no Guard (Johan)
- [#45](https://github.com/johancarloss/ticket-rush/issues/45) CI TS e Go (Johan)
- [#46](https://github.com/johancarloss/ticket-rush/issues/46) Prometheus, Grafana, Jaeger (Johan)
- [#50](https://github.com/johancarloss/ticket-rush/issues/50) Contas de teste (Johan)
- [#52](https://github.com/johancarloss/ticket-rush/issues/52) Plano de testes (Johan)
- [#59](https://github.com/johancarloss/ticket-rush/issues/59) Perguntas ao professor (Johan)
- [#3](https://github.com/johancarloss/ticket-rush/issues/3) Bloquear o console do Keycloak (Johan)

## Tarefas por pessoa

### Johan

Épicos: [#18](https://github.com/johancarloss/ticket-rush/issues/18) [Plataforma] Entrada, CI/CD, observabilidade, deploy e bots, [#19](https://github.com/johancarloss/ticket-rush/issues/19) [Testes] Plano, carga, resiliência e resultados, [#20](https://github.com/johancarloss/ticket-rush/issues/20) [Entrega] Documentação final, release e apresentação

| Issue | Tarefa | Fase | Depende de |
|---|---|---|---|
| [#44](https://github.com/johancarloss/ticket-rush/issues/44) | Rate limit por conta | 1 | nada |
| [#45](https://github.com/johancarloss/ticket-rush/issues/45) | Jobs para serviços TypeScript e Go | 1 | nada |
| [#46](https://github.com/johancarloss/ticket-rush/issues/46) | Prometheus, Grafana e Jaeger (ADR-010) | 1 | nada |
| [#52](https://github.com/johancarloss/ticket-rush/issues/52) | Plano de testes | 1 | nada |
| [#59](https://github.com/johancarloss/ticket-rush/issues/59) | Perguntas ao professor | 1 | nada |
| [#50](https://github.com/johancarloss/ticket-rush/issues/50) | Contas de teste em lote para os bots | 2 | nada |
| [#51](https://github.com/johancarloss/ticket-rush/issues/51) | Fãs e cambistas em Go | 2 | [#50](https://github.com/johancarloss/ticket-rush/issues/50) |
| [#47](https://github.com/johancarloss/ticket-rush/issues/47) | Painéis e alerta de estorno | 3 | [#46](https://github.com/johancarloss/ticket-rush/issues/46), [#39](https://github.com/johancarloss/ticket-rush/issues/39) |
| [#48](https://github.com/johancarloss/ticket-rush/issues/48) | DevSecOps: análise de código e de imagens | 3 | [#45](https://github.com/johancarloss/ticket-rush/issues/45) |
| [#49](https://github.com/johancarloss/ticket-rush/issues/49) | Blue-Green e Canary pelo Traefik (ADR-009) | 3 | [#32](https://github.com/johancarloss/ticket-rush/issues/32) |
| [#54](https://github.com/johancarloss/ticket-rush/issues/54) | Carga: 2.000 bots disputando 500 assentos | 3 | [#53](https://github.com/johancarloss/ticket-rush/issues/53), [#51](https://github.com/johancarloss/ticket-rush/issues/51), [#47](https://github.com/johancarloss/ticket-rush/issues/47) |
| [#58](https://github.com/johancarloss/ticket-rush/issues/58) | Resiliência e segurança da entrada | 3 | [#44](https://github.com/johancarloss/ticket-rush/issues/44), [#54](https://github.com/johancarloss/ticket-rush/issues/54) |
| [#60](https://github.com/johancarloss/ticket-rush/issues/60) | C4, evolução e trade-offs atualizados | 4 | [#54](https://github.com/johancarloss/ticket-rush/issues/54) |
| [#61](https://github.com/johancarloss/ticket-rush/issues/61) | Resultados e lições aprendidas consolidados | 4 | [#54](https://github.com/johancarloss/ticket-rush/issues/54), [#55](https://github.com/johancarloss/ticket-rush/issues/55), [#56](https://github.com/johancarloss/ticket-rush/issues/56), [#57](https://github.com/johancarloss/ticket-rush/issues/57), [#58](https://github.com/johancarloss/ticket-rush/issues/58) |
| [#62](https://github.com/johancarloss/ticket-rush/issues/62) | README final e seção de IA | 4 | [#53](https://github.com/johancarloss/ticket-rush/issues/53) |
| [#63](https://github.com/johancarloss/ticket-rush/issues/63) | Apresentação final presencial (12 a 15 min) | 4 | [#61](https://github.com/johancarloss/ticket-rush/issues/61) |
| [#64](https://github.com/johancarloss/ticket-rush/issues/64) | Release v1.0.0 | 4 | [#60](https://github.com/johancarloss/ticket-rush/issues/60), [#61](https://github.com/johancarloss/ticket-rush/issues/61), [#62](https://github.com/johancarloss/ticket-rush/issues/62) |
| [#3](https://github.com/johancarloss/ticket-rush/issues/3) | Bloquear o console de administração do Keycloak | 1 | nada |

### Bruno

Épicos: [#14](https://github.com/johancarloss/ticket-rush/issues/14) [Checkout] Orquestrador da compra (SAGA)

| Issue | Tarefa | Fase | Depende de |
|---|---|---|---|
| [#31](https://github.com/johancarloss/ticket-rush/issues/31) | Contrato OpenAPI | 1 | nada |
| [#32](https://github.com/johancarloss/ticket-rush/issues/32) | Esqueleto no Compose | 1 | [#31](https://github.com/johancarloss/ticket-rush/issues/31) |
| [#33](https://github.com/johancarloss/ticket-rush/issues/33) | Pedidos e máquina de estados | 2 | [#32](https://github.com/johancarloss/ticket-rush/issues/32) |
| [#34](https://github.com/johancarloss/ticket-rush/issues/34) | Reservar assentos e limite de 4 por comprador | 2 | [#33](https://github.com/johancarloss/ticket-rush/issues/33) |
| [#35](https://github.com/johancarloss/ticket-rush/issues/35) | Pagar e confirmar ou compensar | 2 | [#34](https://github.com/johancarloss/ticket-rush/issues/34) |
| [#36](https://github.com/johancarloss/ticket-rush/issues/36) | Transactional Outbox e publicação do TicketIssued | 2 | [#35](https://github.com/johancarloss/ticket-rush/issues/35) |
| [#53](https://github.com/johancarloss/ticket-rush/issues/53) | Compra ponta a ponta automatizada | 2 | [#36](https://github.com/johancarloss/ticket-rush/issues/36), [#41](https://github.com/johancarloss/ticket-rush/issues/41), [#42](https://github.com/johancarloss/ticket-rush/issues/42) |
| [#37](https://github.com/johancarloss/ticket-rush/issues/37) | Job de recuperação e estornos | 3 | [#35](https://github.com/johancarloss/ticket-rush/issues/35) |
| [#38](https://github.com/johancarloss/ticket-rush/issues/38) | Circuit Breaker e timeouts (ADR-008) | 3 | [#35](https://github.com/johancarloss/ticket-rush/issues/35) |
| [#39](https://github.com/johancarloss/ticket-rush/issues/39) | Métricas e traces | 3 | [#32](https://github.com/johancarloss/ticket-rush/issues/32), [#46](https://github.com/johancarloss/ticket-rush/issues/46) |
| [#55](https://github.com/johancarloss/ticket-rush/issues/55) | Resiliência: Checkout cai no meio da carga | 3 | [#37](https://github.com/johancarloss/ticket-rush/issues/37), [#54](https://github.com/johancarloss/ticket-rush/issues/54) |

### Diego

Épicos: [#15](https://github.com/johancarloss/ticket-rush/issues/15) [Pagamento] Gateway de cartão simulado, [#16](https://github.com/johancarloss/ticket-rush/issues/16) [Notificação] E-mail de confirmação, [#17](https://github.com/johancarloss/ticket-rush/issues/17) [Estatísticas] Contadores de vendas

| Issue | Tarefa | Fase | Depende de |
|---|---|---|---|
| [#27](https://github.com/johancarloss/ticket-rush/issues/27) | Contrato OpenAPI | 1 | nada |
| [#28](https://github.com/johancarloss/ticket-rush/issues/28) | Esqueleto no Compose com schema próprio | 1 | [#27](https://github.com/johancarloss/ticket-rush/issues/27) |
| [#40](https://github.com/johancarloss/ticket-rush/issues/40) | Esqueleto e consumidor do TicketIssued | 1 | nada |
| [#42](https://github.com/johancarloss/ticket-rush/issues/42) | Consumidor e contadores por evento | 1 | nada |
| [#29](https://github.com/johancarloss/ticket-rush/issues/29) | Cobrança e estorno idempotentes e consulta por pedido | 2 | [#28](https://github.com/johancarloss/ticket-rush/issues/28) |
| [#41](https://github.com/johancarloss/ticket-rush/issues/41) | E-mail, idempotência, retry e DLQ | 2 | [#40](https://github.com/johancarloss/ticket-rush/issues/40) |
| [#30](https://github.com/johancarloss/ticket-rush/issues/30) | Simulação de falhas e lentidão | 3 | [#29](https://github.com/johancarloss/ticket-rush/issues/29) |
| [#43](https://github.com/johancarloss/ticket-rush/issues/43) | Métricas e traces | 3 | [#28](https://github.com/johancarloss/ticket-rush/issues/28), [#40](https://github.com/johancarloss/ticket-rush/issues/40), [#42](https://github.com/johancarloss/ticket-rush/issues/42), [#46](https://github.com/johancarloss/ticket-rush/issues/46) |
| [#56](https://github.com/johancarloss/ticket-rush/issues/56) | Resiliência: Pagamento recusando e fora do ar | 3 | [#30](https://github.com/johancarloss/ticket-rush/issues/30), [#37](https://github.com/johancarloss/ticket-rush/issues/37), [#41](https://github.com/johancarloss/ticket-rush/issues/41), [#54](https://github.com/johancarloss/ticket-rush/issues/54) |

### Luís

Épicos: [#13](https://github.com/johancarloss/ticket-rush/issues/13) [Inventário] Serviço de eventos e assentos

| Issue | Tarefa | Fase | Depende de |
|---|---|---|---|
| [#21](https://github.com/johancarloss/ticket-rush/issues/21) | Contrato OpenAPI | 1 | nada |
| [#22](https://github.com/johancarloss/ticket-rush/issues/22) | Esqueleto no Compose | 1 | [#21](https://github.com/johancarloss/ticket-rush/issues/21) |
| [#23](https://github.com/johancarloss/ticket-rush/issues/23) | Tabelas e criação de eventos e assentos | 2 | [#22](https://github.com/johancarloss/ticket-rush/issues/22) |
| [#24](https://github.com/johancarloss/ticket-rush/issues/24) | Segurar, renovar, vender e liberar com UPDATE condicional | 2 | [#23](https://github.com/johancarloss/ticket-rush/issues/23) |
| [#25](https://github.com/johancarloss/ticket-rush/issues/25) | Mapa de assentos com Cache-Aside no Redis | 3 | [#23](https://github.com/johancarloss/ticket-rush/issues/23) |
| [#26](https://github.com/johancarloss/ticket-rush/issues/26) | Métricas e traces | 3 | [#22](https://github.com/johancarloss/ticket-rush/issues/22), [#46](https://github.com/johancarloss/ticket-rush/issues/46) |
| [#57](https://github.com/johancarloss/ticket-rush/issues/57) | Resiliência: Redis fora e disputa pelo mesmo assento | 3 | [#24](https://github.com/johancarloss/ticket-rush/issues/24), [#25](https://github.com/johancarloss/ticket-rush/issues/25) |

## Ciclo de uma tarefa

Vale para toda tarefa, de qualquer pessoa:

1. **Pegar.** Escolha uma issue sua na coluna *Pronta* do quadro (nenhuma dependência aberta). Mova para *Em andamento*.
2. **Branch.** Saia da `develop` com `feature/<escopo>/<descricao>`, conforme o [CONTRIBUTING](../../CONTRIBUTING.md#branches).
3. **Fazer.** Código e testes até todos os itens de *Pronto quando* da issue estarem marcados. Se a tarefa crescer, quebre numa issue nova em vez de aumentar o PR.
4. **Documentar.** Atualize os arquivos de *Documentar ao fechar*. Se usou alguma ferramenta de IA, anote na seção do README (sem ela a entrega é desclassificada).
5. **PR.** Abra para a `develop` com `Closes #N` na descrição e mova a issue para *Em revisão*. CI verde e aprovação de outro integrante; num contrato, quem consome é quem aprova.
6. **Merge.** A issue fecha sozinha. Abra cada issue de *Libera* e, se ela não tiver mais dependência aberta, tire o label `bloqueada` e mova para *Pronta*. Avise o responsável no grupo.

## Criar uma tarefa nova

Use **New issue → Tarefa** no GitHub. O modelo ([`.github/ISSUE_TEMPLATE/tarefa.md`](../../.github/ISSUE_TEMPLATE/tarefa.md)) já traz as seções. Coloque o label da área e da fase, o milestone *Projeto 03*, o responsável, as dependências (*Relationships → Blocked by*) e o épico (*Relationships → Parent*). Depois acrescente a linha na tabela da pessoa e no grafo acima.

