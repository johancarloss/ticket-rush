# ADR-007: Keycloak como servidor de identidade

- **Status:** aceita
- **Data:** 13/10/2026
- **Responsável pela implementação:** Johan

## Contexto

O sistema precisa saber quem é cada comprador e organizador: login, cadastro, senha guardada com segurança, emissão e renovação de tokens, e papéis (comprador e organizador). Com vários serviços, também importa **quem consegue criar um token válido**: se todo serviço que verifica tokens também puder criá-los, um serviço invadido vira uma fábrica de identidades falsas.

## O que é o Keycloak

O Keycloak é um servidor de identidade de código aberto. Ele oferece pronto o login, o cadastro, a recuperação de senha, o armazenamento das senhas com hash, a emissão e renovação de tokens, os papéis, MFA e um painel de administração. Ele implementa **OAuth2 e OpenID Connect (OIDC)**, os protocolos padrão de mercado, os mesmos usados quando um site oferece "Entrar com Google".

## Decisão

Usar o **Keycloak** para autenticação, com tokens JWT assinados em **RS256**:

- Só o Keycloak tem a **chave privada** e assina os tokens.
- Ele publica a **chave pública** num endereço padrão (JWKS). O Guard baixa essa chave e a guarda em memória para verificar os tokens.
- Com a chave pública dá para verificar um token, mas não dá para criar um.

O Keycloak só participa do login e da renovação do token. A cada compra, o Guard verifica o token sozinho, com a chave que já tem, sem chamar o Keycloak (ADR-002).

Papéis: `buyer` (comprador) e `organizer` (organizador). O Guard repassa o papel aos serviços junto com o `X-User-Id`.

## Alternativas consideradas

- **Autenticação própria.** Leve e sob controle total, mas exigiria escrever login, cadastro, hash de senha, emissão e renovação de tokens e recuperação de senha, onde cada detalhe é um ponto de falha de segurança. Com um segredo compartilhado (HS256), todo serviço que verifica tokens também poderia criá-los.
- **Zitadel ou Authelia.** Implementam os mesmos protocolos e são mais leves. Ficam como plano B se o Keycloak pesar demais na VPS.

## Consequências

**Positivas**
- Nenhum código de login escrito pelo grupo.
- Só o Keycloak cria tokens; um serviço invadido não consegue se passar por outro usuário.
- Como o Guard depende só do padrão OIDC, trocar o Keycloak por outro servidor compatível não afeta o resto do sistema.

**Negativas e riscos aceitos**
- **Consumo de memória:** o Keycloak é escrito em Java e usa algo entre 500 MB e 1 GB de RAM.
- **Configuração:** realms, clients e papéis levam um tempo para aprender.
- **Se o Keycloak cair,** ninguém faz login novo nem renova o token. Quem já tem um token válido continua comprando até ele expirar.

## Como validar

- Token adulterado (payload alterado sem reassinar): recusado pelo Guard com 401.
- Token expirado: recusado com 401.
- Derrubar o Keycloak durante a carga: compradores já logados continuam comprando; novos logins falham.
