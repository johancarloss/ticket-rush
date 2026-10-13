# Traefik

O Traefik é a única entrada do sistema: recebe as requisições de fora e encaminha para o serviço certo. O que ele é e por que foi escolhido está no [ADR-002](../adr/0002-TRAEFIK-GUARD.md).

## Como ele descobre as rotas

O Traefik lê os containers do Docker pelo socket (`/var/run/docker.sock`, montado só para leitura). Um container só recebe tráfego se declarar, nas labels do `compose.yaml`, que quer ser exposto e em qual caminho:

```yaml
labels:
  - traefik.enable=true
  - traefik.http.routers.keycloak.rule=PathPrefix(`/auth`)
  - traefik.http.routers.keycloak.entrypoints=web
  - traefik.http.services.keycloak.loadbalancer.server.port=8080
```

| Label | Significado |
|---|---|
| `traefik.enable=true` | Este container pode receber tráfego. Sem ela, o Traefik ignora o container (`exposedbydefault=false`) |
| `routers.<nome>.rule` | Quais requisições vão para ele, aqui as que começam com `/auth` |
| `routers.<nome>.entrypoints` | Por qual porta de entrada (`web`, a porta 80 do container) |
| `services.<nome>.loadbalancer.server.port` | Em que porta o container escuta |

Um container novo com essas labels entra na rota assim que sobe, sem reiniciar o Traefik.

## Rotas atuais

| Caminho | Destino | Middleware |
|---|---|---|
| `/auth` | Keycloak | nenhum (login e chaves públicas precisam ser acessíveis) |
| `/api/whoami` | `traefik/whoami`, rota temporária de teste | `buyer-auth` |

As rotas dos serviços (`/api/...`) entram com cada serviço. A rota `/api/whoami` existe só para testar o Guard de ponta a ponta: o container devolve a requisição que recebeu, com os cabeçalhos, e sai quando os serviços reais existirem.

## Middlewares

Além das labels dos containers, o Traefik lê um arquivo fixo, [`infra/traefik/dynamic.yml`](../../infra/traefik/dynamic.yml) (opção `--providers.file.filename`). Nele ficam os middlewares que não pertencem a um container específico:

| Middleware | O que faz |
|---|---|
| `strip-identity` | Apaga o cabeçalho `X-User-Id` que vier do cliente |
| `guard-auth` | `forwardAuth`: pergunta ao Guard se a requisição pode passar e repassa o `X-User-Id` que ele devolver |
| `buyer-auth` | Corrente com os dois acima, nessa ordem. É o que as rotas de compra usam |

Detalhes do fluxo em [GUARD.md](GUARD.md).

## Redes

O Traefik está só na rede `edge`. Bancos e Redis estão só na rede `data`. Assim, mesmo com uma rota configurada errada, o Traefik não consegue alcançar o Postgres nem o Redis. O Keycloak está nas duas, porque recebe tráfego do Traefik e usa o Postgres.

A opção `--providers.docker.network=ticket-rush_edge` diz ao Traefik por qual rede falar com containers que estão em mais de uma.

## Painel

`http://127.0.0.1:8181/dashboard/` mostra as rotas, os serviços e os middlewares ativos. A porta é publicada só no `127.0.0.1`, porque o painel não tem autenticação (`--api.insecure=true`).

## Problema encontrado: versão da API do Docker

Na primeira versão do Compose, com `traefik:v3.3.4`, nenhuma rota aparecia e o log repetia:

```
Error response from daemon: client version 1.24 is too old.
Minimum supported API version is 1.40
```

O Traefik antigo conversa com o Docker usando uma versão da API que o Docker 29 não aceita mais, então ele nunca recebia a lista de containers. A correção foi fixar a imagem em `traefik:v3.7.13`. A versão escolhida tinha mais de duas semanas de publicação, para não pegar uma versão recém-lançada que ainda não teve tempo de ser avaliada pela comunidade.
