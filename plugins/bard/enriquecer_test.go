package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/casheiro/yby-cli/v5/plugins/bard/tui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// kubectlFalso responde como um kubeconfig com só o contexto ctx-a (namespace nsa) e,
// conforme atual, com ou sem current-context; guarda os comandos.
type kubectlFalso struct {
	atual    string
	comandos []string
}

func (k *kubectlFalso) rodar(args ...string) (string, error) {
	linha := strings.Join(args, " ")
	k.comandos = append(k.comandos, linha)
	contexto := k.atual
	if len(args) > 1 && args[0] == "--context" {
		contexto, args = args[1], args[2:]
	}
	switch strings.Join(args, " ") {
	case "config current-context":
		// Como o kubectl: lê o arquivo e ignora o --context.
		if k.atual == "" {
			return "", errors.New("error: current-context is not set")
		}
		return k.atual + "\n", nil
	case "config get-contexts ctx-a -o name":
		return "ctx-a\n", nil
	case "config view --minify --output=jsonpath={..namespace}":
		if contexto == "ctx-a" {
			return "nsa", nil
		}
		return "", errors.New("contexto inexistente")
	}
	if strings.HasPrefix(strings.Join(args, " "), "config get-contexts ") {
		return "", errors.New("error: context not found")
	}
	if contexto != "ctx-a" {
		return "", errors.New("contexto inexistente")
	}
	if args[0] == "get" && args[1] == "pods" {
		return `{"items":[{"metadata":{"name":"worker-1"},"status":{"phase":"Running"}}]}`, nil
	}
	return `{"items":[]}`, nil
}

// Com --context, o contexto é conferido pelo nome: vale num kubeconfig sem
// current-context (o Viz aberto pelo ↵ do Cloud), e o kubectl segue esse contexto.
func TestEnriquecer_ContextoSemCurrentContext(t *testing.T) {
	k := &kubectlFalso{}
	cc := enriquecer(k.rodar, "ctx-a", "")
	require.NotNil(t, cc, "o contexto dado existe: o contexto do cluster não pode sumir")
	assert.Equal(t, "ctx-a", cc.Cluster)
	assert.Equal(t, "nsa", cc.Namespace)
	require.Len(t, cc.Pods, 1)
	assert.Equal(t, "worker-1", cc.Pods[0].Name)
	for _, c := range k.comandos {
		assert.NotContains(t, c, "current-context", "com --context o current-context não vale")
	}
}

// Um --context que não existe no kubeconfig não passa, mesmo com current-context.
func TestEnriquecer_ContextoInexistente(t *testing.T) {
	k := &kubectlFalso{atual: "ctx-a"}
	cc := enriquecer(k.rodar, "ctx-sumiu", "")
	require.NotNil(t, cc)
	assert.Equal(t, `o contexto "ctx-sumiu" não existe no kubeconfig`, cc.SemCluster)
	assert.Empty(t, cc.Cluster)
}

// Sem --context, vale o current-context e o namespace dele; o -n troca o namespace.
func TestEnriquecer_SemContexto(t *testing.T) {
	cc := enriquecer((&kubectlFalso{atual: "ctx-a"}).rodar, "", "")
	require.NotNil(t, cc)
	assert.Equal(t, "ctx-a", cc.Cluster)
	assert.Equal(t, "nsa", cc.Namespace)

	cc = enriquecer((&kubectlFalso{atual: "ctx-a"}).rodar, "", "payments")
	require.NotNil(t, cc)
	assert.Equal(t, "payments", cc.Namespace)

	cc = enriquecer((&kubectlFalso{}).rodar, "", "")
	require.NotNil(t, cc)
	assert.Equal(t, "o kubeconfig não tem contexto atual (current-context)", cc.SemCluster,
		"sem current-context nem --context, o motivo vai para o prompt")
	assert.Empty(t, cc.Cluster)
	assert.Empty(t, cc.Pods)
}

// Cluster no kubeconfig que não responde: o cabeçalho mantém o cluster, e o prompt diz que
// pods e eventos são desconhecidos (antes saía "nenhum pod", como se estivesse vazio).
func TestEnriquecer_ClusterSemResposta(t *testing.T) {
	rodar := func(args ...string) (string, error) {
		switch strings.Join(args, " ") {
		case "config current-context":
			return "k3d-yby-local\n", nil
		case "config view --minify --output=jsonpath={..namespace}":
			return "", nil
		}
		return "", &exec.ExitError{Stderr: []byte("Unable to connect to the server: dial tcp 127.0.0.1:6443: connectex: No connection could be made\n")}
	}
	cc := enriquecer(rodar, "", "")
	require.NotNil(t, cc)
	assert.Empty(t, cc.SemCluster)
	assert.Equal(t, "k3d-yby-local", cc.Cluster)
	assert.Contains(t, cc.SemResposta, "Unable to connect to the server")

	texto := FormatClusterContext(cc)
	assert.Contains(t, texto, "**Cluster**: k3d-yby-local")
	assert.Contains(t, texto, "**O cluster não respondeu** ao kubectl: Unable to connect")
	assert.Contains(t, texto, "não diga que o namespace está vazio")

	var c tui.Config
	aplicarCluster(&c, cc, opcoesBard{})
	assert.Equal(t, "k3d-yby-local", c.Cluster)
}

// O motivo de não haver cluster sai do erro do kubectl.
func TestMotivoSemCluster(t *testing.T) {
	assert.Equal(t, "o kubectl não está instalado ou não está no PATH",
		motivoSemCluster(&exec.Error{Name: "kubectl", Err: exec.ErrNotFound}, ""))
	assert.Equal(t, `o contexto "x" não existe no kubeconfig`, motivoSemCluster(errors.New("error: context not found"), "x"))
	assert.Equal(t, "o kubeconfig não tem contexto atual (current-context)",
		motivoSemCluster(&exec.ExitError{Stderr: []byte("error: current-context is not set\n")}, ""))
	assert.Equal(t, `o kubeconfig C:\Users\ana\.kube\config não é válido (o kubectl não consegue ler o arquivo)`,
		motivoSemCluster(&exec.ExitError{Stderr: []byte(`error: error loading config file "C:\Users\ana\.kube\config": couldn't get version/kind; json parse error: json: cannot unmarshal string`)}, ""))
	assert.Equal(t, "o kubectl não leu o kubeconfig: error: yaml: line 3: mapping values",
		motivoSemCluster(errors.New("error: yaml: line 3: mapping values\nmais"), ""))
}

// Sem cluster, o prompt diz o motivo e o que fazer, e o cabeçalho fica sem cluster.
func TestSemCluster_PromptECabecalho(t *testing.T) {
	cc := &ClusterContext{SemCluster: "o kubeconfig não tem contexto atual (current-context)"}
	texto := FormatClusterContext(cc)
	assert.Contains(t, texto, "Nenhum cluster conectado**: o kubeconfig não tem contexto atual (current-context)")
	assert.Contains(t, texto, "kubectl config use-context")
	assert.NotContains(t, texto, "**Cluster**:")

	var c tui.Config
	aplicarCluster(&c, cc, opcoesBard{})
	assert.Empty(t, c.Cluster)
	assert.Empty(t, c.Namespace)
}

// Com o cluster lento, o kubectl leva --request-timeout e o conjunto respeita o prazo total.
func TestEnrichContext_PrazoELimiteDeRequisicao(t *testing.T) {
	arq := filepath.Join(t.TempDir(), "args.txt")
	t.Setenv(envKubectlLento, arq)
	anterior := binarioKubectl
	binarioKubectl = os.Args[0]
	t.Cleanup(func() { binarioKubectl = anterior })

	inicio := time.Now()
	cc := EnrichContext(context.Background(), "", "")
	require.NotNil(t, cc)
	assert.Equal(t, "o kubectl não respondeu em 5s", cc.SemCluster)
	assert.Less(t, time.Since(inicio), 10*time.Second, "o prazo total tem de cortar o kubectl lento")

	args, err := os.ReadFile(arq)
	require.NoError(t, err)
	assert.Contains(t, string(args), "--request-timeout=5s")
}
