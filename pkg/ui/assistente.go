package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

const (
	// larguraAssistente é a largura padrão, em células, do assistente (rota e campos).
	larguraAssistente = msgLargura
	// linhasMoldura são as linhas da view fora dos campos: margem, rota (2), separadores
	// e legenda.
	linhasMoldura = 7
)

// PassoAssistente é uma etapa do assistente: um grupo do huh sob um marco da rota.
type PassoAssistente struct {
	// Marco é o rótulo do passo na rota. Passos seguidos com o mesmo marco dividem o marco
	// (ex.: uma pergunta que só aparece conforme a resposta anterior).
	Marco string
	// Campos são as perguntas do passo; um passo sem campos fica de fora do assistente.
	Campos []huh.Field
	// Ocultar, se definido, esconde o passo enquanto devolver true; o huh pula o grupo.
	Ocultar func() bool
}

// Assistente combina a ui.Rota com um único huh.Form, um grupo por passo: a rota mostra
// o marco do campo em foco, ⇧⇥ volta um campo (no primeiro campo do passo, volta um
// passo) e esc ou ctrl+c cancelam. As listas não têm filtro ("/"), para o esc sempre
// cancelar. Ao concluir ou cancelar, a View fica vazia e o
// modelo pede tea.Quit; quem o roda lê Concluido e Cancelado.
type Assistente struct {
	form    *huh.Form
	marcos  []string
	marcoDe map[huh.Field]int
	campos  []huh.Field
	largura int
	maximo  int
}

// NovoAssistente monta o assistente com os passos dados. largura é a máxima em células
// (<= 0 usa a das saídas de comando); num terminal mais estreito, ela acompanha a janela.
func NovoAssistente(largura int, passos ...PassoAssistente) *Assistente {
	if largura <= 0 {
		largura = larguraAssistente
	}
	a := &Assistente{marcoDe: map[huh.Field]int{}, largura: largura, maximo: largura}
	var grupos []*huh.Group
	for _, p := range passos {
		if len(p.Campos) == 0 {
			continue
		}
		if len(a.marcos) == 0 || a.marcos[len(a.marcos)-1] != p.Marco {
			a.marcos = append(a.marcos, p.Marco)
		}
		for _, c := range p.Campos {
			a.marcoDe[c] = len(a.marcos) - 1
			a.campos = append(a.campos, c)
		}
		g := huh.NewGroup(p.Campos...)
		if p.Ocultar != nil {
			g = g.WithHideFunc(p.Ocultar)
		}
		grupos = append(grupos, g)
	}
	if len(grupos) == 0 {
		return a
	}
	a.form = huh.NewForm(grupos...).
		WithTheme(HuhTheme()).
		WithKeyMap(teclasAssistente()).
		WithShowHelp(false).
		WithWidth(largura)
	return a
}

// teclasAssistente é o keymap do huh com esc também cancelando.
func teclasAssistente() *huh.KeyMap {
	k := huh.NewDefaultKeyMap()
	k.Quit = key.NewBinding(key.WithKeys("ctrl+c", "esc"))
	return k
}

// Vazio informa se não há nada a perguntar (nenhum passo com campos).
func (a *Assistente) Vazio() bool { return a.form == nil }

// Marcos devolve os rótulos da rota, na ordem.
func (a *Assistente) Marcos() []string { return a.marcos }

// Chaves devolve as chaves (huh Key) dos campos, na ordem dos passos.
func (a *Assistente) Chaves() []string {
	chaves := make([]string, len(a.campos))
	for i, c := range a.campos {
		chaves[i] = c.GetKey()
	}
	return chaves
}

// Etapa devolve o índice (base 0) do marco do campo em foco.
func (a *Assistente) Etapa() int {
	if a.form == nil {
		return 0
	}
	return a.marcoDe[a.form.GetFocusedField()]
}

// Concluido informa se o usuário passou por todos os passos.
func (a *Assistente) Concluido() bool {
	return a.form == nil || a.form.State == huh.StateCompleted
}

// Cancelado informa se o usuário saiu antes do fim (esc ou ctrl+c).
func (a *Assistente) Cancelado() bool {
	return a.form != nil && a.form.State == huh.StateAborted
}

// Init implementa tea.Model.
func (a *Assistente) Init() tea.Cmd {
	if a.form == nil {
		return tea.Quit
	}
	return a.form.Init()
}

// Update implementa tea.Model: repassa a mensagem ao formulário e pede tea.Quit quando ele
// termina, concluído ou cancelado.
func (a *Assistente) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if a.form == nil {
		return a, tea.Quit
	}
	// as listas do assistente não filtram: com o filtro aberto, o esc sairia do assistente
	// inteiro em vez de fechar o filtro (o formulário trata o cancelar antes do campo)
	if k, ok := msg.(tea.KeyMsg); ok && k.String() == "/" {
		if _, lista := a.form.GetFocusedField().(interface{ GetFiltering() bool }); lista {
			return a, nil
		}
	}
	if m, ok := msg.(tea.WindowSizeMsg); ok && m.Width > 0 {
		a.largura = min(a.maximo, m.Width)
		// altura fixa (a janela menos rota e legenda): o huh mede os grupos uma vez só,
		// e um texto que cresce depois (DescriptionFunc) sairia cortado
		a.form = a.form.WithWidth(a.largura).WithHeight(max(m.Height-linhasMoldura, 4))
	}
	modelo, cmd := a.form.Update(msg)
	a.form = modelo.(*huh.Form)
	if a.form.State != huh.StateNormal {
		return a, tea.Quit
	}
	return a, cmd
}

// View implementa tea.Model: linha em branco, rota, campos do passo e a legenda de teclas.
func (a *Assistente) View() string {
	if a.form == nil || a.form.State != huh.StateNormal {
		return ""
	}
	itens := []string{
		itemTecla(RotuloTecla("↵"), "continuar"),
		itemTecla(RotuloTecla("⇧⇥"), "voltar"),
		itemTecla("esc", "cancelar"),
	}
	legenda := strings.Join(itens, "  ")
	// numa janela estreita, as teclas do fim saem até a legenda caber
	for len(itens) > 1 && lipgloss.Width(legenda) > a.largura {
		itens = itens[:len(itens)-1]
		legenda = strings.Join(itens, "  ")
	}
	// o huh completa cada grupo até a altura do maior; aqui o passo ocupa só o que precisa
	linhas := strings.Split(a.form.View(), "\n")
	for i, l := range linhas {
		linhas[i] = strings.TrimRight(l, " ")
	}
	campos := strings.TrimRight(strings.Join(linhas, "\n"), "\n")
	return "\n" + Rota(a.marcos, a.Etapa(), a.largura) + "\n\n" + campos + "\n\n" + legenda + "\n"
}
