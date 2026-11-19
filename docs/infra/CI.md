# CI

O GitHub Actions roda o pipeline em todo pull request para `develop` ou `main` e em todo push nessas branches. Um PR só deve ser mergeado com tudo verde ([CONTRIBUTING.md](../../CONTRIBUTING.md#pull-requests)). Definição em [`.github/workflows/ci.yml`](../../.github/workflows/ci.yml).

## Jobs

| Job | O que verifica | Falha quando |
|---|---|---|
| **Changes** | Quais pastas de `services/` o PR mexeu ([`scripts/ci-changed-services.sh`](../../scripts/ci-changed-services.sh)) | Nunca falha; só decide quais jobs de serviço rodam |
| **Guard (Go) / Check** | O modelo de Go, descrito abaixo, na pasta `services/guard` | Ver o modelo de Go |
| **Compose** | `docker compose config` com o `.env.example` | O `compose.yaml` tem erro de sintaxe ou referencia algo que não existe |
| **Secrets** | `gitleaks` no histórico inteiro do repositório | Alguma senha, token ou chave aparece em qualquer commit |
| **Docs** | Links entre arquivos Markdown ([`scripts/check-doc-links.sh`](../../scripts/check-doc-links.sh)) | Um documento aponta para um arquivo que não existe, por exemplo depois de renomear |

## Um modelo por linguagem

Os passos de cada linguagem ficam num workflow reutilizável, e o `ci.yml` só chama o modelo com a pasta do serviço. Assim os cinco serviços passam pelas mesmas verificações, e uma mudança (uma ferramenta nova, uma versão de action) é feita num lugar só.

| Modelo | Passos | Falha quando |
|---|---|---|
| [`go-service.yml`](../../.github/workflows/go-service.yml) | `gofmt`, `go vet`, `go test -race`, `govulncheck` e build da imagem | Código fora do padrão, teste quebrado, acesso concorrente sem proteção, vulnerabilidade alcançável pelo código, ou imagem que não compila |
| [`ts-service.yml`](../../.github/workflows/ts-service.yml) | `npm ci`, `npm run lint`, `npm run build` (os dois se existirem no `package.json`), `npm test`, `npm audit --audit-level=high` e build da imagem | Lockfile desatualizado, lint ou build com erro, teste quebrado, dependência com vulnerabilidade alta ou crítica, ou imagem que não compila |

A versão da linguagem vem do próprio serviço: a linha `toolchain` do `go.mod` no Go e o campo `engines.node` do `package.json` no TypeScript. Quando o Go precisar ser atualizado por uma vulnerabilidade (como em [GUARD.md](GUARD.md#dependências-e-supply-chain)), cada serviço sobe a sua versão no próprio PR.

## Só roda o que mudou

O job **Changes** compara o PR com a `develop` e lista os serviços cujas pastas mudaram. Os jobs dos outros serviços aparecem como pulados (*skipped*), o que deixa o CI rápido quando cinco pessoas abrem PRs ao mesmo tempo. Três casos rodam tudo:

- push na `develop` ou na `main` (depois de um merge, tudo é verificado de novo);
- PR que muda algum arquivo em `.github/workflows/`;
- PR que muda o próprio script de detecção.

Os jobs que não são de serviço (Compose, Secrets, Docs) rodam sempre.

## Como incluir um serviço

Um bloco no `ci.yml`, trocando o nome e a pasta (o espaço antes e depois do nome dentro do `contains` é necessário, para `stats` não casar com outro nome que contenha `stats`):

```yaml
  inventory:
    name: Inventory (TypeScript)
    needs: changes
    if: contains(needs.changes.outputs.services, ' inventory ')
    uses: ./.github/workflows/ts-service.yml
    with:
      path: services/inventory
      image: inventory
```

Para Go, o mesmo bloco com `go-service.yml`. O `image` é opcional no modelo de Go: a pasta [`bots/`](../../bots/README.md), que roda fora do Compose, usa o modelo sem ele e não tem imagem. O job **Changes** também acompanha a pasta `bots/`. E duas entradas no [`.github/dependabot.yml`](../../.github/dependabot.yml), copiando as do Guard: uma para as dependências (`npm` ou `gomod`) e outra para o Dockerfile (`docker`). O Dependabot recusa pastas que ainda não existem, então essas entradas entram no mesmo PR do esqueleto.

## Supply chain no próprio pipeline

O pipeline também roda código de terceiros, então segue os mesmos cuidados do código da aplicação:

- **Actions fixadas por hash de commit**, e não por tag (`actions/checkout@3d3c42e...  # v7.0.1`). Uma tag pode ser movida para apontar para outro código; um hash não. Esse foi o vetor de ataques reais contra actions populares, em que a tag de uma versão existente passou a apontar para código malicioso.
- **Ferramentas com versão fixa:** `govulncheck@v1.8.0` e `gitleaks` 8.30.1.
- **Download conferido:** o gitleaks é baixado da página de releases e o hash do arquivo é comparado com o arquivo de checksums publicado. Se não bater, o job falha antes de executar o binário.
- **Permissão mínima:** o workflow só tem permissão de leitura do repositório (`permissions: contents: read`).

## Dependabot

O [`.github/dependabot.yml`](../../.github/dependabot.yml) abre PRs de atualização toda semana para:

| O quê | Onde |
|---|---|
| Dependências Go | `services/guard/go.mod` (e o `go.mod` de cada serviço Go novo) |
| Dependências npm | `package.json` de cada serviço TypeScript |
| Imagens base do Dockerfile | `services/guard/Dockerfile` (e o de cada serviço novo) |
| GitHub Actions | `.github/workflows/` |

Com `cooldown` de 14 dias: uma versão nova só é proposta depois de duas semanas publicada, o mesmo período de espera usado nas escolhas manuais. Atualizações de segurança não esperam esse prazo, pela mesma razão da exceção registrada no [GUARD.md](GUARD.md#dependências-e-supply-chain). Os PRs do Dependabot passam pelo mesmo CI.

## Rodar localmente

Go (dentro da pasta do serviço):

```bash
cd services/guard
test -z "$(gofmt -l .)" && go vet ./... && go test -race ./...
go run golang.org/x/vuln/cmd/govulncheck@v1.8.0 ./...

```

TypeScript (dentro da pasta do serviço):

```bash
npm ci && npm run lint --if-present && npm run build --if-present && npm test
npm audit --audit-level=high
```

Repositório:

```bash
gitleaks git --redact .
./scripts/check-doc-links.sh
```
