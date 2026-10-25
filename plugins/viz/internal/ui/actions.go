package ui

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	yui "github.com/casheiro/yby-cli/pkg/ui"
	"github.com/casheiro/yby-cli/plugins/viz/internal/monitor"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
)

// Menu de ações e confirmação (folha VizConfirmar): x abre, por cima da lista
// apagada, o quadro "AÇÕES · <Tipo> <nome>" com as ações do tipo da linha
// selecionada; ↑↓ e ↵ ou a letra escolhem e esc fecha. Excluir pede o nome do
// recurso (yui.Confirmar destrutivo), reiniciar confirma com ↵ (reversível) e
// escalar pede as réplicas; esc volta ao menu. O resultado sai num yui.Toast
// na linha de estado, e a lista é lida de novo. Perguntar ao Bard é o b da lista
// (bard.go), com a nota do problema da linha.

// tipoAcao é uma ação do menu x.
type tipoAcao int

const (
	acaoReiniciar tipoAcao = iota + 1
	acaoEscalar
	acaoCopiar
	acaoExcluir
	acaoBard
)

// acao é uma entrada do menu x: a tecla, o rótulo e a ação; pergunta diz se
// ela abre um diálogo (o rótulo ganha reticências na tela).
type acao struct {
	tecla, rotulo string
	tipo          tipoAcao
	pergunta      bool
}

var (
	reiniciar = acao{"r", "reiniciar", acaoReiniciar, false}
	escalar   = acao{"s", "escalar", acaoEscalar, true}
	copiar    = acao{"c", "copiar nome", acaoCopiar, false}
	excluir   = acao{"d", "excluir", acaoExcluir, true}
	perguntar = acao{"b", "perguntar ao Bard", acaoBard, false}
)

// rotuloNaTela é o rótulo do menu: "escalar…" (com as reticências do modo
// visual) para a ação que abre um diálogo.
func (a acao) rotuloNaTela() string {
	if a.pergunta {
		return a.rotulo + yui.G().Reticencias
	}
	return a.rotulo
}

// acoesDoTipo são as ações do menu x para o tipo de recurso, na ordem do
// menu (folha VizConfirmar). Excluir vale para os tipos que o
// monitor.DeleteResource apaga; nós e eventos só perguntam ao Bard e copiam o
// nome.
func acoesDoTipo(kind string) []acao {
	switch kind {
	case "deployment", "statefulset":
		return []acao{reiniciar, escalar, perguntar, copiar, excluir}
	case "pod", "job", "service", "configmap", "ingress":
		return []acao{perguntar, copiar, excluir}
	}
	return []acao{perguntar, copiar}
}

// temAcao diz se o tipo tem a ação dada.
func temAcao(kind string, t tipoAcao) bool {
	for _, a := range acoesDoTipo(kind) {
		if a.tipo == t {
			return true
		}
	}
	return false
}

// emX escreve onde a ação foi parar: "x ▸ d".
func emX(tecla string) string {
	return "x " + yui.G().Caminho + " " + tecla
}

// etapaAcao é a camada de ação aberta.
type etapaAcao int

const (
	semAcao     etapaAcao = iota
	noMenu                // o menu x
	confirmando           // excluir (pelo nome) ou reiniciar (↵)
	escalando             // o campo de réplicas
)

// alvoDaAcao é o recurso escolhido quando a ação abriu: a lista pode mudar
// (atualização, busca) até a confirmação, e a ação não pode mudar de alvo.
type alvoDaAcao struct {
	nome, namespace, kind string
	replicas              int32      // réplicas pedidas hoje (deployments e statefulsets)
	pods                  int        // pods do recurso na lista de pods; -1 sem a lista
	dono                  string     // quem controla o pod ("Deployment worker")
	bard                  pedidoBard // o pedido do "b perguntar ao Bard"
	notaBard              string     // a nota dele no menu ("por que está 0/2")
}

// acaoAberta é o menu ou a confirmação na tela.
type acaoAberta struct {
	etapa     etapaAcao
	alvo      alvoDaAcao
	foco      int      // linha do menu com ◆
	tipo      tipoAcao // a ação escolhida no menu
	confirmar yui.Confirmar
	replicas  string // o campo do escalar
	intocado  bool   // o campo ainda tem o valor de hoje: a primeira tecla o troca
	invalido  bool   // o ↵ achou um número que não serve
}

// aberta diz se há menu ou confirmação na tela.
func (a acaoAberta) aberta() bool { return a.etapa != semAcao }

// resultadoAcaoMsg é o resultado de uma ação, para o Toast.
type resultadoAcaoMsg struct {
	texto      string
	nota       string // depois de " · ", em tinta suave ("RBAC")
	falha      bool
	recarregar bool // a ação mudou o cluster: a lista é lida de novo
	doCluster  bool // a resposta da ação mandada ao cluster: a próxima já pode ir
}

// tipoNaTela é o tipo do recurso como o Kubernetes o chama ("Deployment").
func tipoNaTela(kind string) string {
	if kind == "event" {
		return "Evento"
	}
	if t, ok := kindsDaAPI[kind]; ok {
		return t
	}
	return kind
}

// replicasNaTela escreve "1 réplica" ou "3 réplicas".
func replicasNaTela(n int32) string {
	if n == 1 {
		return "1 réplica"
	}
	return strconv.Itoa(int(n)) + " réplicas"
}

// alvoDaLinha guarda o recurso da linha e o que o menu e a confirmação
// mostram dele: as réplicas, os pods do seletor (quando a lista de pods já
// chegou) e o dono do pod.
func (m Model) alvoDaLinha(kind string, it itemLista) alvoDaAcao {
	a := alvoDaAcao{nome: it.nome, namespace: it.namespace, kind: kind, pods: -1}
	seletor := ""
	switch o := it.obj.(type) {
	case monitor.Deployment:
		a.replicas, seletor = o.Replicas, o.Seletor
	case monitor.StatefulSet:
		a.replicas, seletor = o.Replicas, o.Seletor
	case monitor.Pod:
		a.dono = o.Dono
	}
	if seletor != "" && m.carregada(TabPods) {
		if sel, err := labels.Parse(seletor); err == nil {
			a.pods = 0
			for _, p := range m.pods {
				if p.Namespace == a.namespace && sel.Matches(labels.Set(p.Labels)) {
					a.pods++
				}
			}
		}
	}
	return a
}

// abrirAcoes abre o menu x da linha selecionada, guardando o alvo.
func (m Model) abrirAcoes() (tea.Model, tea.Cmd) {
	it, ok := m.itemSelecionado()
	if !ok {
		return m, nil
	}
	alvo := m.alvoDaLinha(abas[m.activeTab].kind, it)
	alvo.bard, alvo.notaBard = m.pedidoSobre(it), notaBard(it, m.agora())
	m.acao = acaoAberta{etapa: noMenu, alvo: alvo}
	return m, nil
}

// fecharAcoes fecha o menu e esquece o alvo.
func (m *Model) fecharAcoes() {
	m.acao = acaoAberta{}
}

// cancelarAcaoSemLista fecha o menu ou a confirmação quando a conexão cai e o
// erro toma o lugar da lista: a View não desenha mais o diálogo, e as teclas
// seguintes não podem confirmar uma ação que a pessoa não vê. A linha de
// estado diz por que o diálogo sumiu. A janela pequena fecha as camadas antes
// (cancelarPelaJanela).
func (m *Model) cancelarAcaoSemLista() {
	m.fecharAcoes()
	m.actionFeedback = frase(yui.StatusParado, "ação cancelada: sem conexão com o cluster")
}

// teclaMenu trata o menu x: ↑↓ movem o ◆, ↵ escolhe a ação focada, a letra
// escolhe direto e esc fecha.
func (m Model) teclaMenu(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	g, l := mapa().Global, mapa().Lista
	acoes := acoesDoTipo(m.acao.alvo.kind)
	switch {
	case key.Matches(k, g.Voltar):
		m.fecharAcoes()
		return m, nil
	case key.Matches(k, g.Sair):
		m.actionFeedback = avisoQ()
		return m, nil
	case key.Matches(k, g.Ajuda):
		m.ajuda, m.ajudaScroll = true, 0
		return m, nil
	case key.Matches(k, l.Baixo):
		m.acao.foco = min(len(acoes)-1, m.acao.foco+1)
		return m, nil
	case key.Matches(k, l.Cima):
		m.acao.foco = max(0, m.acao.foco-1)
		return m, nil
	case k.Type == tea.KeyEnter:
		return m.escolher(acoes[m.acao.foco])
	}
	for _, a := range acoes {
		if k.String() == a.tecla {
			return m.escolher(a)
		}
	}
	return m, nil
}

// escolher abre a confirmação da ação; copiar o nome não pergunta nada. Uma
// ação no cluster por vez: enquanto a anterior não responde, só copiar vale.
func (m Model) escolher(a acao) (tea.Model, tea.Cmd) {
	alvo := m.acao.alvo
	m.acao.tipo = a.tipo
	if a.tipo != acaoCopiar && a.tipo != acaoBard && m.executando != "" {
		m.fecharAcoes()
		m.actionFeedback = frase(yui.StatusParado, "uma ação por vez: "+m.executando+yui.G().Reticencias)
		return m, nil
	}
	switch a.tipo {
	case acaoCopiar:
		m.fecharAcoes()
		return m, m.copiarNome(alvo.nome)
	case acaoBard:
		m.fecharAcoes()
		return m.abrirBard(alvo.bard)
	case acaoEscalar:
		m.acao.etapa = escalando
		m.acao.replicas = strconv.Itoa(int(alvo.replicas))
		m.acao.intocado, m.acao.invalido = true, false
	case acaoReiniciar:
		m.acao.etapa = confirmando
		m.acao.confirmar = yui.NovoConfirmarReversivel("reiniciar "+nomeDoTipo(alvo.kind), "")
	case acaoExcluir:
		m.acao.etapa = confirmando
		m.acao.confirmar = yui.NovoConfirmarDestrutivo("excluir "+nomeDoTipo(alvo.kind), alvo.nome)
	}
	return m, nil
}

// teclaConfirmacao trata a confirmação: esc volta ao menu; ↵ executa quando
// a confirmação libera (no excluir, só com o nome exato); o resto é o texto.
func (m Model) teclaConfirmacao(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.Type {
	case tea.KeyEsc:
		m.acao.etapa = noMenu
		return m, nil
	case tea.KeyEnter:
		if !m.acao.confirmar.Liberado() {
			return m, nil
		}
		tipo, alvo := m.acao.tipo, m.acao.alvo
		m.fecharAcoes()
		return m.mandar(tipo, alvo, 0)
	}
	c, _ := m.acao.confirmar.Update(k)
	m.acao.confirmar = c.(yui.Confirmar)
	return m, nil
}

// teclaEscalar trata o campo de réplicas: começa com o valor de hoje, que a
// primeira tecla troca; ↵ escala com um inteiro de 0 para cima.
func (m Model) teclaEscalar(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.Type {
	case tea.KeyEsc:
		m.acao.etapa = noMenu
		return m, nil
	case tea.KeyEnter:
		n, err := strconv.ParseInt(strings.TrimSpace(m.acao.replicas), 10, 32)
		if err != nil || n < 0 {
			m.acao.invalido = true
			return m, nil
		}
		alvo := m.acao.alvo
		m.fecharAcoes()
		return m.mandar(acaoEscalar, alvo, int32(n))
	case tea.KeyBackspace:
		if m.acao.intocado {
			m.acao.replicas = ""
		} else if _, n := utf8.DecodeLastRuneInString(m.acao.replicas); n > 0 {
			m.acao.replicas = m.acao.replicas[:len(m.acao.replicas)-n]
		}
	case tea.KeyRunes:
		if m.acao.intocado {
			m.acao.replicas = ""
		}
		m.acao.replicas += string(k.Runes)
	default:
		return m, nil
	}
	m.acao.intocado, m.acao.invalido = false, false
	return m, nil
}

// errSemAcesso é o erro do client que só lista (sem acesso direto à API).
var errSemAcesso = errors.New("este client não alcança a API do cluster")

// O verbo, o gerúndio (linha de estado enquanto a ação roda) e o particípio
// (frase da falha) de cada ação no cluster.
var (
	verbos      = map[tipoAcao]string{acaoExcluir: "excluir", acaoEscalar: "escalar", acaoReiniciar: "reiniciar"}
	gerundios   = map[tipoAcao]string{acaoExcluir: "excluindo", acaoEscalar: "escalando", acaoReiniciar: "reiniciando"}
	participios = map[tipoAcao]string{acaoExcluir: "excluído", acaoEscalar: "escalado", acaoReiniciar: "reiniciado"}
)

// mandar manda a ação ao cluster e marca, na linha de estado, que ela está em
// andamento até a resposta chegar.
func (m Model) mandar(tipo tipoAcao, alvo alvoDaAcao, replicas int32) (tea.Model, tea.Cmd) {
	m.executando = gerundios[tipo] + " " + tipoNaTela(alvo.kind) + " " + alvo.nome
	return m, m.executar(tipo, alvo, replicas)
}

// executar devolve o comando que faz a ação no cluster e diz o resultado.
func (m Model) executar(tipo tipoAcao, alvo alvoDaAcao, replicas int32) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		msg := fazer(client, tipo, alvo, replicas)
		msg.doCluster = true
		return msg
	}
}

// fazer executa a ação no cluster (o monitor dá a ela o PrazoDaAcao).
func fazer(client monitor.Client, tipo tipoAcao, alvo alvoDaAcao, replicas int32) resultadoAcaoMsg {
	rotulo := tipoNaTela(alvo.kind) + " " + alvo.nome
	falhou := func(err error) resultadoAcaoMsg {
		slog.Debug("viz: ação falhou", "acao", verbos[tipo], "recurso", rotulo, "namespace", alvo.namespace, "erro", err)
		texto, nota := fraseDaFalha(tipo, rotulo, err)
		return resultadoAcaoMsg{texto: texto, nota: nota, falha: true, recarregar: true}
	}
	k := resolveK8sClient(client)
	if k == nil {
		return falhou(errSemAcesso)
	}
	cs := k.Clientset()
	sts := alvo.kind == "statefulset"
	switch tipo {
	case acaoExcluir:
		if err := monitor.DeleteResource(cs, alvo.kind, alvo.nome, alvo.namespace); err != nil {
			return falhou(err)
		}
		return resultadoAcaoMsg{texto: rotulo + " excluído", recarregar: true}
	case acaoEscalar:
		escalarFn := monitor.ScaleDeployment
		if sts {
			escalarFn = monitor.ScaleStatefulSet
		}
		if err := escalarFn(cs, alvo.nome, alvo.namespace, replicas); err != nil {
			return falhou(err)
		}
		return resultadoAcaoMsg{texto: rotulo + " escalado para " + replicasNaTela(replicas), recarregar: true}
	case acaoReiniciar:
		reiniciarFn := monitor.RestartDeployment
		if sts {
			reiniciarFn = monitor.RestartStatefulSet
		}
		if err := reiniciarFn(cs, alvo.nome, alvo.namespace); err != nil {
			return falhou(err)
		}
		return resultadoAcaoMsg{texto: rotulo + " reiniciado", recarregar: true}
	}
	return resultadoAcaoMsg{texto: "ação desconhecida", falha: true}
}

// fraseDaFalha resume a falha da ação para o Toast, como na prancha: "sem
// permissão para escalar · RBAC". O texto da API (em inglês, longo) vai só
// para o log em debug; o erro sem frase própria mostra a causa de dentro, sem
// os embrulhos do monitor (que repetiriam o verbo).
func fraseDaFalha(tipo tipoAcao, rotulo string, err error) (texto, nota string) {
	naoFoi := rotulo + " não foi " + participios[tipo]
	var rede net.Error
	switch {
	case errors.Is(err, errSemAcesso):
		return "não deu para " + verbos[tipo] + " " + rotulo, err.Error()
	case apierrors.IsForbidden(err), apierrors.IsUnauthorized(err):
		return "sem permissão para " + verbos[tipo], "RBAC"
	case apierrors.IsNotFound(err):
		return rotulo + " não existe mais", ""
	case apierrors.IsConflict(err):
		return naoFoi, "ele mudou no meio; tente de novo"
	case errors.Is(err, context.DeadlineExceeded), apierrors.IsTimeout(err), apierrors.IsServerTimeout(err),
		errors.As(err, &rede) && rede.Timeout():
		return naoFoi, "o cluster não respondeu a tempo"
	case errors.As(err, &rede):
		return naoFoi, "sem conexão com o cluster"
	}
	causa := err
	for u := errors.Unwrap(causa); u != nil; u = errors.Unwrap(causa) {
		causa = u
	}
	return "não deu para " + verbos[tipo] + " " + rotulo, causa.Error()
}

// copiarNome manda o nome para a área de transferência (OSC 52, como o ctrl+y
// do Bard). O terminal não responde ao OSC 52: o texto foi enviado, não
// necessariamente copiado.
func (m Model) copiarNome(nome string) tea.Cmd {
	copiarFn := m.copiar
	return func() tea.Msg {
		if copiarFn == nil {
			copiarFn = yui.CopiarOSC52
		}
		if err := copiarFn(nome); err != nil {
			return resultadoAcaoMsg{texto: "não deu para copiar o nome: " + err.Error(), falha: true}
		}
		return resultadoAcaoMsg{texto: "nome " + nome + " enviado à área de transferência"}
	}
}

// mostrarResultado põe o resultado no Toast e, se a ação mudou o cluster, lê
// a lista de novo.
func (m Model) mostrarResultado(msg resultadoAcaoMsg) (tea.Model, tea.Cmd) {
	tipo := yui.ToastOk
	if msg.falha {
		tipo = yui.ToastErro
	}
	m.actionFeedback = "" // o resultado é a frase mais nova
	if msg.doCluster {
		m.executando = ""
	}
	var mostrar tea.Cmd
	m.toast, mostrar = m.toast.MostrarComNota(tipo, umaLinha(msg.texto), umaLinha(msg.nota))
	if !msg.recarregar {
		return m, mostrar
	}
	return m, tea.Batch(mostrar, m.ler(false, m.activeTab))
}

// Desenho do menu e das confirmações.

// notaDaAcao é a nota curta à direita da ação no menu.
func notaDaAcao(a acao, alvo alvoDaAcao) string {
	switch a.tipo {
	case acaoReiniciar:
		if alvo.kind == "statefulset" {
			return "rollout restart, um pod por vez"
		}
		return "rollout restart, sem downtime"
	case acaoEscalar:
		return "hoje " + replicasNaTela(alvo.replicas)
	case acaoExcluir:
		return "pede confirmação"
	case acaoBard:
		return alvo.notaBard
	}
	return ""
}

// larguraRotuloMenu é a coluna do rótulo da ação no menu, como na prancha. Um
// rótulo mais largo ("perguntar ao Bard") alarga a coluna do menu inteiro, com
// dois espaços até a nota, para as notas seguirem alinhadas.
const larguraRotuloMenu = 16

// quadroMenu é o quadro "AÇÕES · Deployment worker" com uma linha por ação:
// ◆ na focada (em tinta invertida) e ◇ nas outras; excluir em cor de erro.
func (m Model) quadroMenu(largura int) string {
	s, g := yui.S(), yui.G()
	alvo := m.acao.alvo
	acoes := acoesDoTipo(alvo.kind)
	type linha struct{ plano, estilizado string }
	linhas := make([]linha, len(acoes))
	interno := 0
	coluna := larguraRotuloMenu
	for _, a := range acoes {
		coluna = max(coluna, lipgloss.Width(a.rotuloNaTela())+2)
	}
	for i, a := range acoes {
		rotulo := a.rotuloNaTela()
		rotulo += strings.Repeat(" ", max(1, coluna-lipgloss.Width(rotulo)))
		nota := notaDaAcao(a, alvo)
		marca, tecla, nome := s.TintaSuave.Render(g.Pendente), s.Negrito.Render(a.tecla), s.Tinta.Render(rotulo)
		if a.tipo == acaoExcluir {
			marca, tecla, nome = s.Erro.Render(g.Pendente), s.Erro.Bold(true).Render(a.tecla), s.Erro.Render(rotulo)
		}
		linhas[i] = linha{
			plano:      g.Pendente + " " + a.tecla + "  " + rotulo + nota,
			estilizado: marca + " " + tecla + "  " + nome + s.TintaSuave.Render(nota),
		}
		interno = max(interno, lipgloss.Width(linhas[i].plano))
	}
	titulo := tipoNaTela(alvo.kind) + " " + alvo.nome
	interno = max(interno, lipgloss.Width("AÇÕES · "+titulo)+2) // o rótulo no fio de cima também cabe
	largura = min(largura, interno+4)
	interno = largura - 4
	corpo := make([]string, len(linhas))
	for i, l := range linhas {
		corpo[i] = l.estilizado
		if i == m.acao.foco {
			plano := g.Ativo + strings.TrimPrefix(l.plano, g.Pendente)
			plano = ansi.Truncate(plano, interno, g.Reticencias)
			corpo[i] = s.Selecionado.Render(plano + strings.Repeat(" ", max(0, interno-lipgloss.Width(plano))))
		}
	}
	return yui.QuadroNomeado("ações", titulo, strings.Join(corpo, "\n"), largura, yui.QuadroFoco)
}

// resumoDoAlvo é a linha do recurso na confirmação: "worker em payments · 2
// réplicas · 2 pods", com o nome em negrito.
func resumoDoAlvo(alvo alvoDaAcao, comPods bool) string {
	s := yui.S()
	txt := ""
	if alvo.namespace != "" {
		txt = " em " + alvo.namespace
	}
	switch alvo.kind {
	case "deployment", "statefulset":
		txt += " · " + replicasNaTela(alvo.replicas)
		if comPods && alvo.pods >= 0 {
			txt += " · " + abas[TabPods].contar(alvo.pods)
		}
	case "pod":
		if alvo.dono != "" {
			txt += " · controlado por " + alvo.dono
		}
	}
	return s.Negrito.Render(alvo.nome) + s.Tinta.Render(txt)
}

// argoCD é o lembrete de que o recurso apagado volta se o Argo CD o sincroniza.
const argoCD = "Se ele vem do Argo CD, a próxima sincronização cria de novo."

// explicacaoExcluir diz o que acontece quando o recurso é apagado.
func explicacaoExcluir(alvo alvoDaAcao) string {
	switch alvo.kind {
	case "deployment":
		return "Os pods param e o Deployment some do cluster. " + argoCD
	case "statefulset":
		return "Os pods param e o StatefulSet some do cluster; os volumes (PVCs) ficam. " + argoCD
	case "pod":
		if alvo.dono != "" {
			return "O pod para e o " + alvo.dono + " cria outro no lugar."
		}
		return "O pod para e não volta: nenhum controlador cria outro."
	case "job":
		return "O Job e os pods dele somem do cluster. Se ele vem de um CronJob, a próxima execução cria outro."
	case "service":
		return "O Service some e o tráfego para os pods dele para. " + argoCD
	case "configmap":
		return "O ConfigMap some do cluster; os pods que o usam falham ao subir de novo. " + argoCD
	case "ingress":
		return "O Ingress some e as rotas dele param de responder. " + argoCD
	}
	return "O " + tipoNaTela(alvo.kind) + " some do cluster."
}

// explicacaoReiniciar diz o que o rollout restart faz.
func explicacaoReiniciar(alvo alvoDaAcao) string {
	if alvo.kind == "statefulset" {
		return "Rollout restart: os pods do StatefulSet reiniciam um por vez, do último para o primeiro."
	}
	return "Rollout restart: o Deployment sobe pods novos e só então para os antigos, sem downtime."
}

// quebrar quebra o texto na largura, em linhas de tinta suave.
func quebrar(texto string, largura int) []string {
	var linhas []string
	for _, l := range strings.Split(ansi.Wrap(texto, max(1, largura), ""), "\n") {
		linhas = append(linhas, yui.S().TintaSuave.Render(l))
	}
	return linhas
}

// quadroConfirmacao é o diálogo da ação escolhida na largura dada; completo
// cabe a explicação (sem espaço, ela sai).
func (m Model) quadroConfirmacao(largura int, completo bool) string {
	alvo := m.acao.alvo
	switch m.acao.etapa {
	case escalando:
		return m.quadroEscalar(largura, completo)
	case confirmando:
		if m.acao.tipo == acaoReiniciar {
			linhas := []string{resumoDoAlvo(alvo, completo)}
			if completo {
				linhas = append(linhas, quebrar(explicacaoReiniciar(alvo), largura-4)...)
			}
			linhas = append(linhas, "") // a dica do ↵ fica separada, como os botões do excluir
			c := yui.NovoConfirmarReversivel("reiniciar "+nomeDoTipo(alvo.kind), strings.Join(linhas, "\n"))
			v, _ := c.Update(tea.WindowSizeMsg{Width: largura})
			return v.View()
		}
		explicacao := ""
		if completo {
			explicacao = explicacaoExcluir(alvo)
		}
		c := m.acao.confirmar.ComDetalhes(resumoDoAlvo(alvo, completo), explicacao, "excluir")
		v, _ := c.Update(tea.WindowSizeMsg{Width: largura})
		return v.View()
	}
	return ""
}

// quadroEscalar é o campo de réplicas com o valor de hoje (em tinta invertida
// até a primeira tecla) e os botões; o número inválido avisa ao lado do campo.
func (m Model) quadroEscalar(largura int, completo bool) string {
	s, g := yui.S(), yui.G()
	alvo := m.acao.alvo
	largura = min(largura, larguraDoDialogo)
	interno := max(1, largura-4)
	valor := s.Tinta.Render(m.acao.replicas) + cursor()
	if m.acao.intocado {
		valor = s.Selecionado.Render(m.acao.replicas)
	}
	aviso := ""
	if m.acao.invalido {
		aviso = s.Aviso.Render(g.Alerta) + " " + s.TintaSuave.Render("um número inteiro, 0 ou mais")
	}
	linhas := []string{
		resumoDoAlvo(alvo, completo),
		"",
		s.TintaSuave.Render("Quantas réplicas?"),
		esqDir(s.Tinta.Render(g.Prompt)+" "+valor, aviso, interno),
		"",
		s.TintaSuave.Render("[ esc cancelar ]") + "   " + s.Negrito.Render("[ "+yui.RotuloTecla("↵")+" escalar ]"),
	}
	if !completo {
		linhas = append(linhas[:1], linhas[2:]...)
	}
	return yui.Quadro("escalar "+nomeDoTipo(alvo.kind), strings.Join(linhas, "\n"), largura, yui.QuadroFoco)
}

// larguraDoDialogo é a largura máxima das confirmações (a do yui.Confirmar).
const larguraDoDialogo = 72

// reset encerra os estilos de um pedaço cortado de uma linha com cor.
const reset = "\x1b[0m"

// camada desenha o quadro da ação por cima da lista apagada (em tinta de
// fio, sem as cores dela), a partir da coluna x e da linha y do corpo; as
// linhas manter (no menu, o cabeçalho e a selecionada) ficam com as cores delas.
func camada(fundo, quadro string, x, y, altura int, manter ...int) string {
	fio := yui.S().Fio
	linhas := strings.Split(fundo, "\n")
	for len(linhas) < altura {
		linhas = append(linhas, "")
	}
	q := strings.Split(quadro, "\n")
	for i, l := range linhas {
		// pintar mantém a largura: os espaços do fim ficam sem estilo.
		pintar := func(t string) string {
			if slices.Contains(manter, i) && t != "" {
				return t + reset
			}
			plano := ansi.Strip(t)
			texto := strings.TrimRight(plano, " ")
			if texto == "" {
				return plano
			}
			return fio.Render(texto) + plano[len(texto):]
		}
		j := i - y
		if j < 0 || j >= len(q) {
			linhas[i] = pintar(l)
			continue
		}
		wq := lipgloss.Width(q[j])
		esq := ansi.Cut(l, 0, x)
		falta := strings.Repeat(" ", max(0, x-ansi.StringWidth(esq)))
		dir := ansi.Cut(l, x+wq, max(x+wq, ansi.StringWidth(l)))
		linhas[i] = pintar(esq) + falta + q[j] + pintar(dir)
	}
	return strings.Join(linhas, "\n")
}

// apagado é o texto com cada linha em tinta de fio: o painel atrás da camada.
func apagado(texto string) string {
	if texto == "" {
		return ""
	}
	return camada(texto, "", 0, -1, 0)
}

// corpoComAcao é a lista apagada com o menu ou a confirmação por cima; o
// painel ao lado fica apagado, e o por cima sai: a lista atrás ocupa o corpo
// todo, com a geometria de quando o painel está fechado.
func (m Model) corpoComAcao(itens []itemLista) (lista, detalhe string) {
	altura := m.alturaCorpo()
	largura := m.larguraDaLista()
	f := m
	if f.painelPorCima() {
		f.detalheAberto = false
		f.seguirSelecao()
	}
	fundo := f.corpoLista(itens)
	if m.painelAoLado() {
		detalhe = apagado(m.detalheNaTela())
	}
	var quadro string
	if m.acao.etapa == noMenu {
		quadro = m.quadroMenu(largura)
	} else {
		quadro = m.quadroConfirmacao(min(largura, larguraDoDialogo), true)
		if strings.Count(quadro, "\n")+1 > altura-1 {
			quadro = m.quadroConfirmacao(min(largura, larguraDoDialogo), false)
		}
	}
	w := lipgloss.Width(quadro)
	x := max(0, (largura-w)/2)
	var manter []int
	if m.acao.etapa == noMenu && len(itens) > 0 {
		// A lista fica à vista à esquerda, com o cabeçalho em tinta suave e a
		// linha da ação ainda selecionada.
		x = max(0, min(largura/3, largura-w))
		antes := f.linhasAntesDaTabela()
		manter = []int{antes - 1, antes + f.selectedIndex - f.scrollY}
	}
	return camada(fundo, quadro, x, 1, altura, manter...), detalhe
}

// teclasDaAcao é a legenda do menu e das confirmações.
func (m Model) teclasDaAcao() []key.Binding {
	cancelar := atalho("esc", "cancelar", "esc")
	switch m.acao.etapa {
	case noMenu:
		t := []key.Binding{atalho(setas(), "mover", "up", "down", "j", "k"), atalho("↵", "escolher", "enter")}
		fechar := atalho("esc", "fechar", "esc")
		if m.width < yui.LarguraColuna {
			// A legenda compacta guarda só três teclas: a saída do menu vem primeiro.
			return append([]key.Binding{fechar}, t...)
		}
		return append(t, fechar)
	case escalando:
		return []key.Binding{atalho("↵", "escalar", "enter"), cancelar}
	}
	verbo := "excluir"
	if m.acao.tipo == acaoReiniciar {
		verbo = "reiniciar"
	}
	return []key.Binding{atalho("↵", verbo, "enter"), cancelar}
}
