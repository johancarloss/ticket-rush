# C4 nível 1: contexto

O sistema visto de fora, como uma caixa só. Mostra quem usa o Ticket Rush e com quais sistemas externos ele conversa.

```mermaid
flowchart TB
  buyer["<b>Comprador</b><br/>[Pessoa]<br/>Fã que quer comprar um ingresso<br/>na abertura das vendas"]
  organizer["<b>Organizador</b><br/>[Pessoa]<br/>Cadastra eventos, setores<br/>e assentos"]
  scalper["<b>Cambista</b><br/>[Pessoa externa]<br/>Usa bots para reservar assentos<br/>em massa e revender"]

  ticketrush["<b>Ticket Rush</b><br/>[Sistema]<br/>Venda de ingressos com reserva<br/>temporária de assentos"]

  payment["<b>Provedor de pagamento</b><br/>[Sistema externo]<br/>Gateway de cartão. Simulado pelo grupo,<br/>com atrasos e falhas configuráveis"]
  email["<b>Provedor de e-mail</b><br/>[Sistema externo]<br/>Entrega a confirmação da compra"]

  buyer -- "Consulta assentos, reserva e paga<br/>[HTTPS]" --> ticketrush
  organizer -- "Cadastra eventos<br/>[HTTPS]" --> ticketrush
  scalper -- "Dispara reservas automatizadas<br/>[HTTPS]" --> ticketrush
  ticketrush -- "Cobra e estorna<br/>[HTTPS]" --> payment
  ticketrush -- "Envia confirmação<br/>[SMTP]" --> email

  classDef person fill:#08427b,stroke:#052e56,color:#fff
  classDef adversary fill:#8b1e1e,stroke:#5c1414,color:#fff
  classDef system fill:#1168bd,stroke:#0b4884,color:#fff
  classDef external fill:#999,stroke:#6b6b6b,color:#fff
  class buyer,organizer person
  class scalper adversary
  class ticketrush system
  class payment,email external
```

## Decisões deste nível

- **O cambista aparece como ator, em vermelho.** Ele é parte do problema que o sistema resolve, então o diagrama mostra o adversário. Rate limit e mitigação de bots existem por causa dele.
- **O pagamento é um sistema externo.** Na vida real seria um gateway de cartão de terceiros. No projeto, o grupo escreve um simulador que atrasa, falha e cai de propósito, mas ele fica fora da fronteira do Ticket Rush. O sistema só fala com ele por HTTP, como falaria com um provedor real.
- **O e-mail também é externo.** Em desenvolvimento ele é substituído por um servidor SMTP de teste.
- Login (Keycloak) e infraestrutura (Postgres, Redis, observabilidade) ficam dentro da caixa e só aparecem no nível 2.
