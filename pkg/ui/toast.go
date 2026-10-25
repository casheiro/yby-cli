package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// DuracaoToast é quanto tempo o Toast fica visível.
const DuracaoToast = 6 * time.Second

// TipoToast define o glifo e a cor do Toast.
type TipoToast int

const (
	// ToastOk indica sucesso (✓ em Ok).
	ToastOk TipoToast = iota
	// ToastAviso indica alerta (▲ em Aviso).
	ToastAviso
	// ToastErro indica falha (✕ em Erro).
	ToastErro
)

// ToastSomeMsg avisa que o tempo do Toast com o ID indicado acabou.
type ToastSomeMsg struct{ ID int }

// Agendador devolve um comando que entrega msg depois de d. O padrão usa tea.Tick;
// testes injetam um que entrega na hora.
type Agendador func(d time.Duration, msg tea.Msg) tea.Cmd

func agendadorPadrao(d time.Duration, msg tea.Msg) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg { return msg })
}

// Toast é a mensagem curta da linha de estado, que some sozinha em 6 s.
type Toast struct {
	tipo      TipoToast
	texto     string
	visivel   bool
	id        int
	duracao   time.Duration
	agendador Agendador
}

// NovoToast cria um Toast oculto com a duração padrão (6 s).
func NovoToast() Toast {
	return Toast{duracao: DuracaoToast, agendador: agendadorPadrao}
}

// ComAgendador troca o agendador do tempo (injeção para testes).
func (m Toast) ComAgendador(a Agendador) Toast { m.agendador = a; return m }

// ComDuracao troca o tempo de exibição.
func (m Toast) ComDuracao(d time.Duration) Toast { m.duracao = d; return m }

// Visivel informa se há mensagem na tela.
func (m Toast) Visivel() bool { return m.visivel }

// Mostrar exibe uma mensagem e devolve o comando que a esconde depois da duração.
// Uma nova mensagem substitui a anterior e cancela, na prática, o tempo dela.
func (m Toast) Mostrar(tipo TipoToast, texto string) (Toast, tea.Cmd) {
	m.tipo, m.texto, m.visivel = tipo, texto, true
	m.id++
	return m, m.agendador(m.duracao, ToastSomeMsg{ID: m.id})
}

// Init não faz nada: o Toast nasce oculto.
func (m Toast) Init() tea.Cmd { return nil }

// Update esconde o Toast quando chega o aviso do seu tempo atual.
func (m Toast) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if s, ok := msg.(ToastSomeMsg); ok && s.ID == m.id {
		m.visivel = false
	}
	return m, nil
}

// View desenha "✓ texto" (ou ✕/▲), ou nada quando oculto.
func (m Toast) View() string {
	if !m.visivel {
		return ""
	}
	s, g := S(), G()
	switch m.tipo {
	case ToastErro:
		return s.Erro.Render(g.Falha) + " " + s.Tinta.Render(m.texto)
	case ToastAviso:
		return s.Aviso.Render(g.Alerta) + " " + s.Tinta.Render(m.texto)
	}
	return s.Ok.Render(g.Feito) + " " + s.Tinta.Render(m.texto)
}
