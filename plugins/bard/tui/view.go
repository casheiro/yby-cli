package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/casheiro/yby-cli/pkg/ui"
	"github.com/casheiro/yby-cli/pkg/ui/uimarkdown"
	"github.com/charmbracelet/bubbles/key"
)

// Textos fixos da tela, como na prancha "conversa".
const (
	rotuloInstrumento    = "instr. 03 · orientação"
	textoPlaceholder     = "Pergunte sobre o cluster, ou / para comandos"
	textoPlaceholderResp = "a próxima pergunta pode ser escrita enquanto o Bard responde"
	recuo                = "  " // recuo do texto das mensagens sob o autor
)

// View renderiza a TUI na moldura de texto da carta: cabeçalho com cluster,
// namespace e provider, a conversa, o campo, a linha de estado e as teclas.
func (m Model) View() string {
	if m.width == 0 {
		return "Carregando..."
	}
	return ui.MolduraTexto(m.width, m.height, ui.PartesTexto{
		Instrumento: "bard",
		Rotulo:      rotuloInstrumento,
		Contexto:    m.contexto(),
		Corpo:       m.corpo(),
		Campo:       m.campo(),
		Mensagem:    m.mensagemEstado(),
		Direita:     m.resumoSessao(),
		Sair:        ui.DefaultKeys().Global.SairForcado,
		Teclas:      m.teclas(),
	})
}

// contexto é o lado direito do cabeçalho: "staging ▸ default   claude-cli".
func (m Model) contexto() string {
	var caminho []string
	for _, seg := range []string{m.config.Cluster, m.config.Namespace} {
		if seg != "" {
			caminho = append(caminho, seg)
		}
	}
	var partes []string
	if len(caminho) > 0 {
		partes = append(partes, ui.CaminhoContexto(caminho...))
	}
	if m.config.AIModel != "" {
		partes = append(partes, ui.S().TintaSuave.Render(m.config.AIModel))
	}
	return strings.Join(partes, "   ")
}

// corpo é a conversa na janela. Rolada para cima com mensagens novas abaixo, as
// duas últimas linhas dão lugar a uma linha em branco e ao aviso "↓ 2 mensagens novas".
// Nada se perde: o pgdn anda só as linhas que apareceram (ver teclar), e
// mensagensNovas conta também o que começa nas linhas cobertas.
func (m Model) corpo() string {
	vista := m.viewport.View()
	n := m.mensagensNovas()
	if n == 0 {
		return vista
	}
	linhas := strings.Split(vista, "\n")
	linhas[len(linhas)-1] = ui.AvisoRolagem(ui.SetaAbaixo, plural(n, "mensagem nova", "mensagens novas"))
	if m.linhasAviso() == 2 && len(linhas) > 1 {
		linhas[len(linhas)-2] = ""
	}
	return strings.Join(linhas, "\n")
}

// campo devolve as linhas do campo de texto. Vazio, mostra o cursor e a dica no
// lugar do texto; durante a resposta a dica, apagada como o prompt, avisa que a
// próxima pergunta já pode ser escrita (como na prancha "trabalhando").
func (m Model) campo() string {
	s, g := ui.S(), ui.G()
	var linhas []string
	switch {
	case m.textarea.Value() == "" && m.state == stateStreaming:
		linhas = []string{s.TintaSuave.Render(g.Prompt + " " + textoPlaceholderResp)}
	case m.textarea.Value() == "":
		cursor := m.textarea.Cursor
		cursor.SetChar(" ")
		linhas = []string{s.Ok.Render(g.Prompt) + " " + cursor.View() + s.TintaSuave.Render(textoPlaceholder)}
	default:
		linhas = strings.Split(m.textarea.View(), "\n")
	}
	// A moldura descarta linhas vazias do fim; um espaço mantém a altura do campo.
	altura := m.textarea.Height()
	for len(linhas) < altura {
		linhas = append(linhas, " ")
	}
	return strings.Join(linhas[:altura], "\n")
}

// mensagemEstado é a frase da linha de estado: o que está em andamento ou como
// terminou a última rodada.
func (m Model) mensagemEstado() string {
	s, g := ui.S(), ui.G()
	if m.state == stateStreaming {
		texto := "trabalhando"
		switch {
		case m.interrompendo:
			texto = "interrompendo"
		case m.etapaAtual() != "":
			texto = m.etapaAtual()
		}
		return s.Tinta.Render(g.Andamento + " " + texto)
	}
	f := m.ultimoFim
	switch {
	case f == nil:
		return ""
	case f.Interrompida:
		return s.Aviso.Render(g.Alerta) + s.Tinta.Render(" resposta interrompida")
	case f.Err != nil:
		return s.Erro.Render(g.Falha) + s.Tinta.Render(" a resposta falhou")
	default:
		return s.Ok.Render(g.Feito) + s.Tinta.Render(" resposta em "+ui.FormatarDuracao(f.Duracao))
	}
}

// resumoSessao é o lado direito da linha de estado: "sessão de hoje · 2 mensagens".
func (m Model) resumoSessao() string {
	n := 0
	for _, msg := range m.messages {
		if msg.role == "user" || msg.role == "assistant" {
			n++
		}
	}
	mensagens := plural(n, "mensagem", "mensagens")
	if n == 0 {
		mensagens = "nenhuma mensagem"
	}
	inicio, err := time.ParseInLocation("20060102-150405", m.config.SessionID, time.Local)
	if err != nil {
		return mensagens
	}
	sessao := "sessão de " + inicio.Format("02/01")
	if hoje := m.agora(); inicio.Year() == hoje.Year() && inicio.YearDay() == hoje.YearDay() {
		sessao = "sessão de hoje"
	}
	return sessao + " · " + mensagens
}

// teclas devolve a legenda da tela, na ordem da prancha. Só aparecem as teclas que já
// funcionam: pgup quando a conversa passa da altura do histórico, ↑ quando há uma
// pergunta para editar e o campo está vazio; / e ? esperam a T015 e até lá ficam
// desligadas.
func (m Model) teclas() []key.Binding {
	k := ui.DefaultKeys()
	rolar := k.Texto.RolarCima
	rolar.SetHelp(rolar.Help().Key, "rolar")
	if m.viewport.TotalLineCount() <= m.viewport.Height {
		rolar.SetEnabled(false)
	}
	editar := k.Texto.EditarUltima
	editar.SetEnabled(m.podeEditarUltima())
	pendentes := []key.Binding{k.Texto.Comandos, k.Global.Ajuda}
	for i := range pendentes {
		pendentes[i].SetEnabled(false)
	}
	comandos, ajuda := pendentes[0], pendentes[1]
	if m.state == stateStreaming {
		return []key.Binding{k.Texto.Interromper, rolar, ajuda}
	}
	// A prancha mostra ⇧↵, mas o Bubble Tea v1 recebe o shift+enter como ↵ e a
	// pergunta iria pela metade; a legenda mostra a tecla que chega (ver novaLinha).
	quebra := k.Texto.NovaLinha
	quebra.SetHelp(ui.RotuloTecla("ctrl+j"), quebra.Help().Desc)
	return []key.Binding{k.Texto.Enviar, quebra, comandos, editar, rolar, ajuda}
}

// renderizarMensagem devolve o bloco de uma mensagem, terminado por uma linha em branco.
func (m Model) renderizarMensagem(msg chatMessage) string {
	s, g := ui.S(), ui.G()
	largura := m.viewport.Width - len(recuo)
	var b strings.Builder
	switch msg.role {
	case "user":
		b.WriteString(s.Rotulo.Render(strings.ToUpper("você · "+hora(msg.hora))) + "\n")
		escreverRecuado(&b, quebrarTexto(msg.content, largura), s.Tinta.Render)
	case "assistant", "error":
		marca := s.Ok.Render(g.Ativo)
		if msg.role == "error" {
			marca = s.Erro.Render(g.Falha)
		}
		b.WriteString(marca + " " + s.Rotulo.Render(cabecalhoBard(msg)) + "\n")
		for _, f := range msg.ferramentas {
			b.WriteString(recuo + linhaFerramenta(f) + "\n")
		}
		if len(msg.ferramentas) > 0 {
			b.WriteString("\n")
		}
		if msg.role == "error" {
			escreverRecuado(&b, quebrarTexto(msg.content, largura), s.Erro.Render)
		} else {
			b.WriteString(m.corpoResposta(msg.content, largura))
		}
	case "nota":
		b.WriteString(recuo + s.TintaSuave.Render(msg.content) + "\n")
	}
	b.WriteString("\n")
	return b.String()
}

// renderizarParcial mostra a resposta enquanto ela chega, ainda sem markdown.
func (m Model) renderizarParcial() string {
	s, g := ui.S(), ui.G()
	var b strings.Builder
	b.WriteString(s.Tinta.Render(g.Andamento) + " " + s.Rotulo.Render("BARD · trabalhando") + "\n")
	if m.parcial != "" {
		escreverRecuado(&b, quebrarTexto(m.parcial, m.viewport.Width-len(recuo)), s.Tinta.Render)
	}
	return b.String()
}

// corpoResposta passa a resposta pelo markdown e recua as linhas sob o autor.
// Sem markdown (ou se ele falhar), mostra o texto cru quebrado na largura.
func (m Model) corpoResposta(texto string, largura int) string {
	var linhas []string
	if m.markdown != nil {
		if r, err := m.markdown(texto, largura); err == nil {
			linhas = strings.Split(strings.TrimRight(r, "\n"), "\n")
		}
	}
	if linhas == nil {
		linhas = quebrarTexto(texto, largura)
		for i, l := range linhas {
			linhas[i] = ui.S().Tinta.Render(l)
		}
	}
	var b strings.Builder
	escreverRecuado(&b, linhas, nil)
	return b.String()
}

// cabecalhoBard monta "BARD · 11:52 · 1 ferramenta · 21 s": o autor em caixa-alta,
// como o VOCÊ, e o resto como nota.
func cabecalhoBard(msg chatMessage) string {
	partes := []string{"BARD", hora(msg.hora)}
	if n := len(msg.ferramentas); n > 0 {
		partes = append(partes, plural(n, "ferramenta", "ferramentas"))
	}
	if msg.duracao > 0 {
		partes = append(partes, ui.FormatarDuracao(msg.duracao))
	}
	return strings.Join(partes, " · ")
}

// linhaFerramenta monta "✓ kubectl get pvc,pv -A · 14 linhas" (ou ✕ com o erro).
func linhaFerramenta(f ferramentaUsada) string {
	s, g := ui.S(), ui.G()
	texto := strings.TrimSpace(f.rotulo + " " + f.alvo)
	if f.falhou {
		if f.nota != "" {
			texto += " · " + f.nota
		}
		return s.Erro.Render(g.Falha) + " " + s.TintaSuave.Render(texto)
	}
	if f.linhas > 0 {
		texto += " · " + plural(f.linhas, "linha", "linhas")
	}
	return s.Ok.Render(g.Feito) + " " + s.TintaSuave.Render(texto)
}

// quebrarTexto quebra cada linha do texto na largura, entre palavras.
func quebrarTexto(texto string, largura int) []string {
	var linhas []string
	for _, l := range strings.Split(strings.TrimRight(texto, "\n"), "\n") {
		linhas = append(linhas, ui.Quebrar(l, largura)...)
	}
	return linhas
}

// escreverRecuado escreve as linhas com o recuo das mensagens; estilo nil mantém a
// linha como veio. Linhas vazias ficam vazias, sem espaços.
func escreverRecuado(b *strings.Builder, linhas []string, estilo func(...string) string) {
	for _, l := range linhas {
		if strings.TrimSpace(l) == "" {
			b.WriteString("\n")
			continue
		}
		if estilo != nil {
			l = estilo(l)
		}
		b.WriteString(recuo + l + "\n")
	}
}

func hora(t time.Time) string { return t.Format("15:04") }

func plural(n int, um, varios string) string {
	if n == 1 {
		return "1 " + um
	}
	return fmt.Sprintf("%d %s", n, varios)
}

// markdownCache guarda o renderizador da última largura; o tema não muda durante a
// sessão, então só a largura nova pede outro.
type markdownCache struct {
	largura int
	r       *uimarkdown.Renderizador
}

func (c *markdownCache) renderizar(texto string, largura int) (string, error) {
	if c.r == nil || c.largura != largura {
		r, err := uimarkdown.Novo(largura)
		if err != nil {
			return "", err
		}
		c.r, c.largura = r, largura
	}
	return c.r.Renderizar(texto)
}
