package ui

import (
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
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
	// Detalhes do destrutivo (ComDetalhes): o diálogo completo da folha VizConfirmar.
	detalhado  bool
	resumo     string // linha do recurso, já com estilo ("worker em payments · 2 réplicas")
	explicacao string // o que acontece, quebrada na largura do quadro
	verbo      string // o botão de confirmar ("excluir")
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

// ComDetalhes completa o diálogo destrutivo como na folha VizConfirmar: o resumo do
// recurso (já com estilo) e a explicação do que acontece acima do pedido "Digite
// <nome> para confirmar", quantas letras faltam ao lado do campo e os botões, com o
// verbo no de confirmar ("excluir"). Resumo e explicação vazios ficam de fora.
func (m Confirmar) ComDetalhes(resumo, explicacao, verbo string) Confirmar {
	m.detalhado, m.resumo, m.explicacao, m.verbo = true, resumo, explicacao, verbo
	return m
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
	if m.destrutivo && m.detalhado {
		return m.viewDetalhada()
	}
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

// viewDetalhada é o destrutivo com resumo, explicação, contagem e botões. Com a
// explicação (o diálogo completo), o campo fica num quadro de fio, como na prancha;
// sem ela (quem chama tira a explicação quando falta altura), o campo é uma linha só:
//
//	┌ EXCLUIR DEPLOYMENT ───────────────────────┐
//	│ worker em payments · 2 réplicas · 2 pods  │
//	│ Os pods param e o Deployment some.        │
//	│                                           │
//	│ Digite worker para confirmar              │
//	│ ┌───────────────────────────────────────┐ │
//	│ │ › work█               faltam 2 letras │ │
//	│ └───────────────────────────────────────┘ │
//	│                                           │
//	│ [ esc cancelar ]   [ ↵ excluir ]          │
//	└───────────────────────────────────────────┘
func (m Confirmar) viewDetalhada() string {
	s, g := S(), G()
	interno := max(1, max(m.largura, larguraMinQuadro)-4)
	var linhas []string
	if m.resumo != "" {
		linhas = append(linhas, m.resumo)
	}
	if m.explicacao != "" {
		for _, l := range strings.Split(ansi.Wrap(m.explicacao, interno, ""), "\n") {
			linhas = append(linhas, s.TintaSuave.Render(l))
		}
	}
	if len(linhas) > 0 {
		linhas = append(linhas, "")
	}
	linhas = append(linhas, s.TintaSuave.Render("Digite ")+s.Negrito.Render(m.nome)+s.TintaSuave.Render(" para confirmar"))
	campo := func(largura int) string {
		return linhaEsqDir(s.Erro.Render(g.Prompt)+" "+s.Tinta.Render(string(m.entrada))+s.Selecionado.Render(" "), m.falta(), largura)
	}
	if m.explicacao != "" && interno >= larguraMinQuadro {
		linhas = append(linhas, strings.Split(Quadro("", campo(interno-4), interno, QuadroNormal), "\n")...)
	} else {
		linhas = append(linhas, campo(interno))
	}
	linhas = append(linhas, "", m.botoes())
	return Quadro(m.acao, strings.Join(linhas, "\n"), m.largura, QuadroDestrutivo)
}

// falta diz, ao lado do campo, quanto falta para o nome conferir.
func (m Confirmar) falta() string {
	s, g := S(), G()
	digitado := string(m.entrada)
	switch {
	case digitado == m.nome:
		return s.Ok.Render(g.Feito) + " " + s.TintaSuave.Render("confere")
	case !strings.HasPrefix(m.nome, digitado):
		return s.Aviso.Render(g.Alerta) + " " + s.TintaSuave.Render("não confere")
	}
	n := len([]rune(m.nome)) - len(m.entrada)
	if n == 1 {
		return s.TintaSuave.Render("falta 1 letra")
	}
	return s.TintaSuave.Render("faltam " + strconv.Itoa(n) + " letras")
}

// botoes são "[ esc cancelar ]   [ ↵ excluir ]"; o de confirmar fica apagado até o
// nome conferir.
func (m Confirmar) botoes() string {
	s := S()
	verbo := m.verbo
	if verbo == "" {
		verbo = "confirmar"
	}
	confirmar := s.Fio.Render("[ " + RotuloTecla("↵") + " " + verbo + " ]")
	if m.Liberado() {
		confirmar = s.Negrito.Render("[ " + RotuloTecla("↵") + " " + verbo + " ]")
	}
	return s.TintaSuave.Render("[ esc cancelar ]") + "   " + confirmar
}
