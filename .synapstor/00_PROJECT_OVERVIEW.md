# Project Overview & Context Map (Yby CLI)

> **Canonical Knowledge Source**
> Este arquivo descreve a arquitetura, objetivos e identidade do projeto `yby-cli`.
> **Última Atualização:** 2025-12-24

## 1. Identidade
**Slogan:** Bootstrap facilitado para o ecossistema Yby.
**Propósito:** Abstrair a complexidade de ferramentas como Helm, Argo CD e Kubernetes, oferecendo uma experiência de desenvolvedor (DX) fluida para iniciar projetos e gerenciar clusters.
**Maturidade:** **Active (Governed)** - Processos ágeis, mas exigência de UKI para mudanças semânticas.

## 2. Visão Técnica
- **Linguagem:** Go (Golang) 1.22+.
- **Principal Artefato:** Binário CLI `yby`.
- **Dependências Chave:** `k3d`, `kubectl`, `helm`.
- **Padrão de Release:** Semantic Versioning via Conventional Commits + GoReleaser.

## 3. Governança
Este projeto segue a [Governança Organizacional](../../.synapstor/00_PROJECT_OVERVIEW.md) da Casheiro Org.

### Personas da Squad
- **Product Owner:** Neto (Usuário). Define visão e requisitos.
- **Platform Engineer (AI):** CUIDADOR DO CÓDIGO. Implementação técnica, performance, segurança.
- **DevEx Guardian (AI):** CUIDADOR DO USUÁRIO. Usabilidade, feedback, documentação.
- **Governance Steward (AI):** CUIDADOR DO CONTEXTO. Organização do `.synapstor`, UKIs e integridade semântica.

## 4. Mapa de Referência
- **Backlog & Dívidas:** [02_BACKLOG_AND_DEBT.md](./02_BACKLOG_AND_DEBT.md)
- **Diagramas & Fluxos:** [03_DIAGRAMS.md](./03_DIAGRAMS.md)
- **Especificação de UKI:** [UKI_SPEC.md](./UKI_SPEC.md)
- **Log de Execução:** [01_EXECUTION_LOG.md](./01_EXECUTION_LOG.md)
- **Documentação Pública:** [Wiki Oficial](https://github.com/casheiro/yby-cli/wiki)
