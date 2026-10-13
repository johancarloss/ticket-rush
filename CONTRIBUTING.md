# Como contribuir

Regras de trabalho do grupo no repositório. A nota individual leva em conta os commits de cada integrante, então cada um commita o próprio trabalho, com o próprio usuário do GitHub.

## Branches

```
main      ← só recebe entregas (Projeto 01, 02, 03); cada merge aqui é uma versão
develop   ← integração do que está pronto e revisado
  └── feature/<escopo>/<descricao>   ← trabalho do dia a dia
```

| Tipo | Sai de | Volta para | Exemplo |
|---|---|---|---|
| `feature/` | `develop` | `develop` | `feature/inventory/seat-hold` |
| `fix/` | `develop` | `develop` | `fix/checkout/duplicate-payment` |
| `docs/` | `develop` | `develop` | `docs/adr/circuit-breaker` |
| `chore/` | `develop` | `develop` | `chore/infra/compose-redis` |
| `hotfix/` | `main` | `main` e `develop` | `hotfix/guard/token-expiry` |

Nomes de branch em inglês e em kebab-case. O escopo é o serviço ou a área afetada:

`inventory`, `checkout`, `payment`, `notification`, `stats`, `guard`, `bots`, `infra`, `ci`, `docs`, `adr`.

Regras:
- Ninguém commita direto na `main`.
- Na `develop`, só por pull request.
- Uma branch por assunto, de vida curta. Antes de abrir o PR, atualizar a branch com a `develop` mais recente.

## Commits

Formato [Conventional Commits](https://www.conventionalcommits.org/pt-br/v1.0.0/), em inglês:

```
tipo(escopo): descrição curta no imperativo
```

| Tipo | Quando |
|---|---|
| `feat` | funcionalidade nova |
| `fix` | correção de bug |
| `refactor` | mudança de código sem mudar comportamento |
| `test` | testes |
| `docs` | documentação |
| `chore` | build, dependências, configuração, infraestrutura |

Exemplos:

```
feat(inventory): add conditional update for seat hold
fix(checkout): send idempotency key on payment retry
docs(adr): add ADR-008 circuit breaker
```

Uma mudança lógica por commit. Detalhes e motivação vão na descrição do pull request.

## Pull requests

1. Abrir o PR da branch para a `develop`, com:
   - **Resumo:** o que mudou e por quê;
   - **Como testar:** passos para conferir;
   - a issue relacionada, se houver (`Closes #N`).
2. O CI precisa estar verde.
3. Pelo menos **um outro integrante** revisa e aprova.
4. Quem abriu o PR faz o merge depois da aprovação.

O PR de release (`develop → main`) é aberto a cada marco concluído, sem esperar o fim de um Projeto. Junto com ele:
- o `CHANGELOG.md` é atualizado, movendo o que está em `[Unreleased]` para a versão nova;
- depois do merge, a `main` recebe uma tag com a versão (`v0.X.0`).

O CHANGELOG indica qual versão foi entregue em cada Projeto. A versão `v1.0.0` fica para a entrega final (Projeto 03).

## Decisões de arquitetura

Decisão que muda a arquitetura (padrão novo, tecnologia nova, mudança de fronteira entre serviços) ganha um ADR em `docs/adr/`, seguindo o formato dos existentes, e entra no índice `docs/adr/README.md`. ADR aceito não é editado para mudar de ideia; a decisão nova vira outro ADR, que indica qual substitui.

## Uso de IA

O README tem a seção obrigatória **"Ferramentas de IA utilizadas"**. Quem usar uma ferramenta de IA numa parte do trabalho acrescenta ou atualiza a linha correspondente no mesmo PR, informando:
- a ferramenta;
- onde atuou;
- como foi orientada;
- o que funcionou, o que foi corrigido e o que foi descartado.
