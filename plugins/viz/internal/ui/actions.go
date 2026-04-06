package ui

import (
	"fmt"
	"strings"
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

// renderActionPrompt renderiza o prompt de confirmação/input da ação atual
func renderActionPrompt(mode ActionMode, resourceName, inputBuffer string) string {
	var sb strings.Builder

	switch mode {
	case ActionConfirmDelete:
		sb.WriteString(errorStyle.Render(fmt.Sprintf(
			"Deletar '%s'? Digite 'yes' para confirmar: %s_",
			resourceName, inputBuffer)))
	case ActionInputScale:
		sb.WriteString(filterStyle.Render(fmt.Sprintf(
			"Escalar '%s' para quantas réplicas? %s_",
			resourceName, inputBuffer)))
	case ActionConfirmRestart:
		sb.WriteString(filterStyle.Render(fmt.Sprintf(
			"Reiniciar '%s'? Digite 'yes' para confirmar: %s_",
			resourceName, inputBuffer)))
	}

	return sb.String()
}

// renderFeedback renderiza a mensagem de feedback após uma ação
func renderFeedback(message string) string {
	return strings.TrimSpace(runningStyle.Render(message))
}
