# Kit de avaliação da disciplina

O professor avalia e compara os cinco projetos de ingressos da turma com o [kit de avaliação](https://github.com/rkulesza/esd-carga-tickets): a mesma massa de dados e os mesmos seis cenários (carga, pico, corrida, idempotência, expiração e compensação) contra todos os grupos. O Ticket Rush é o grupo 09.

**O código do kit não foi alterado.** O Ticket Rush se liga a ele só pelo adaptador [`grupo_09.json`](grupo_09.json), que traduz os nomes de rota, campos, códigos e status da nossa API (`/api/orders`, `order_id`, `CONFIRMED`) para os do contrato do kit (`/reservas`, `reserva_id`, `confirmada`). O que o sistema mudou para isso, e por quê, está no [ADR-011](../docs/adr/0011-KIT-DE-AVALIACAO.md): o perfil de teste (`EVALUATION_MODE`), a reserva e o pagamento idempotentes, o tempo da reserva configurável, a simulação de pagamento por requisição e o serviço das rotas `/_teste`.

| Item | Valor |
|---|---|
| Versão do kit usada | commit [`07ef200`](https://github.com/rkulesza/esd-carga-tickets/tree/07ef200c101a72704803e37da8247a0d7e0d4152) (13/11/2026) |
| Token | Conta de serviço `evaluation-kit` do Keycloak, papel `buyer`, token de 1 hora; o script de execução preenche `{{TOKEN_DO_KIT}}` |
| Chave das rotas de teste | `{{CHAVE_DE_TESTE}}`, preenchida a partir do `.env` |
| Serviço derrubado no teste de caos | `checkout` |

O script que prepara o ambiente (perfil de teste, conta de serviço, token) e roda o kit entra com o serviço de teste. Os resultados de cada rodada ficam no [plano de testes](../docs/TESTES.md#kit-de-avaliação-da-disciplina).
