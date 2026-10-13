# ADR-004: Controle do hold com UPDATE condicional no Postgres

- **Status:** aceita
- **Data:** 13/10/2026
- **Responsável pela implementação:** Inventário

## Contexto

Na abertura das vendas, muitos compradores tentam o mesmo assento ao mesmo tempo, e cada requisição pode cair numa réplica diferente do Inventário. Só um pode ganhar. A forma mais simples de escrever a reserva quebra:

```ts
const seat = await db.get(42)            // A lê: livre      B lê: livre
if (seat.status === "available") {
  await db.set(42, "held")               // A grava: held    B grava: held → os dois acham que ganharam
}
```

Entre ler e gravar existe um intervalo, e o outro processo entra por ele. Como os processos estão em máquinas diferentes, um lock em memória não resolve; a coordenação precisa acontecer num lugar que todos compartilham.

## Decisão

O Postgres é a **única fonte da verdade** sobre de quem é cada assento. A reserva é um `UPDATE` condicional, que lê e grava numa operação só:

```sql
UPDATE inventory.seats
   SET status = 'held', held_by = :buyer_id, hold_until = now() + interval '5 minutes'
 WHERE id = :seat_id
   AND (status = 'available' OR (status = 'held' AND hold_until < now()))
RETURNING id;
```

- Retornou uma linha: o comprador ganhou o assento.
- Retornou zero linhas: alguém chegou antes, e a resposta é "assento indisponível".

O Postgres trava a linha durante o `UPDATE`, então dois `UPDATE` no mesmo assento acontecem um depois do outro, e o segundo já encontra o assento como `held`. Ninguém fica esperando numa fila de locks por mais que o tempo do próprio `UPDATE`.

### Expiração sem timer

O trecho `OR (status = 'held' AND hold_until < now())` faz um hold vencido contar como livre para o próximo que tentar. A expiração não depende de nenhum job rodar na hora certa. Um job de limpeza pode existir só para deixar o mapa de assentos atualizado, mas a corretude não depende dele.

### Outras operações no mesmo formato

Todas as mudanças de estado do assento seguem o mesmo padrão de `UPDATE ... WHERE <estado esperado>`:

| Operação | Condição |
|---|---|
| Renovar o hold ao pagar (ADR-001) | `status = 'held' AND held_by = :buyer_id` |
| Vender | `status = 'held' AND held_by = :buyer_id` |
| Liberar (compensação) | `status = 'held' AND held_by = :buyer_id` |

Se a condição não bate (o hold já era de outro, ou já foi vendido), nada muda e quem chamou recebe a recusa. Repetir uma chamada que já deu certo também não muda nada, o que a torna segura para os retries da SAGA.

### O Redis só como cache de leitura

O mapa de assentos que os compradores consultam vem de um cache no Redis com validade curta (cerca de 2 segundos). Ele pode mostrar um assento como livre por um instante depois de ele ser reservado; quando o comprador tenta reservar, o `UPDATE` no Postgres dá a resposta certa. O cache serve para aguentar as consultas, e não decide nada.

## Alternativas consideradas

- **Reserva no Redis com `SET seat:42 <comprador> NX EX 300`.** Mais rápido, e o `EX` expira sozinho. Mas a reserva ficaria no Redis e a venda no Postgres, com duas fontes da verdade. Como o Redis guarda os dados em memória, uma reinicialização apagaria todos os holds no meio da venda, e assentos em pagamento voltariam a aparecer como livres.
- **Lock pessimista (`SELECT ... FOR UPDATE` e depois `UPDATE`).** Correto, mas segura a linha durante duas idas ao banco. No pico, as conexões ficam presas esperando, e o pool se esgota.
- **Lock otimista com coluna de versão.** Também correto, mas no pico quase todas as tentativas no mesmo assento falham e precisam ser refeitas. O `UPDATE` condicional dá o mesmo resultado em uma ida ao banco.

## Consequências

**Positivas**
- Uma fonte da verdade, então não existe estado divergente entre Redis e Postgres.
- Uma ida ao banco por tentativa de reserva, sem locks segurados entre chamadas.
- A expiração não depende de timer.

**Negativas e riscos aceitos**
- Toda tentativa de reserva vai ao Postgres. No pico, ele é o gargalo. O teste de carga mede se aguenta; se não aguentar, o primeiro passo é dar ao Inventário uma instância de Postgres só dele (ver o C4 nível 2).
- O mapa pode mostrar um assento já reservado como livre por até cerca de 2 segundos. O comprador recebe "indisponível" ao tentar, o que é aceitável.

## Como validar

- 1.000 bots tentando o mesmo assento ao mesmo tempo: exatamente uma reserva com sucesso.
- 2.000 bots disputando 500 assentos: ao final, nenhum assento com dois donos (consulta no banco).
- Hold vencido: depois de 5 minutos sem pagamento, outro comprador consegue reservar o assento sem nenhum job rodar.
