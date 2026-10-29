package cmd

import (
	stderrors "errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/casheiro/yby-cli/v5/pkg/errors"
	"github.com/casheiro/yby-cli/v5/pkg/plugin"
	"github.com/casheiro/yby-cli/v5/pkg/ui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"k8s.io/client-go/tools/clientcmd"
)

// O ↵ do painel abre o Viz no cluster da linha (folha Atalhos: "↵ abre no Viz") e, quando
// o Viz fecha, o painel volta. O contexto do kubeconfig sai, sem mexer no kubeconfig, do
// ambiente do projeto que aponta para o cluster ou do nome que o ConfigureKubeconfig do
// provider costuma dar; se nenhum estiver no kubeconfig, a linha de estado mostra o
// yby cloud connect que o cria.

// alvoViz é onde o Viz abre: o kubeconfig, o contexto e o namespace inicial.
type alvoViz struct {
	kubeconfig string // o kube_config do ambiente; vazio, o padrão (KUBECONFIG ou ~/.kube/config)
	contexto   string
	namespace  string // do ambiente do projeto; vazio, o Viz usa o do contexto
}

// args são os args do hook command do Viz.
func (a alvoViz) args() []string {
	var args []string
	if a.kubeconfig != "" {
		args = append(args, "--kubeconfig", a.kubeconfig)
	}
	args = append(args, "--context", a.contexto)
	if a.namespace != "" {
		args = append(args, "--namespace", a.namespace)
	}
	return args
}

// contextosCandidatos são os nomes que o contexto do cluster pode ter no kubeconfig, na
// ordem de preferência: o kube_context do ambiente do projeto; o que o update-kubeconfig
// da AWS dá (o ARN do cluster) ou o get-credentials do GKE (gke_<projeto>_<região>_<nome>);
// e o nome do cluster, que é o que o az aks get-credentials dá.
func contextosCandidatos(c clusterPainel) []string {
	var nomes []string
	if c.kubeContexto != "" {
		nomes = append(nomes, c.kubeContexto)
	}
	i := c.info
	switch i.Provider {
	case "aws":
		if i.ARN != "" {
			nomes = append(nomes, i.ARN)
		} else if c.conta != "" && i.Region != "" {
			nomes = append(nomes, fmt.Sprintf("arn:aws:eks:%s:%s:cluster/%s", i.Region, c.conta, i.Name))
		}
	case "gcp":
		projeto := i.ProjectID
		if projeto == "" {
			projeto = c.conta
		}
		if projeto != "" && i.Region != "" {
			nomes = append(nomes, "gke_"+projeto+"_"+i.Region+"_"+i.Name)
		}
	}
	return append(nomes, i.Name)
}

// alvoDoViz escolhe o primeiro candidato que existe no kubeconfig; false se nenhum existe.
// No GKE sem projeto conhecido, vale o único contexto gke_<projeto>_<região>_<nome>.
func alvoDoViz(c clusterPainel, contextos map[string]bool) (alvoViz, bool) {
	for _, nome := range contextosCandidatos(c) {
		if contextos[nome] {
			return alvoViz{contexto: nome, namespace: c.namespace}, true
		}
	}
	if c.info.Provider == "gcp" && c.info.Region != "" {
		var achados []string
		sufixo := "_" + c.info.Region + "_" + c.info.Name
		for nome := range contextos {
			if strings.HasPrefix(nome, "gke_") && strings.HasSuffix(nome, sufixo) &&
				!strings.Contains(strings.TrimSuffix(strings.TrimPrefix(nome, "gke_"), sufixo), "_") {
				achados = append(achados, nome)
			}
		}
		if len(achados) == 1 { // em dois projetos, não dá para saber qual
			return alvoViz{contexto: achados[0], namespace: c.namespace}, true
		}
	}
	return alvoViz{}, false
}

// comandoConectar é o yby cloud connect que põe o cluster no kubeconfig.
func comandoConectar(c clusterPainel) string {
	partes := []string{"yby cloud connect --provider", c.info.Provider}
	if c.info.Region != "" {
		partes = append(partes, "--region", c.info.Region)
	}
	partes = append(partes, "--cluster", c.info.Name)
	return strings.Join(partes, " ")
}

// contextosDoKubeconfig lê os nomes dos contextos do kubeconfig, sem alterar nada: o
// arquivo dado ou, vazio, o padrão (KUBECONFIG ou ~/.kube/config), como o kubectl.
func contextosDoKubeconfig(caminho string) (map[string]bool, error) {
	regras := clientcmd.NewDefaultClientConfigLoadingRules()
	regras.ExplicitPath = caminho
	raw, err := regras.Load()
	if err != nil {
		return nil, err
	}
	nomes := make(map[string]bool, len(raw.Contexts))
	for nome := range raw.Contexts {
		nomes[nome] = true
	}
	return nomes, nil
}

// pluginsDescobertos é o Manager que o Execute já descobriu (com a verificação de
// confiança), reaproveitado pelo ↵: descobrir de novo roda o manifest de cada plugin, com o
// terminal solto, a cada abertura.
var pluginsDescobertos *plugin.Manager

// abrirVizPeloManager roda o hook command do Viz pelo Manager de plugins, como o yby viz.
func abrirVizPeloManager(args []string) error {
	pm := pluginsDescobertos
	if pm == nil {
		pm = newRootPluginManager()
		pm.EnableTrustCheck()
		if err := pm.Discover(); err != nil {
			return errors.Wrap(err, errors.ErrCodePlugin, "não deu para procurar os plugins")
		}
		pluginsDescobertos = pm
	}
	// O cluster vai nos args (--context); o ambiente do yby não entra.
	pm.DefinirAmbiente("")
	return pm.ExecuteCommandHook("viz", args)
}

// execViz é o tea.ExecCommand do ↵: o Bubble Tea solta o terminal, o Viz roda e o painel
// volta quando ele fecha. O RunInteractive liga o plugin direto ao stdin e ao stdout do
// processo, que são os do painel; por isso os Set* não guardam nada.
type execViz struct {
	args  []string
	abrir func(args []string) error
}

func (e execViz) Run() error        { return e.abrir(e.args) }
func (execViz) SetStdin(io.Reader)  {}
func (execViz) SetStdout(io.Writer) {}
func (execViz) SetStderr(io.Writer) {}

// vizFechadoMsg chega quando o Viz fecha.
type vizFechadoMsg struct {
	nome string
	err  error
}

// abrirNoViz abre o Viz no cluster selecionado; sem contexto no kubeconfig, avisa na
// linha de estado o comando que conecta.
func (m painelCloud) abrirNoViz() (painelCloud, tea.Cmd) {
	c, ok := m.selecionado()
	if !ok || m.fontes.abrirViz == nil {
		return m, nil
	}
	exec, aviso := m.comandoDoViz(c)
	if aviso != "" {
		m.aviso = aviso
		return m, nil
	}
	nome := c.info.Name
	executar := m.fontes.executar
	if executar == nil {
		executar = tea.Exec
	}
	return m, executar(exec, func(err error) tea.Msg {
		return vizFechadoMsg{nome: nome, err: err}
	})
}

// comandoDoViz monta o Viz do cluster; sem o contexto no kubeconfig (ou sem conseguir
// lê-lo), devolve a frase da linha de estado no lugar. O ambiente com kube_config próprio
// abre o Viz nesse arquivo quando o kube_context está nele; senão vale o kubeconfig padrão,
// que é onde o yby cloud connect grava.
func (m painelCloud) comandoDoViz(c clusterPainel) (execViz, string) {
	ler := m.fontes.contextos
	if ler == nil {
		ler = func(string) (map[string]bool, error) { return map[string]bool{}, nil }
	}
	if c.kubeconfig != "" && c.kubeContexto != "" {
		lidos, err := ler(c.kubeconfig)
		if err == nil && lidos[c.kubeContexto] {
			alvo := alvoViz{kubeconfig: c.kubeconfig, contexto: c.kubeContexto, namespace: c.namespace}
			return execViz{args: alvo.args(), abrir: m.fontes.abrirViz}, ""
		}
		slog.Debug("cloud.dashboard.kubeconfig_do_ambiente", "arquivo", c.kubeconfig, "contexto", c.kubeContexto, "erro", err)
	}
	contextos, err := ler("")
	if err != nil {
		return execViz{}, fraseErro("não deu para ler o kubeconfig: " + resumoDoErro(err))
	}
	alvo, ok := alvoDoViz(c, contextos)
	if !ok {
		return execViz{}, fraseSemContexto(c, m.largura)
	}
	return execViz{args: alvo.args(), abrir: m.fontes.abrirViz}, ""
}

// fraseSemContexto é o aviso do cluster fora do kubeconfig, com o comando que conecta.
// Quando a frase inteira não cabe ao lado da hora, sai só o comando, que é o que a pessoa
// copia; se nem ele cabe com a hora, a View tira a hora.
func fraseSemContexto(c clusterPainel, largura int) string {
	s, g := ui.S(), ui.G()
	comando := s.Negrito.Render(comandoConectar(c))
	inteira := s.Aviso.Render(g.Alerta) + " " + s.Tinta.Render(c.info.Name+" não está no kubeconfig: ") + comando
	if lipgloss.Width(inteira)+1+len("atualizado 15:04:05") <= largura {
		return inteira
	}
	return s.Aviso.Render(g.Alerta) + " " + comando
}

// codigoCtrlC é o código com que o Viz sai no ctrl+c (128 + SIGINT, como o Bard).
const codigoCtrlC = 130

// vizFechado religa o mouse (o Bubble Tea o desliga ao soltar o terminal e não o religa)
// e mostra a falha, se houve. O ctrl+c no Viz sai de tudo (folha Atalhos: "ctrl+c sai de
// qualquer lugar"), então fecha o painel também.
func (m painelCloud) vizFechado(msg vizFechadoMsg) (painelCloud, tea.Cmd) {
	if msg.err == nil {
		return m, tea.EnableMouseCellMotion
	}
	var saida *plugin.SaidaDoPlugin
	if stderrors.As(msg.err, &saida) && saida.Codigo == codigoCtrlC {
		return m, tea.Quit
	}
	var cmd tea.Cmd
	m.aviso = ""
	m.toast, cmd = m.toast.MostrarComNota(ui.ToastErro, "não deu para abrir o Viz em "+msg.nome, notaDoViz(msg.err))
	return m, tea.Batch(tea.EnableMouseCellMotion, cmd)
}

// notaDoViz é a nota curta da falha do Viz.
func notaDoViz(err error) string {
	var saida *plugin.SaidaDoPlugin
	if stderrors.As(err, &saida) {
		return fmt.Sprintf("o Viz saiu com o código %d", saida.Codigo)
	}
	var ye *errors.YbyError
	if !stderrors.As(err, &ye) {
		return resumoDoErro(err)
	}
	if ye.Code == errors.ErrCodePluginNotFound {
		return "instale com yby plugin install viz"
	}
	// O Error() do YbyError traz o código entre colchetes e perde a dica (ex.: a do
	// yby plugin trust no plugin que mudou de checksum). Na linha de estado cabe pouco: vai
	// a dica, que diz o que fazer; a mensagem inteira fica no log.
	slog.Debug("cloud.dashboard.viz", "erro", ye.Error(), "dica", ye.GetHint())
	if dica := ye.GetHint(); dica != "" {
		r, n := utf8.DecodeRuneInString(dica)
		return string(unicode.ToLower(r)) + dica[n:]
	}
	return ye.Message
}
