# ADR-005: Um Postgres com um schema por serviço

- **Status:** aceita
- **Data:** 13/10/2026

## Contexto

Inventário, Checkout, Notificação e Estatísticas guardam dados. Se dois serviços puderem ler e gravar as mesmas tabelas, eles ficam acoplados pelo banco: uma mudança de coluna feita por um quebra o outro, e a queda de um serviço deixa de ser uma falha real, porque o outro continua lendo os dados dele direto. Ao mesmo tempo, tudo roda numa VPS só, e cada instância de Postgres consome memória e precisa de backup e monitoramento.

## Decisão

Uma única instância de PostgreSQL, com um **schema por serviço** e um **usuário de banco por serviço**:

| Serviço | Schema | Usuário | Permissão |
|---|---|---|---|
| Inventário | `inventory` | `inventory_svc` | só no schema `inventory` |
| Checkout | `checkout` | `checkout_svc` | só no schema `checkout` |
| Notificação | `notification` | `notification_svc` | só no schema `notification` |
| Estatísticas | `stats` | `stats_svc` | só no schema `stats` |

Cada usuário só tem permissão no próprio schema, então um serviço não consegue ler a tabela de outro nem por engano. Quando o Checkout precisa saber de um assento, ele pergunta ao Inventário pela API.

O isolamento é **lógico** (permissões) e não **físico** (máquinas ou instâncias separadas).

A mesma instância também tem um **database separado, `keycloak`**, com um usuário próprio, onde o Keycloak cria e gerencia as tabelas dele (ADR-007). É um database à parte, e não um schema no nosso, porque as tabelas pertencem ao Keycloak e são migradas por ele.

## Alternativas consideradas

- **Banco compartilhado.** Todos os serviços nas mesmas tabelas. Simples e permite JOIN entre tudo, mas acopla os serviços pelo banco e impede testar a queda de um serviço de verdade.
- **Uma instância por serviço.** Isolamento físico completo, mas três instâncias para operar, fazer backup e monitorar, consumindo mais memória da VPS.

## Consequências

**Positivas**
- Nenhum serviço depende das tabelas de outro; cada um pode mudar o próprio schema livremente.
- A queda de um serviço é uma falha real e testável.
- Uma instância só para operar.

**Negativas e riscos aceitos**
- **Se a instância cair, todos os serviços com banco caem juntos.**
- **Vizinho barulhento:** uma consulta pesada de um serviço pode deixar os outros lentos, porque dividem CPU e disco.
- Não existe JOIN entre dados de serviços diferentes; relatórios que cruzam dados precisam passar pelas APIs ou pelos eventos.

Se os testes de carga mostrarem o Inventário prejudicando o Checkout, o Inventário ganha uma instância própria. Como cada serviço já só enxerga o próprio schema, a mudança é trocar a string de conexão.

## Como validar

- Conectar com o usuário `checkout_svc` e tentar `SELECT` em `inventory.seats`: deve ser negado.
- Comparar o p95 do Checkout com e sem carga pesada no Inventário, para medir o efeito de vizinho barulhento.
