# Como executar

## Requisitos

- Docker com o plugin Compose. Docker 25 ou mais recente.
- Portas livres na máquina: `8180` (entrada HTTP), `8181` (painel do Traefik), `8182` (Grafana), `8183` (Prometheus), `8184` (Jaeger) e `8185` (console do Keycloak). Todas podem ser trocadas no `.env`.

## Subir o sistema

```bash
cp .env.example .env      # depois troque todos os valores change-me
docker compose up -d
docker compose ps         # espere todos ficarem (healthy); o Keycloak leva cerca de 30 s
```

O `.env` guarda senhas e nunca vai para o repositório (está no `.gitignore`). Para gerar valores aleatórios:

```bash
python3 -c "import secrets; print(secrets.token_urlsafe(18))"
```

## Endereços

| Endereço | O quê |
|---|---|
| `http://localhost:8180/auth/` | Keycloak, passando pelo Traefik: login dos compradores e chaves públicas do realm `ticket-rush` |
| `http://127.0.0.1:8185/auth/admin/` | Console de administração do Keycloak, com `KEYCLOAK_ADMIN_USER` e `KEYCLOAK_ADMIN_PASSWORD`. Só da própria máquina |
| `http://127.0.0.1:8181/dashboard/` | Painel do Traefik, com as rotas ativas. Só acessível da própria máquina |
| `http://localhost:8180/api/whoami` | Rota temporária que passa pelo Guard. Exige um token do Keycloak ([como obter](KEYCLOAK.md#obter-um-token)); sem ele responde 401 |
| `http://127.0.0.1:8182/` | Grafana, com os painéis na pasta Ticket Rush. Login com `GRAFANA_ADMIN_USER` e `GRAFANA_ADMIN_PASSWORD` |
| `http://127.0.0.1:8183/targets` | Prometheus, com o estado de cada serviço coletado |
| `http://127.0.0.1:8184/` | Jaeger, para procurar traces |

## Comandos do dia a dia

| Comando | Para quê |
|---|---|
| `docker compose ps` | Estado e saúde de cada container |
| `docker compose logs -f <serviço>` | Acompanhar o log de um serviço (`traefik`, `guard`, `postgres`, `redis`, `keycloak`) |
| `docker compose up -d --build` | Subir recompilando as imagens dos serviços (necessário depois de mudar o código do Guard) |
| `docker compose restart <serviço>` | Reiniciar um serviço |
| `docker compose down` | Parar tudo, mantendo os dados |
| `docker compose down -v` | Parar tudo e **apagar os dados** (volumes do Postgres, do Redis, do Prometheus e do Grafana) |

## Recomeçar do zero

O script que cria os schemas e usuários do Postgres só roda quando o volume está vazio, ou seja, na primeira subida. Se o script mudar, ou se as senhas do `.env` mudarem, é preciso recriar o volume:

```bash
docker compose down -v
docker compose up -d
```

Isso apaga todos os dados do Postgres, do Redis e do Keycloak.

## O que sobe hoje

| Container | Documentação |
|---|---|
| Traefik | [TRAEFIK.md](TRAEFIK.md) |
| PostgreSQL | [POSTGRES.md](POSTGRES.md) |
| Redis | [REDIS.md](REDIS.md) |
| Keycloak | [KEYCLOAK.md](KEYCLOAK.md) |
| Guard (2 réplicas) | [GUARD.md](GUARD.md) |
| whoami (temporário) | [TRAEFIK.md](TRAEFIK.md#rotas-atuais) |
| Prometheus, Jaeger e Grafana | [OBSERVABILIDADE.md](OBSERVABILIDADE.md) |
| Estatísticas | [ESTATISTICAS.md](../services/ESTATISTICAS.md) |

Os outros serviços da aplicação (Inventário, Checkout, Pagamento, Notificação) ainda não estão no Compose; cada um entra com o próprio esqueleto.
