# PostgreSQL

Uma instância de PostgreSQL 17 guarda os dados de todos os serviços e do Keycloak, com isolamento por schema e por usuário. A decisão e os trade-offs estão no [ADR-005](../adr/0005-SCHEMA-POR-SERVICO.md).

## Organização

```
instância postgres
├── database ticketrush
│   ├── schema inventory      ← dono: inventory_svc
│   ├── schema checkout       ← dono: checkout_svc
│   ├── schema notification   ← dono: notification_svc
│   ├── schema stats          ← dono: stats_svc
│   └── schema public         ← ninguém cria nada aqui
└── database keycloak         ← dono: keycloak_svc (tabelas criadas pelo próprio Keycloak)
```

| Usuário | Pode | Não pode |
|---|---|---|
| `inventory_svc` | Criar e usar tabelas no schema `inventory` | Ler ou criar nos outros schemas e no `public`; conectar no database `keycloak` |
| `checkout_svc` | Idem, no schema `checkout` | Idem |
| `notification_svc` | Idem, no schema `notification` | Idem |
| `stats_svc` | Idem, no schema `stats` | Idem |
| `keycloak_svc` | Tudo no database `keycloak` | Conectar no database `ticketrush` |

Cada usuário tem o `search_path` apontando para o próprio schema, então o serviço pode escrever `CREATE TABLE seats` sem prefixo e a tabela vai para o lugar certo.

## Como é criado

O script [`infra/postgres/init.sh`](../../infra/postgres/init.sh) roda na primeira subida do container, quando o volume `postgres-data` ainda está vazio. Ele:

1. tira do `PUBLIC` (todos os usuários) a permissão no schema `public` e a de conectar no database `ticketrush`;
2. cria os quatro usuários de serviço com as senhas do `.env`;
3. cria um schema para cada um, já com o usuário como dono;
4. dá a cada um a permissão de conectar no `ticketrush` e ajusta o `search_path`;
5. cria o usuário e o database do Keycloak.

O script não roda de novo nas subidas seguintes. Para aplicar uma mudança nele, recrie o volume ([COMO-EXECUTAR.md](COMO-EXECUTAR.md#recomeçar-do-zero)).

## Conectar para inspecionar

```bash
set -a; . ./.env; set +a
docker compose exec -e PGPASSWORD=$CHECKOUT_DB_PASSWORD postgres \
  psql -h localhost -U checkout_svc -d ticketrush
```

O Postgres não tem porta publicada na máquina; o acesso é pelo `docker compose exec`. Os serviços o alcançam pela rede interna `data`, no endereço `postgres:5432`.

## Testar o isolamento

Conectado como `checkout_svc`:

```sql
SELECT current_schema();          -- checkout
CREATE TABLE orders (id int);     -- funciona
SELECT * FROM inventory.seats;    -- ERROR: permission denied for schema inventory
CREATE TABLE public.x (id int);   -- ERROR: permission denied for schema public
DROP TABLE orders;
```

E com `keycloak_svc` no database da aplicação:

```bash
psql -h localhost -U keycloak_svc -d ticketrush
# FATAL: permission denied for database "ticketrush"
# DETAIL: User does not have CONNECT privilege.
```

Resultado verificado na versão v0.3 (ver [EVOLUCAO.md](../architecture/EVOLUCAO.md)).
