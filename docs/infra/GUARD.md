# Guard

O Guard é o serviço que decide se uma requisição de compra pode entrar no sistema. Quem pergunta é o Traefik, antes de encaminhar a requisição ao serviço de destino; o comprador nunca fala com o Guard diretamente. O porquê dessa peça está no [ADR-002](../adr/0002-TRAEFIK-GUARD.md).

Código em [`services/guard/`](../../services/guard/), escrito em Go.

## Como uma requisição passa por ele

```
Comprador ──► Traefik ── rota com o middleware buyer-auth
                 │
                 │ 1. strip-identity: apaga qualquer X-User-Id que o comprador tenha mandado
                 │ 2. guard-auth: pergunta ao Guard (GET http://guard:8080/verify, com os cabeçalhos originais)
                 ▼
               Guard
                 │  token presente?   não → 401, o Traefik devolve ao comprador e o serviço nem fica sabendo
                 │                    sim → 200 (+ X-User-Id, a partir da próxima versão)
                 ▼
              Traefik encaminha ao serviço, levando o X-User-Id definido pelo Guard
```

Os middlewares ficam em [`infra/traefik/dynamic.yml`](../../infra/traefik/dynamic.yml). Uma rota passa a exigir comprador logado só por declarar `middlewares=buyer-auth@file` nas labels.

### Por que apagar o X-User-Id antes

Os serviços vão confiar no `X-User-Id` para saber quem é o comprador. Se um comprador mandasse `X-User-Id: 999` por conta própria e o Guard não sobrescrevesse esse cabeçalho, o valor falso chegaria ao serviço. O middleware `strip-identity` apaga o cabeçalho que veio de fora, então o único `X-User-Id` que pode chegar ao serviço é o que o Guard devolveu.

## O que ele verifica hoje

| Versão | Regra |
|---|---|
| **v0.3 (atual)** | Exige o cabeçalho `Authorization: Bearer <token>`. Não confere a assinatura ainda: qualquer texto passa |
| próxima | Confere a assinatura do token com as chaves públicas do Keycloak (JWKS) e a validade, e devolve o `X-User-Id` |
| depois | Limite de 10 requisições de compra por segundo por conta, com contadores no Redis |

## Rotas

| Rota | Quem chama | Resposta |
|---|---|---|
| `GET /health` | Healthcheck do Docker | `200 {"status":"ok"}` |
| `/verify` | Traefik (forwardAuth) | `200` deixa passar; `401 {"error": ...}` barra |

Cada decisão é registrada em log JSON, com o motivo, o método e o caminho da requisição original:

```json
{"level":"INFO","msg":"request denied","reason":"missing_token","method":"GET","uri":"/api/whoami"}
```

## Duas réplicas e fail-closed

O Compose sobe duas cópias do Guard (`deploy.replicas: 2`). O Traefik chama `http://guard:8080`, e o DNS interno do Docker distribui as chamadas entre as duas.

Resultado medido na v0.3, com a rota de teste `/api/whoami`:

| Situação | Compra (`/api/whoami`) | Keycloak (`/auth`) |
|---|---|---|
| Duas réplicas no ar | 200; as chamadas se dividiram entre as duas (16 e 13) | 200 |
| Uma réplica derrubada | 10 de 10 requisições com 200 | 200 |
| As duas derrubadas | **500**, compra bloqueada (fail-closed) | 200, não passa pelo Guard |
| As duas de volta | 200 | 200 |

## Imagem

O Dockerfile tem duas etapas. A primeira compila o Guard numa imagem com o Go; a segunda copia só o binário para uma imagem **distroless**, que não tem shell, gerenciador de pacotes nem nenhum outro programa, e roda como usuário sem privilégios. Como não existe `curl` na imagem, o healthcheck é o próprio binário: `/guard -healthcheck` consulta o `/health` local e sai com 0 ou 1.

## Testes

```bash
cd services/guard
go test ./...
```

Os testes cobrem o `/health` e o `/verify` sem cabeçalho, com outro esquema (`Basic`), com `Bearer` vazio e com um token presente.

## Go neste serviço

Três características da linguagem que aparecem aqui e explicam por que ela foi escolhida para o Guard:

- **Binário único e estático.** O `go build` gera um executável que não depende de nada instalado na máquina. Por isso a imagem final tem só esse arquivo e fica com poucos megabytes, e sobe em milissegundos.
- **Concorrência embutida.** O servidor HTTP da biblioteca padrão atende cada requisição numa *goroutine*, uma tarefa leve gerenciada pelo próprio Go. Milhares de requisições simultâneas não exigem configuração nem framework.
- **Erros como valor.** Em vez de exceções, as funções devolvem o erro junto com o resultado, e quem chama decide o que fazer na hora. Isso deixa explícito, em cada ponto, o que acontece quando algo falha.

O serviço usa só a biblioteca padrão do Go, sem dependências externas.
