---
uki_id: UKI_ARCH_PRINCIPLES
version: 1.0.0
status: active
tags: [architecture, principles, stateless]
---

# Architectural Principles (Yby CLI)

## 1. Statelessness
A CLI não deve guardar estado local complexo.
- **Prefer:** Ler o estado do Cluster (Kubernetes) ou do Git.
- **Avoid:** Bancos de dados locais (SQLite) ou arquivos de config escondidos que desincronizam.

## 2. Zero Touch Discovery
O usuário não deve configurar IPs ou URLs manualmente.
- A CLI deve descobrir onde está o cluster (via Kubeconfig).
- A CLI deve descobrir onde está o repo (via Git Remote).

## 3. Auto-Repair
Se um arquivo de configuração estiver faltando, a CLI deve ser capaz de baixá-lo novamente do template oficial. Nunca deixe o usuário "quebrado".
