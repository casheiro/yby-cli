package ui

import (
	yui "github.com/casheiro/yby-cli/pkg/ui"
)

// ActionMode controla o modo de ação do usuário
type ActionMode int

const (
	// ActionNone indica nenhuma ação em andamento
	ActionNone ActionMode = iota
	// ActionConfirmDelete aguarda confirmação para deletar recurso
	ActionConfirmDelete
	// ActionInputScale aguarda input do número de réplicas
	ActionInputScale
	// ActionConfirmRestart aguarda confirmação para restart
	ActionConfirmRestart
)

// renderActionPrompt renderiza, na linha de estado, a pergunta da ação atual
// com o que já foi digitado. O menu e a confirmação por nome são a T022.
func renderActionPrompt(mode ActionMode, kind, resourceName, inputBuffer string) string {
	s := yui.S()
	digitado := s.Negrito.Render(inputBuffer) + cursor()
	switch mode {
	case ActionConfirmDelete:
		return frase(yui.StatusFalha, "excluir "+kind+" "+resourceName+"? digite yes para confirmar: ") + digitado
	case ActionInputScale:
		return frase(yui.StatusAlerta, "escalar "+kind+" "+resourceName+" para quantas réplicas? ") + digitado
	case ActionConfirmRestart:
		return frase(yui.StatusAlerta, "reiniciar "+kind+" "+resourceName+"? digite yes para confirmar: ") + digitado
	}
	return ""
}
