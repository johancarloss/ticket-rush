# Guard

O Guard é o serviço que decide se uma requisição de compra pode entrar no sistema. Quem pergunta é o Traefik, antes de encaminhar a requisição ao serviço de destino; o comprador nunca fala com o Guard diretamente. O porquê dessa peça está no [ADR-002](../adr/0002-TRAEFIK-GUARD.md).

Código em [`services/guard/`](../../services/guard/), escrito em Go.

## Como uma requisição passa por ele

```
Comprador ──► Traefik ── rota com o middleware buyer-auth
                 │
                 │ 1. strip-identity: apaga X-User-Id e X-User-Roles que o comprador tenha mandado
                 │ 2. guard-auth: pergunta ao Guard (GET http://guard:8080/verify, com os cabeçalhos originais)
                 ▼
               Guard ── confere o token com as chaves públicas do Keycloak (guardadas em memória)
                 │
                 │  token inválido → 401, o Traefik devolve ao comprador e o serviço nem fica sabendo
                 │  token válido   → 200 + X-User-Id (quem é) + X-User-Roles (papéis)
                 ▼
              Traefik encaminha ao serviço, levando os dois cabeçalhos definidos pelo Guard
```

Os middlewares ficam em [`infra/traefik/dynamic.yml`](../../infra/traefik/dynamic.yml). Uma rota passa a exigir usuário logado só por declarar `middlewares=buyer-auth@file` nas labels.

O Guard responde **quem é** o usuário, e não **o que ele pode fazer**. Um organizador também passa pelo `buyer-auth`; é o serviço que confere o `X-User-Roles` quando uma rota exige um papel específico.

### Por que apagar os cabeçalhos de identidade antes

Os serviços confiam no `X-User-Id` e no `X-User-Roles`. Se um comprador mandasse `X-User-Id: 999` por conta própria e o Guard não sobrescrevesse esse cabeçalho, o valor falso chegaria ao serviço. O middleware `strip-identity` apaga os dois cabeçalhos que vieram de fora, então os únicos valores que chegam ao serviço são os que o Guard devolveu.

## O que ele verifica

| Versão | Regra |
|---|---|
| v0.3, primeira fatia | Exigia só a presença do cabeçalho `Authorization: Bearer <token>` |
| v0.3 | Confere o token emitido pelo Keycloak e devolve `X-User-Id` e `X-User-Roles` |
| **v0.4 (atual)** | Limite de requisições de compra por segundo por conta (10 por padrão), com contadores no Redis |

Para um token ser aceito, todas as condições precisam valer:

| Condição | Recusa se |
|---|---|
| Algoritmo | Não for RS256. Bloqueia `alg: none` e a troca por HS256 usando a chave pública como segredo |
| Assinatura | Não bater com uma chave pública do realm `ticket-rush` |
| Emissor (`iss`) | Não for o realm `ticket-rush` (um token do realm `master`, por exemplo) |
| Audiência (`aud`) | Não incluir `ticket-rush-api` |
| Validade (`exp`) | Estiver vencido ou não tiver data de validade |
| Usuário (`sub`) | Estiver vazio |

### Chaves públicas

O Guard busca as chaves no Keycloak pela rede interna (`KEYCLOAK_JWKS_URL`) e as guarda em memória. A partir daí, cada token é conferido sem falar com o Keycloak. Ele só busca de novo quando chega um token assinado com uma chave que ainda não conhece, o que acontece na primeira requisição e quando o Keycloak troca de chave. Essa nova busca acontece no máximo a cada 30 segundos, para que tokens com IDs de chave inventados não sobrecarreguem o Keycloak.

| Variável | Valor no Compose |
|---|---|
| `KEYCLOAK_JWKS_URL` | `http://keycloak:8080/auth/realms/ticket-rush/protocol/openid-connect/certs` |
| `TOKEN_ISSUER` | `http://localhost:8180/auth/realms/ticket-rush` |
| `TOKEN_AUDIENCE` | `ticket-rush-api` |

Sem qualquer uma delas, o Guard não sobe.

## Rotas

| Rota | Quem chama | Resposta |
|---|---|---|
| `GET /health` | Healthcheck do Docker | `200 {"status":"ok"}` |
| `/verify` | Traefik (forwardAuth) | `200` com `X-User-Id` e `X-User-Roles`; `401 {"error": ...}` |
| `GET /metrics` | Prometheus | Métricas no formato do Prometheus: decisões por resultado e duração de cada decisão ([OBSERVABILIDADE.md](OBSERVABILIDADE.md#o-que-já-é-coletado)) |

Cada decisão é registrada em log JSON, com o motivo:

```json
{"level":"INFO","msg":"request allowed","user_id":"7e582f2d-...","method":"GET","uri":"/api/whoami"}
{"level":"INFO","msg":"request denied","reason":"unknown_key","method":"GET","uri":"/api/whoami"}
```

Motivos possíveis: `missing_token`, `malformed_token`, `bad_algorithm`, `bad_signature`, `unknown_key`, `wrong_issuer`, `wrong_audience`, `token_expired`, `invalid_token`. Os mesmos motivos são o rótulo `result` da métrica `guard_decisions_total`, com `allowed` para as liberadas.

Cada requisição ao `/verify` também vira um trecho de trace no Jaeger, dentro do trace que o Traefik abriu para a compra (o Traefik repassa o cabeçalho `traceparent`). Sem a variável `OTEL_EXPORTER_OTLP_ENDPOINT`, o tracing fica desligado.

## Limite por conta (rate limit)

Depois de conferir o token, o Guard soma a requisição no contador da conta no segundo atual. Passou do limite, responde `429 Too Many Requests` com `Retry-After: 1`, e o Traefik devolve isso ao comprador sem a requisição chegar ao serviço. O limite vem de `GUARD_RATE_LIMIT_PER_SECOND` no `.env` (padrão 10, o valor inicial do ADR-002, a ser calibrado com os bots).

| Escolha | Por quê |
|---|---|
| **Por conta, não por IP** | Um cambista com muitos IPs continua sendo uma conta só (ADR-002). Por isso o contador vem depois do token: sem token válido não existe conta, e a requisição já foi recusada com 401 |
| **Contador no Redis** | As duas réplicas do Guard dividem as requisições. Um contador em memória deixaria passar o dobro, metade em cada réplica |
| **Janela fixa de 1 segundo** | Uma ida ao Redis por requisição (`INCR` e `EXPIRE` num script só, para a chave nunca ficar sem validade). O preço: na virada de um segundo para o outro, uma conta pode fazer até o dobro do limite. Uma janela deslizante tiraria esse pico, mas guardaria um registro por requisição. Para barrar o bot que martela sem parar, a janela fixa basta |
| **Chave `guard:rl:<conta>:<segundo>`, com validade de 2 s** | O Redis apaga os contadores sozinho. Não acumulam, mesmo com milhares de contas |
| **Tempo-limite de 50 ms** | Um Redis lento não pode segurar todas as compras |

### Se o Redis cair: deixa passar (fail-open)

Sem o Redis, o Guard continua conferindo o token e libera a requisição sem contar. Cada vez que isso acontece, ele registra um aviso no log e soma em `guard_rate_limit_errors_total`.

É o contrário do que acontece quando o Guard cai (fail-closed, ADR-002), e a diferença é o que cada falha põe em risco. Sem o Guard, o sistema não sabe quem é o comprador, e vender assim criaria pedidos sem dono. Sem o contador, o sistema ainda sabe quem é o comprador e só perde a proteção contra excesso; o limite de 4 assentos por comprador continua no Checkout, então um bot que passar nesse intervalo não leva mais que 4 assentos por conta. Bloquear todas as compras por causa do contador transformaria o Redis num ponto único de falha da venda. O teste de resiliência com o Redis fora (issue #57) mede esse cenário com carga.

## Resultados verificados (v0.4)

Numa cópia isolada do sistema, em 19/11/2026, com limite 10:

| Situação | Resultado |
|---|---|
| Um bot com a mesma conta, 300 requisições a 49 por segundo durante 6 s | 70 liberadas (11,4 por segundo) e 230 recusadas com 429. O que passa de 10 por segundo vem da virada entre segundos |
| 30 requisições simultâneas da mesma conta | 20 liberadas e 10 recusadas: caíram em dois segundos diferentes, o pico da janela fixa |
| Outra conta no mesmo instante | 3 de 3 liberadas; o contador de uma conta não afeta a outra |
| Redis parado | 5 de 5 liberadas, cada uma cerca de 60 ms mais lenta (o tempo-limite), e `guard_rate_limit_errors_total` em 5 |

## Resultados verificados (v0.3)

Com o Keycloak real e a rota de teste `/api/whoami`:

| Situação | Resultado |
|---|---|
| Token do `buyer1` | 200; o serviço recebe o ID do `buyer1` e `X-User-Roles: buyer` |
| Token do `buyer1` com `X-User-Id: 999` e `X-User-Roles: organizer` forjados | 200, mas o serviço recebe o ID e o papel verdadeiros |
| Token do `organizer1` | 200 com `X-User-Roles: organizer` |
| Token inventado (`Bearer abc`) | 401 (`malformed_token`) |
| Token válido do realm `master` | 401 (`unknown_key`) |
| Duas réplicas no ar | As chamadas se dividem entre as duas |
| Uma réplica derrubada | 10 de 10 requisições com 200 |
| As duas derrubadas | 500, compra bloqueada (fail-closed); o Keycloak continua respondendo |

## Testes

```bash
cd services/guard
go test ./...
```

Os testes geram um par de chaves RSA e um servidor de chaves falso, e cobrem um token válido e 14 casos de recusa: sem cabeçalho, outro esquema, `Bearer` vazio, texto que não é token, vencido, sem validade, sem usuário, outro emissor, outra audiência, chave desconhecida, payload adulterado, `alg: none`, HS256 com a chave pública como segredo, e assinatura de outra chave.

O rate limit tem dois grupos de testes. No handler, um limitador falso cobre a recusa com 429 e `Retry-After`, a passagem com o Redis fora (fail-open) e o fato de que requisições sem token válido nem chegam ao contador. O contador em si roda contra um Redis em memória ([miniredis](https://github.com/alicebob/miniredis), só nos testes): 10 liberadas e a 11ª recusada no mesmo segundo, outra conta não afetada, o contador zerando no segundo seguinte, as chaves expirando e o erro quando o Redis não responde.

## Dependências e supply chain

O Guard tem quatro dependências diretas, mais o `miniredis` usado só nos testes:

| Biblioteca | Para quê | Versão |
|---|---|---|
| [`github.com/golang-jwt/jwt/v5`](https://github.com/golang-jwt/jwt) | Conferir os tokens. Criptografia é o tipo de código que não vale escrever à mão: erros como aceitar `alg: none` são vulnerabilidades conhecidas de implementações caseiras | v5.3.1 (janeiro de 2026) |
| [`github.com/prometheus/client_golang`](https://github.com/prometheus/client_golang) | Expor `GET /metrics` | v1.25.0 (08/10/2026) |
| [`github.com/redis/go-redis/v9`](https://github.com/redis/go-redis) | Contadores do rate limit | v9.23.0 (05/10/2026) |
| [OpenTelemetry Go](https://github.com/open-telemetry/opentelemetry-go) (`otel`, `sdk`, `otlptracehttp` e `otelhttp`) | Traces | v1.47.0 e v0.72.0 (02/10/2026) |

As do Prometheus e do OpenTelemetry entraram com a observabilidade ([ADR-010](../adr/0010-OBSERVABILIDADE.md)). Elas trazem indiretamente o `golang.org/x/net` na v0.59.0, com cinco vulnerabilidades no código de HTTP/2 que o Guard alcança (GO-2026-6603, 6610, 6611, 6612 e 6617), todas corrigidas na v0.60.0. O `govulncheck` acusou na hora, e o `go.mod` fixa a v0.60.0, publicada em 08/10/2026, bem depois do período de espera.

| Cuidado | Como |
|---|---|
| Versão fixa | Todas as versões acima têm mais de duas semanas de publicação |
| Integridade | O `go.sum` guarda o hash do código da dependência; se o código baixado mudar, o build falha |
| Toolchain fixo | `toolchain go1.27.4` no `go.mod` e `golang:1.27.4-alpine` no Dockerfile |
| Vulnerabilidades conhecidas | `govulncheck` (ferramenta oficial do Go) |

O `govulncheck` encontrou 9 vulnerabilidades alcançáveis pelo código, todas na biblioteca padrão do Go 1.27.1 (servidor e cliente HTTP), e nenhuma na `golang-jwt`. Todas foram corrigidas no Go 1.27.2, e com ele o resultado foi "No vulnerabilities found". O Go 1.27.2 tinha só três dias de publicação, abaixo das duas semanas de espera adotadas para dependências. A regra de espera existe para dar tempo de a comunidade descobrir pacotes de terceiros comprometidos; uma correção de segurança publicada pela própria equipe do Go, na imagem oficial, é o caso contrário: esperar deixaria as falhas conhecidas abertas.

Depois disso, a GO-2026-6809 (`crypto/x509`, corrigida no Go 1.27.3) reprovou o CI em 05/11/2026, em um PR só de documentação. O código não tinha mudado: a vulnerabilidade foi publicada depois, e o `govulncheck` passou a acusar. O Guard foi atualizado para o Go 1.27.3.

Em 13/11/2026 aconteceu o mesmo com a GO-2026-6859 (`net/http`, corrigida no Go 1.27.4), também num PR só de documentação, e o Guard foi atualizado para o Go 1.27.4. Esse tipo de falha é esperado: o `govulncheck` consulta a base de vulnerabilidades do dia, então uma atualização da biblioteca padrão pode ser necessária sem nenhuma mudança no código.

```bash
cd services/guard
go run golang.org/x/vuln/cmd/govulncheck@v1.8.0 ./...
```

## Imagem

O Dockerfile tem duas etapas. A primeira compila o Guard numa imagem com o Go; a segunda copia só o binário para uma imagem **distroless**, que não tem shell, gerenciador de pacotes nem nenhum outro programa, e roda como usuário sem privilégios. A imagem final tem cerca de 16 MB. Como não existe `curl` na imagem, o healthcheck é o próprio binário: `/guard -healthcheck` consulta o `/health` local e sai com 0 ou 1.

## Go neste serviço

Três características da linguagem que aparecem aqui e explicam por que ela foi escolhida para o Guard:

- **Binário único e estático.** O `go build` gera um executável que não depende de nada instalado na máquina. Por isso a imagem final tem só esse arquivo e sobe em milissegundos.
- **Concorrência embutida.** O servidor HTTP da biblioteca padrão atende cada requisição numa *goroutine*, uma tarefa leve gerenciada pelo próprio Go. Milhares de requisições simultâneas não exigem configuração nem framework. As chaves públicas, que todas as goroutines leem, ficam protegidas por um *mutex*, para que duas requisições não tentem atualizá-las ao mesmo tempo.
- **Erros como valor.** Em vez de exceções, as funções devolvem o erro junto com o resultado, e quem chama decide o que fazer na hora. É o que permite ao Guard transformar cada tipo de falha num motivo específico no log.
