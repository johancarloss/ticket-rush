# ADR-002: Entrada pelo Traefik com um Guard próprio em Go

- **Status:** aceita
- **Data:** 13/10/2026
- **Responsável pela implementação:** Johan (Traefik e Guard)

## Contexto

Toda requisição de fora precisa passar por um ponto único que faça cinco coisas: HTTPS, encaminhar para o serviço certo, validar quem é o comprador, conter bots de cambistas e dividir o tráfego entre versões durante um deploy (Blue-Green e Canary).

Três dessas coisas são genéricas e iguais em qualquer sistema (HTTPS, roteamento, divisão de tráfego). As outras dependem do nosso domínio: saber quem é o comprador e o que conta como abuso numa venda de ingressos.

## Decisão

Separar a entrada em duas peças:

- **Traefik (pronto)** faz o trabalho genérico: TLS, roteamento por labels do Docker e divisão de tráfego por peso, que é o que o Blue-Green e o Canary usam.
- **Guard (Go, escrito pelo grupo)** é consultado pelo Traefik antes de cada requisição de compra, pelo middleware `forwardAuth`. Ele responde "pode passar" ou "não pode".

```
Internet ──► Traefik ──────────────────────────► Inventário / Checkout
                │ forwardAuth (só rotas de compra)
                ▼
             Guard [Go] ──► Keycloak (chaves para validar o token)
                       └──► Redis (contadores por conta)
```

### Quem aplica cada regra

| Regra | Onde | Motivo |
|---|---|---|
| Token válido | Guard | Vale para toda rota de compra |
| No máximo 10 requisições de compra por segundo por conta | Guard | Contagem de requisições, sem dado de negócio; barra o bot que martela a API |
| No máximo 4 assentos por comprador por evento | Checkout | O Checkout é dono dos pedidos e conta no próprio banco; se o Guard contasse, teria uma cópia do dado que pode divergir da real |

O limite é por conta, e não por IP, porque um cambista com muitos IPs continua sendo uma conta só. Os valores (10 por segundo, 4 assentos) são iniciais e serão calibrados com os testes de bots.

### Se o Guard cair: bloqueia (fail-closed)

Sem o Guard, o sistema não sabe quem é o comprador, e deixar passar criaria pedidos sem dono, contra a ADR-001. Por isso as rotas de compra ficam bloqueadas enquanto ele estiver fora. Esse já é o comportamento padrão do `forwardAuth` do Traefik. Para doer menos:

- **Duas réplicas do Guard.** O Traefik divide as requisições entre elas; se uma cai, a outra atende.
- **Leitura pública não passa pelo Guard.** Ver eventos e o mapa de assentos não exige login, então essas rotas vão direto ao Inventário e continuam funcionando com o Guard fora. Só reservar e pagar ficam bloqueados.

## Alternativas consideradas

- **Gateway inteiro escrito em Go.** Teríamos que reimplementar TLS, roteamento, health check e divisão de tráfego por peso, que já existem prontos e testados. O tempo iria para a parte genérica.
- **Só Traefik.** O rate limit dele é por IP ou por cabeçalho e não entende o que é um comprador; um cambista com vários IPs passaria.
- **Kong.** Tem plugins mais ricos, mas precisa de banco próprio e é mais pesado para configurar, e também não tem regra de domínio.

## Consequências

**Positivas**
- Blue-Green e Canary usam um recurso nativo e confiável do Traefik.
- O código próprio fica concentrado no que é específico do projeto: validar o comprador e conter bots.
- Inventário e Checkout não ficam expostos; tudo que vem de fora passa pelo Traefik.

**Negativas e riscos aceitos**
- **Um salto de rede a mais** em cada requisição de compra (Traefik → Guard). O custo será medido no Jaeger.
- **O Guard é peça crítica para comprar.** Mitigado com duas réplicas; se as duas caírem, a compra para, mas a consulta de assentos continua.
- **O Redis também vira dependência do Guard** (contadores). Se ele cair, o Guard bloqueia as compras, pela mesma regra de fail-closed.

## Como validar

- Bots cambistas acima de 10 requisições por segundo: pelo menos 95% do excesso recusado com `429`, e menos de 1% de erro para os bots de fãs.
- Comprador tentando o 5º assento no mesmo evento: recusado pelo Checkout.
- Derrubar uma réplica do Guard durante a carga: sem erro perceptível. Derrubar as duas: compra bloqueada, mapa de assentos funcionando.
- Medir no Jaeger o tempo gasto no Guard (p95).
