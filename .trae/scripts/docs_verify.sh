#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
cd "$REPO_ROOT"

if [ ! -d "docs/wiki" ]; then
  echo "❌ Submódulo docs/wiki ausente"
  exit 1
fi

CMD_FILES=$(grep -RIl "var .*Cmd = &cobra.Command" cmd || true)
if [ -z "$CMD_FILES" ]; then
  echo "❌ Nenhum comando Cobra encontrado em cmd/"
  exit 1
fi

declare -a USES
while IFS= read -r f; do
  u=$(grep -n 'Use:' "$f" | sed -E 's/.*Use:\s*"([^"]+)".*/\1/' || true)
  if [ -n "$u" ]; then
    USES+=("$u")
  fi
done <<< "$CMD_FILES"

DOC_FILE="docs/wiki/CLI-Reference.md"
if [ ! -f "$DOC_FILE" ]; then
  echo "❌ CLI-Reference.md ausente em docs/wiki"
  exit 1
fi

MISSING=0
for u in "${USES[@]}"; do
  if ! grep -qE "##\s*\\\`?yby ${u}\\\`?" "$DOC_FILE"; then
    echo "❌ Comando 'yby ${u}' não documentado em CLI-Reference.md"
    MISSING=1
  fi
done

if [ "$MISSING" -eq 1 ]; then
  exit 1
fi

echo "✅ Documentação de CLI consistente"
