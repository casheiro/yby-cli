package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// ConfirmadoMsg é emitida quando o usuário confirma a ação.
type ConfirmadoMsg struct{ Nome string }

// CanceladoMsg é emitida quando o usuário cancela (esc ou ctrl+c).
type CanceladoMsg struct{ Nome string }

// larguraConfirmarPadrao é a largura do Quadro antes de receber o tamanho da janela.
const larguraConfirmarPadrao = 48

// Confirmar é o diálogo de confirmação. No modo destrutivo, ↵ só confirma depois de
// digitado o nome exato do recurso; no reversível, ↵ confirma e esc cancela.
type Confirmar struct {
	destrutivo bool
	acao       string // rótulo do Quadro, ex.: "excluir pod"
	nome       string // nome a digitar (destrutivo) ou assunto (reversível)
	pergunta   string // texto do modo reversível
	entrada    []rune
	largura    int
}

// NovoConfirmarDestrutivo cria a confirmação por nome: acao é o rótulo ("excluir pod")
// e nome é o que o usuário precisa digitar.
func NovoConfirmarDestrutivo(acao, nome string) Confirmar {
	return Confirmar{destrutivo: true, acao: acao, nome: nome, largura: larguraConfirmarPadrao}
}

// NovoConfirmarReversivel cria a confirmação simples: ↵ confirma, esc cancela.
func NovoConfirmarReversivel(acao, pergunta string) Confirmar {
	return Confirmar{acao: acao, pergunta: pergunta, largura: larguraConfirmarPadrao}
}

// Digitado devolve o texto já digitado no campo.
func (m Confirmar) Digitado() string { return string(m.entrada) }

// Liberado informa se ↵ confirmaria agora.
func (m Confirmar) Liberado() bool {
	return !m.destrutivo || string(m.entrada) == m.nome
}

// Init não agenda nada.
func (m Confirmar) Init() tea.Cmd { return nil }

// Update trata teclas e o tamanho da janela.
func (m Confirmar) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.largura = min(msg.Width, 72)
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc, tea.KeyCtrlC:
			return m, func() tea.Msg { return CanceladoMsg{Nome: m.nome} }
		case tea.KeyEnter:
			if m.Liberado() {
				return m, func() tea.Msg { return ConfirmadoMsg{Nome: m.nome} }
			}
		case tea.KeyBackspace:
			if n := len(m.entrada); n > 0 && m.destrutivo {
				m.entrada = m.entrada[:n-1]
			}
		case tea.KeyRunes, tea.KeySpace:
			if m.destrutivo {
				m.entrada = append(append([]rune{}, m.entrada...), msg.Runes...)
			}
		}
	}
	return m, nil
}

// View desenha o diálogo: no destrutivo, um Quadro com fio Erro.
func (m Confirmar) View() string {
	s, g := S(), G()
	if !m.destrutivo {
		corpo := m.pergunta + "\n" +
			s.TintaSuave.Render(RotuloTecla("↵")+" confirma · esc cancela")
		if Atual().ASCII {
			corpo = m.pergunta + "\n" + s.TintaSuave.Render("enter confirma - esc cancela")
		}
		return Quadro(m.acao, corpo, m.largura, QuadroFoco)
	}
	cursor := s.Selecionado.Render(" ")
	campo := s.Tinta.Render(g.Prompt+" ") + string(m.entrada) + cursor
	dica := "esc cancelar"
	if m.Liberado() {
		dica = RotuloTecla("↵") + " confirmar · esc cancelar"
		if Atual().ASCII {
			dica = "enter confirmar - esc cancelar"
		}
	}
	corpo := strings.Join([]string{
		"digite  " + s.Negrito.Render(m.nome),
		campo,
		s.TintaSuave.Render(dica),
	}, "\n")
	return Quadro(m.acao, corpo, m.largura, QuadroDestrutivo)
}
