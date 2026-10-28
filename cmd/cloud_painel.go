package cmd

import (
	gocontext "context"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/casheiro/yby-cli/v5/pkg/cloud"
	"github.com/casheiro/yby-cli/v5/pkg/errors"
	"github.com/casheiro/yby-cli/v5/pkg/ui"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// Painel do yby cloud dashboard (prancha CloudPainel): resumo antes da tabela, validade
// do token como cota e o detalhe com identidade e auditoria recente. As teclas seguem a
// folha Atalhos: esc fecha a camada de cima (campo, ajuda, detalhe por cima) e, sem
// camada, limpa a busca; q sai só do nível de cima; ctrl+c sai de qualquer lugar.

const (
	intervaloLeitura = 30 * time.Second // as clouds são lidas a cada 30 s
	intervaloTique   = time.Second      // o tique só redesenha (validade e contagem)
	tempoLeitura     = time.Minute      // limite de uma leitura de todas as clouds
	tempoRenovacao   = 2 * time.Minute  // limite de um RefreshToken
)

// painelCloud é o modelo Bubble Tea do painel.
type painelCloud struct {
	fontes fontesPainel
	agora  func() time.Time

	largura, altura int

	carregado  bool
	lendo      bool
	provedores int
	clusters   []clusterPainel
	auditoria  []cloud.CloudAuditEvent
	falhas     map[string]error // providers sem resposta na última leitura (os outros responderam)
	err        error
	atualizado time.Time
	proxima    time.Time // quando o tique dispara a próxima leitura

	cursor  int // índice em visiveis()
	rolagem int

	detalheAberto bool // abaixo de 120 colunas, o detalhe por cima da lista (↵)
	ajuda         bool
	rolagemAjuda  int

	campo         bool // campo de busca aberto na linha de estado
	textoCampo    string
	busca         string
	buscaAnterior string

	renovando map[string]string // chave do cluster → nome, enquanto o RefreshToken roda
	// releitura: um RefreshToken terminou com uma leitura em voo, que pode ter lido a
	// validade velha; o fim dela dispara outra.
	releitura bool
	toast     ui.Toast
	aviso     string // frase da última tecla; some na tecla seguinte
}

type tiquePainelMsg struct{}

type leituraPainelMsg struct {
	leitura leituraPainel
	recarga bool // veio do r: o fim avisa na linha de estado
}

type renovadoMsg struct {
	chave, nome string
	err         error
}

// auditoriaMsg traz o audit log relido depois de um t.
type auditoriaMsg struct {
	eventos []cloud.CloudAuditEvent
}

// novoPainelCloud cria o painel; a primeira leitura sai no Init.
func novoPainelCloud(f fontesPainel, agora func() time.Time) painelCloud {
	return painelCloud{
		fontes:    f,
		agora:     agora,
		lendo:     true,
		renovando: map[string]string{},
		toast:     ui.NovoToast(),
	}
}

// Init lê as clouds e liga o tique de 1 s.
func (m painelCloud) Init() tea.Cmd {
	return tea.Batch(m.lerCmd(false), tiquePainel())
}

func tiquePainel() tea.Cmd {
	return tea.Tick(intervaloTique, func(time.Time) tea.Msg { return tiquePainelMsg{} })
}

// lerCmd lê as clouds em segundo plano.
func (m painelCloud) lerCmd(recarga bool) tea.Cmd {
	ler := m.fontes.ler
	return func() tea.Msg {
		ctx, cancel := gocontext.WithTimeout(gocontext.Background(), tempoLeitura)
		defer cancel()
		return leituraPainelMsg{leitura: ler(ctx), recarga: recarga}
	}
}

// Update trata uma mensagem e mantém a seleção dentro da janela.
func (m painelCloud) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	nm, cmd := m.processar(msg)
	nm.ajustarRolagem()
	return nm, cmd
}

func (m painelCloud) processar(msg tea.Msg) (painelCloud, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		k, fica := ui.TeclaAltGr(msg)
		if !fica {
			return m, nil
		}
		if key.Matches(k, ui.DefaultKeys().Global.SairForcado) {
			return m, tea.Quit
		}
		m.aviso = ""
		return m.tecla(k)

	case tea.MouseMsg:
		return m.mouse(msg), nil

	case tea.WindowSizeMsg:
		m.largura, m.altura = msg.Width, msg.Height
		if ui.ModoPara(m.largura, m.altura) == ui.ModoLadoALado {
			m.detalheAberto = false // a partir de 120 colunas o detalhe está sempre na tela
		}
		return m, nil

	case tiquePainelMsg:
		cmds := []tea.Cmd{tiquePainel()}
		if !m.lendo && !m.proxima.IsZero() && !m.agora().Before(m.proxima) {
			m.lendo = true
			cmds = append(cmds, m.lerCmd(false))
		}
		return m, tea.Batch(cmds...)

	case leituraPainelMsg:
		return m.receber(msg)

	case renovadoMsg:
		return m.renovado(msg)

	case auditoriaMsg:
		m.auditoria = msg.eventos
		return m, nil

	case vizFechadoMsg:
		return m.vizFechado(msg)

	case ui.ToastSomeMsg:
		t, _ := m.toast.Update(msg)
		m.toast = t.(ui.Toast)
	}
	return m, nil
}

// receber guarda uma leitura, mantendo a seleção no mesmo cluster. Uma renovação que
// terminou durante a leitura pede outra logo em seguida.
func (m painelCloud) receber(msg leituraPainelMsg) (painelCloud, tea.Cmd) {
	nm, cmd := m.guardar(msg)
	if nm.releitura {
		nm.releitura, nm.lendo = false, true
		cmd = tea.Batch(cmd, nm.lerCmd(false))
	}
	return nm, cmd
}

func (m painelCloud) guardar(msg leituraPainelMsg) (painelCloud, tea.Cmd) {
	chave := m.chaveSelecionada()
	agora := m.agora()
	m.lendo, m.carregado = false, true
	m.proxima = agora.Add(intervaloLeitura)
	l := msg.leitura
	if l.err != nil {
		// Uma falha passageira não apaga a última leitura boa.
		m.err = l.err
		if msg.recarga {
			var cmd tea.Cmd
			m.aviso = ""
			m.toast, cmd = m.toast.MostrarComNota(ui.ToastErro, "não deu para ler as clouds", resumoDoErro(l.err))
			return m, cmd
		}
		return m, nil
	}
	m.err = nil
	m.atualizado = agora
	m.provedores = l.provedores
	lidos := make([]clusterPainel, len(l.clusters))
	for i, c := range l.clusters {
		c.lidoEm = agora
		lidos[i] = c
	}
	m.clusters = comAnteriores(lidos, m.clusters, l.falhas)
	m.falhas = l.falhas
	sort.SliceStable(m.clusters, func(a, b int) bool {
		ca, cb := m.clusters[a].info, m.clusters[b].info
		if ca.Provider != cb.Provider {
			return ca.Provider < cb.Provider
		}
		return ca.Name < cb.Name
	})
	m.auditoria = l.auditoria
	m.reselecionar(chave)
	if msg.recarga {
		var cmd tea.Cmd
		m.aviso = ""
		if nomes := m.providersSemResposta(); len(nomes) > 0 {
			// recarga parcial: o aviso diz quem ficou de fora, e não um ✓ por cima da falha
			verbo := " não respondeu"
			if len(nomes) > 1 {
				verbo = " não responderam"
			}
			m.toast, cmd = m.toast.Mostrar(ui.ToastAviso, strings.Join(nomes, ", ")+verbo+"; os outros clusters foram recarregados")
			return m, cmd
		}
		m.toast, cmd = m.toast.Mostrar(ui.ToastOk, "clusters recarregados")
		return m, cmd
	}
	return m, nil
}

// renovado mostra o resultado do RefreshToken e, se deu certo, lê de novo para trazer a
// validade nova; se falhou, relê só a auditoria, para o registro da falha aparecer já.
func (m painelCloud) renovado(msg renovadoMsg) (painelCloud, tea.Cmd) {
	delete(m.renovando, msg.chave)
	m.aviso = "" // o resultado vale mais que a frase da última tecla
	var cmd tea.Cmd
	if msg.err != nil {
		m.toast, cmd = m.toast.MostrarComNota(ui.ToastErro, "não deu para renovar o token de "+msg.nome, resumoDoErro(msg.err))
		return m, tea.Batch(cmd, m.auditoriaCmd())
	}
	m.toast, cmd = m.toast.Mostrar(ui.ToastOk, "token de "+msg.nome+" renovado")
	if m.lendo {
		m.releitura = true // a leitura em voo pode ter começado antes da renovação
		return m, cmd
	}
	m.lendo = true
	return m, tea.Batch(cmd, m.lerCmd(false))
}

// auditoriaCmd relê o audit log em segundo plano; sem a fonte, nada.
func (m painelCloud) auditoriaCmd() tea.Cmd {
	ler := m.fontes.auditoria
	if ler == nil {
		return nil
	}
	return func() tea.Msg { return auditoriaMsg{eventos: ler()} }
}

// comAnteriores junta aos clusters lidos os da leitura anterior dos providers que não
// responderam agora, marcados como desatualizados: um SSO vencido não some com a linha.
func comAnteriores(lidos, anteriores []clusterPainel, falhas map[string]error) []clusterPainel {
	saida := append([]clusterPainel(nil), lidos...)
	if len(falhas) == 0 {
		return saida
	}
	presentes := map[string]bool{}
	for _, c := range lidos {
		presentes[c.chave()] = true
	}
	for _, c := range anteriores {
		if _, falhou := falhas[c.info.Provider]; falhou && !presentes[c.chave()] {
			c.desatualizado = true
			saida = append(saida, c)
		}
	}
	return saida
}

// prefixoDoCLI é o "aws eks get-token: exit status 255: " que os providers põem antes da
// saída do CLI (sem saída, o erro termina no código).
var prefixoDoCLI = regexp.MustCompile(`^[^:]+: exit status (\d+)(?:: |$)`)

// resumoDoErro é a nota curta de um erro do CLI do provider, sem o comando nem o código
// de saída; quando o CLI não disse nada, só o código.
func resumoDoErro(err error) string {
	msg := err.Error()
	if m := prefixoDoCLI.FindStringSubmatch(msg); m != nil {
		if resto := strings.TrimSpace(msg[len(m[0]):]); resto != "" {
			return errors.ResumirSaida(resto)
		}
		return "o CLI saiu com o código " + m[1]
	}
	return errors.ResumirSaida(msg)
}

// visiveis são os clusters que passam na busca (por nome, sem diferenciar maiúsculas).
func (m painelCloud) visiveis() []clusterPainel {
	if m.busca == "" {
		return m.clusters
	}
	var saida []clusterPainel
	for _, i := range m.indicesVisiveis() {
		saida = append(saida, m.clusters[i])
	}
	return saida
}

// indicesVisiveis são as posições, em m.clusters, dos clusters que passam na busca.
func (m painelCloud) indicesVisiveis() []int {
	termo := strings.ToLower(m.busca)
	saida := make([]int, 0, len(m.clusters))
	for i, c := range m.clusters {
		if termo == "" || strings.Contains(strings.ToLower(c.info.Name), termo) {
			saida = append(saida, i)
		}
	}
	return saida
}

// selecionado é o cluster da linha selecionada.
func (m painelCloud) selecionado() (clusterPainel, bool) {
	v := m.visiveis()
	if m.cursor < 0 || m.cursor >= len(v) {
		return clusterPainel{}, false
	}
	return v[m.cursor], true
}

func (m painelCloud) chaveSelecionada() string {
	c, ok := m.selecionado()
	if !ok {
		return ""
	}
	return c.chave()
}

// reselecionar põe a seleção de volta no cluster da chave; se ele saiu da lista, fica
// na mesma posição (dentro do tamanho novo).
func (m *painelCloud) reselecionar(chave string) {
	for i, c := range m.visiveis() {
		if c.chave() == chave {
			m.cursor = i
			return
		}
	}
	m.cursor = max(0, min(m.cursor, len(m.visiveis())-1))
}

// ajustarRolagem mantém a linha selecionada na janela da tabela.
func (m *painelCloud) ajustarRolagem() {
	n := len(m.visiveis())
	m.cursor = max(0, min(m.cursor, n-1))
	linhas := m.linhasDeDados()
	if m.cursor < m.rolagem {
		m.rolagem = m.cursor
	}
	if m.cursor >= m.rolagem+linhas {
		m.rolagem = m.cursor - linhas + 1
	}
	m.rolagem = max(0, min(m.rolagem, n-linhas))
}

// atalhoPainel cria um binding para a legenda e a ajuda, com o rótulo no modo visual atual.
func atalhoPainel(rotulo, descricao string, teclas ...string) key.Binding {
	return key.NewBinding(key.WithKeys(teclas...), key.WithHelp(ui.RotuloTecla(rotulo), descricao))
}

// setasPainel é o rótulo de mover: ↑↓ no Unicode, up/down no ASCII.
func setasPainel() string {
	if ui.Atual().ASCII {
		return "up/down"
	}
	return "↑↓"
}

// teclaViz é o ↵ que abre o cluster selecionado no Viz.
func teclaViz() key.Binding { return atalhoPainel("↵", "abrir no Viz", "enter") }

// teclaRenovar é o t, que renova o token do cluster selecionado.
func teclaRenovar() key.Binding { return atalhoPainel("t", "renovar token", "t") }

// tecla despacha a tecla para a camada de cima.
func (m painelCloud) tecla(k tea.KeyMsg) (painelCloud, tea.Cmd) {
	switch {
	case m.campo:
		return m.teclaCampo(k), nil
	case m.ajuda:
		return m.teclaAjuda(k), nil
	}
	return m.teclaLista(k)
}

// teclaLista trata o nível de cima: a tabela (com o detalhe por cima, abaixo de 120 colunas).
func (m painelCloud) teclaLista(k tea.KeyMsg) (painelCloud, tea.Cmd) {
	g, l := ui.DefaultKeys().Global, ui.DefaultKeys().Lista
	n := len(m.visiveis())
	pagina := max(1, m.linhasDeDados())
	switch {
	case key.Matches(k, g.Sair) && m.porCima():
		m.aviso = fraseComTeclas("`q` sai só da lista; `esc` volta")
	case key.Matches(k, g.Sair):
		return m, tea.Quit
	case key.Matches(k, g.Ajuda):
		m.ajuda, m.rolagemAjuda = true, 0
	case key.Matches(k, g.Voltar) && m.porCima():
		m.detalheAberto = false
	case key.Matches(k, g.Voltar) && m.busca != "":
		chave := m.chaveSelecionada()
		m.busca = ""
		m.reselecionar(chave)
	case key.Matches(k, g.Recarregar):
		if m.lendo {
			return m, nil // uma leitura por vez
		}
		m.lendo = true
		return m, m.lerCmd(true)
	case k.String() == "R":
		// R recarregava e r renovava o token; agora r recarrega e t renova.
		m.aviso = fraseComTeclas("recarregar agora é `r`; `t` renova o token")
	case key.Matches(k, teclaRenovar()):
		return m.renovar()
	case key.Matches(k, l.Baixo):
		m.cursor = min(n-1, m.cursor+1)
	case key.Matches(k, l.Cima):
		m.cursor = max(0, m.cursor-1)
	case key.Matches(k, l.PaginaBaixo):
		m.cursor = min(n-1, m.cursor+pagina)
	case key.Matches(k, l.PaginaCima):
		m.cursor = max(0, m.cursor-pagina)
	case key.Matches(k, l.Inicio):
		m.cursor = 0
	case key.Matches(k, l.Fim):
		m.cursor = n - 1
	case key.Matches(k, l.Buscar):
		m.campo, m.textoCampo, m.buscaAnterior = true, m.busca, m.busca
	case key.Matches(k, l.Abrir) && n > 0:
		// Com o detalhe na tela (a partir de 120 colunas, ou aberto por cima), o ↵ abre o
		// cluster no Viz; abaixo de 120 colunas, o primeiro ↵ abre o detalhe.
		if ui.ModoPara(m.largura, m.altura) == ui.ModoLadoALado || m.porCima() {
			return m.abrirNoViz()
		}
		m.detalheAberto = true
	}
	return m, nil
}

// renovar chama o RefreshToken do cluster selecionado em segundo plano.
func (m painelCloud) renovar() (painelCloud, tea.Cmd) {
	c, ok := m.selecionado()
	if !ok {
		return m, nil
	}
	chave, nome := c.chave(), c.info.Name
	if _, ja := m.renovando[chave]; ja {
		m.aviso = fraseNeutra("o token de " + nome + " já está sendo renovado")
		return m, nil
	}
	m.renovando[chave] = nome
	renovar := m.fontes.renovar
	return m, func() tea.Msg {
		ctx, cancel := gocontext.WithTimeout(gocontext.Background(), tempoRenovacao)
		defer cancel()
		return renovadoMsg{chave: chave, nome: nome, err: renovar(ctx, c)}
	}
}

// teclaCampo trata o campo de busca: filtra enquanto digita; ↵ confirma, esc volta ao
// termo de antes. Tudo que não é ↵, esc ou apagar é texto (inclusive q).
func (m painelCloud) teclaCampo(k tea.KeyMsg) painelCloud {
	chave := m.chaveSelecionada()
	switch k.Type {
	case tea.KeyEnter:
		m.campo = false
		m.busca = strings.TrimSpace(m.textoCampo)
	case tea.KeyEsc:
		m.campo = false
		m.busca = m.buscaAnterior
	case tea.KeyBackspace:
		if _, n := utf8.DecodeLastRuneInString(m.textoCampo); n > 0 {
			m.textoCampo = m.textoCampo[:len(m.textoCampo)-n]
		}
		m.busca = strings.TrimSpace(m.textoCampo)
	case tea.KeyRunes, tea.KeySpace:
		m.textoCampo += string(k.Runes)
		m.busca = strings.TrimSpace(m.textoCampo)
	default:
		return m
	}
	m.reselecionar(chave)
	return m
}

// teclaAjuda trata a tela ?: esc ou ? fecham; as setas, a página e g G rolam quando a
// ajuda não cabe.
func (m painelCloud) teclaAjuda(k tea.KeyMsg) painelCloud {
	g, l := ui.DefaultKeys().Global, ui.DefaultKeys().Lista
	switch {
	case key.Matches(k, g.Voltar), key.Matches(k, g.Ajuda):
		m.ajuda = false
	case key.Matches(k, g.Sair):
		m.aviso = fraseComTeclas("`q` sai só da lista; `esc` volta")
	case key.Matches(k, l.Baixo):
		m.rolagemAjuda++
	case key.Matches(k, l.Cima):
		m.rolagemAjuda--
	case key.Matches(k, l.PaginaBaixo):
		m.rolagemAjuda += m.linhasVisiveisDaAjuda()
	case key.Matches(k, l.PaginaCima):
		m.rolagemAjuda -= m.linhasVisiveisDaAjuda()
	case key.Matches(k, l.Inicio):
		m.rolagemAjuda = 0
	case key.Matches(k, l.Fim):
		m.rolagemAjuda = m.maxRolagemAjuda()
	}
	m.rolagemAjuda = max(0, min(m.rolagemAjuda, m.maxRolagemAjuda()))
	return m
}

// mouse: a roda move a seleção (na ajuda, rola); o clique numa linha da tabela a
// seleciona. Com campo ou janela pequena, o mouse não age.
func (m painelCloud) mouse(msg tea.MouseMsg) painelCloud {
	if m.campo || ui.ModoPara(m.largura, m.altura) == ui.ModoJanelaPequena || msg.Action != tea.MouseActionPress {
		return m
	}
	if m.ajuda {
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m.rolagemAjuda = max(0, m.rolagemAjuda-1)
		case tea.MouseButtonWheelDown:
			m.rolagemAjuda = min(m.maxRolagemAjuda(), m.rolagemAjuda+1)
		}
		return m
	}
	n := len(m.visiveis())
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		m.cursor = max(0, m.cursor-1)
	case tea.MouseButtonWheelDown:
		m.cursor = min(n-1, m.cursor+1)
	case tea.MouseButtonLeft:
		if i, ok := m.linhaNoPonto(msg.X, msg.Y); ok {
			m.cursor = i
		}
	}
	return m
}

// linhaNoPonto devolve o cluster (índice em visiveis) desenhado na célula x, y.
func (m painelCloud) linhaNoPonto(x, y int) (int, bool) {
	lay := m.layout()
	if lay == layoutPorCima || !m.temTabela() {
		return 0, false // por cima, a lista fica embaixo do detalhe: o clique não seleciona
	}
	if lay == layoutAoLado {
		if wLista, _ := ui.LargurasLadoALado(m.largura); x >= wLista {
			return 0, false
		}
	}
	// masthead, régua e resumo; no corpo, a linha em branco e o cabeçalho da tabela.
	linha := y - linhasTopo - 2
	if linha < 0 || linha >= m.linhasDeDados() {
		return 0, false
	}
	i := m.rolagem + linha
	if i >= len(m.visiveis()) {
		return 0, false
	}
	return i, true
}

// porCima diz se o detalhe está aberto por cima da lista (abaixo de 120 colunas).
func (m painelCloud) porCima() bool {
	return m.layout() == layoutPorCima
}
