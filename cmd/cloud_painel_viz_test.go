package cmd

import (
	stderrors "errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/casheiro/yby-cli/v5/pkg/cloud"
	"github.com/casheiro/yby-cli/v5/pkg/errors"
	"github.com/casheiro/yby-cli/v5/pkg/plugin"
	"github.com/casheiro/yby-cli/v5/pkg/ui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// arnStaging é o contexto que o aws eks update-kubeconfig dá ao staging da prancha.
const arnStaging = "arn:aws:eks:us-east-1:482100009310:cluster/staging"

// conectarStaging é o comando que põe o staging da prancha no kubeconfig.
const conectarStaging = "yby cloud connect --provider aws --region us-east-1 --cluster staging"

// TestAlvoDoViz: o contexto e os args do Viz para cada caso — o ambiente do projeto, o
// nome que o provider dá e o contexto ausente.
func TestAlvoDoViz(t *testing.T) {
	staging := cloud.ClusterInfo{Provider: "aws", Name: "staging", Region: "us-east-1"}
	gkeSemProjeto := cloud.ClusterInfo{Provider: "gcp", Name: "gke-ml", Region: "us-central1"}
	casos := []struct {
		nome      string
		cluster   clusterPainel
		contextos []string
		args      []string // nil: sem contexto no kubeconfig
	}{
		{
			nome:      "ambiente do projeto com kube_context e namespace",
			cluster:   clusterPainel{info: staging, conta: "482100009310", kubeContexto: "yby-staging", namespace: "payments"},
			contextos: []string{"yby-staging", arnStaging},
			args:      []string{"--context", "yby-staging", "--namespace", "payments"},
		},
		{
			nome:      "kube_context do ambiente fora do kubeconfig: vale o nome do provider, com o namespace do ambiente",
			cluster:   clusterPainel{info: staging, conta: "482100009310", kubeContexto: "sumiu", namespace: "payments"},
			contextos: []string{arnStaging},
			args:      []string{"--context", arnStaging, "--namespace", "payments"},
		},
		{
			nome:      "aws: o ARN do describe-cluster",
			cluster:   clusterPainel{info: cloud.ClusterInfo{Provider: "aws", Name: "prod-core", Region: "us-east-1", ARN: "arn:aws:eks:us-east-1:111:cluster/prod-core"}},
			contextos: []string{"arn:aws:eks:us-east-1:111:cluster/prod-core"},
			args:      []string{"--context", "arn:aws:eks:us-east-1:111:cluster/prod-core"},
		},
		{
			nome:      "aws sem ARN: montado com a conta e a região",
			cluster:   clusterPainel{info: staging, conta: "482100009310"},
			contextos: []string{"outro", arnStaging},
			args:      []string{"--context", arnStaging},
		},
		{
			nome:      "azure: o nome do cluster",
			cluster:   clusterPainel{info: cloud.ClusterInfo{Provider: "azure", Name: "aks-dados", Region: "brazilsouth"}},
			contextos: []string{"aks-dados"},
			args:      []string{"--context", "aks-dados"},
		},
		{
			nome:      "gcp: gke_<projeto>_<região>_<nome>",
			cluster:   clusterPainel{info: cloud.ClusterInfo{Provider: "gcp", Name: "gke-ml", Region: "us-central1", ProjectID: "ml-plataforma"}},
			contextos: []string{"gke_ml-plataforma_us-central1_gke-ml"},
			args:      []string{"--context", "gke_ml-plataforma_us-central1_gke-ml"},
		},
		{
			nome:      "gcp sem projeto: o único gke_*_<região>_<nome> do kubeconfig",
			cluster:   clusterPainel{info: gkeSemProjeto},
			contextos: []string{"k3d-yby-local", "gke_x_us-central1_gke-ml", "gke_x_us-east1_gke-ml"},
			args:      []string{"--context", "gke_x_us-central1_gke-ml"},
		},
		{
			nome:      "gcp sem projeto, o mesmo cluster em dois projetos: não dá para escolher",
			cluster:   clusterPainel{info: gkeSemProjeto},
			contextos: []string{"gke_x_us-central1_gke-ml", "gke_y_us-central1_gke-ml"},
		},
		{
			nome:      "contexto ausente",
			cluster:   clusterPainel{info: staging, conta: "482100009310"},
			contextos: []string{"k3d-yby-local"},
		},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			contextos := map[string]bool{}
			for _, n := range c.contextos {
				contextos[n] = true
			}
			alvo, ok := alvoDoViz(c.cluster, contextos)
			if c.args == nil {
				assert.False(t, ok, "alvo %+v", alvo)
				return
			}
			require.True(t, ok)
			assert.Equal(t, c.args, alvo.args())
		})
	}
}

func TestComandoConectar(t *testing.T) {
	assert.Equal(t, conectarStaging,
		comandoConectar(clusterPainel{info: cloud.ClusterInfo{Provider: "aws", Name: "staging", Region: "us-east-1"}}))
	assert.Equal(t, "yby cloud connect --provider azure --cluster aks-dados",
		comandoConectar(clusterPainel{info: cloud.ClusterInfo{Provider: "azure", Name: "aks-dados"}}))
}

// abrirComEnter manda o ↵ e roda o comando que ele devolveu (as fontes falsas rodam o
// ExecCommand na hora); devolve a mensagem da volta do Viz.
func abrirComEnter(t *testing.T, m painelCloud) (painelCloud, vizFechadoMsg) {
	t.Helper()
	m, cmd := enviar(t, m, enter)
	require.NotNil(t, cmd, "o ↵ solta o terminal para o Viz")
	volta, ok := cmd().(vizFechadoMsg)
	require.True(t, ok, "esperava a volta do Viz")
	return m, volta
}

// TestPainelCloud_EnterAbreNoViz: a partir de 120 colunas, o ↵ na linha do staging roda o
// Viz com o contexto do kubeconfig e, na volta, o painel continua.
func TestPainelCloud_EnterAbreNoViz(t *testing.T) {
	usarSaidaTexto(t, false)
	f := novasFontesDeExemplo()
	f.contextos = map[string]bool{arnStaging: true}
	m := teclar(t, painelCarregado(t, f, 140, 30), baixo)

	m, volta := abrirComEnter(t, m)
	assert.Equal(t, [][]string{{"--context", arnStaging}}, f.abertos)
	assert.Equal(t, vizFechadoMsg{nome: "staging"}, volta)
	assert.Empty(t, m.aviso)
}

// TestPainelCloud_EnterComKubeconfigDoAmbiente: o ambiente com kube_config próprio abre o
// Viz nesse arquivo; sem o kube_context nele, vale o kubeconfig padrão.
func TestPainelCloud_EnterComKubeconfigDoAmbiente(t *testing.T) {
	usarSaidaTexto(t, false)
	f := novasFontesDeExemplo()
	for i := range f.leitura.clusters {
		if f.leitura.clusters[i].info.Name == "staging" {
			f.leitura.clusters[i].kubeconfig = "infra/kubeconfig-staging"
			f.leitura.clusters[i].kubeContexto = "yby-staging"
			f.leitura.clusters[i].namespace = "payments"
		}
	}
	f.porArquivo = map[string]map[string]bool{"infra/kubeconfig-staging": {"yby-staging": true}}
	f.contextos = map[string]bool{arnStaging: true}
	m := teclar(t, painelCarregado(t, f, 140, 30), baixo)

	_, _ = abrirComEnter(t, m)
	assert.Equal(t, [][]string{{"--kubeconfig", "infra/kubeconfig-staging", "--context", "yby-staging", "--namespace", "payments"}}, f.abertos)

	f.abertos = nil
	f.porArquivo = map[string]map[string]bool{"infra/kubeconfig-staging": {"outro": true}}
	_, _ = abrirComEnter(t, m)
	assert.Equal(t, [][]string{{"--context", arnStaging, "--namespace", "payments"}}, f.abertos)
}

// TestPainelCloud_EnterSemContextoMostraOConnect: sem o contexto no kubeconfig, nada é
// alterado nem aberto; a linha de estado diz o comando que conecta, inteiro.
func TestPainelCloud_EnterSemContextoMostraOConnect(t *testing.T) {
	usarSaidaTexto(t, false)
	f := novasFontesDeExemplo()
	f.contextos = map[string]bool{"k3d-yby-local": true}
	m := teclar(t, painelCarregado(t, f, 140, 30), baixo)

	m, cmd := enviar(t, m, enter)
	assert.Nil(t, cmd)
	assert.Empty(t, f.abertos)
	assert.Equal(t, "▲ staging não está no kubeconfig: "+conectarStaging, m.mensagem())

	// kubeconfig ilegível
	f.erroKubeconf = stderrors.New("yaml: line 3: mapping values are not allowed")
	m, cmd = enviar(t, m, enter)
	assert.Nil(t, cmd)
	assert.Contains(t, m.mensagem(), "não deu para ler o kubeconfig")
}

// linhaDeEstado é a linha da tela com o yby cloud connect (a linha de estado).
func linhaDeEstado(t *testing.T, tela string) string {
	t.Helper()
	for _, l := range strings.Split(tela, "\n") {
		if strings.Contains(l, "yby cloud connect") {
			return strings.TrimRight(l, " ")
		}
	}
	t.Fatalf("sem a linha de estado com o connect:\n%s", tela)
	return ""
}

// comHora é a linha de estado com a frase à esquerda e a hora da leitura à direita.
func comHora(frase string, largura int) string {
	const hora = "atualizado 12:07:00"
	return frase + strings.Repeat(" ", largura-utf8.RuneCountInString(frase)-len(hora)) + hora
}

// TestPainelCloud_SemContextoCabeAbaixoDe120: abaixo de 120 colunas o ↵ que chega ao Viz é
// o do detalhe por cima, e o comando que conecta não pode sair cortado.
func TestPainelCloud_SemContextoCabeAbaixoDe120(t *testing.T) {
	for _, c := range []struct {
		largura int
		linha   string
	}{
		{72, "▲ " + conectarStaging}, // nem o comando cabe com a hora: a hora sai
		{80, "▲ " + conectarStaging},
		{100, comHora("▲ "+conectarStaging, 100)},
		{120, comHora("▲ "+conectarStaging, 120)},
		{160, comHora("▲ staging não está no kubeconfig: "+conectarStaging, 160)},
	} {
		t.Run(fmt.Sprint(c.largura), func(t *testing.T) {
			usarSaidaTexto(t, false)
			f := novasFontesDeExemplo()
			f.contextos = map[string]bool{"k3d-yby-local": true}
			m := teclar(t, painelCarregado(t, f, c.largura, 30), baixo)
			if ui.ModoPara(c.largura, 30) != ui.ModoLadoALado {
				m = teclar(t, m, enter)
				require.True(t, m.porCima())
			}
			m, cmd := enviar(t, m, enter)
			require.Nil(t, cmd)
			assert.Equal(t, c.linha, linhaDeEstado(t, m.View()))
		})
	}
}

// TestPainelCloud_EnterAbaixoDe120: o primeiro ↵ abre o detalhe por cima; o segundo roda o
// Viz no cluster do detalhe.
func TestPainelCloud_EnterAbaixoDe120(t *testing.T) {
	usarSaidaTexto(t, false)
	f := novasFontesDeExemplo()
	f.contextos = map[string]bool{arnStaging: true}
	m := teclar(t, painelCarregado(t, f, 100, 30), baixo)

	m, cmd := enviar(t, m, enter)
	assert.Nil(t, cmd)
	require.True(t, m.porCima())
	assert.Empty(t, f.abertos)

	_, volta := abrirComEnter(t, m)
	assert.Equal(t, [][]string{{"--context", arnStaging}}, f.abertos)
	assert.Equal(t, "staging", volta.nome)
}

// TestPainelCloud_PorCimaMostraEsc: de 80 a 119 colunas, com o detalhe por cima, a legenda
// mantém o esc voltar (a única saída da camada), também no ASCII.
func TestPainelCloud_PorCimaMostraEsc(t *testing.T) {
	for _, ascii := range []bool{false, true} {
		for _, largura := range []int{80, 90, 100, 119} {
			t.Run(fmt.Sprint(largura, " ascii=", ascii), func(t *testing.T) {
				usarSaidaTexto(t, ascii)
				m := teclar(t, painelCarregado(t, novasFontesDeExemplo(), largura, 24), baixo, enter)
				require.True(t, m.porCima())
				linhas := strings.Split(m.View(), "\n")
				legenda := linhas[len(linhas)-1]
				assert.Contains(t, legenda, "esc voltar")
				assert.Contains(t, legenda, "abrir no Viz")
			})
		}
	}
}

// mensagensDe roda o comando e devolve os tipos das mensagens (desmonta o tea.Batch).
func mensagensDe(cmd tea.Cmd) []string {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	lote, ok := msg.(tea.BatchMsg)
	if !ok {
		return []string{fmt.Sprintf("%T", msg)}
	}
	var tipos []string
	for _, c := range lote {
		tipos = append(tipos, mensagensDe(c)...)
	}
	return tipos
}

// TestPainelCloud_VizFechado: ao voltar, o mouse é religado (também na falha, que vira
// toast); o ctrl+c no Viz (código 130) fecha o painel também.
func TestPainelCloud_VizFechado(t *testing.T) {
	usarSaidaTexto(t, false)
	m := painelCarregado(t, novasFontesDeExemplo(), 140, 30)
	const religa = "tea.enableMouseCellMotionMsg"

	nm, cmd := enviar(t, m, vizFechadoMsg{nome: "staging"})
	assert.Equal(t, []string{religa}, mensagensDe(cmd))
	assert.False(t, nm.toast.Visivel())

	nm, cmd = enviar(t, m, vizFechadoMsg{nome: "staging", err: &plugin.SaidaDoPlugin{Codigo: 1}})
	assert.Equal(t, "✕ não deu para abrir o Viz em staging · o Viz saiu com o código 1", nm.mensagem())
	assert.Contains(t, mensagensDe(cmd), religa)

	nm, _ = enviar(t, m, vizFechadoMsg{nome: "staging", err: errors.New(errors.ErrCodePluginNotFound, "plugin viz não encontrado")})
	assert.Contains(t, nm.mensagem(), "instale com yby plugin install viz")

	// o plugin não confiável (ou recompilado): a dica do trust, sem o código entre colchetes
	confianca := errors.New(errors.ErrCodePlugin, "o plugin viz não é confiável: não está registrado ou o binário mudou depois do registro").
		WithHint("Confira a origem do binário e rode yby plugin trust viz")
	nm, _ = enviar(t, m, vizFechadoMsg{nome: "staging", err: confianca})
	assert.Equal(t, "✕ não deu para abrir o Viz em staging · confira a origem do binário e rode yby plugin trust viz", nm.mensagem())

	_, cmd = enviar(t, m, vizFechadoMsg{nome: "staging", err: &plugin.SaidaDoPlugin{Codigo: codigoCtrlC}})
	assert.True(t, ehQuit(cmd), "ctrl+c no Viz sai de tudo")
}

// TestContextosDoKubeconfig lê os nomes dos contextos sem alterar o arquivo, do KUBECONFIG
// ou do arquivo pedido.
func TestContextosDoKubeconfig(t *testing.T) {
	dir := t.TempDir()
	kubeconfig := func(nome string, contextos ...string) (string, string) {
		conteudo := "apiVersion: v1\nkind: Config\ncurrent-context: " + contextos[0] + "\ncontexts:\n"
		for _, c := range contextos {
			conteudo += "- name: " + c + "\n  context: {cluster: c}\n"
		}
		conteudo += "clusters:\n- name: c\n  cluster: {server: https://127.0.0.1:6443}\n"
		caminho := filepath.Join(dir, nome)
		require.NoError(t, os.WriteFile(caminho, []byte(conteudo), 0o600))
		return caminho, conteudo
	}
	padrao, conteudo := kubeconfig("config", "a", arnStaging)
	proprio, _ := kubeconfig("do-ambiente", "yby-staging")
	t.Setenv("KUBECONFIG", padrao)

	contextos, err := contextosDoKubeconfig("")
	require.NoError(t, err)
	assert.Equal(t, map[string]bool{"a": true, arnStaging: true}, contextos)

	contextos, err = contextosDoKubeconfig(proprio)
	require.NoError(t, err)
	assert.Equal(t, map[string]bool{"yby-staging": true}, contextos)

	depois, err := os.ReadFile(padrao)
	require.NoError(t, err)
	assert.Equal(t, conteudo, string(depois))
}

// TestAmbientesCloud_LeKubeContextENamespace: o primeiro passo da busca do contexto lê o
// kube_config, o kube_context e o namespace do ambiente em .yby/environments.yaml.
func TestAmbientesCloud_LeKubeContextENamespace(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".yby"), 0o755))
	manifesto := `current: staging
environments:
  staging:
    type: eks
    kube_config: infra/kubeconfig-staging
    kube_context: yby-staging
    namespace: payments
    cloud:
      provider: aws
      region: us-east-1
      cluster: staging
  local:
    type: local
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".yby", "environments.yaml"), []byte(manifesto), 0o600))
	t.Chdir(dir)

	saida := ambientesCloud()
	require.Contains(t, saida, "aws/staging")
	amb := saida["aws/staging"]
	assert.Equal(t, "staging", amb.nome)
	assert.Equal(t, "infra/kubeconfig-staging", amb.kubeconfig)
	assert.Equal(t, "yby-staging", amb.kubeContexto)
	assert.Equal(t, "payments", amb.namespace)
	assert.Len(t, saida, 1, "o ambiente local não é cloud")
}

var _ tea.ExecCommand = execViz{}
