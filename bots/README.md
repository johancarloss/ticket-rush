# Bots de carga

Ferramentas em Go que fazem o papel do comprador e do cambista do C4 nos testes de carga. Rodam fora do Compose, de preferência de outra máquina, para a CPU dos bots não disputar com o sistema medido. O plano dos testes fica em `docs/TESTES.md` (issue #52).

| Ferramenta | O que faz | Issue |
|---|---|---|
| `cmd/testusers` | Cria as contas de carga no Keycloak, gera os tokens delas e apaga as contas depois | #50 |
| `cmd/bots` | Fãs e cambistas comprando | #51 |

Sem dependências externas: só a biblioteca padrão do Go.

## Contas de carga (`testusers`)

```bash
cd bots
set -a; . ../.env; set +a                      # KEYCLOAK_ADMIN_USER, KEYCLOAK_ADMIN_PASSWORD, LOAD_USERS_PASSWORD

go run ./cmd/testusers create                  # 2.000 fãs e 5 cambistas
go run ./cmd/testusers tokens -out tokens.json # login de todas as contas
go run ./cmd/testusers delete                  # remove todas as contas de carga
```

| Comando | Flags | Observações |
|---|---|---|
| `create` | `-buyers 2000 -scalpers 5 -batch 500` | Importa em lotes pela importação parcial do Keycloak, muito mais rápida que uma chamada por usuário. Contas que já existem são puladas, então rodar de novo é seguro |
| `tokens` | `-buyers 2000 -scalpers 5 -out tokens.json -concurrency 20` | Faz o login de cada conta pela entrada pública, o mesmo caminho do navegador de um comprador. Grava o arquivo com permissão `600`. O `.gitignore` bloqueia `tokens*.json` |
| `delete` | nenhuma | Remove só as contas com prefixo `loadbuyer` e `loadscalper`; `buyer1`, `buyer2` e `organizer1` ficam |

As contas se chamam `loadbuyer0001` a `loadbuyer2000` e `loadscalper01` a `loadscalper05`, com e-mail em `load.ticketrush.test` (um domínio reservado para testes, que nunca recebe e-mail de verdade). Todas usam a senha `LOAD_USERS_PASSWORD` e têm o papel `buyer`: para o sistema, o cambista é um comprador como os outros, e quem o segura são o limite por conta do Guard e o limite de 4 assentos do Checkout.

| Variável | Padrão | Para quê |
|---|---|---|
| `KEYCLOAK_ADMIN_URL` | `http://127.0.0.1:8185/auth` | API de administração, só pela própria máquina ([KEYCLOAK.md](../docs/infra/KEYCLOAK.md#console-de-administração-fechado-na-entrada-pública)) |
| `KEYCLOAK_URL` | `http://localhost:8180/auth` | Login das contas, pela entrada pública |

### Os tokens duram 5 minutos

O realm emite tokens de acesso de 5 minutos, e a configuração continua assim: aumentar valeria para todos os usuários, e um token roubado ficaria útil por mais tempo. O arquivo guarda também o *refresh token* de cada conta, e os bots renovam o acesso quando faltar pouco para vencer, como um site faz com o usuário logado.

```json
[
 {
  "username": "loadbuyer0001",
  "access_token": "eyJ...",
  "refresh_token": "eyJ...",
  "expires_at": "2026-11-19T23:44:04Z"
 }
]
```

### Resultados na VPS (19/11/2026)

| Operação | Tempo |
|---|---|
| Criar 2.005 contas (5 lotes) | 2 min 8 s, quase todo gasto pelo Keycloak calculando o hash de cada senha |
| Rodar `create` de novo | 2 s, com as 2.005 puladas |
| Login das 2.005 contas, 20 em paralelo | 1 min 28 s |
| Apagar as 2.005 contas | 43 s |

Tokens sorteados, incluindo o de um cambista, passaram pelo Guard com `X-User-Roles: buyer`.

### Dois problemas encontrados no primeiro uso

- **O token do administrador dura 60 segundos.** A primeira versão pegava um token no começo e usava em todos os lotes; no terceiro lote ele tinha vencido e o Keycloak respondeu 401. Agora o cliente renova o token antes de cada chamada quando faltam menos de 15 segundos.
- **A importação parcial não aplica os papéis padrão do realm.** As contas saíram sem papel nenhum, ao contrário de um usuário que se cadastra. Agora cada conta declara o papel `buyer`, como os usuários do arquivo do realm.

## Testes

```bash
cd bots
go test ./...
```

Os testes do cliente do Keycloak usam um servidor falso (`httptest`): login com a data de expiração, erro do Keycloak repassado com o código e a mensagem, importação com `SKIP`, remoção só das contas com o prefixo certo, e a renovação do token de administrador. O CI roda os mesmos passos dos serviços em Go, sem a imagem Docker ([CI.md](../docs/infra/CI.md)).
