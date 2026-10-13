# Como criar um serviço

Passo a passo para colocar um serviço novo no Ticket Rush, do primeiro commit até ele aparecer no Compose, no Traefik e no CI. O [Guard](../../services/guard/) é o exemplo completo que já segue tudo isto.

O primeiro PR de cada serviço é só o **esqueleto**: o serviço sobe, responde `/health`, entra no Compose e passa no CI. A lógica de negócio vem nos PRs seguintes.

## Antes do código: o contrato

Escreva primeiro o contrato HTTP do serviço em `docs/contracts/<SERVICO>.yaml` (OpenAPI 3): as rotas, o que cada uma recebe, o que devolve e os erros possíveis. Os outros integrantes programam contra esse arquivo, então ele pode ser revisado e mergeado antes de existir qualquer implementação ([ADR-003](../adr/0003-STACK.md)). Para eventos, o formato fica em [EVENTOS.md](../contracts/EVENTOS.md).

## Convenções

| Item | Convenção |
|---|---|
| Pasta | `services/<nome>/`, com o nome em inglês: `inventory`, `checkout`, `payment`, `notification`, `stats` |
| Porta | O processo escuta na porta `8080` dentro do container, lida da variável `PORT`. Nenhuma porta é publicada na máquina; o acesso de fora é só pelo Traefik |
| Health | `GET /health` responde `200 {"status":"ok"}` |
| Logs | Uma linha JSON por evento, na saída padrão, com o campo `msg` |
| Desligamento | Ao receber `SIGTERM`, termina as requisições em andamento antes de sair, para um deploy não cortar compras pela metade |
| Rotas públicas | Prefixo `/api/` mais o recurso: `/api/events`, `/api/orders` |
| Nomes | Código, tabelas, status e eventos em inglês ([CONTRIBUTING.md](../../CONTRIBUTING.md)) |

## Passo a passo

### 1. Branch

```bash
git checkout develop && git pull
git checkout -b feature/<nome>/skeleton
```

### 2. Código mínimo

Um servidor HTTP com `GET /health`, lendo `PORT`, com logs JSON e desligamento no `SIGTERM`. O framework é escolha do responsável.

**Dependências com versão exata.** Em TypeScript, sem `^` nem `~` no `package.json`, com o `package-lock.json` commitado, e instalando com `npm ci`. Em Go, o `go.sum` commitado. Antes de adotar uma versão, confira se ela tem pelo menos duas semanas de publicação ([GUARD.md](../infra/GUARD.md#dependências-e-supply-chain)).

### 3. Dockerfile

Duas etapas: uma para compilar, outra só com o necessário para rodar, como usuário sem privilégios.

TypeScript (Node):

```dockerfile
FROM node:<versão LTS exata>-alpine AS build
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
RUN npm run build && npm prune --omit=dev

FROM node:<mesma versão>-alpine
WORKDIR /app
ENV NODE_ENV=production
COPY --from=build /app/node_modules ./node_modules
COPY --from=build /app/dist ./dist
USER node
EXPOSE 8080
CMD ["node", "dist/main.js"]
```

Go: o mesmo modelo do [Dockerfile do Guard](../../services/guard/Dockerfile), com imagem final distroless e o próprio binário fazendo o healthcheck.

Inclua um `.dockerignore` com `node_modules`, testes e o próprio Dockerfile.

### 4. Compose

Acrescente o serviço ao `compose.yaml`:

```yaml
  inventory:
    build: ./services/inventory
    image: ticket-rush/inventory:dev
    environment:
      DATABASE_URL: postgres://inventory_svc:${INVENTORY_DB_PASSWORD}@postgres:5432/ticketrush
      REDIS_URL: redis://redis:6379
    depends_on:
      postgres:
        condition: service_healthy
    networks:
      - edge          # recebe tráfego do Traefik e fala com os outros serviços
      - data          # só se usa Postgres ou Redis
    healthcheck:
      # Imagens alpine têm o wget do busybox; imagens distroless usam o próprio binário
      test: ["CMD", "wget", "-qO-", "http://127.0.0.1:8080/health"]
      interval: 10s
      timeout: 3s
      retries: 3
    labels:
      # Só para serviços que recebem tráfego de fora (ver passo 5)
      - traefik.enable=true
      - traefik.http.services.inventory.loadbalancer.server.port=8080
```

**Banco de dados.** Cada serviço com banco já tem schema, usuário e senha criados ([POSTGRES.md](../infra/POSTGRES.md)): use o usuário do seu serviço e nunca o administrador. As tabelas são criadas pelo próprio serviço, no schema dele; como o `search_path` do usuário já aponta para o schema certo, `CREATE TABLE seats` vai para `inventory.seats`.

**Variáveis novas** entram no `.env.example` com o valor `change-me` e uma linha de comentário.

### 5. Rotas no Traefik

Só Inventário e Checkout recebem tráfego de fora. Pagamento, Notificação e Estatísticas **não** levam `traefik.enable=true`: eles só são alcançados pela rede interna (o Checkout chama `http://payment:8080`).

Para cada grupo de rotas, um router. Rotas de **consulta pública** não passam pelo Guard; rotas de **compra** usam o middleware `buyer-auth@file`:

```yaml
      # Consulta pública: eventos e mapa de assentos (ADR-002)
      - traefik.http.routers.inventory-public.rule=PathPrefix(`/api/events`) && Method(`GET`)
      - traefik.http.routers.inventory-public.entrypoints=web
      - traefik.http.routers.inventory-public.service=inventory

      # Rotas que exigem usuário logado
      - traefik.http.routers.inventory-auth.rule=PathPrefix(`/api/events`) && !Method(`GET`)
      - traefik.http.routers.inventory-auth.entrypoints=web
      - traefik.http.routers.inventory-auth.middlewares=buyer-auth@file
      - traefik.http.routers.inventory-auth.service=inventory
```

### 6. Quem é o usuário

Nas rotas com `buyer-auth`, o serviço recebe dois cabeçalhos já conferidos pelo Guard:

| Cabeçalho | Conteúdo |
|---|---|
| `X-User-Id` | ID do usuário no Keycloak |
| `X-User-Roles` | Papéis separados por vírgula, por exemplo `buyer` ou `organizer` |

Regras:
- **Não valide o token no serviço.** O Guard já validou; o serviço usa só os cabeçalhos ([GUARD.md](../infra/GUARD.md)).
- **Confira o papel** quando a rota exige um específico. Cadastrar evento exige `organizer`; o Guard só garante que o usuário está logado.
- **Nunca publique a porta do serviço** na máquina. Os cabeçalhos só são confiáveis porque todo tráfego de fora passa pelo Traefik, que apaga os valores mandados pelo cliente.

Para testar, obtenha um token como em [KEYCLOAK.md](../infra/KEYCLOAK.md#obter-um-token).

### 7. CI

Acrescente um job ao [`.github/workflows/ci.yml`](../../.github/workflows/ci.yml), com as actions fixadas pelos mesmos hashes já usados no arquivo.

TypeScript:

```yaml
  inventory:
    name: Inventory (TypeScript)
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: services/inventory
    steps:
      - uses: actions/checkout@<hash>  # mesmo hash dos outros jobs
      - uses: actions/setup-node@<hash> # fixar por hash, como o checkout
        with:
          node-version-file: services/inventory/package.json
      - run: npm ci
      - run: npm run lint
      - run: npm test
      - run: npm audit --audit-level=high
      - run: docker build -t ticket-rush/inventory:ci .
```

Go: copie o job do Guard trocando a pasta.

E uma entrada no [`.github/dependabot.yml`](../../.github/dependabot.yml) para as dependências (`npm` ou `gomod`) e para o Dockerfile (`docker`) do serviço, com o mesmo `cooldown`.

### 8. Documentação

- `docs/services/<SERVICO>.md`: o que o serviço faz, rotas, variáveis de ambiente, tabelas e como testar. O [GUARD.md](../infra/GUARD.md) serve de modelo.
- Uma linha no [COMO-EXECUTAR.md](../infra/COMO-EXECUTAR.md#o-que-sobe-hoje), na tabela do que sobe.
- Se o serviço tomou uma decisão de arquitetura nova, um ADR ([CONTRIBUTING.md](../../CONTRIBUTING.md#decisões-de-arquitetura)).
- Se usou IA, a linha na seção "Ferramentas de IA utilizadas" do README.

### 9. Pull request

PR para a `develop` ([CONTRIBUTING.md](../../CONTRIBUTING.md#pull-requests)), com o CI verde e a aprovação de outro integrante.

## Checklist do esqueleto

- [ ] Contrato em `docs/contracts/` (pode ser um PR separado, antes)
- [ ] `services/<nome>/` com `/health`, `PORT`, logs JSON e desligamento no `SIGTERM`
- [ ] Dependências com versão exata e lockfile commitado
- [ ] Dockerfile em duas etapas, usuário sem privilégios, `.dockerignore`
- [ ] Bloco no `compose.yaml` com healthcheck, redes certas e sem porta publicada
- [ ] Labels do Traefik só se o serviço recebe tráfego de fora; rotas de compra com `buyer-auth@file`
- [ ] Variáveis novas no `.env.example`
- [ ] Job no CI e entrada no Dependabot
- [ ] `docs/services/<SERVICO>.md` e linha no COMO-EXECUTAR
- [ ] `docker compose up -d --build` com o serviço `(healthy)`
