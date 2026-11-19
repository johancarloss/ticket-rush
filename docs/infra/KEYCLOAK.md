# Keycloak

Servidor de identidade do sistema: login, cadastro, papéis e emissão dos tokens que o Guard confere. O porquê da escolha está no [ADR-007](../adr/0007-KEYCLOAK.md).

## Configuração como código

O realm não é configurado clicando no painel. Ele está descrito em [`infra/keycloak/ticket-rush-realm.json`](../../infra/keycloak/ticket-rush-realm.json), que o Keycloak importa na subida (`start-dev --import-realm`). Assim a configuração fica versionada no Git e é igual na máquina de todos.

A importação usa a estratégia "ignorar se já existe": se o realm `ticket-rush` já estiver no banco, o arquivo não é aplicado de novo. Para aplicar uma mudança no arquivo, é preciso recriar os dados ([COMO-EXECUTAR.md](COMO-EXECUTAR.md#recomeçar-do-zero)).

| Item | Valor | Por quê |
|---|---|---|
| Realm | `ticket-rush` | Separado do `master`, que é só para administrar o Keycloak |
| Papéis | `buyer`, `organizer` | Comprador e organizador (ADR-007). Todo usuário novo recebe `buyer` |
| Client | `ticket-rush-web` (público, com PKCE) | O aplicativo pelo qual o comprador faz login |
| Audiência | `ticket-rush-api` | Todo token do client sai com `aud: ticket-rush-api`; o Guard recusa tokens sem ela |
| Validade do token | 5 minutos | Um token roubado vale pouco tempo; o refresh token renova |
| Proteção contra força bruta | ligada | Bloqueia temporariamente a conta depois de várias senhas erradas |

## Usuários de teste

| Usuário | Papel |
|---|---|
| `buyer1` | `buyer` |
| `buyer2` | `buyer` |
| `organizer1` | `organizer` |

A senha dos três vem da variável `KC_TEST_USERS_PASSWORD` do `.env`. O arquivo do realm só tem o marcador `${KC_TEST_USERS_PASSWORD}`, que o Keycloak substitui na importação, então nenhuma senha fica no repositório. Esses usuários existem para desenvolvimento e testes; uma instalação pública não deve importá-los.

## Obter um token

O client aceita o fluxo de senha direta, usado pelos testes e pelos bots de carga:

```bash
set -a; . ./.env; set +a
TOKEN=$(curl -s \
  -d grant_type=password -d client_id=ticket-rush-web \
  -d username=buyer1 -d "password=$KC_TEST_USERS_PASSWORD" \
  http://localhost:8180/auth/realms/ticket-rush/protocol/openid-connect/token \
  | python3 -c "import sys, json; print(json.load(sys.stdin)['access_token'])")

curl -H "Authorization: Bearer $TOKEN" http://localhost:8180/api/whoami
```

O fluxo de senha direta foi removido no OAuth 2.1 porque o aplicativo vê a senha do usuário. Aqui ele fica ligado só para automatizar testes; um front-end usaria o fluxo de autorização com PKCE, que o client também aceita.

## O que tem dentro de um token

```json
{
  "iss": "http://localhost:8180/auth/realms/ticket-rush",
  "aud": "ticket-rush-api",
  "azp": "ticket-rush-web",
  "sub": "7e582f2d-a79f-41eb-bfe6-420d5ab589b2",
  "preferred_username": "buyer1",
  "realm_access": { "roles": ["buyer"] },
  "exp": 1792000000
}
```

| Campo | Uso no sistema |
|---|---|
| `iss` | Quem emitiu. O Guard só aceita este realm |
| `aud` | Para quem o token foi emitido. O Guard só aceita `ticket-rush-api` |
| `sub` | ID do usuário. Vira o `X-User-Id` |
| `realm_access.roles` | Papéis. Vira o `X-User-Roles` |
| `exp` | Validade. Token vencido é recusado |

## Endereços

| Endereço | O quê |
|---|---|
| `http://127.0.0.1:8185/auth/admin/` | Console de administração (usuário e senha `KEYCLOAK_ADMIN_*` do `.env`). Só abre da própria máquina; de fora, use um túnel SSH (`ssh -L 8185:127.0.0.1:8185 <vps>`) |
| `http://localhost:8180/auth/realms/ticket-rush/.well-known/openid-configuration` | Descrição pública do realm (endereços de login, token e chaves) |
| `http://keycloak:8080/auth/realms/ticket-rush/protocol/openid-connect/certs` | Chaves públicas (JWKS), pela rede interna. É de onde o Guard as busca |

### Console de administração fechado na entrada pública

Pela entrada pública (porta 8180), `/auth/admin` e `/auth/realms/master` respondem **403**. O `master` também fica fechado porque é o realm onde o administrador faz login; sem ele, nem a tela de login do administrador aparece para quem está de fora. O realm `ticket-rush` continua aberto, porque o login dos compradores, os tokens e as chaves públicas precisam ser alcançáveis.

O console inteiro continua acessível pela porta `8185`, publicada só em `127.0.0.1`, como o painel do Traefik. O Keycloak monta os links do console com esse endereço (`KC_HOSTNAME_ADMIN`). Os scripts que usam a API de administração (por exemplo, a criação de contas de teste) também usam essa porta.

Verificado em 19/11/2026:

| Pela entrada pública (8180) | Resultado |
|---|---|
| `/auth/admin/`, `/auth/admin/master/console/` | 403 |
| `/auth/realms/master/...` | 403 |
| API de administração com um token válido de administrador | 403 |
| Variações do caminho: `//admin`, `./admin`, `%61dmin`, `admin%2F`, `ticket-rush/../master`, `%2e%2e/master`, `%6Daster` | 403 (o Traefik normaliza o caminho antes de aplicar a regra) |
| `/auth/ADMIN/`, `/auth/realms/MASTER/...` | 404 do próprio Keycloak, que diferencia maiúsculas de minúsculas |
| Login e chaves públicas do realm `ticket-rush` | 200 |
| Token de comprador passando pelo Guard | 200 |

| Pela porta local (8185) | Resultado |
|---|---|
| Console e login do administrador | 200 |
| API de administração | Responde (contagem de usuários do realm) |

O emissor gravado nos tokens é sempre o endereço público (`localhost:8180`), definido por `KC_HOSTNAME`, mesmo quando o token é pedido pela rede interna.
