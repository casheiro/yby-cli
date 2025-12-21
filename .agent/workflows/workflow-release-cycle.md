---
description: Pipeline de Release Automatizado (Conventional Commits)
---

# Release Cycle Workflow

**Objetivo:** Gerar nova versão, changelog e binários automaticamente.

## 1. Commit & Push
- Desenvolvedor commita na branch `feature/*` usando Conventional Commits.
- Abre PR para `develop`.
- **CI Checa:** Lint, Testes.

## 2. Merge to Main
- Maintainer faz merge de `develop` para `main`.
- **Release Please** Action roda e cria um "Release PR" com o novo Changelog e bump de versão.

## 3. Tagging
- Maintainer faz merge do "Release PR".
- **Release Please** cria a Tag Git (ex: `v1.2.0`).

## 4. Building
- **GoReleaser** dispara ao detectar a Tag.
- Compila binários (Linux, Mac, Windows).
- Cria Docker Image.
- Publica Release no GitHub.
- Atualiza Homebrew Tap.
