---
uki_id: UKI_TECH_HYBRID_GITOPS
version: 0.1.0
status: draft
tags: [architecture, gitops, dev-experience, kubernetes, inner-loop]
created_at: 2025-12-25
---

# Hybrid GitOps: Local Mirror Strategy

## 1. Contexto e Definição
A estratégia **Hybrid GitOps** (ou "Local Mirror") resolve o problema de latência no ciclo de desenvolvimento (inner loop) em ambientes Kubernetes. Ela permite que mudanças locais sejam refletidas quase instantaneamente no cluster, mantendo a semântica declarativa do GitOps, mas sem poluir o repositório remoto principal (GitHub/GitLab) com commits de teste.

## 2. O Problema
No GitOps tradicional, cada mudança requer: `git commit` -> `git push` -> `CI Build` -> `CD Sync`. Esse ciclo pode levar minutos, o que é inviável para desenvolvimento rápido e depuração (debug).

## 3. A Solução: Local Mirror
O padrão utiliza um servidor Git intermediário rodando dentro do próprio cluster de desenvolvimento.

### Fluxo de Funcionamento
1.  **Watcher Local (`yby dev`)**: Monitora alterações nos arquivos do projeto (ex: manifestos Kubernetes, código fonte).
2.  **Sync Automático**: Ao detectar mudanças, o watcher realiza um `git push` forçado para o **Git Server In-Cluster**.
3.  **Reconciliação**: O controlador GitOps do cluster (ex: FluxCD, ArgoCD ou script simples) está configurado para observar esse Git Server interno (quando em modo `local` ou `mirror`), aplicando as mudanças imediatamente.

## 4. Regras de Uso
- **Ambiente Dev Apenas**: Este padrão deve ser usado exclusivamente em clusters de desenvolvimento/sandbox.
- **Efemeralidade**: O estado do Git Server interno é volátil. Commits importantes devem ser feitos manualmente no repositório remoto oficial (Origin) ao final da sessão de trabalho.
- **Toggle**: A ativação ocorre via contexto `local` ou variável de ambiente `YBY_MODE=mirror`.

## 5. Implementação de Referência
- **CLI**: `yby-cli` (comando `dev`).
- **Infra**: Pod `git-server` no namespace de infraestrutura.
