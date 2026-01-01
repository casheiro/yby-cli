#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
cd "$REPO_ROOT"

OUT_DIR="$REPO_ROOT/.trae/metrics"
mkdir -p "$OUT_DIR"

go test ./test/e2e/... -v | tee "$OUT_DIR/e2e_report.txt"
echo "✅ E2E executado"
