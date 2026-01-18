# Regras Globais para Agentes

> **Princípio Fundamental:** Todo código deve ter um propósito semântico claro e rastreável.

## 1. Fonte da Verdade
- **Codebase atual** é a verdade técnica.
- **.synapstor/** é a verdade semântica.
- **Se houver conflito**: Pare e pergunte ao usuário ou invoque o Governance Steward.

## 2. Fluxo de Decisão
Antes de executar qualquer mudança:
1.  **Leia** as UKIs relevantes em `.synapstor/.uki/`.
2.  **Verifique** se existe um padrão estabelecido.
3.  **Se não existir**: Proponha a criação de uma UKI (via `uki-capture`) *antes* ou *junto* com a implementação.

## 3. Idioma
- Comunique-se com o usuário em **Português**.
- Escreva commits em **Inglês** (Conventional Commits).
- Escreva UKIs em **Português**.

## 4. Segurança e Integridade
- Nunca faça hardcode de credenciais.
- Nunca ignore testes falhando.
- Nunca altere `.synapstor` manualmente sem justificativa clara (ou ser o Steward).
