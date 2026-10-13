# ADR-002: Entrada pelo Traefik com um Guard próprio em Go

- **Status:** aceita
- **Data:** 13/10/2026
- **Responsável pela implementação:** Johan (Traefik e Guard)

## Contexto

Toda requisição de fora precisa passar por um ponto único que faça cinco coisas: HTTPS, encaminhar para o serviço certo, validar quem é o comprador, conter bots de cambistas e dividir o tráfego entre versões durante um deploy (Blue-Green e Canary).

Três dessas coisas são genéricas e iguais em qualquer sistema (HTTPS, roteamento, divisão de tráfego). As outras dependem do nosso domínio: saber quem é o comprador e o que conta como abuso numa venda de ingressos.

## O que são o Traefik e o Guard

### Traefik

O Traefik é um **proxy reverso** de código aberto. Um proxy reverso é um servidor que fica na frente dos serviços e recebe todas as requisições de fora no lugar deles. Funciona como a recepção de um prédio comercial: o visitante não sabe em que sala cada empresa fica, ele fala com a recepção, que confere quem ele é e indica o andar certo.

No Ticket Rush, o comprador só conhece um endereço (por exemplo `https://ticketrush.exemplo`). O Traefik olha o caminho da requisição e decide para onde mandar:

```
https://ticketrush.exemplo/api/events/...    →  Inventário
https://ticketrush.exemplo/api/orders/...    →  Checkout
```

Três coisas fazem o Traefik encaixar bem neste projeto:

- **Configuração pelos próprios containers.** Cada serviço declara no `docker-compose.yml`, por meio de labels, qual caminho ele atende. O Traefik lê essas labels e monta as rotas sozinho; subir um serviço novo não exige editar um arquivo central.
- **HTTPS automático.** Ele pede e renova os certificados no Let's Encrypt.
- **Divisão de tráfego por peso.** É possível dizer "mande 90% para a versão 1 do Checkout e 10% para a versão 2". É isso que o Canary usa para testar uma versão nova com parte dos compradores, e o Blue-Green usa para trocar 100% de uma vez.

### Guard

O Guard é um serviço pequeno, escrito pelo grupo em Go. O nome é nosso. Ele não atende o comprador; quem fala com ele é só o Traefik, por meio de um recurso chamado **`forwardAuth`** ("encaminhar para autenticação").

Com o `forwardAuth` ligado numa rota, o Traefik, antes de encaminhar a requisição, faz uma pergunta ao Guard e obedece a resposta:

```
1. Comprador → Traefik:   POST /api/orders   (com o token no cabeçalho Authorization)
2. Traefik → Guard:       "posso deixar passar?" (repassa os cabeçalhos da requisição original)
3. Guard confere:
     - o token é válido? (assinatura conferida com as chaves do Keycloak)
     - essa conta já passou de 10 requisições de compra neste segundo? (contador no Redis)
4. Guard → Traefik:
     - 200 OK + cabeçalho X-User-Id: 42   → o Traefik encaminha ao Checkout, levando o X-User-Id
     - 401 (token inválido) ou 429 (muitas requisições) → o Traefik devolve essa resposta ao comprador
                                                          e o Checkout nem fica sabendo
```

Duas consequências práticas:

- **Os serviços confiam no `X-User-Id`.** O Checkout não precisa validar token nenhum; ele recebe o ID do comprador já conferido pelo Guard. Isso só é seguro porque o Checkout não é acessível de fora, apenas pelo Traefik.
- **O Guard é a única parte da entrada com código nosso.** Tudo que é genérico fica no Traefik; o que é específico do Ticket Rush (quem é o comprador e o que conta como excesso) fica no Guard.

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
