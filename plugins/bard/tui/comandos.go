package tui

import (
	"strings"
	"time"

	"github.com/casheiro/yby-cli/pkg/ui"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Comandos / do Bard (prancha "comandos"): uma / no começo do campo abre a lista sobre
// ele. O texto depois da barra filtra, ↑↓ escolhem, ⇥ completa, ↵ executa e esc fecha.
// Os comandos rodam no Bard, sem passar pela IA, e não vão para o histórico.

// comando é uma linha da lista de comandos.
type comando struct {
	nome      string   // sem a barra, como aparece na lista
	descricao string   // em minúsculas e sem ponto
	apelidos  []string // nomes aceitos só por extenso (os antigos e em inglês)
}

// listaComandos está na ordem da prancha.
var listaComandos = []comando{
	{"sessões", "lista as conversas anteriores e retoma uma delas", []string{"sessions", "session"}},
	{"limpar", "começa uma conversa nova; a atual fica salva", []string{"clear"}},
	{"contexto", "mostra o que o Bard sabe: cluster, namespace, notas", []string{"context"}},
	{"ferramentas", "lista as ferramentas e quais estão disponíveis", []string{"tools"}},
	{"modelo", "troca o provider de IA nesta sessão", []string{"model", "provider"}},
	{"copiar", "copia a última resposta", []string{"copy"}},
	{"exportar", "salva a conversa em markdown no projeto", []string{"export"}},
	{"ajuda", "atalhos e exemplos de perguntas", []string{"help"}},
}

// textoDicaComandos é a linha de estado com a lista aberta.
const textoDicaComandos = "digite para filtrar · /se mostra só /sessões"

// semAcento tira os acentos do português, para "/sessoes" achar "/sessões".
var semAcento = strings.NewReplacer(
	"á", "a", "à", "a", "â", "a", "ã", "a", "é", "e", "ê", "e", "í", "i",
	"ó", "o", "ô", "o", "õ", "o", "ú", "u", "ü", "u", "ç", "c",
).Replace

func normalizar(s string) string { return semAcento(strings.ToLower(strings.TrimSpace(s))) }

// comandosFiltrados devolve os comandos cujo nome começa pelo filtro (sem acento e sem
// caixa). Os apelidos ficam fora da lista: "/c" mostra /contexto e /copiar, não /limpar.
func comandosFiltrados(filtro string) []comando {
	f := normalizar(filtro)
	var lista []comando
	for _, c := range listaComandos {
		if strings.HasPrefix(normalizar(c.nome), f) {
			lista = append(lista, c)
		}
	}
	return lista
}

// acharComando acha o comando pelo nome por extenso ou por um apelido.
func acharComando(nome string) (comando, bool) {
	n := normalizar(nome)
	for _, c := range listaComandos {
		if n == normalizar(c.nome) {
			return c, true
		}
		for _, a := range c.apelidos {
			if n == a {
				return c, true
			}
		}
	}
	return comando{}, false
}

// comandoDigitado reconhece "/nome argumentos" no texto enviado. Uma barra que não é de
// comando ("/etc/hosts está certo?") segue como pergunta.
func comandoDigitado(texto string) (nome, args string, ok bool) {
	if !strings.HasPrefix(texto, "/") {
		return "", "", false
	}
	primeiro, resto, _ := strings.Cut(strings.TrimPrefix(texto, "/"), " ")
	c, ok := acharComando(primeiro)
	if !ok {
		return "", "", false
	}
	return c.nome, strings.TrimSpace(resto), true
}

// comandosVisiveis são os comandos da lista aberta; nil com ela fechada. A lista abre
// fora da resposta, sem outra camada, com o campo numa linha que começa por / e não tem
// espaço, e só enquanto algum comando combina com o filtro.
func (m Model) comandosVisiveis() []comando {
	if m.state != stateIdle || m.saida != nil || m.seletor != nil || m.ajuda {
		return nil
	}
	v := m.textarea.Value()
	if !strings.HasPrefix(v, "/") || strings.ContainsAny(v, " \t\n") {
		return nil
	}
	return comandosFiltrados(v[1:])
}

// comandoMarcado é o índice do comando marcado na lista; o primeiro se o marcado saiu
// do filtro.
func (m Model) comandoMarcado(lista []comando) int {
	for i, c := range lista {
		if c.nome == m.comandoSel {
			return i
		}
	}
	return 0
}

// teclarComandos trata as teclas da lista aberta. As outras (as letras do filtro, o
// apagar) seguem para o campo: tratou diz se a tecla parou aqui.
func (m Model) teclarComandos(msg tea.KeyMsg, lista []comando) (_ tea.Model, _ tea.Cmd, tratou bool) {
	i := m.comandoMarcado(lista)
	switch {
	case msg.Type == tea.KeyUp:
		m.comandoSel = lista[(i-1+len(lista))%len(lista)].nome
	case msg.Type == tea.KeyDown:
		m.comandoSel = lista[(i+1)%len(lista)].nome
	case msg.Type == tea.KeyTab:
		m.comandoSel = lista[i].nome
		m.textarea.SetValue("/" + lista[i].nome)
		m.reposicionarCampo()
	case msg.Type == tea.KeyEnter && !msg.Alt:
		modelo, cmd := m.executarComando(lista[i].nome, "")
		return modelo, cmd, true
	case msg.Type == tea.KeyEsc:
		// Fechar apaga a barra e o filtro: com o texto no campo a lista abriria de novo
		// na tecla seguinte.
		m.textarea.Reset()
		m.comandoSel = ""
		m.ajustarCampo()
		m.updateViewport()
	default:
		return m, nil, false
	}
	return m, nil, true
}

// executarComando roda o comando com os argumentos e limpa o campo.
func (m Model) executarComando(nome, args string) (tea.Model, tea.Cmd) {
	vazio := m.campoVazio()
	m.textarea.Reset()
	m.comandoSel = ""
	m.ajustarCampo()
	m.seCampoMudou(vazio)
	agora := m.agora()
	texto := strings.TrimSpace("/" + nome + " " + args)

	var cmd tea.Cmd
	switch nome {
	case "sessões":
		cmd = m.abrirSessoes(args, agora)
	case "limpar":
		m.limpar(agora)
	case "contexto":
		m.mostrarQuadro(texto, agora, m.quadroContexto(agora))
	case "ferramentas":
		m.mostrarQuadro(texto, agora, m.quadroFerramentas())
	case "modelo":
		cmd = m.abrirModelos(args)
	case "copiar":
		cmd = m.copiarResposta()
	case "exportar":
		cmd = m.exportar(args)
	case "ajuda":
		m.mostrarQuadro(texto, agora, quadroAjuda(true, m.config.Voltar))
	}
	return m, cmd
}

// mostrarQuadro põe na conversa o comando, como fala do usuário, e o quadro com o
// resultado, e leva a conversa para o fim.
func (m *Model) mostrarQuadro(texto string, agora time.Time, q quadroMsg) {
	m.messages = append(m.messages,
		chatMessage{role: "user", content: texto, hora: agora, comando: true},
		chatMessage{role: "quadro", hora: agora, quadro: &q})
	m.irParaOFim()
}

// irParaOFim redesenha a conversa e a leva para o fim, como depois de uma pergunta.
func (m *Model) irParaOFim() {
	m.updateViewport()
	m.viewport.GotoBottom()
	m.marcarLidas()
}

// itemLista é uma linha das listas sobre o campo (comandos, sessões, providers).
type itemLista struct {
	nome, descricao string
}

// larguraNomeLista é a coluna dos nomes nas listas, como na prancha (16 células).
const larguraNomeLista = 16

// quadroLista desenha a lista no quadro em foco: ◆ e tinta invertida no item marcado, ◇
// nos outros. Com mais itens que a altura, a janela acompanha o marcado.
func quadroLista(rotulo string, itens []itemLista, marcado, largura, altura int) string {
	s, g := ui.S(), ui.G()
	interno := max(largura-4, 1) // borda e respiro do quadro
	coluna := larguraNomeLista
	for _, it := range itens {
		coluna = max(coluna, lipgloss.Width(it.nome)+2)
	}
	visiveis := max(min(len(itens), altura-2), 1)
	inicio := max(marcado-visiveis+1, 0)
	linhas := make([]string, 0, visiveis)
	for i := inicio; i < min(inicio+visiveis, len(itens)); i++ {
		it := itens[i]
		nome := it.nome + strings.Repeat(" ", coluna-lipgloss.Width(it.nome))
		if i == marcado {
			// Na tinta invertida só o nome vai em negrito, como na prancha e na tabela
			// do pkg/ui; o glifo, a descrição e o preenchimento ficam sem negrito.
			resto := s.Selecionado.Bold(false)
			texto := ansi.Truncate(g.Ativo+" "+nome+it.descricao, interno, g.Reticencias)
			texto += strings.Repeat(" ", max(interno-lipgloss.Width(texto), 0))
			glifo, depois := ansi.Cut(texto, 0, 2), ansi.Cut(texto, 2, interno)
			nomeVisivel := ansi.Cut(depois, 0, lipgloss.Width(it.nome))
			linhas = append(linhas, resto.Render(glifo)+s.Selecionado.Render(nomeVisivel)+
				resto.Render(ansi.Cut(depois, lipgloss.Width(it.nome), interno)))
			continue
		}
		linhas = append(linhas, s.TintaSuave.Render(g.Pendente)+" "+s.Tinta.Render(nome)+s.TintaSuave.Render(it.descricao))
	}
	return ui.Quadro(rotulo, strings.Join(linhas, "\n"), largura, ui.QuadroFoco)
}

// quadroComandos é a lista de comandos filtrada, na largura da conversa.
func (m Model) quadroComandos(lista []comando) string {
	itens := make([]itemLista, len(lista))
	for i, c := range lista {
		itens[i] = itemLista{nome: "/" + c.nome, descricao: c.descricao}
	}
	return quadroLista("comandos", itens, m.comandoMarcado(lista), m.viewport.Width, m.viewport.Height)
}

// corpoComLista põe o quadro de uma lista no pé da janela, sobre o fim da conversa, que
// fica apagada no fio, como na prancha "comandos".
func (m Model) corpoComLista(quadro string) string {
	fio := ui.S().Fio
	q := strings.Split(quadro, "\n")
	h := m.viewport.Height
	acima := max(h-len(q), 0)
	vista := strings.Split(m.corpo(), "\n")
	linhas := make([]string, 0, h)
	for i := 0; i < acima; i++ {
		l := ""
		if i < len(vista) {
			l = strings.TrimRight(ansi.Strip(vista[i]), " ")
		}
		if l != "" {
			l = fio.Render(l)
		}
		linhas = append(linhas, l)
	}
	linhas = append(linhas, q...)
	if len(linhas) > h {
		linhas = linhas[len(linhas)-h:]
	}
	return strings.Join(linhas, "\n")
}

// juntarDireita põe dir no canto direito da linha esq, cortando esq se preciso. O
// preenchimento do campo (espaços no fim) é o que sai primeiro.
func juntarDireita(esq, dir string, largura int) string {
	wd := lipgloss.Width(dir)
	esq = ansi.Truncate(esq, max(largura-wd-2, 0), "")
	return esq + strings.Repeat(" ", max(largura-lipgloss.Width(esq)-wd, 1)) + dir
}

// contagemCampo é o que vai à direita do campo com uma lista aberta: "8 comandos".
func (m Model) contagemCampo() string {
	if sel := m.seletor; sel != nil {
		if sel.estado != nil {
			return ""
		}
		if sel.tipo == seletorSessoes {
			return plural(len(sel.itens), "conversa", "conversas")
		}
		return plural(len(sel.itens), "provider", "providers")
	}
	if lista := m.comandosVisiveis(); len(lista) > 0 {
		return plural(len(lista), "comando", "comandos")
	}
	return ""
}

// teclaEscolher é o ↑↓ das listas ("up/down" em ASCII, como na especificação).
func teclaEscolher() key.Binding {
	rotulo := ui.RotuloTecla("↑↓")
	if ui.Atual().ASCII {
		rotulo = "up/down"
	}
	return key.NewBinding(key.WithKeys("up", "down"), key.WithHelp(rotulo, "escolher"))
}

// teclaFechar é o esc das listas e das camadas que ele fecha.
func teclaFechar() key.Binding {
	return key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "fechar"))
}

// teclasComandos é a legenda com a lista de comandos aberta, como na prancha.
func teclasComandos() []key.Binding {
	return []key.Binding{
		teclaEscolher(),
		key.NewBinding(key.WithKeys("tab"), key.WithHelp(ui.RotuloTecla("⇥"), "completar")),
		key.NewBinding(key.WithKeys("enter"), key.WithHelp(ui.RotuloTecla("↵"), "executar")),
		teclaFechar(),
	}
}
