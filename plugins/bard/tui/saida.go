package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/casheiro/yby-cli/v5/pkg/ui"
	"github.com/casheiro/yby-cli/v5/pkg/ui/uimarkdown"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// Teclas desta tela que o keymap da carta não tem: ↵ abre a saída da ferramenta (no
// campo vazio) e ctrl+y copia o comando ou a saída, como dizem os quadros "COMANDO ·
// ctrl+y copia" e "SAÍDA · ctrl+y copia". A prancha mostra "c copia", mas no Bard o
// teclado é do campo: um c solto comeria a primeira letra de "como...". ctrl+y não
// escreve nada no campo e chega igual em todo terminal (no Windows também).
var teclaCopiar = key.NewBinding(key.WithKeys("ctrl+y"))

// rotuloCopiar é a tecla da cópia como aparece nos quadros e na legenda.
const rotuloCopiar = "ctrl+y"

// linhasRoda é quanto a roda do mouse rola a saída, como no viewport.
const linhasRoda = 3

// duracaoAviso é quanto o aviso da cópia fica na linha de estado, como o Toast da
// carta (variável para os testes).
var duracaoAviso = 6 * time.Second

// teclaVerEtapa é o ↵ da legenda durante a resposta ("↵ ver etapa").
func teclaVerEtapa() key.Binding {
	return key.NewBinding(key.WithKeys("enter"), key.WithHelp(ui.RotuloTecla("↵"), "ver etapa"))
}

// teclaSelecionar é o shift+arraste da legenda. O Bard pega o mouse por causa da roda, e
// com ele o arraste simples deixa de selecionar texto; com o shift, o terminal seleciona
// mesmo assim (no iTerm2 do Mac, com a option). A tecla é do terminal, não do Bard: a
// combinação só serve à legenda e nunca casa com uma tecla que chega.
func teclaSelecionar() key.Binding {
	return key.NewBinding(key.WithKeys("shift+arraste"), key.WithHelp("shift+arraste", "selecionar"))
}

// teclaCopiarSaida é o ctrl+y da legenda da saída aberta.
func teclaCopiarSaida() key.Binding {
	return key.NewBinding(key.WithKeys("ctrl+y"), key.WithHelp(rotuloCopiar, "copiar"))
}

// vistaSaida é a camada que mostra a saída completa de uma ferramenta por cima da
// conversa (↵ no campo vazio); esc fecha.
type vistaSaida struct {
	ferramenta ferramentaUsada
	texto      string   // saída como veio, para copiar
	linhas     []string // saída quebrada na largura do quadro
	largura    int      // largura em que linhas foi quebrada
	topo       int      // primeira linha visível
}

// copiadoMsg traz o resultado de uma cópia para a área de transferência.
type copiadoMsg struct {
	oque string // "comando" ou "saída"
	err  error
}

// avisoEstado é uma frase passageira da linha de estado (o resultado de uma cópia); some
// em duracaoAviso ou na próxima tecla.
type avisoEstado struct {
	ok    bool
	texto string
	id    int // identifica o aviso, para o prazo de um não apagar o seguinte
}

// limparAvisoMsg é o fim do prazo do aviso id.
type limparAvisoMsg struct{ id int }

// mostrarAviso põe o aviso na linha de estado e agenda o fim do prazo dele.
func (m *Model) mostrarAviso(a *avisoEstado) tea.Cmd {
	m.avisos++
	a.id = m.avisos
	m.aviso = a
	id := a.id
	return tea.Tick(duracaoAviso, func(time.Time) tea.Msg { return limparAvisoMsg{id: id} })
}

// copiarOSC52 manda o texto para a área de transferência do terminal pela sequência
// OSC 52 (ui.CopiarOSC52, a mesma do Viz).
func copiarOSC52(texto string) error {
	return ui.CopiarOSC52(texto)
}

// escreverOSC52 é a escrita da sequência, com o destino e o ambiente injetáveis.
var escreverOSC52 = ui.EscreverOSC52

// copiarTexto devolve o comando que copia o texto e avisa o resultado.
func (m Model) copiarTexto(texto, oque string) tea.Cmd {
	copiar := m.copiar
	return func() tea.Msg { return copiadoMsg{oque: oque, err: copiar(texto)} }
}

// avisoCopia é a frase da linha de estado depois de uma cópia. O OSC 52 não tem
// resposta: o terminal que não aceita a sequência a descarta calado (o conhost antigo
// do Windows, o tmux sem set-clipboard). Por isso a frase diz que o texto foi enviado,
// não que foi copiado.
func avisoCopia(msg copiadoMsg) *avisoEstado {
	if msg.err != nil {
		return &avisoEstado{texto: "não deu para copiar " + artigo(msg.oque) + msg.oque}
	}
	return &avisoEstado{ok: true, texto: msg.oque + " enviad" + genero(msg.oque) + " à área de transferência"}
}

func artigo(oque string) string { return genero(oque) + " " }

// genero devolve a vogal do gênero: "o comando", "a saída".
func genero(oque string) string {
	if strings.HasSuffix(oque, "a") {
		return "a"
	}
	return "o"
}

// ultimoComando devolve o último bloco de shell da resposta mais recente, o que o
// ctrl+y copia, e o índice dele entre os comandos da resposta; vazio e -1 se a conversa
// não termina numa resposta com comando.
func (m Model) ultimoComando() (string, int) {
	i := m.ultimaResposta()
	if i < 0 || m.messages[i].role != "assistant" {
		return "", -1
	}
	comandos := uimarkdown.Comandos(m.messages[i].content)
	if len(comandos) == 0 {
		return "", -1
	}
	return comandos[len(comandos)-1], len(comandos) - 1
}

// podeCopiarComando diz se o ctrl+y copia um comando: fora da resposta, com a conversa
// terminando numa resposta com comando. O campo pode ter texto: o ctrl+y não escreve.
func (m Model) podeCopiarComando() bool {
	if m.state != stateIdle {
		return false
	}
	c, _ := m.ultimoComando()
	return c != ""
}

// mensagemComCopia é a mensagem cujo último comando leva "ctrl+y copia" no quadro: a
// que o ctrl+y copia. -1 durante a resposta ou sem comando para copiar.
func (m Model) mensagemComCopia() int {
	if !m.podeCopiarComando() {
		return -1
	}
	return m.ultimaResposta()
}

// ultimaResposta é o índice da última mensagem do Bard (resposta ou erro), se a conversa
// termina nela; as notas do fim não contam. -1 se a última fala é do usuário.
func (m Model) ultimaResposta() int {
	for i := len(m.messages) - 1; i >= 0; i-- {
		switch m.messages[i].role {
		case "nota":
			continue
		case "assistant", "error":
			return i
		}
		return -1
	}
	return -1
}

// saidaDisponivel devolve a ferramenta cuja saída o ↵ abre: durante a resposta, a da
// rodada em andamento; fora dela, a da última resposta. Só com o campo vazio, porque com
// texto o ↵ envia.
func (m Model) saidaDisponivel() (ferramentaUsada, bool) {
	if m.saida != nil || !m.campoVazio() {
		return ferramentaUsada{}, false
	}
	var lista []ferramentaUsada
	if m.state == stateStreaming {
		lista = m.ferramentasDaRodada()
	} else if i := m.ultimaResposta(); i >= 0 {
		lista = m.messages[i].ferramentas
	}
	if i := ultimaComSaida(lista); i >= 0 {
		return lista[i], true
	}
	return ferramentaUsada{}, false
}

// abrirSaida abre a camada com a saída da ferramenta, se houver uma.
func (m *Model) abrirSaida() bool {
	f, ok := m.saidaDisponivel()
	if !ok {
		return false
	}
	m.saida = &vistaSaida{ferramenta: f, texto: f.saida}
	m.ajustarSaida()
	return true
}

// alturaSaida são as linhas do quadro que mostram a saída: o corpo menos a linha da
// ferramenta e as duas bordas.
func (m Model) alturaSaida() int {
	return max(m.viewport.Height-3, 1)
}

// ajustarSaida quebra a saída na largura atual e mantém o topo dentro do texto.
func (m *Model) ajustarSaida() {
	v := m.saida
	if v == nil {
		return
	}
	interno := max(m.viewport.Width-4, 1) // borda e respiro do quadro
	if v.linhas == nil || v.largura != interno {
		v.linhas, v.largura = quebrarSaida(v.texto, interno), interno
	}
	v.topo = min(max(v.topo, 0), max(len(v.linhas)-m.alturaSaida(), 0))
}

// quebrarSaida tira o ANSI e as tabulações da saída e quebra as linhas longas sem
// juntar espaços, para as colunas da saída do kubectl seguirem alinhadas.
func quebrarSaida(texto string, largura int) []string {
	texto = strings.ReplaceAll(ansi.Strip(texto), "\r\n", "\n")
	var linhas []string
	for _, l := range strings.Split(strings.TrimRight(texto, "\n"), "\n") {
		l = strings.TrimRight(strings.ReplaceAll(l, "\t", "    "), " ")
		linhas = append(linhas, strings.Split(ansi.Hardwrap(l, largura, true), "\n")...)
	}
	return linhas
}

// rolarSaida move a saída aberta n linhas (negativo sobe).
func (m *Model) rolarSaida(n int) {
	m.saida.topo += n
	m.ajustarSaida()
}

// teclarSaida trata as teclas com a saída aberta: esc fecha, pgup e pgdn rolam e ctrl+y
// copia a saída inteira. O resto é ignorado até a camada fechar.
func (m Model) teclarSaida(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := ui.DefaultKeys()
	switch {
	case msg.Type == tea.KeyEsc:
		m.saida = nil
		m.updateViewport()
	case key.Matches(msg, k.Texto.RolarCima):
		m.rolarSaida(-m.alturaSaida())
	case key.Matches(msg, k.Texto.RolarBaixo):
		m.rolarSaida(m.alturaSaida())
	case key.Matches(msg, teclaCopiar):
		return m, m.copiarTexto(m.saida.texto, "saída")
	}
	return m, nil
}

// corpoSaida desenha a camada: a linha da ferramenta e o quadro "SAÍDA · ctrl+y copia" com
// as linhas visíveis, na altura do corpo.
func (m Model) corpoSaida() string {
	v := m.saida
	h := m.alturaSaida()
	fim := min(v.topo+h, len(v.linhas))
	visiveis := append([]string(nil), v.linhas[v.topo:fim]...)
	for len(visiveis) < h {
		visiveis = append(visiveis, "")
	}
	quadro := ui.QuadroComTecla("saída", rotuloCopiar, "copia", strings.Join(visiveis, "\n"), m.viewport.Width, ui.QuadroFoco)
	return linhaFerramenta(v.ferramenta) + "\n" + quadro
}

// posicaoSaida é o lado direito da linha de estado com a saída aberta.
func (m Model) posicaoSaida() string {
	v := m.saida
	total := len(v.linhas)
	fim := min(v.topo+m.alturaSaida(), total)
	if v.topo == 0 && fim == total {
		return plural(total, "linha", "linhas")
	}
	return fmt.Sprintf("linhas %d a %d de %d", v.topo+1, fim, total)
}

// teclasSaida é a legenda com a saída aberta.
func (m Model) teclasSaida() []key.Binding {
	k := ui.DefaultKeys()
	rolar := k.Texto.RolarCima
	rolar.SetHelp(rolar.Help().Key, "rolar")
	rolar.SetEnabled(len(m.saida.linhas) > m.alturaSaida())
	return []key.Binding{k.Global.Voltar, rolar, teclaCopiarSaida(), teclaSelecionar()}
}
