---
uki_id: UKI_TECH_STACK
version: 1.0.0
status: active
tags: [stack, technology, tools]
---

# Pilha Tecnológica (Yby CLI)

## Núcleo
- **Language:** Go 1.26+
- **CLI Framework:** Cobra + Viper
- **Logging:** `log/slog` estruturado (text/json via `--log-format`)
- **UI:** Charmbracelet Bubbletea (TUI interativa)

## Integração de Infraestrutura
- **Kubernetes:** Client-go
- **Helm:** Helm SDK
- **GitOps:** Argo CD API

## Build & CI
- **Linter:** golangci-lint
- **Release:** GoReleaser
- **Action:** Release Please (Google)
