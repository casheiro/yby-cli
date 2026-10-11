#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
cd "$REPO_ROOT"

echo "🔍 Iniciando verificação de documentação..."

if [ ! -d "docs/wiki" ]; then
  echo "❌ Submódulo docs/wiki ausente"
  exit 1
fi

# 1. Freshness Check (Doc-as-Code)
echo "🔄 Verificando se a documentação está sincronizada com o código..."

# Gera docs temporariamente
echo "   Executando 'yby gen-docs'..."
if ! go run ./cmd/yby gen-docs > /dev/null; then
    echo "❌ Falha ao executar gerador de documentação."
    exit 1
fi

# (A sidebar agora é atualizada automaticamente pelo yby gen-docs)

# Verifica se houve mudanças
if [[ -n $(git status --porcelain docs/wiki) ]]; then
  echo "❌ Documentação desatualizada detectada!"
  echo "   As seguintes alterações foram geradas mas não estão no commit:"
  git status --porcelain docs/wiki
  echo ""
  echo "   👉 Solução: Rode 'yby gen-docs' (ou 'go run ./cmd/yby gen-docs') e comite os arquivos gerados."
  echo "   👉 Dica: O comando 'yby gen-docs' agora atualiza a sidebar automaticamente."
  
  # Opcional: Mostrar diff
  # git diff docs/wiki
  
  exit 1
else
  echo "✅ Documentação (Markdown) está sincronizada com o código."
fi

echo "✅ Documentação verificada com sucesso."
