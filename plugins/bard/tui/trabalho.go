package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/casheiro/yby-cli/v5/pkg/ui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Tela "trabalhando": enquanto o Bard responde, o bloco dele mostra as etapas com
// resultado e tempo e a resposta chegando; a linha de estado diz o que está em
// andamento e há quanto tempo.

// intervaloRelogio é de quanto em quanto tempo os tempos ao vivo andam sem eventos
// (variável para os testes).
var intervaloRelogio = time.Second

// larguraRotuloEtapa alinha as notas das etapas numa coluna, como na prancha (o glifo
// ocupa 2 células, o rótulo 23 e o respiro 3: a nota começa na célula 28).
const larguraRotuloEtapa = 23

// tiqueMsg avisa que passou um intervalo sem eventos da rodada.
type tiqueMsg struct{ rodada int }

// esperarEvento lê o próximo evento do canal; sem nenhum em intervaloRelogio, devolve um
// tiqueMsg para os tempos da tela andarem. Um só comando fica pendente por vez.
func esperarEvento(ch <-chan tea.Msg, rodada int) tea.Cmd {
	return func() tea.Msg {
		t := time.NewTimer(intervaloRelogio)
		defer t.Stop()
		select {
		case msg, ok := <-ch:
			if !ok {
				return nil
			}
			return msg
		case <-t.C:
			return tiqueMsg{rodada: rodada}
		}
	}
}

// cronometro formata o tempo corrido da rodada em segundos inteiros ("9 s"), que é o
// que um relógio de um segundo consegue mostrar; acima de 10 s segue FormatarDuracao.
func cronometro(d time.Duration) string {
	d = max(d.Truncate(time.Second), 0)
	if d < 10*time.Second {
		return fmt.Sprintf("%d s", int(d/time.Second))
	}
	return ui.FormatarDuracao(d)
}

// decorrido é o tempo desde o início da rodada, no último instante amostrado.
func (m Model) decorrido() time.Duration {
	if m.inicioRodada.IsZero() {
		return 0
	}
	return m.instante.Sub(m.inicioRodada)
}

// emAndamento descreve a etapa em andamento para a linha de estado ("gerando resposta").
func (m Model) emAndamento() string {
	if m.interrompendo {
		return "interrompendo"
	}
	for i := len(m.etapas) - 1; i >= 0; i-- {
		e := m.etapas[i]
		if e.Estado != ui.EtapaAtual {
			continue
		}
		switch e.Etapa {
		case EtapaContexto:
			return "buscando contexto"
		case EtapaIntencao:
			return "entendendo o pedido"
		case EtapaFerramenta:
			return "rodando " + e.Rotulo
		default:
			return "gerando resposta"
		}
	}
	return "trabalhando"
}

// estadoTrabalho é a frase da linha de estado durante a resposta: "◈ gerando resposta ·
// 9 s", com o ui.Spinner da rodada (◇ ◈ ◆ ◈ num terminal com cor; ◈ fixo sem).
func (m Model) estadoTrabalho() string {
	texto := m.emAndamento()
	if !m.inicioRodada.IsZero() {
		texto += " · " + cronometro(m.decorrido())
	}
	return m.spinner.DefinirRotulo(texto).View()
}

// resumoRodada é o lado direito da linha de estado durante a resposta: as ferramentas
// da rodada ("1 ferramenta"); sem nenhuma, o resumo da sessão.
func (m Model) resumoRodada() string {
	n := 0
	for _, e := range m.etapas {
		if e.Etapa == EtapaFerramenta {
			n++
		}
	}
	if n == 0 {
		return m.resumoSessao()
	}
	return plural(n, "ferramenta", "ferramentas")
}

// registrarEtapa guarda o estado mais recente de cada etapa da rodada e o instante em
// que ela começou, para o tempo ao vivo da etapa em andamento.
func (m *Model) registrarEtapa(msg EtapaMsg) {
	for i := range m.etapas {
		if m.etapas[i].Etapa == msg.Etapa && m.etapas[i].Rotulo == msg.Rotulo {
			m.etapas[i] = msg
			return
		}
	}
	m.etapas = append(m.etapas, msg)
	m.iniciosEtapas = append(m.iniciosEtapas, m.instante)
}

// etapasVista monta as etapas da rodada com ui.Etapas, na largura dada: as notas em
// coluna, o tempo da etapa em andamento contado ao vivo e "↵ ver" na ferramenta cuja
// saída o ↵ abre.
func (m Model) etapasVista(largura int) string {
	verSaida := -1
	if f, ok := m.saidaDisponivel(); ok {
		for i, e := range m.etapas {
			if e.Etapa == EtapaFerramenta && e.Rotulo == f.rotulo && e.Estado == ui.EtapaFeita {
				verSaida = i
			}
		}
	}
	coluna := larguraRotuloEtapa
	for _, e := range m.etapas {
		coluna = max(coluna, lipgloss.Width(e.Rotulo))
	}
	lista := make([]ui.Etapa, 0, len(m.etapas))
	for i, e := range m.etapas {
		d := e.Duracao
		if e.Estado == ui.EtapaAtual && i < len(m.iniciosEtapas) {
			d = m.instante.Sub(m.iniciosEtapas[i])
		}
		nota := e.Nota
		if i == verSaida {
			ver := ui.RotuloTecla("↵") + " ver"
			if nota != "" {
				ver = nota + " · " + ver
			}
			nota = ver
		}
		rotulo := e.Rotulo
		if nota != "" || d > 0 {
			rotulo += strings.Repeat(" ", coluna-lipgloss.Width(rotulo))
		}
		lista = append(lista, ui.Etapa{Rotulo: rotulo, Estado: e.Estado, Duracao: d, Nota: nota})
	}
	return ui.Etapas(lista, largura)
}

// renderizarParcial é o bloco do Bard durante a resposta: "◈ BARD · trabalhando · 9 s",
// as etapas e, abaixo, a resposta que já chegou, com o cursor no fim.
func (m *Model) renderizarParcial() string {
	s, g := ui.S(), ui.G()
	largura := m.viewport.Width - len(recuo)
	cabecalho := "BARD · trabalhando"
	if !m.inicioRodada.IsZero() {
		cabecalho += " · " + cronometro(m.decorrido())
	}
	var b strings.Builder
	b.WriteString(s.Tinta.Render(g.Andamento) + " " + s.Rotulo.Render(cabecalho) + "\n")
	if len(m.etapas) > 0 {
		escreverRecuado(&b, strings.Split(m.etapasVista(largura), "\n"), nil)
	}
	if m.parcial != "" {
		if len(m.etapas) > 0 {
			b.WriteString("\n")
		}
		linhas := m.parcialRenderizado(largura)
		// O cursor ocupa uma célula depois do texto; na linha que já tem a largura toda
		// ele seria cortado na borda e desce para uma linha própria.
		cursor := s.Selecionado.Render(" ")
		if ultima := linhas[len(linhas)-1]; lipgloss.Width(ultima) >= largura {
			linhas = append(linhas, cursor)
		} else {
			linhas[len(linhas)-1] = ultima + cursor
		}
		escreverRecuado(&b, linhas, nil)
	}
	return b.String()
}

// parcialRenderizado passa a resposta em andamento pelo markdown, como a final, e
// guarda o resultado: o relógio e as etapas redesenham o bloco sem renderizar de novo
// o mesmo texto.
func (m *Model) parcialRenderizado(largura int) []string {
	c := &m.parcialCache
	if c.linhas == nil || c.texto != m.parcial || c.largura != largura {
		c.texto, c.largura, c.linhas = m.parcial, largura, m.linhasResposta(m.parcial, largura, -1)
	}
	return append([]string(nil), c.linhas...)
}

// cacheParcial guarda a última renderização da resposta em andamento.
type cacheParcial struct {
	texto   string
	largura int
	linhas  []string
}
