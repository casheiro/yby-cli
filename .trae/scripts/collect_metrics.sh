#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
METRICS_DIR="$REPO_ROOT/.trae/metrics"
mkdir -p "$METRICS_DIR"

STEP="${1:-desconhecido}"
START_TS="$(date +%s)"

shift || true
"$@" || true

END_TS="$(date +%s)"
DURATION="$((END_TS - START_TS))"

printf '{"passo":"%s","duracao_segundos":%d,"timestamp":%d}\n' "$STEP" "$DURATION" "$END_TS" >> "$METRICS_DIR/metrics.json"
echo "✅ Métrica registrada para $STEP"
