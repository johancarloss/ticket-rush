# Proposta de Projeto — Engenharia de Sistemas Distribuídos 2026.2

**Título:** Venda de ingressos sob alta demanda: reserva de assentos resistente a falhas e a bots

**Equipe:**

| Nome | Matrícula | E-mail |
|---|---|---|
| Bruno Almeida dos Santos | 20230013099 | almeidabruno1506@gmail.com |
| Diego de Carvalho Dias | 20230089696 | diegoddias007@gmail.com |
| Johan Carlos Pereira Fernandes | 20230126771 | johancarlos62@gmail.com |
| Luis Antonio Feliciano da Silva | 20230012752 | luis.feliciano@academico.ufpb.br |

## Problema

Quando a venda de um show grande abre, milhares de pessoas tentam comprar poucos assentos no mesmo instante. Isso gera três problemas que só aparecem em um sistema distribuído:

1. **Disputa pelo mesmo assento.** Dois compradores atendidos por réplicas diferentes podem comprar o mesmo lugar se a reserva não for coordenada.
2. **Falha no meio da compra.** A compra passa por reserva, pagamento e confirmação, cada etapa em um serviço diferente. Se o pagamento cai depois que o assento foi reservado, o sistema precisa desfazer a reserva sem perder nem duplicar nada.
3. **Bots de cambistas.** Scripts reservam assentos em massa sem pagar e sobrecarregam a API, tirando a vez de quem quer comprar de verdade.

O sistema terá um fluxo de compra com reserva temporária (o assento fica bloqueado por 5 minutos e volta à venda se não for pago), um serviço de pagamento simulado que falha de propósito, e bots que simulam tanto fãs quanto cambistas para testar o sistema sob carga e sob ataque.

## Padrões arquiteturais

| Área | Padrão | Uso no projeto |
|---|---|---|
| Confiabilidade | SAGA com orquestração | Fluxo reserva → pagamento → confirmação, com compensação quando o pagamento falha |
| Confiabilidade | Coreografia com Pub/Sub | Após a compra, notificação e estatísticas reagem ao evento de ingresso emitido |
| Confiabilidade | Retry e DLQ | Mensagens que falham repetidamente vão para uma fila separada em vez de se perder |
| Desempenho | Cache-Aside | Consulta ao mapa de assentos sob milhares de acessos simultâneos |
| Desempenho | Rate Limit / Throttling | Limite por comprador para conter os bots de cambistas |
| Segurança | API Gateway | Ponto único de entrada com autenticação e rate limit |
| Deployment | Pipeline CI/CD com Blue-Green e Canary | Publicar versões novas durante a venda sem derrubar o sistema |

Além disso, o projeto terá observabilidade (métricas, logs e rastreamento distribuído) e verificações de segurança no pipeline (SAST, análise de dependências e de imagens).

## Tecnologias previstas

TypeScript (Node.js) nos serviços principais e Go em um serviço isolado, PostgreSQL, Redis (cache e filas), Docker Compose, GitHub Actions, Prometheus, Grafana e Jaeger, k6 para os bots de carga. O sistema será hospedado em uma VPS própria, e os testes de carga rodarão a partir de outra máquina.

## Critérios de sucesso

Os valores serão calibrados na entrega 02, mas a proposta é:

1. **Zero assentos vendidos duas vezes** com 2.000 bots disputando 500 assentos.
2. **Nenhum assento preso:** derrubando o serviço de pagamento durante a carga, toda reserva termina vendida ou liberada após a expiração.
3. **Nenhuma mensagem perdida:** toda mensagem é processada ou termina na DLQ.
4. **p95 abaixo de 200 ms** na consulta ao mapa de assentos com 1.000 usuários simultâneos.
5. **Bots contidos:** pelo menos 95% das requisições acima do limite são recusadas, com menos de 1% de erro para os fãs.
6. **Deploy sem queda:** uma versão nova publicada durante o teste de carga com taxa de erro abaixo de 1%.
