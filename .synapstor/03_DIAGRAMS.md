# Diagramas e Fluxos

> Use [Mermaid](https://mermaid.js.org/) para descrever fluxos.

## 1. Fluxo de Init (Bootstrap)

A força do `yby up` (contexto local) reside na criação de um "loop fechado" de GitOps dentro do seu computador.

```mermaid
flowchart TD
    A[Dev] -->|Commit (feat/fix)| B(Branch develop)
    B -->|PR| C(Branch main)
    C -->|Merge| D{GitHub Action}
    D -->|GoReleaser| E[Release vX.Y.Z]
    E -->|Artifacts| F[Binaries / Docker]
```
