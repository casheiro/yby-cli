//go:build k8s

package main

import (
	"io"
	"os"

	"github.com/casheiro/yby-cli/pkg/ui"
	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"
)

// Confirmação do --fix, substituíveis nos testes. O Prompter do core fica no pacote cmd
// e não chega ao plugin: a pergunta usa o ui.Confirmar direto.
var (
	// entradaTerminal informa se a entrada padrão é um terminal; sem ele, o --fix não
	// tem a quem perguntar e exige --yes.
	entradaTerminal = func() bool { return term.IsTerminal(int(os.Stdin.Fd())) }
	// confirmar mostra a pergunta em w e devolve se a pessoa confirmou.
	confirmar = confirmarNoTerminal
)

// confirmarNoTerminal roda o ui.Confirmar reversível (↵ confirma, esc cancela) como
// programa, escrevendo em w.
func confirmarNoTerminal(w io.Writer, acao, pergunta string) (bool, error) {
	final, err := tea.NewProgram(confirmacao{dialogo: ui.NovoConfirmarReversivel(acao, pergunta)}, tea.WithOutput(w)).Run()
	if err != nil {
		return false, err
	}
	return final.(confirmacao).confirmado, nil
}

// confirmacao sai ao confirmar ou cancelar e apaga o quadro da tela.
type confirmacao struct {
	dialogo    ui.Confirmar
	confirmado bool
	fim        bool
}

func (m confirmacao) Init() tea.Cmd { return m.dialogo.Init() }

func (m confirmacao) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case ui.ConfirmadoMsg:
		m.confirmado, m.fim = true, true
		return m, tea.Quit
	case ui.CanceladoMsg:
		m.fim = true
		return m, tea.Quit
	}
	d, cmd := m.dialogo.Update(msg)
	m.dialogo = d.(ui.Confirmar)
	return m, cmd
}

func (m confirmacao) View() string {
	if m.fim {
		return ""
	}
	return m.dialogo.View()
}
