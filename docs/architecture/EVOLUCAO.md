# Evolução da arquitetura

Registro de como a arquitetura do Ticket Rush foi sendo construída, versão por versão: o que entrou, como o sistema estava naquele momento e o que aprendemos. Os detalhes de cada decisão ficam nos [ADRs](../adr/README.md); aqui fica a sequência.

## v0.1: proposta (09/10/2026)

Definição do tema e do problema: venda de ingressos sob alta demanda, com disputa por assentos, falhas no meio da compra e bots de cambistas. A [proposta](../PROPOSTA.md) listava sete padrões e as tecnologias previstas, ainda sem desenho de serviços.

**Aprendizado:** o tema foi escolhido a partir das áreas que o grupo queria aprofundar (confiabilidade, desempenho, segurança e deployment), e não o contrário. O critério que decidiu foi ter atores disputando o mesmo recurso, para que os problemas distribuídos aparecessem sozinhos nos testes com bots.

## v0.2: arquitetura no papel (13/10/2026)

O sistema foi dividido em seis serviços, com a regra de que cada um é dono de um dado e só ele escreve nesse dado. Foram escritos os diagramas C4 ([contexto](C4-1-CONTEXTO.md) e [containers](C4-2-CONTAINERS.md)), sete ADRs e o contrato do evento `TicketIssued`.

```
Comprador ─► Traefik ─► Guard (token, limite por conta)
                │
                ├─► Inventário ─► Postgres (inventory) + Redis (cache)
                └─► Checkout (SAGA) ─► Pagamento (externo)
                        └─► Redis Streams ─► Notificação, Estatísticas
```

**Aprendizados:**
- A primeira versão da SAGA tinha um mecanismo para cada caso de canto. Ela foi simplificada para três regras (tempo de 5 minutos, repetir é seguro, um job termina o que ficou pela metade), que cobrem os mesmos casos com menos peças ([ADR-001](../adr/0001-SAGA-HIBRIDA.md)).
- O limite de assentos por comprador tinha sido colocado no Guard, que não é dono dos pedidos. A regra "cada dado tem um dono" mostrou o erro, e o limite foi para o Checkout.
- A revisão cruzada dos documentos achou três lacunas que nenhum documento mostrava sozinho: a Notificação precisava de um lugar para guardar os pedidos já notificados, o Keycloak não tinha banco definido, e os nomes de estados e eventos estavam em dois idiomas.

## v0.3: infraestrutura rodando (em andamento)

Primeira versão que sobe com `docker compose up`: Traefik, PostgreSQL, Redis e Keycloak, todos com healthcheck. Ainda sem nenhum serviço da aplicação.

```
             :8180
Internet ──► Traefik ──/auth──► Keycloak ──┐
            [rede edge]                     │ [rede data]
                                            ▼
                                 Postgres (inventory, checkout,
                                 notification, stats, keycloak)
                                 Redis (AOF ligado)
```

O que foi verificado na prática, e não só no papel:
- `checkout_svc` não consegue ler `inventory.seats` nem criar tabelas no `public` ([POSTGRES.md](../infra/POSTGRES.md)). O isolamento do ADR-005 é imposto pelo banco.
- `keycloak_svc` não consegue conectar no database da aplicação.
- O Keycloak responde pelo Traefik e já publica o endereço das chaves públicas (JWKS), que o Guard vai usar.
- O Redis está com AOF ligado.

**Aprendizados:**
- O Traefik `v3.3.4` não enxergava os containers, porque usa uma versão da API do Docker que o Docker 29 não aceita mais. A imagem foi fixada em `v3.7.13` ([TRAEFIK.md](../infra/TRAEFIK.md)). Fixar versões evita surpresas, mas uma versão fixada pode envelhecer em relação ao ambiente.
- Separar as redes `edge` e `data` impede que o Traefik alcance os bancos, mesmo com uma rota errada.
