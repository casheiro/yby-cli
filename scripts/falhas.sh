#!/usr/bin/env bash
# Lista os testes que falham num checkout (core, Bard, Atlas, Sentinel, Viz, Synapstor).
# Uso: scripts/falhas.sh <dir> <saida>. Para comparar com a develop, rode também num
# worktree da origin/develop e faça diff das duas saídas.
export PATH="$PATH:/c/Program Files/Go/bin"
cd "$1" || exit 1
{
  go test -count=1 ./pkg/... ./plugins/bard/... ./plugins/atlas/... ./cmd/... 2>&1
  go test -count=1 -tags k8s ./plugins/sentinel/... 2>&1
  (cd plugins/viz && go test -count=1 ./... 2>&1)
  (cd plugins/synapstor && go test -count=1 ./... 2>&1)
} | grep -E '^\s*--- FAIL|^FAIL\s|build failed|cannot|undefined' | sed 's/ (.*//' | sort -u > "$2"
