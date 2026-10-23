package ui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// TipoEstado distingue os três estados de uma lista ou painel.
type TipoEstado int

const (
	// EstadoVazio indica que não há nada a mostrar (○).
	EstadoVazio TipoEstado = iota
	// EstadoCarregando indica leitura em andamento (spinner ◈).
	EstadoCarregando
	// EstadoErro indica falha (✕ em Erro).
	EstadoErro
)

// Estado é o modelo dos estados vazio, carregando e erro.
type Estado struct {
	tipo    TipoEstado
	texto   string
	dica    string
	spinner Spinner
}

// NovoEstadoVazio cria o estado vazio: "○ nenhum pod com esse filtro · esc limpa".
func NovoEstadoVazio(texto, dica string) Estado {
	return Estado{tipo: EstadoVazio, texto: texto, dica: dica}
}

// NovoEstadoCarregando cria o estado de carregamento: "◈ lendo pods de payments".
func NovoEstadoCarregando(texto string) Estado {
	return Estado{tipo: EstadoCarregando, texto: texto, spinner: NovoSpinner(texto)}
}

// NovoEstadoCarregandoCom é como NovoEstadoCarregando, com a detecção de TTY injetável.
func NovoEstadoCarregandoCom(texto string, ehTTY func() bool) Estado {
	return Estado{tipo: EstadoCarregando, texto: texto, spinner: NovoSpinnerCom(texto, ehTTY)}
}

// NovoEstadoErro cria o estado de erro: "✕ sem acesso ao cluster · r tenta de novo".
func NovoEstadoErro(texto, dica string) Estado {
	return Estado{tipo: EstadoErro, texto: texto, dica: dica}
}

// Tipo devolve o tipo do estado.
func (m Estado) Tipo() TipoEstado { return m.tipo }

// Init inicia o spinner no estado carregando.
func (m Estado) Init() tea.Cmd {
	if m.tipo == EstadoCarregando {
		return m.spinner.Init()
	}
	return nil
}

// Update repassa os ticks ao spinner.
func (m Estado) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.tipo != EstadoCarregando {
		return m, nil
	}
	novo, cmd := m.spinner.Update(msg)
	m.spinner = novo.(Spinner)
	return m, cmd
}

// View desenha o estado numa linha.
func (m Estado) View() string {
	s, g := S(), G()
	sufixo := ""
	if m.dica != "" {
		sufixo = s.TintaSuave.Render(" · " + m.dica)
	}
	switch m.tipo {
	case EstadoCarregando:
		return m.spinner.View()
	case EstadoErro:
		return s.Erro.Render(g.Falha) + " " + m.texto + sufixo
	}
	return s.TintaSuave.Render(g.Parado+" "+m.texto) + sufixo
}
