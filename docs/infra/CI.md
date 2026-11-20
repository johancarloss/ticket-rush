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
| [`go-service.yml`](../../.github/workflows/go-service.yml) | `gofmt`, `go vet`, `go test -race`, `govulncheck`, build da imagem, varredura da imagem e SBOM ([DevSecOps](#devsecops)) | Código fora do padrão, teste quebrado, acesso concorrente sem proteção, vulnerabilidade alcançável pelo código, ou imagem que não compila |
| [`ts-service.yml`](../../.github/workflows/ts-service.yml) | `npm ci`, `npm run lint`, `npm run build` (os dois se existirem no `package.json`), `npm test`, `npm audit --audit-level=high`, build da imagem, varredura da imagem e SBOM | Lockfile desatualizado, lint ou build com erro, teste quebrado, dependência com vulnerabilidade alta ou crítica, ou imagem que não compila |

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

## DevSecOps

Segurança como parte do pipeline, em quatro camadas, cada uma olhando uma coisa diferente:

| Camada | Ferramenta | O que olha | Quando reprova | Onde |
|---|---|---|---|---|
| Código | [CodeQL](https://codeql.github.com/), consultas `security-extended` | Padrões de vulnerabilidade no código Go (injeção, SSRF, uso inseguro de criptografia) e nos próprios workflows (injeção pelo título de um PR, permissões demais, actions sem hash) | Alerta novo aparece no PR e na aba **Security** do GitHub | [`codeql.yml`](../../.github/workflows/codeql.yml), em todo PR, push e toda segunda-feira |
| Dependências alcançáveis | `govulncheck` (Go), `npm audit` (TypeScript) | Vulnerabilidade conhecida numa função que o código **chama** | Sempre | Modelos por linguagem |
| Imagem | [Trivy](https://trivy.dev/) | Tudo que está **dentro** da imagem: o binário com as bibliotecas compiladas e os pacotes do sistema da imagem base | Vulnerabilidade alta ou crítica **com correção disponível** | Modelos por linguagem, depois do build |
| Inventário | Trivy, formato CycloneDX | SBOM (*software bill of materials*): a lista de cada componente da imagem e a versão | Não reprova; o arquivo fica como artefato da execução (`sbom-<serviço>`) | Modelos por linguagem |

Mais o que já existia: `gitleaks` no histórico inteiro e Dependabot.

Na primeira execução, o CodeQL achou um problema no próprio CI: os modelos colavam `${{ inputs.image }}` direto dentro dos scripts. Hoje o valor vem do `ci.yml` e não de quem abre o PR, mas é o padrão que vira injeção de código quando o valor passa a vir de fora (um título de PR, um nome de branch). Os modelos passaram a receber o valor por variável de ambiente (`IMAGE`, `SBOM`), e o script só lê `"$IMAGE"`.

### Por que `govulncheck` e Trivy juntos

Os dois acham coisas diferentes, e a primeira execução mostrou isso no Guard. A imagem tinha o `golang.org/x/net` v0.60.0, com a CVE-2026-78661 (alta, no pacote `html`, corrigida na v0.62.0). O `govulncheck` não acusou, porque o Guard nunca chama o pacote `html`: ele só reprova o que o código alcança. O Trivy acusou, porque olha o que está compilado no binário, e não o que é chamado. A dependência foi atualizada para a v0.62.0.

O `govulncheck` evita alarme falso; o Trivy pega o que pode virar alcançável numa mudança futura e o que vem da imagem base. Reprovar só quando existe correção (`--ignore-unfixed`) evita travar o PR por algo que ninguém consegue resolver ainda.

### Imagens de terceiros

As imagens que o projeto usa sem construir (Postgres, Redis, Keycloak, Traefik, Prometheus, Jaeger, Grafana) não passam pelos modelos. O workflow [`third-party-images.yml`](../../.github/workflows/third-party-images.yml) varre todas toda segunda-feira e escreve uma tabela no resumo da execução, **sem reprovar**: essas imagens são atualizadas de propósito, num PR próprio, depois de ler o que mudou em cada versão. Ele roda da `main` (o GitHub só agenda workflows da branch padrão) e pode ser disparado à mão em *Actions → Third-party images → Run workflow*.

A primeira varredura, feita na VPS em 20/11/2026 com o Trivy 0.76.1, contando só o que tem correção:

| Imagem | Altas | Críticas | Onde estão |
|---|---|---|---|
| `postgres:17.4-alpine` | 92 | 7 | OpenSSL, libxml2, musl |
| `redis:7.4.2-alpine` | 78 | 6 | OpenSSL, musl, zlib e o Go do binário |
| `quay.io/keycloak/keycloak:26.1.4` | 96 | 9 | Bibliotecas Java (Jackson, Guava, drivers JDBC) e certificados |
| `traefik:v3.7.13` | 10 | 0 | `go-jose`, `x/net`, musl |
| `traefik/whoami:v1.11.0` | 26 | 1 | Biblioteca padrão do Go (rota temporária, sai quando os serviços existirem) |
| `grafana/grafana:13.2.6` | 19 | 0 | Biblioteca padrão do Go, `x/net` |
| `jaegertracing/jaeger:2.22.0` | 9 | 0 | musl, zlib, `x/net` |
| `prom/prometheus:v3.15.1` | 2 | 0 | `x/net` |

Postgres, Redis e Keycloak foram fixados em versões que já estavam defasadas quando o projeto começou. A atualização das três está na issue #75, porque o Keycloak pode mudar de comportamento entre versões e o Postgres precisa continuar lendo o volume de dados atual.

## Supply chain no próprio pipeline

O pipeline também roda código de terceiros, então segue os mesmos cuidados do código da aplicação:

- **Actions fixadas por hash de commit**, e não por tag (`actions/checkout@3d3c42e...  # v7.0.1`). Uma tag pode ser movida para apontar para outro código; um hash não. Esse foi o vetor de ataques reais contra actions populares, em que a tag de uma versão existente passou a apontar para código malicioso.
- **Ferramentas com versão fixa:** `govulncheck@v1.8.0` e `gitleaks` 8.30.1.
- **Download conferido:** o gitleaks e o Trivy são baixados da página de releases e o hash do arquivo é comparado com o arquivo de checksums publicado ([`scripts/install-trivy.sh`](../../scripts/install-trivy.sh)). Se não bater, o job falha antes de executar o binário.
- **Trivy pelo binário, e não pela action.** Em 19/03/2026, credenciais do projeto Trivy foram roubadas: publicaram uma versão maliciosa (v0.69.4) e moveram 76 das 77 tags da `aquasecurity/trivy-action` para código que rouba credenciais do runner ([GHSA-69fq-xp46-6x23](https://github.com/aquasecurity/trivy/security/advisories/GHSA-69fq-xp46-6x23)). Quem usava a action pela tag executou o código malicioso. O projeto usa a v0.76.1, publicada depois que as releases do Trivy passaram a ser imutáveis no GitHub, com o checksum conferido. É o mesmo motivo de fixar as actions por hash.
- **Permissão mínima:** o workflow só tem permissão de leitura do repositório (`permissions: contents: read`).

## Dependabot

O [`.github/dependabot.yml`](../../.github/dependabot.yml) abre PRs de atualização toda semana para:

| O quê | Onde |
|---|---|
| Dependências Go | `services/guard/go.mod` (e o `go.mod` de cada serviço Go novo) |
| Dependências npm | `package.json` de cada serviço TypeScript |
| Imagens base do Dockerfile | `services/guard/Dockerfile` (e o de cada serviço novo) |
| GitHub Actions | `.github/workflows/` (inclusive as do CodeQL e do `upload-artifact`) |

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
