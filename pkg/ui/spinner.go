package ui

import (
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"
)

// IntervaloSpinner é o tempo entre quadros do spinner.
const IntervaloSpinner = 120 * time.Millisecond

var proximoIDSpinner int

// SpinnerTickMsg avança o spinner um quadro.
type SpinnerTickMsg struct{ id int }

// Spinner é um indicador de atividade com os quadros ◇ ◈ ◆ ◈. Sem animação
// (quadro fixo ◈) com NO_COLOR ou quando a saída não é um TTY.
type Spinner struct {
	rotulo  string
	quadro  int
	animado bool
	id      int
}

// ttyStdout informa se a saída padrão é um terminal.
func ttyStdout() bool { return term.IsTerminal(int(os.Stdout.Fd())) }

// NovoSpinner cria um spinner com o rótulo informado, detectando o TTY de stdout.
func NovoSpinner(rotulo string) Spinner { return NovoSpinnerCom(rotulo, ttyStdout) }

// NovoSpinnerCom cria um spinner com detecção de TTY injetável (útil em testes).
func NovoSpinnerCom(rotulo string, ehTTY func() bool) Spinner {
	proximoIDSpinner++
	return Spinner{
		rotulo:  rotulo,
		animado: ehTTY() && !Atual().SemCor,
		id:      proximoIDSpinner,
	}
}

// Animado informa se o spinner gira.
func (m Spinner) Animado() bool { return m.animado }

// DefinirRotulo troca o texto exibido ao lado do spinner.
func (m Spinner) DefinirRotulo(r string) Spinner { m.rotulo = r; return m }

func (m Spinner) tick() tea.Cmd {
	id := m.id
	return tea.Tick(IntervaloSpinner, func(time.Time) tea.Msg { return SpinnerTickMsg{id: id} })
}

// Init inicia a animação (nenhum comando quando parado).
func (m Spinner) Init() tea.Cmd {
	if !m.animado {
		return nil
	}
	return m.tick()
}

// Update avança o quadro nos ticks do próprio spinner.
func (m Spinner) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if t, ok := msg.(SpinnerTickMsg); ok && m.animado && t.id == m.id {
		m.quadro = (m.quadro + 1) % len(G().Spinner)
		return m, m.tick()
	}
	return m, nil
}

// View desenha o quadro atual e o rótulo.
func (m Spinner) View() string {
	s, g := S(), G()
	quadro := g.Andamento
	if m.animado {
		quadro = g.Spinner[m.quadro%len(g.Spinner)]
	}
	out := s.Tinta.Render(quadro)
	if m.rotulo != "" {
		out += " " + s.Tinta.Render(m.rotulo)
	}
	return out
}
