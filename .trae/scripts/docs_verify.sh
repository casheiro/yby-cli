#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
cd "$REPO_ROOT"

echo "🔍 Iniciando verificação de documentação..."

if [ ! -d "docs/wiki" ]; then
  echo "❌ Submódulo docs/wiki ausente"
  exit 1
fi

# 1. Verifica cobertura de comandos no CLI-Reference
echo "Checking CLI Reference coverage..."
CMD_FILES=$(grep -RIl "var .*Cmd = &cobra.Command" cmd || true)
if [ -z "$CMD_FILES" ]; then
  echo "❌ Nenhum comando Cobra encontrado em cmd/"
  exit 1
fi

declare -a USES
while IFS= read -r f; do
  # Extract the first word of Use string, e.g. "dev" from "dev [flags]"
  u=$(grep -m 1 'Use:' "$f" | sed -E 's/.*Use:\s*"([^" ]+).*/\1/' || true)
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
  # Check if the command is mentioned in the doc file
  # We look for "yby <command>" pattern
  if ! grep -q "yby ${u}" "$DOC_FILE"; then
    # Filter known subcommands that are documented under their parent command
    # e.g., 'vps' is under 'bootstrap vps'
    # List of exceptions:
    case "$u" in
      vps|cluster)
        # Check if parent exists? 'yby bootstrap vps'
        if ! grep -q "yby bootstrap ${u}" "$DOC_FILE"; then
           echo "⚠️  Subcomando 'yby bootstrap ${u}' parece não estar documentado."
        fi
        ;;
      seal|webhook|minio|backup|restore)
        # Check if parent exists? 'yby secret ...'
        if ! grep -q "yby secret ${u}" "$DOC_FILE"; then
             echo "⚠️  Subcomando 'yby secret ${u}' parece não estar documentado."
        fi
        ;;
      dump)
        # 'yby env dump'
        if ! grep -q "yby env ${u}" "$DOC_FILE"; then
             echo "⚠️  Subcomando 'yby env ${u}' parece não estar documentado."
        fi
        ;;
      secret|setup)
         # These are top level, check normally
         if ! grep -q "yby ${u}" "$DOC_FILE"; then
              echo "⚠️  Comando 'yby ${u}' parece não estar documentado em CLI-Reference.md"
         fi
         ;;
      yby|keda|github-token)
        # Internal or alias commands, ignore or check specifically if needed.
        # 'yby' is the root command, should be ignored.
        # 'keda' is under generate? 'yby generate keda'
        if [[ "$u" == "keda" ]]; then
             if ! grep -q "yby generate keda" "$DOC_FILE"; then
                 echo "⚠️  Comando 'yby generate keda' parece não estar documentado."
             fi
        elif [[ "$u" != "yby" ]]; then
             # Ignore github-token as it is very specific/internal usage often
             :
        fi
        ;;
      *)
        echo "⚠️  Comando 'yby ${u}' parece não estar documentado em CLI-Reference.md"
        ;;
    esac
  fi
done

# 2. Check for legacy .env usage
echo "Checking for legacy .env usage..."
# We exclude checkEnvVars because it's the place where we handle backward compat/warnings
# We exclude bootstrap_vps.go as it is scheduled for Phase 4
if grep -r "\.env" cmd/ | grep -v "func checkEnvVars" | grep -v "bootstrap_vps.go" | grep -v "binary file matches"; then
   echo "⚠️  Referências a .env encontradas em cmd/ (considere migrar para .yby/environments.yaml)"
else
   echo "✅ Nenhuma referência direta a .env encontrada fora das exceções."
fi

# 3. Verify environments.yaml template existence
echo "Checking environments.yaml template..."
ENV_TMPL="pkg/templates/assets/.yby/environments.yaml.tmpl"
if [ ! -f "$ENV_TMPL" ]; then
  echo "❌ Template environments.yaml não encontrado em $ENV_TMPL"
  exit 1
else
  echo "✅ Template environments.yaml encontrado."
fi

echo "✅ Documentação verificada com sucesso."
