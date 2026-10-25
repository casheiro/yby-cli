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
	rotuloInstrumento     = "instr. 03 · orientação"
	textoPlaceholder      = "Pergunte sobre o cluster, ou / para comandos"
	textoPlaceholderResp  = "a próxima pergunta pode ser escrita enquanto o Bard responde"
	textoPlaceholderSaida = "esc volta para a conversa"
	recuo                 = "  " // recuo do texto das mensagens sob o autor
)

// View renderiza a TUI na moldura de texto da carta: cabeçalho com cluster,
// namespace e provider, a conversa, o campo, a linha de estado e as teclas.
func (m Model) View() string {
	if m.width == 0 {
		return "Carregando..."
	}
	partes := ui.PartesTexto{
		Instrumento: "bard",
		Rotulo:      rotuloInstrumento,
		Contexto:    m.contexto(),
		Campo:       m.campo(),
		Mensagem:    m.mensagemEstado(),
		Sair:        ui.DefaultKeys().Global.SairForcado,
	}
	switch lista := m.comandosVisiveis(); {
	case m.saida != nil:
		partes.Corpo, partes.Direita, partes.Teclas = m.corpoSaida(), m.posicaoSaida(), m.teclasSaida()
	case m.seletor != nil:
		partes.Corpo, partes.Direita, partes.Teclas = m.corpoComLista(m.quadroSeletor()), m.resumoSessao(), m.teclasSeletor()
		partes.Mensagem = m.estadoSeletor()
	case m.ajuda:
		// A camada da ajuda fica sobre a conversa; a linha de estado segue a da resposta.
		partes.Corpo, partes.Teclas = m.corpoComLista(m.camadaAjuda()), []key.Binding{teclaFechar()}
		partes.Direita = m.resumoSessao()
		if m.state == stateStreaming {
			partes.Direita = m.resumoRodada()
		}
	case len(lista) > 0:
		partes.Corpo, partes.Direita, partes.Teclas = m.corpoComLista(m.quadroComandos(lista)), m.resumoSessao(), teclasComandos()
		partes.Mensagem = ui.S().TintaSuave.Render(textoDicaComandos)
	case m.state == stateStreaming:
		partes.Corpo, partes.Direita, partes.Teclas = m.corpo(), m.resumoRodada(), m.teclas()
	default:
		partes.Corpo, partes.Direita, partes.Teclas = m.corpo(), m.resumoSessao(), m.teclas()
	}
	return ui.MolduraTexto(m.width, m.height, partes)
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

// corpo é a conversa na janela. Com a conversa rolada, as duas primeiras linhas dão
// lugar ao aviso "↑ 2 mensagens anteriores" e a uma linha em branco, como na prancha
// "trabalhando"; com mensagens novas abaixo, as duas últimas dão lugar a uma linha em
// branco e ao aviso "↓ 2 mensagens novas". Nada se perde: pgup e pgdn andam só as
// linhas que apareceram (ver rolarPagina), e os avisos contam também o que começa nas
// linhas cobertas.
func (m Model) corpo() string {
	vista := m.viewport.View()
	acima, abaixo := m.mensagensAnteriores(), m.mensagensNovas()
	if acima == 0 && abaixo == 0 {
		return vista
	}
	linhas := strings.Split(vista, "\n")
	dupla := m.linhasAviso() == 2 && len(linhas) > 1
	if acima > 0 {
		linhas[0] = ui.AvisoRolagem(ui.SetaAcima, plural(acima, "mensagem anterior", "mensagens anteriores"))
		if dupla {
			linhas[1] = ""
		}
	}
	if abaixo > 0 {
		linhas[len(linhas)-1] = ui.AvisoRolagem(ui.SetaAbaixo, plural(abaixo, "mensagem nova", "mensagens novas"))
		if dupla {
			linhas[len(linhas)-2] = ""
		}
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
	case m.seletor != nil || m.ajuda:
		// Com o seletor ou a ajuda abertos as letras não vão para o campo, como com a saída.
		linhas = []string{s.TintaSuave.Render(g.Prompt + " " + textoPlaceholderSaida)}
	case m.textarea.Value() == "" && m.saida != nil:
		// Com a saída aberta as letras não vão para o campo: a dica diz como voltar.
		linhas = []string{s.TintaSuave.Render(g.Prompt + " " + textoPlaceholderSaida)}
	case m.textarea.Value() == "" && m.state == stateStreaming:
		linhas = []string{s.TintaSuave.Render(g.Prompt + " " + textoPlaceholderResp)}
	case m.textarea.Value() == "":
		cursor := m.textarea.Cursor
		cursor.SetChar(" ")
		linhas = []string{s.Ok.Render(g.Prompt) + " " + cursor.View() + s.TintaSuave.Render(textoPlaceholder)}
	default:
		linhas = strings.Split(m.textarea.View(), "\n")
	}
	// Com uma lista aberta, a contagem dela vai no canto direito: "8 comandos".
	if c := m.contagemCampo(); c != "" {
		linhas[0] = juntarDireita(linhas[0], s.TintaSuave.Render(c), m.width)
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
	if a := m.aviso; a != nil {
		if a.ok {
			return s.Ok.Render(g.Feito) + s.Tinta.Render(" "+a.texto)
		}
		return s.Erro.Render(g.Falha) + s.Tinta.Render(" "+a.texto)
	}
	if m.state == stateStreaming {
		return m.estadoTrabalho()
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
		if msg.role == "user" || msg.role == "assistant" || msg.role == "quadro" {
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

// teclas devolve a legenda da tela, na ordem da prancha. Só aparecem as teclas que
// funcionam: pgup quando a conversa passa da altura do histórico, ↑ quando há uma
// pergunta para editar e o campo está vazio, ↵ ver etapa quando há uma saída de
// ferramenta para abrir. / e ? só valem no campo vazio (com texto, viram letras); a / só
// fora da resposta, e o ? também durante ela, quando abre a ajuda numa camada.
func (m Model) teclas() []key.Binding {
	k := ui.DefaultKeys()
	rolar := k.Texto.RolarCima
	rolar.SetHelp(rolar.Help().Key, "rolar")
	if m.viewport.TotalLineCount() <= m.viewport.Height {
		rolar.SetEnabled(false)
	}
	editar := k.Texto.EditarUltima
	editar.SetEnabled(m.podeEditarUltima())
	comandos, ajuda := k.Texto.Comandos, k.Global.Ajuda
	comandos.SetEnabled(m.textarea.Value() == "")
	ajuda.SetEnabled(m.textarea.Value() == "")
	if m.state == stateStreaming {
		ver := teclaVerEtapa()
		_, ok := m.saidaDisponivel()
		ver.SetEnabled(ok)
		return []key.Binding{k.Texto.Interromper, rolar, ver, ajuda}
	}
	// A prancha mostra ⇧↵, mas o Bubble Tea v1 recebe o shift+enter como ↵ e a
	// pergunta iria pela metade; a legenda mostra a tecla que chega (ver novaLinha).
	quebra := k.Texto.NovaLinha
	quebra.SetHelp(ui.RotuloTecla("ctrl+j"), quebra.Help().Desc)
	return []key.Binding{k.Texto.Enviar, quebra, comandos, editar, rolar, ajuda}
}

// renderizarMensagem devolve o bloco de uma mensagem, terminado por uma linha em branco.
// Com dica, a ferramenta cuja saída o ↵ abre leva "· ↵ ver saída"; com copia, o último
// comando leva "ctrl+y copia" no quadro (os outros ficam só "COMANDO").
func (m Model) renderizarMensagem(msg chatMessage, dica, copia bool) string {
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
		ver := -1
		if dica {
			ver = ultimaComSaida(msg.ferramentas)
		}
		for i, f := range msg.ferramentas {
			linha := linhaFerramenta(f)
			if i == ver {
				linha += s.TintaSuave.Render(" · ") + s.Negrito.Render(ui.RotuloTecla("↵")) + s.TintaSuave.Render(" ver saída")
			}
			b.WriteString(recuo + linha + "\n")
		}
		if len(msg.ferramentas) > 0 {
			b.WriteString("\n")
		}
		if msg.role == "error" {
			escreverRecuado(&b, quebrarTexto(msg.content, largura), s.Erro.Render)
		} else {
			comando := -1
			if copia {
				comando = len(uimarkdown.Comandos(msg.content)) - 1
			}
			b.WriteString(m.corpoResposta(msg.content, largura, comando))
		}
	case "nota":
		b.WriteString(recuo + s.TintaSuave.Render(msg.content) + "\n")
	case "quadro":
		if msg.quadro != nil {
			escreverRecuado(&b, strings.Split(renderizarQuadro(*msg.quadro, largura), "\n"), nil)
		}
	}
	b.WriteString("\n")
	return b.String()
}

// corpoResposta passa a resposta pelo markdown e recua as linhas sob o autor.
func (m Model) corpoResposta(texto string, largura, comando int) string {
	var b strings.Builder
	escreverRecuado(&b, m.linhasResposta(texto, largura, comando), nil)
	return b.String()
}

// linhasResposta são as linhas da resposta pelo markdown, sem o recuo; o comando de
// índice comando (-1 nenhum) leva a tecla da cópia no quadro. Sem markdown (ou se ele
// falhar), o texto cru quebrado na largura.
func (m Model) linhasResposta(texto string, largura, comando int) []string {
	var linhas []string
	if m.markdown != nil {
		if r, err := m.markdown(texto, largura, comando); err == nil {
			linhas = strings.Split(strings.TrimRight(r, "\n"), "\n")
		}
	}
	if linhas == nil {
		linhas = quebrarTexto(texto, largura)
		for i, l := range linhas {
			linhas[i] = ui.S().Tinta.Render(l)
		}
	}
	return linhas
}

// ultimaComSaida é o índice da última ferramenta com saída para abrir, ou -1.
func ultimaComSaida(lista []ferramentaUsada) int {
	for i := len(lista) - 1; i >= 0; i-- {
		if !lista[i].falhou && strings.TrimSpace(lista[i].saida) != "" {
			return i
		}
	}
	return -1
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

func (c *markdownCache) renderizar(texto string, largura, comando int) (string, error) {
	if c.r == nil || c.largura != largura {
		r, err := uimarkdown.Novo(largura)
		if err != nil {
			return "", err
		}
		c.r, c.largura = r, largura
	}
	return c.r.RenderizarComTecla(texto, comando, rotuloCopiar, "copia")
}
