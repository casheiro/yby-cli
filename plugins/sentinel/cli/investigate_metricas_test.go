//go:build k8s

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/casheiro/yby-cli/v5/pkg/plugin"
	"github.com/casheiro/yby-cli/v5/pkg/plugin/sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/fake"
)

// topDeTeste é a linha que o kubectl top falso devolve.
const topDeTeste = "worker-5d8b6-q7tvn   15m   48Mi"

// TestAuxiliarKubectlTop é o kubectl falso: só faz algo quando o teste o roda como
// processo, e então escreve a linha do top e sai com 0 (com SENTINEL_KUBECTL_DEMORA=1,
// antes fica um minuto parado, como um cluster que não responde).
func TestAuxiliarKubectlTop(t *testing.T) {
	if os.Getenv("SENTINEL_KUBECTL_FALSO") != "1" {
		return
	}
	if os.Getenv("SENTINEL_KUBECTL_DEMORA") == "1" {
		time.Sleep(time.Minute)
	}
	fmt.Println(topDeTeste)
	os.Exit(0)
}

// usarKubectlFalso troca o execCommand por um kubectl que responde o top, devolve os
// argumentos de cada chamada e restaura o execCommand anterior no fim.
func usarKubectlFalso(t *testing.T, env ...string) *[][]string {
	t.Helper()
	anterior := execCommand
	t.Cleanup(func() { execCommand = anterior })
	var chamadas [][]string
	execCommand = func(ctx context.Context, nome string, args ...string) *exec.Cmd {
		chamadas = append(chamadas, append([]string{nome}, args...))
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestAuxiliarKubectlTop$")
		cmd.Env = append(append(os.Environ(), "SENTINEL_KUBECTL_FALSO=1"), env...)
		return cmd
	}
	return &chamadas
}

// usarContextoDoYby faz o sdk ler o pedido do core com o contexto dado (nil: sem
// ambiente pedido, como no yby sentinel … sem -c nem YBY_ENV) e o esquece no fim.
func usarContextoDoYby(t *testing.T, contexto *plugin.PluginFullContext) {
	t.Helper()
	pedido := plugin.PluginRequest{Hook: "command"}
	if contexto != nil {
		var ctxMap map[string]interface{}
		b, _ := json.Marshal(contexto)
		require.NoError(t, json.Unmarshal(b, &ctxMap))
		pedido.Context = ctxMap
	}
	b, err := json.Marshal(pedido)
	require.NoError(t, err)
	t.Setenv("YBY_PLUGIN_REQUEST", string(b))
	t.Cleanup(func() {
		// o sdk guarda o pedido: os outros testes não podem herdar este contexto
		os.Setenv("YBY_PLUGIN_REQUEST", `{"hook":""}`)
		_ = sdk.Init()
	})
	require.NoError(t, sdk.Init())
}

// TestInvestigate_MetricasSemAmbiente: sem ambiente pedido, o core não manda contexto, e o
// kubectl top roda mesmo assim, no contexto atual do kubeconfig, o do cliente do Sentinel.
func TestInvestigate_MetricasSemAmbiente(t *testing.T) {
	pod, ev := podComProblema()
	prov := &provedorFalso{resposta: respostaDaIA}
	usarDependencias(t, fake.NewSimpleClientset(pod, ev), nil, nil, prov)
	chamadas := usarKubectlFalso(t)
	usarContextoDoYby(t, nil)
	require.Nil(t, sdk.GetFullContext())

	saida, _, err := rodar(t, "investigate", "worker-5d8b6-q7tvn", "-n", "payments", "--no-cache")
	require.NoError(t, err)

	require.Len(t, *chamadas, 1)
	assert.Equal(t, []string{"kubectl", "top", "pod", "worker-5d8b6-q7tvn", "-n", "payments", "--no-headers", "--context", "kind-yby"}, (*chamadas)[0])
	assert.Contains(t, saida, "kubectl top")
	assert.NotContains(t, saida, "indisponíveis")
	require.Len(t, prov.recebidos, 1)
	assert.Contains(t, prov.recebidos[0], "METRICS:\n"+topDeTeste)
}

// TestInvestigate_MetricasSemAmbienteNemContextoAtual: sem contexto atual no kubeconfig,
// o kubectl top vai sem --context, como o cliente, que também fica sem um.
func TestInvestigate_MetricasSemAmbienteNemContextoAtual(t *testing.T) {
	pod, ev := podComProblema()
	usarDependencias(t, fake.NewSimpleClientset(pod, ev), nil, nil, &provedorFalso{resposta: respostaDaIA})
	nomeDoContexto = func() string { return "" }
	chamadas := usarKubectlFalso(t)
	usarContextoDoYby(t, nil)

	_, _, err := rodar(t, "investigate", "worker-5d8b6-q7tvn", "-n", "payments", "--no-cache")
	require.NoError(t, err)

	require.Len(t, *chamadas, 1)
	assert.Equal(t, []string{"kubectl", "top", "pod", "worker-5d8b6-q7tvn", "-n", "payments", "--no-headers"}, (*chamadas)[0])
}

// TestInvestigate_MetricasComAmbiente: com yby -c staging sentinel investigate …, o
// kubectl top vai ao kube_config e ao kube_context do ambiente, como antes.
func TestInvestigate_MetricasComAmbiente(t *testing.T) {
	pod, ev := podComProblema()
	prov := &provedorFalso{resposta: respostaDaIA}
	usarDependencias(t, fake.NewSimpleClientset(pod, ev), nil, nil, prov)
	nomeDoContexto = func() string {
		t.Error("com ambiente, o contexto vem do core, não do kubeconfig padrão")
		return "kind-yby"
	}
	chamadas := usarKubectlFalso(t)
	kubeconfig := filepath.Join(t.TempDir(), "staging.yaml")
	usarContextoDoYby(t, &plugin.PluginFullContext{
		Environment: "staging",
		Infra:       plugin.PluginInfrastructure{KubeConfig: kubeconfig, KubeContext: "staging"},
	})

	saida, _, err := rodar(t, "investigate", "worker-5d8b6-q7tvn", "-n", "payments", "--no-cache")
	require.NoError(t, err)

	require.Len(t, *chamadas, 1)
	assert.Equal(t, []string{"kubectl", "top", "pod", "worker-5d8b6-q7tvn", "-n", "payments", "--no-headers", "--kubeconfig", kubeconfig, "--context", "staging"}, (*chamadas)[0])
	assert.Contains(t, saida, "kubectl top")
	require.Len(t, prov.recebidos, 1)
	assert.Contains(t, prov.recebidos[0], "METRICS:\n"+topDeTeste)
}

// TestInvestigate_MetricasQueFalhamFicamIndisponiveis: o kubectl top que falha (sem
// metrics-server, sem kubectl) deixa a etapa como indisponível, e a análise segue.
func TestInvestigate_MetricasQueFalhamFicamIndisponiveis(t *testing.T) {
	pod, ev := podComProblema()
	prov := &provedorFalso{resposta: respostaDaIA}
	usarDependencias(t, fake.NewSimpleClientset(pod, ev), nil, nil, prov)
	usarContextoDoYby(t, nil)

	saida, _, err := rodar(t, "investigate", "worker-5d8b6-q7tvn", "-n", "payments", "--no-cache")
	require.NoError(t, err)

	assert.Contains(t, saida, "indisponíveis")
	require.Len(t, prov.recebidos, 1)
	assert.Contains(t, prov.recebidos[0], "METRICS:\nmétricas indisponíveis")
}

// TestInvestigate_MetricasComPrazo: o kubectl top que não responde morre no prazo, a
// etapa fica como indisponível e a análise segue sem esperar por ele.
func TestInvestigate_MetricasComPrazo(t *testing.T) {
	pod, ev := podComProblema()
	prov := &provedorFalso{resposta: respostaDaIA}
	usarDependencias(t, fake.NewSimpleClientset(pod, ev), nil, nil, prov)
	chamadas := usarKubectlFalso(t, "SENTINEL_KUBECTL_DEMORA=1")
	usarContextoDoYby(t, nil)
	prazo := prazoDoKubectlTop
	t.Cleanup(func() { prazoDoKubectlTop = prazo })
	prazoDoKubectlTop = 300 * time.Millisecond

	inicio := time.Now()
	saida, _, err := rodar(t, "investigate", "worker-5d8b6-q7tvn", "-n", "payments", "--no-cache")
	require.NoError(t, err)

	assert.Less(t, time.Since(inicio), 20*time.Second, "o investigate esperou o kubectl top além do prazo")
	require.Len(t, *chamadas, 1)
	assert.Contains(t, saida, "indisponíveis")
	require.Len(t, prov.recebidos, 1)
	assert.Contains(t, prov.recebidos[0], "METRICS:\nmétricas indisponíveis")
	assert.NotContains(t, prov.recebidos[0], topDeTeste)
}

// TestContextoDoKube_SemAmbienteEOContextoDoCliente: sem ambiente, o contexto que o
// kubectl top recebe é o current-context do KUBECONFIG, o mesmo cluster em que o
// cliente do Sentinel lê.
func TestContextoDoKube_SemAmbienteEOContextoDoCliente(t *testing.T) {
	kubeconfig := filepath.Join(t.TempDir(), "config")
	require.NoError(t, os.WriteFile(kubeconfig, []byte(kubeconfigDeDoisClusters), 0o600))
	t.Setenv("KUBECONFIG", kubeconfig)
	usarContextoDoYby(t, nil)

	assert.Equal(t, "dev", contextoDoKube())
	cfg, err := getRestConfig()
	require.NoError(t, err)
	assert.Contains(t, cfg.Host, "dev.invalid:6443", "o cliente deveria ir ao cluster dev")
}

// TestIdentidadeDoPod_SemAmbienteUsaOContextoAtual: sem ambiente, o cache de análises
// separa os clusters pelo contexto atual do kubeconfig.
func TestIdentidadeDoPod_SemAmbienteUsaOContextoAtual(t *testing.T) {
	pod, _ := podComProblema()
	usarDependencias(t, fake.NewSimpleClientset(), nil, nil, nil)
	usarContextoDoYby(t, nil)

	a := identidadeDoPod(pod)
	nomeDoContexto = func() string { return "outro" }
	b := identidadeDoPod(pod)
	assert.Equal(t, "|kind-yby", a.Contexto)
	assert.NotEqual(t, cacheKey(a, "logs"), cacheKey(b, "logs"))
}
