#!/usr/bin/env bash
# Mostra o estado da sessão de trabalho: plano ativo, próxima tarefa, branch,
# commits sem push e mudanças pendentes. Usado pelo hook SessionStart e pela
# skill /retomar. Lê agile/sessoes/estado.md, mantido pela skill /encerrar.
set -u

cd "$(git rev-parse --show-toplevel 2>/dev/null || pwd)" || exit 0

ESTADO="agile/sessoes/estado.md"
MODO_HOOK=0
[ "${1:-}" = "--hook" ] && MODO_HOOK=1

# campo lê um valor entre crases de uma linha "- **Nome:** `valor`" do estado.md
campo() {
	grep -m1 -E "^- \*\*$1:\*\*" "$ESTADO" 2>/dev/null | sed -n 's/.*`\([^`]*\)`.*/\1/p'
}

if [ ! -f "$ESTADO" ]; then
	echo "Controle de sessão: $ESTADO não encontrado; nada a retomar."
	exit 0
fi

PLANO="$(campo Plano)"
BRANCH_ESPERADA="$(campo Branch)"
BRANCH_ATUAL="$(git branch --show-current 2>/dev/null)"

[ "$MODO_HOOK" = 1 ] && echo "Contexto carregado pelo hook SessionStart (controle de sessão do yby-cli). Use /retomar para continuar e /encerrar ao parar."
echo "═══ SESSÃO · yby-cli ═══"

# Branch
if [ -n "$BRANCH_ESPERADA" ] && [ "$BRANCH_ATUAL" != "$BRANCH_ESPERADA" ]; then
	echo "Branch: $BRANCH_ATUAL  ▲ o plano usa $BRANCH_ESPERADA"
else
	echo "Branch: $BRANCH_ATUAL"
fi

# Commits locais que não estão em nenhum remoto e mudanças não commitadas
SEM_PUSH="$(git rev-list --count HEAD --not --remotes 2>/dev/null || echo "?")"
ALTERADOS="$(git status --porcelain 2>/dev/null | wc -l | tr -d ' ')"
echo "Commits sem push: $SEM_PUSH · arquivos alterados: $ALTERADOS"
if [ "$ALTERADOS" != "0" ]; then
	git status --porcelain 2>/dev/null | head -8 | sed 's/^/  /'
fi

# Progresso do plano
if [ -n "$PLANO" ] && [ -f "$PLANO" ]; then
	# IDs de tarefa: uma ou duas letras e o número (T001, C01, B02), com ou sem negrito.
	ID='\**[A-Z]{1,2}[0-9]+'
	FEITAS="$(grep -cE "^- \[x\] $ID" "$PLANO")"
	ANDAMENTO="$(grep -cE "^- \[~\] $ID" "$PLANO")"
	BLOQUEADAS="$(grep -cE "^- \[!\] $ID" "$PLANO")"
	TOTAL="$(grep -cE "^- \[[ x~!]\] $ID" "$PLANO")"
	echo "Plano: $PLANO"
	echo "Progresso: $FEITAS/$TOTAL concluídas · $ANDAMENTO em andamento · $BLOQUEADAS bloqueadas"
	PROXIMA="$(grep -m1 -E "^- \[~\] $ID" "$PLANO")"
	[ -z "$PROXIMA" ] && PROXIMA="$(grep -m1 -E "^- \[ \] $ID" "$PLANO")"
	if [ -n "$PROXIMA" ]; then
		echo "Próxima: $(echo "$PROXIMA" | sed 's/^- //; s/\*\*//g' | cut -c1-150)"
	else
		echo "Próxima: nenhuma tarefa pendente no plano"
	fi
elif [ -n "$PLANO" ]; then
	echo "Plano: $PLANO  ▲ arquivo não encontrado nesta branch"
fi

# Últimos commits
echo "Últimos commits:"
git log --oneline -3 2>/dev/null | sed 's/^/  /'

# Go no PATH (no Windows, sessões abertas antes da instalação não enxergam o Go)
if command -v go >/dev/null 2>&1; then
	echo "Go: $(go env GOVERSION 2>/dev/null)"
elif [ -x "/c/Program Files/Go/bin/go.exe" ]; then
	echo "Go: ▲ fora do PATH; prefixe os comandos com export PATH=\"\$PATH:/c/Program Files/Go/bin\""
else
	echo "Go: ▲ não encontrado"
fi

# Seção "Estado atual" do estado.md, até a próxima seção
echo "─── estado.md ───"
awk '/^## Estado atual/{f=1; next} /^## /{f=0} f' "$ESTADO" | sed '/^[[:space:]]*$/d'
