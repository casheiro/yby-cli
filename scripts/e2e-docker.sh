#!/bin/bash
set -e

echo "🚀 Iniciando Teste E2E Isolado (Docker)"
echo "--------------------------------------"

# 1. Verifica Instalação
echo "✅ Verificando versão..."
yby version

# 2. Verifica Dependências
echo "✅ Verificando dependências (Doctor)..."
# Doctor might fail on docker if docker socket is not mounted, so we just run it to see output but don't exit on error
yby doctor || true

# 3. Teste de Init (Scaffold)
echo "✅ Testando 'yby init'..."
mkdir -p /tmp/test-project
cd /tmp/test-project
yby init --git-repo https://github.com/casheiro/yby-demo --offline \
  --topology standard \
  --workflow gitflow \
  --target-dir . \
  --non-interactive

if [ ! -f "yby.yaml" ] && [ ! -d ".yby" ]; then
    echo "❌ Falha: Arquivos não gerados corretamente."
    exit 1
fi
echo "   Scaffold gerado com sucesso."

# 4. Teste de Workload (Chart Create)
echo "✅ Testando 'yby chart create'..."
yby chart create demo-app

if [ ! -f "charts/demo-app/Chart.yaml" ]; then
    echo "❌ Falha: Chart não gerado."
    exit 1
fi
echo "   Chart 'demo-app' gerado com sucesso."

# 5. Verifica Helpers
echo "✅ Verificando templates gerados..."
if grep -q "app-template" "charts/demo-app/Chart.yaml"; then
    echo "   Chart.yaml correto."
else
    echo "⚠️  Aviso: Conteúdo do Chart.yaml suspeito."
fi

echo "--------------------------------------"
echo "🎉 Teste Isolado Concluído com Sucesso!"
