package ui

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
)

// TeclasGlobais são as teclas válidas em toda TUI.
type TeclasGlobais struct {
	Ajuda       key.Binding // ? abre a tela de ajuda
	Voltar      key.Binding // esc fecha a camada de cima, limpa filtros ou cancela
	Sair        key.Binding // q sai (só no nível de cima)
	SairForcado key.Binding // ctrl+c sai de qualquer lugar
	Recarregar  key.Binding // r recarrega
}

// TeclasLista são as teclas das telas de lista (Viz, Cloud).
type TeclasLista struct {
	Cima, Baixo              key.Binding
	PaginaCima, PaginaBaixo  key.Binding
	Inicio, Fim              key.Binding
	AbaSeguinte, AbaAnterior key.Binding
	AbaNumero                key.Binding // 1 a 9 (o tratamento é por dígito)
	Buscar                   key.Binding
	Filtrar                  key.Binding
	Namespace                key.Binding
	Abrir                    key.Binding
	Acoes                    key.Binding
	Logs                     key.Binding
	Yaml                     key.Binding
	Eventos                  key.Binding
	Bard                     key.Binding
}

// TeclasTexto são as teclas das telas de texto (Bard, assistentes).
type TeclasTexto struct {
	Enviar        key.Binding
	NovaLinha     key.Binding
	Comandos      key.Binding // / no início da linha
	EditarUltima  key.Binding
	RolarCima     key.Binding
	RolarBaixo    key.Binding
	Interromper   key.Binding
	EtapaAnterior key.Binding
	Marcar        key.Binding
	MarcarTodos   key.Binding
}

// Teclas é o keymap padrão (seção 8 da especificação). Implementa help.KeyMap.
type Teclas struct {
	Global TeclasGlobais
	Lista  TeclasLista
	Texto  TeclasTexto
}

func tecla(teclas []string, rotulo, descricao string) key.Binding {
	return key.NewBinding(key.WithKeys(teclas...), key.WithHelp(rotulo, descricao))
}

// DefaultKeys devolve o keymap padrão.
func DefaultKeys() Teclas {
	return Teclas{
		Global: TeclasGlobais{
			Ajuda:       tecla([]string{"?"}, "?", "ajuda"),
			Voltar:      tecla([]string{"esc"}, "esc", "voltar"),
			Sair:        tecla([]string{"q"}, "q", "sair"),
			SairForcado: tecla([]string{"ctrl+c"}, "ctrl+c", "sair"),
			Recarregar:  tecla([]string{"r"}, "r", "recarregar"),
		},
		Lista: TeclasLista{
			Cima:        tecla([]string{"up", "k"}, "↑/k", "subir"),
			Baixo:       tecla([]string{"down", "j"}, "↓/j", "descer"),
			PaginaCima:  tecla([]string{"pgup"}, "pgup", "página acima"),
			PaginaBaixo: tecla([]string{"pgdown"}, "pgdn", "página abaixo"),
			Inicio:      tecla([]string{"g", "home"}, "g", "início"),
			Fim:         tecla([]string{"G", "end"}, "G", "fim"),
			AbaSeguinte: tecla([]string{"tab"}, "⇥", "próxima aba"),
			AbaAnterior: tecla([]string{"shift+tab"}, "⇧⇥", "aba anterior"),
			AbaNumero:   tecla([]string{"1", "2", "3", "4", "5", "6", "7", "8", "9"}, "1-9", "ir para aba"),
			Buscar:      tecla([]string{"/"}, "/", "buscar"),
			Filtrar:     tecla([]string{"f"}, "f", "filtrar"),
			Namespace:   tecla([]string{"n"}, "n", "namespace"),
			Abrir:       tecla([]string{"enter"}, "↵", "abrir"),
			Acoes:       tecla([]string{"x"}, "x", "ações"),
			Logs:        tecla([]string{"l"}, "l", "logs"),
			Yaml:        tecla([]string{"y"}, "y", "yaml"),
			Eventos:     tecla([]string{"e"}, "e", "eventos"),
			Bard:        tecla([]string{"b"}, "b", "perguntar ao Bard"),
		},
		Texto: TeclasTexto{
			Enviar:        tecla([]string{"enter"}, "↵", "enviar"),
			NovaLinha:     tecla([]string{"shift+enter", "ctrl+j"}, "⇧↵", "nova linha"),
			Comandos:      tecla([]string{"/"}, "/", "comandos"),
			EditarUltima:  tecla([]string{"up"}, "↑", "editar última"),
			RolarCima:     tecla([]string{"pgup"}, "pgup", "rolar acima"),
			RolarBaixo:    tecla([]string{"pgdown"}, "pgdn", "rolar abaixo"),
			Interromper:   tecla([]string{"esc"}, "esc", "interromper"),
			EtapaAnterior: tecla([]string{"shift+tab"}, "⇧⇥", "etapa anterior"),
			Marcar:        tecla([]string{"space"}, "␣", "marcar"),
			MarcarTodos:   tecla([]string{"a"}, "a", "marcar todos"),
		},
	}
}

// ShortHelp implementa help.KeyMap: o resumo mínimo exibido na legenda.
func (t Teclas) ShortHelp() []key.Binding {
	return []key.Binding{t.Global.Ajuda, t.Global.Sair}
}

// FullHelp implementa help.KeyMap: uma coluna por grupo (global, listas, texto).
func (t Teclas) FullHelp() [][]key.Binding {
	return [][]key.Binding{t.Global.Todas(), t.Lista.Todas(), t.Texto.Todas()}
}

// Todas devolve os bindings do grupo global em ordem de exibição.
func (g TeclasGlobais) Todas() []key.Binding {
	return []key.Binding{g.Ajuda, g.Voltar, g.Sair, g.SairForcado, g.Recarregar}
}

// Todas devolve os bindings do grupo de listas em ordem de exibição.
func (l TeclasLista) Todas() []key.Binding {
	return []key.Binding{l.Cima, l.Baixo, l.PaginaCima, l.PaginaBaixo, l.Inicio, l.Fim,
		l.AbaSeguinte, l.AbaAnterior, l.AbaNumero, l.Buscar, l.Filtrar, l.Namespace,
		l.Abrir, l.Acoes, l.Logs, l.Yaml, l.Eventos, l.Bard}
}

// Todas devolve os bindings do grupo de texto em ordem de exibição.
func (x TeclasTexto) Todas() []key.Binding {
	return []key.Binding{x.Enviar, x.NovaLinha, x.Comandos, x.EditarUltima, x.RolarCima,
		x.RolarBaixo, x.Interromper, x.EtapaAnterior, x.Marcar, x.MarcarTodos}
}

// NovoAjuda devolve um help.Model estilizado com a paleta atual, para a tela `?`.
// Chame-o na hora de renderizar (ou ao mudar tema), pois as cores são resolvidas no momento.
func NovoAjuda(largura int) help.Model {
	s := S()
	m := help.New()
	m.Width = largura
	m.ShortSeparator = "  "
	m.FullSeparator = "    "
	m.Ellipsis = G().Reticencias
	chave := s.Negrito
	m.Styles = help.Styles{
		Ellipsis:       s.TintaSuave,
		ShortKey:       chave,
		ShortDesc:      s.TintaSuave,
		ShortSeparator: s.TintaSuave,
		FullKey:        chave,
		FullDesc:       s.TintaSuave,
		FullSeparator:  s.TintaSuave,
	}
	return m
}
