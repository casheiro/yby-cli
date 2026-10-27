package ui

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"
	"unicode/utf8"

	yui "github.com/casheiro/yby-cli/pkg/ui"
	"github.com/casheiro/yby-cli/plugins/viz/internal/monitor"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// Ligação Viz → Bard (folha Atalhos: "b pergunta ao Bard ▸ yby bard · pergunta já
// preenchida com o recurso"). O b monta, sobre a linha selecionada (o mesmo alvo do x),
// uma pergunta com o tipo, o namespace, o nome e o problema que a tela mostra, e roda
// "<yby> bard --pergunta … --context … --namespace … --voltar Viz" com tea.Exec: o Bard
// abre com a pergunta escrita no campo, sem enviar, e o Viz volta quando ele fecha. O x
// tem a mesma ação no menu ("b perguntar ao Bard", folha VizConfirmar).
//
// A saída segue a folha Atalhos ("esc sempre sobe um nível; ctrl+c sai de qualquer
// lugar"): o esc fora da resposta fecha o Bard e volta ao Viz (o yby sai com 0), e o
// ctrl+c em qualquer momento do Bard, na TUI ou ainda carregando, faz o yby sair com 130 e
// fecha o Viz também, como o Viz aberto pelo Cloud.
//
// O Bard é outro plugin, e plugin só roda pelo core, com a verificação de confiança: o
// Viz chama o mesmo yby que o abriu, cujo caminho o core passa em YBY_BIN
// (plugin.EnvBinarioDoCore, no RunInteractive). Sem ele (o binário do Viz chamado direto),
// vale o yby do PATH; sem nenhum dos dois, um toast explica.

// envBinarioDoCore é a variável com o caminho do yby que abriu o Viz (a mesma de
// plugin.EnvBinarioDoCore; o módulo do Viz não importa o executor do core).
const envBinarioDoCore = "YBY_BIN"

// ligacaoBard abre o Bard; os testes trocam as três funções.
type ligacaoBard struct {
	localizar func() (string, error)                          // o yby que abre o Bard
	executar  func(tea.ExecCommand, tea.ExecCallback) tea.Cmd // tea.Exec
	ambiente  func() []string                                 // os.Environ
}

// novaLigacaoBard é a ligação do processo: YBY_BIN, depois o PATH.
func novaLigacaoBard() ligacaoBard {
	return ligacaoBard{
		localizar: func() (string, error) { return localizarYby(os.Getenv, exec.LookPath, existeArquivo) },
		executar:  tea.Exec,
		ambiente:  os.Environ,
	}
}

// existeArquivo diz se o caminho é um arquivo que existe.
func existeArquivo(caminho string) bool {
	st, err := os.Stat(caminho)
	return err == nil && !st.IsDir()
}

// errSemYby é o yby que não está nem em YBY_BIN nem no PATH.
var errSemYby = errors.New("yby não encontrado")

// localizarYby acha o yby que abre o Bard: o que abriu o Viz (YBY_BIN), se o arquivo
// existe, ou o do PATH.
func localizarYby(getenv func(string) string, procurar func(string) (string, error), existe func(string) bool) (string, error) {
	if bin := strings.TrimSpace(getenv(envBinarioDoCore)); bin != "" && existe(bin) {
		return bin, nil
	}
	if bin, err := procurar("yby"); err == nil && bin != "" {
		return bin, nil
	}
	return "", errSemYby
}

// pedidoBard é o que o b manda ao Bard.
type pedidoBard struct {
	pergunta   string
	contexto   string // contexto do kubeconfig do Viz
	namespace  string // o do recurso ou, sem ele (nós), o do Viz
	kubeconfig string // o --kubeconfig do Viz, que vai ao Bard no KUBECONFIG
}

// instrumentoViz é o nome com que o Bard chama o Viz na legenda ("esc voltar ao Viz").
const instrumentoViz = "Viz"

// args são os args do yby: o Bard com a pergunta, o cluster ▸ namespace do Viz e o esc que
// volta ao Viz.
func (p pedidoBard) args() []string {
	a := []string{"bard", "--pergunta", p.pergunta, "--voltar", instrumentoViz}
	if p.contexto != "" {
		a = append(a, "--context", p.contexto)
	}
	if p.namespace != "" {
		a = append(a, "--namespace", p.namespace)
	}
	return a
}

// ambienteDoBard é o ambiente do yby que abre o Bard: o do Viz sem o pedido do core ao
// Viz (o core monta o do Bard) e, com --kubeconfig no Viz, esse arquivo no KUBECONFIG.
func (p pedidoBard) ambienteDoBard(base []string) []string {
	env := make([]string, 0, len(base)+1)
	for _, kv := range base {
		if strings.HasPrefix(kv, "YBY_PLUGIN_REQUEST=") || strings.HasPrefix(kv, "YBY_PLUGIN_REQUEST_FILE=") ||
			(p.kubeconfig != "" && strings.HasPrefix(kv, "KUBECONFIG=")) {
			continue
		}
		env = append(env, kv)
	}
	if p.kubeconfig != "" {
		env = append(env, "KUBECONFIG="+p.kubeconfig)
	}
	return env
}

// comKubeconfig é o client que sabe o --kubeconfig pedido (o K8sClient e o RetryClient).
type comKubeconfig interface {
	Kubeconfig() string
}

// pedidoAoBard monta o pedido sobre a linha selecionada; sem linha, nada.
func (m Model) pedidoAoBard() (pedidoBard, bool) {
	itens := m.itensDaAba(m.activeTab)
	if m.selectedIndex < 0 || m.selectedIndex >= len(itens) {
		return pedidoBard{}, false
	}
	return m.pedidoSobre(itens[m.selectedIndex]), true
}

// pedidoSobre monta o pedido ao Bard sobre o item da aba aberta.
func (m Model) pedidoSobre(it itemLista) pedidoBard {
	p := pedidoBard{
		pergunta:  perguntaSobre(abas[m.activeTab], it, m.agora()),
		contexto:  m.contextoDoCluster(),
		namespace: it.namespace,
	}
	if p.namespace == "" {
		p.namespace = m.filter.Namespace
	}
	if c, ok := m.client.(comKubeconfig); ok {
		p.kubeconfig = c.Kubeconfig()
	}
	return p
}

// notaBard é a nota curta do "b perguntar ao Bard" no menu x: o problema da linha, como
// "por que está 0/2" (folha VizConfirmar), ou o que o Bard vai explicar.
func notaBard(it itemLista, agora time.Time) string {
	switch o := it.obj.(type) {
	case monitor.Pod:
		g, _, _ := classificarPod(o, agora)
		switch {
		case g == grupoFalha:
			return "por que está em " + o.Status
		case g != grupoAlerta:
		case o.Status == "Running" && o.Containers > 0 && o.Prontos < o.Containers:
			return fmt.Sprintf("por que está %d/%d", o.Prontos, o.Containers)
		case o.Status == "Running":
			return "por que reiniciou"
		default:
			return "por que está em " + o.Status
		}
	case monitor.Deployment:
		if o.Ready < o.Replicas {
			return fmt.Sprintf("por que está %d/%d", o.Ready, o.Replicas)
		}
	case monitor.StatefulSet:
		if o.Ready < o.Replicas {
			return fmt.Sprintf("por que está %d/%d", o.Ready, o.Replicas)
		}
	case monitor.Job:
		g, _, st, _ := classificarJob(o)
		switch {
		case st == "Falhou" || (g == grupoAlerta && o.Active > 0):
			return "por que falhou"
		case g == grupoAlerta:
			return "por que está aguardando"
		}
	case monitor.Node:
		if o.Status != "Ready" {
			return "por que está em " + o.Status
		}
	case monitor.Event:
		if o.Type == "Warning" {
			return "o que significa"
		}
		return "explicar o evento"
	}
	return "explicar o estado"
}

// alvoNaPergunta é o recurso como o kubectl o escreve: namespace/nome, ou só o nome.
func alvoNaPergunta(namespace, nome string) string {
	if namespace == "" {
		return nome
	}
	return namespace + "/" + nome
}

// reiniciosNaPergunta é " (14 reinícios)", ou nada sem reinício.
func reiniciosNaPergunta(n int32) string {
	switch {
	case n <= 0:
		return ""
	case n == 1:
		return " (1 reinício)"
	}
	return fmt.Sprintf(" (%d reinícios)", n)
}

// maxMensagemNaPergunta é quantas runas da mensagem de um evento vão na pergunta.
const maxMensagemNaPergunta = 160

// perguntaSobre é a pergunta ao Bard sobre a linha: "Por que…?" com o problema que a
// tela mostra (o mesmo da linha de estado) ou, sem problema, "Explique o estado…".
func perguntaSobre(d defAba, it itemLista, agora time.Time) string {
	alvo := alvoNaPergunta(it.namespace, it.nome)
	explicar := "Explique o estado do " + d.singular + " " + alvo + "."
	switch o := it.obj.(type) {
	case monitor.Pod:
		g, _, _ := classificarPod(o, agora)
		switch {
		case g == grupoFalha:
			return fmt.Sprintf("Por que o pod %s está em %s%s?", alvo, o.Status, reiniciosNaPergunta(o.Reinicios))
		case g != grupoAlerta:
			return explicar
		case o.Status == "Running" && o.Containers > 0 && o.Prontos < o.Containers:
			return fmt.Sprintf("Por que o pod %s está com %d/%d contêineres prontos?", alvo, o.Prontos, o.Containers)
		case o.Status == "Running":
			return fmt.Sprintf("Por que o pod %s reiniciou %s, a última há %s?", alvo, vezes(o.Reinicios), monitor.Idade(o.UltimoReinicio, agora))
		}
		texto := fmt.Sprintf("Por que o pod %s está em %s", alvo, o.Status)
		if idade := monitor.Idade(o.CriadoEm, agora); idade != "" {
			texto += " há " + idade
		}
		return texto + reiniciosNaPergunta(o.Reinicios) + "?"
	case monitor.Deployment:
		return perguntaReplicas(d.singular, alvo, o.Ready, o.Replicas, explicar)
	case monitor.StatefulSet:
		return perguntaReplicas(d.singular, alvo, o.Ready, o.Replicas, explicar)
	case monitor.Job:
		g, _, st, _ := classificarJob(o)
		switch {
		case st == "Falhou":
			return fmt.Sprintf("Por que o job %s falhou %s?", alvo, vezes(o.Failed))
		case g == grupoAlerta && o.Active > 0:
			return fmt.Sprintf("Por que o job %s falhou %s e tenta de novo?", alvo, vezes(o.Failed))
		case g == grupoAlerta:
			return fmt.Sprintf("Por que o job %s está aguardando pods?", alvo)
		}
		return explicar
	case monitor.Node:
		if o.Status != "Ready" {
			return fmt.Sprintf("Por que o nó %s está em %s?", alvo, o.Status)
		}
		return explicar
	case monitor.Event:
		objeto := strings.TrimSpace(strings.ToLower(o.Kind) + " " + alvo)
		if o.Kind == "Node" {
			objeto = "nó " + alvo
		}
		if o.Type == "Warning" {
			return fmt.Sprintf("O que significa o evento %s no %s: \"%s\"?", o.Reason, objeto, cortarRunas(umaLinha(o.Message), maxMensagemNaPergunta))
		}
		return fmt.Sprintf("Explique o evento %s no %s.", o.Reason, objeto)
	}
	return explicar
}

// perguntaReplicas é a pergunta de deployments e statefulsets, pelas réplicas prontas.
func perguntaReplicas(tipo, alvo string, prontas, desejadas int32, explicar string) string {
	switch {
	case desejadas > 0 && prontas == 0:
		return fmt.Sprintf("Por que o %s %s está sem réplicas prontas (0/%d)?", tipo, alvo, desejadas)
	case prontas < desejadas:
		return fmt.Sprintf("Por que o %s %s está com %d/%d réplicas prontas?", tipo, alvo, prontas, desejadas)
	}
	return explicar
}

// cortarRunas corta o texto em n runas, com reticências.
func cortarRunas(s string, n int) string {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return strings.TrimSpace(string([]rune(s)[:n-1])) + yui.G().Reticencias
}

// execBard é o tea.ExecCommand do b: o yby recebe o terminal do Viz (stdin e stdout) e o
// stderr, que também fica guardado para a nota da falha.
type execBard struct {
	cmd   *exec.Cmd
	erros *inicioDoTexto
	fim   *fimDoBard
}

// fimDoBard é o que o Run do yby devolveu: o tea.Exec passa ao callback o erro do Run ou,
// com o Run sem erro, o de devolver o terminal ao Viz.
type fimDoBard struct {
	rodou bool  // o Run voltou (o yby rodou, ou não começou)
	err   error // o erro do Run
}

func (e execBard) Run() error {
	err := e.cmd.Run()
	if e.fim != nil {
		e.fim.rodou, e.fim.err = true, err
	}
	return err
}

func (e execBard) SetStdin(r io.Reader) {
	if e.cmd.Stdin == nil {
		e.cmd.Stdin = r
	}
}

func (e execBard) SetStdout(w io.Writer) {
	if e.cmd.Stdout == nil {
		e.cmd.Stdout = w
	}
}

func (e execBard) SetStderr(w io.Writer) {
	if e.cmd.Stderr == nil {
		e.cmd.Stderr = io.MultiWriter(w, e.erros)
	}
}

// inicioDoTexto guarda os primeiros bytes escritos: o título do erro vem primeiro.
type inicioDoTexto struct{ b bytes.Buffer }

// maxErrosGuardados é quanto do stderr do Bard fica para a nota.
const maxErrosGuardados = 4096

func (t *inicioDoTexto) Write(p []byte) (int, error) {
	if falta := maxErrosGuardados - t.b.Len(); falta > 0 {
		t.b.Write(p[:min(len(p), falta)])
	}
	return len(p), nil
}

// primeiraLinha é a primeira linha com texto, sem cor, sem o glifo de falha e sem o
// código que o ui.WriteError põe à direita.
func (t *inicioDoTexto) primeiraLinha() string {
	if t == nil {
		return ""
	}
	for _, l := range strings.Split(ansi.Strip(t.b.String()), "\n") {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		for _, g := range []string{yui.G().Falha, "✕", "x "} {
			l = strings.TrimSpace(strings.TrimPrefix(l, g))
		}
		// O código (ERR_…) fica à direita, depois de dois espaços ou mais.
		if i := strings.LastIndex(l, "  "); i >= 0 && strings.HasPrefix(strings.TrimSpace(l[i:]), "ERR_") {
			l = strings.TrimSpace(l[:i])
		}
		return l
	}
	return ""
}

// bardFechadoMsg chega quando o yby que abriu o Bard termina.
type bardFechadoMsg struct {
	err   error
	erros *inicioDoTexto
	fim   *fimDoBard
}

// codigoCtrlCBard é o código do yby interrompido por ctrl+c (128 + SIGINT); o core já troca
// por ele o do Windows. statusControlCExit é o do yby que o próprio ctrl+c derrubou no
// Windows antes de tratar o sinal.
const (
	codigoCtrlCBard    = 130
	statusControlCExit = 0xC000013A
)

// saiuPorCtrlC diz se o código de saída do yby é o do ctrl+c.
func saiuPorCtrlC(codigo int) bool {
	return codigo == codigoCtrlCBard || uint32(codigo) == statusControlCExit
}

// perguntarAoBard é o b: abre o Bard com a pergunta sobre a linha selecionada.
func (m Model) perguntarAoBard() (tea.Model, tea.Cmd) {
	p, ok := m.pedidoAoBard()
	if !ok {
		m.actionFeedback = frase(yui.StatusParado, "nada selecionado para perguntar ao Bard")
		return m, nil
	}
	return m.abrirBard(p)
}

// abrirBard roda o yby que abre o Bard com o pedido. Um Bard de cada vez: o b que chega
// antes de o primeiro abrir (toque duplo, tecla repetida) não abre outro Bard depois.
func (m Model) abrirBard(p pedidoBard) (tea.Model, tea.Cmd) {
	if m.bardAbrindo || m.bard.localizar == nil || m.bard.executar == nil {
		return m, nil
	}
	bin, err := m.bard.localizar()
	if err != nil {
		var mostrar tea.Cmd
		m.toast, mostrar = m.toast.MostrarComNota(yui.ToastErro, "não achei o yby para abrir o Bard",
			"abra o Viz pelo yby viz ou ponha o yby no PATH")
		return m, mostrar
	}
	cmd := exec.Command(bin, p.args()...)
	base := os.Environ
	if m.bard.ambiente != nil {
		base = m.bard.ambiente
	}
	cmd.Env = p.ambienteDoBard(base())
	erros, fim := &inicioDoTexto{}, &fimDoBard{}
	m.bardAbrindo = true
	return m, m.bard.executar(execBard{cmd: cmd, erros: erros, fim: fim}, func(err error) tea.Msg {
		return bardFechadoMsg{err: err, erros: erros, fim: fim}
	})
}

// bardFechado religa o mouse (o Bubble Tea o desliga ao soltar o terminal e não o
// religa) e mostra a falha, se houve. O yby que saiu pelo ctrl+c fecha o Viz também (folha
// Atalhos: "ctrl+c sai de qualquer lugar"), como o Viz aberto pelo Cloud; o esc do Bard
// volta com 0.
func (m Model) bardFechado(msg bardFechadoMsg) (tea.Model, tea.Cmd) {
	m.bardAbrindo = false
	if msg.err == nil {
		return m, tea.EnableMouseCellMotion
	}
	var saida *exec.ExitError
	if errors.As(msg.err, &saida) {
		if saiuPorCtrlC(saida.ExitCode()) {
			m.interrompido = true
			return m, tea.Quit
		}
		// O yby rodou e saiu com erro: o Bard não abriu (sem IA, sem terminal) ou caiu
		// depois de abrir. A nota é a primeira linha do erro dele.
		nota := msg.erros.primeiraLinha()
		if nota == "" {
			nota = fmt.Sprintf("o yby saiu com o código %d", saida.ExitCode())
		}
		return m.avisarBard("o Bard fechou com erro", nota)
	}
	if msg.fim != nil && msg.fim.rodou && msg.fim.err == nil {
		// O Bard fechou bem; o erro é o de devolver o terminal ao Viz, que segue.
		slog.Warn("o terminal não voltou inteiro ao Viz depois do Bard", "erro", msg.err)
		return m, tea.EnableMouseCellMotion
	}
	// O yby nem começou (o executável não roda) ou o terminal não foi solto.
	return m.avisarBard("o Bard não abriu", umaLinha(msg.err.Error()))
}

// avisarBard mostra a falha do Bard num toast e religa o mouse.
func (m Model) avisarBard(titulo, nota string) (tea.Model, tea.Cmd) {
	var mostrar tea.Cmd
	m.toast, mostrar = m.toast.MostrarComNota(yui.ToastErro, titulo, nota)
	return m, tea.Batch(tea.EnableMouseCellMotion, mostrar)
}
