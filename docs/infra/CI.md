# CI

O GitHub Actions roda o pipeline em todo pull request para `develop` ou `main` e em todo push nessas branches. Um PR só deve ser mergeado com tudo verde ([CONTRIBUTING.md](../../CONTRIBUTING.md#pull-requests)). Definição em [`.github/workflows/ci.yml`](../../.github/workflows/ci.yml).

## Jobs

| Job | O que verifica | Falha quando |
|---|---|---|
| **Guard (Go)** | Formatação (`gofmt`), análise estática (`go vet`), testes com detector de condição de corrida (`go test -race`), vulnerabilidades conhecidas (`govulncheck`) e build da imagem Docker | Código fora do padrão, teste quebrado, acesso concorrente sem proteção, vulnerabilidade alcançável pelo código, ou imagem que não compila |
| **Compose** | `docker compose config` com o `.env.example` | O `compose.yaml` tem erro de sintaxe ou referencia algo que não existe |
| **Secrets** | `gitleaks` no histórico inteiro do repositório | Alguma senha, token ou chave aparece em qualquer commit |
| **Docs** | Links entre arquivos Markdown ([`scripts/check-doc-links.sh`](../../scripts/check-doc-links.sh)) | Um documento aponta para um arquivo que não existe, por exemplo depois de renomear |

Cada serviço novo ganha o próprio job, com os testes e a análise da sua linguagem.

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
| Dependências Go | `services/guard/go.mod` |
| Imagens base do Dockerfile | `services/guard/Dockerfile` |
| GitHub Actions | `.github/workflows/` |

Com `cooldown` de 14 dias: uma versão nova só é proposta depois de duas semanas publicada, o mesmo período de espera usado nas escolhas manuais. Atualizações de segurança não esperam esse prazo, pela mesma razão da exceção registrada no [GUARD.md](GUARD.md#dependências-e-supply-chain). Os PRs do Dependabot passam pelo mesmo CI.

## Rodar localmente

```bash
cd services/guard
test -z "$(gofmt -l .)" && go vet ./... && go test -race ./...
go run golang.org/x/vuln/cmd/govulncheck@v1.8.0 ./...

cd ../..
gitleaks git --redact .
./scripts/check-doc-links.sh
```
