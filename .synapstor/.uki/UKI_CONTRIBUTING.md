---
uki_id: UKI_CONTRIBUTING
version: 1.0.0
status: active
tags: [contributing, git-flow, conventional-commits]
---

# Contributing Rules (Yby CLI)

## 1. Flow de Desenvolvimento
- **Git-Flow:**
    - `main`: Produção (Tags).
    - `develop`: Integração (Base para PRs).
- **Branching:** `feature/nome`, `fix/nome`.

## 2. Commit Standards
Adesão estrita ao **Conventional Commits** para release automático.
- `feat:` -> Minor
- `fix:` -> Patch
- `BREAKING CHANGE:` -> Major

## 3. Qualidade
- **Linters:** `golangci-lint` deve passar.
- **Testes:** `go test ./...` obrigatório.
