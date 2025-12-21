# Project Overview & Context Map (Yby CLI)

> **Canonical Knowledge Source**
> Este arquivo descreve a arquitetura, objetivos e identidade do projeto `yby-cli`.

## 1. Identidade
**Slogan:** Bootstrap facilitado para o ecossistema Yby.
**Propósito:** Abstrair a complexidade de ferramentas como Helm, Argo CD e Kubernetes, oferecendo uma experiência de desenvolvedor (DX) fluida para iniciar projetos e gerenciar clusters.
**Maturidade:** Active (Governed).

## 2. Visão Técnica
- **Linguagem:** Go (Golang).
- **Principal Artefato:** Binário CLI `yby`.
- **Dependências Chave:** `k3d`, `kubectl`, `helm`.
- **Padrão de Release:** Conventional Commits + GoReleaser (via GitHub Actions).

## 3. Governança
Este projeto segue a [Governança Organizacional](../../.synapstor/00_PROJECT_OVERVIEW.md).

### Personas Relevantes
- **Platform Engineer:** Mantém o core da CLI e os templates.
- **DevEx Guardian:** Garante que a CLI seja fácil e intuitiva.

## 4. Mapa de Referência
- **Documentação Pública:** [Wiki Oficial](https://github.com/casheiro/yby-cli/wiki)
- **Código Fonte:** `cmd/`, `pkg/`
