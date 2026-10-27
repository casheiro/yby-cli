package tools

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// O Bard aberto pelo Viz (yby bard --context X --namespace Y) consulta o mesmo cluster:
// o kubectl das ferramentas leva o --context e, sem namespace da IA, o da sessão.
func TestKube_ContextoENamespaceDaSessao(t *testing.T) {
	defer DefinirKube("", "")

	DefinirKube("", "")
	assert.Equal(t, []string{"get", "pods"}, argsKubectl([]string{"get", "pods"}))
	assert.Equal(t, "", namespaceDe(map[string]string{}))

	DefinirKube("k3d-yby-local", "payments")
	assert.Equal(t, []string{"--context", "k3d-yby-local", "get", "pods"}, argsKubectl([]string{"get", "pods"}))
	assert.Equal(t, "payments", namespaceDe(map[string]string{}))
	assert.Equal(t, "kube-system", namespaceDe(map[string]string{"namespace": "kube-system"}), "o namespace da IA vale")

	cmd := comandoKubectl(context.Background(), []string{"logs", "worker"})
	assert.Equal(t, []string{"--context", "k3d-yby-local", "logs", "worker"}, cmd.Args[1:])
}

// O Sentinel das ferramentas investiga e escaneia no cluster da sessão: o contexto vai no
// infra do pedido (o SDK do Sentinel o usa no client) e, sem namespace da IA, o da sessão.
func TestSentinel_PedidoNoClusterDaSessao(t *testing.T) {
	defer DefinirKube("", "")

	DefinirKube("", "")
	req := pedidoSentinelInvestigate("worker-1", map[string]string{})
	assert.Equal(t, []string{"investigate", "worker-1", "-o", "json", "--no-ai"}, req.Args)
	assert.Nil(t, req.Context, "sem sessão, valem o contexto e o namespace do kubeconfig")

	DefinirKube("k3d-yby-local", "payments")
	req = pedidoSentinelInvestigate("worker-1", map[string]string{})
	assert.Equal(t, "command", req.Hook)
	assert.Equal(t, []string{"investigate", "worker-1", "-n", "payments", "-o", "json", "--no-ai"}, req.Args)
	assert.Equal(t, map[string]interface{}{"infra": map[string]interface{}{"kube_context": "k3d-yby-local", "namespace": "payments"}}, req.Context)

	req = pedidoSentinelInvestigate("worker-1", map[string]string{"namespace": "kube-system"})
	assert.Equal(t, []string{"investigate", "worker-1", "-n", "kube-system", "-o", "json", "--no-ai"}, req.Args, "o namespace da IA vale")

	req = pedidoSentinelScan(map[string]string{})
	assert.Equal(t, []string{"scan", "-n", "payments", "-o", "json", "--no-ai"}, req.Args)
	assert.Equal(t, "k3d-yby-local", req.Context["infra"].(map[string]interface{})["kube_context"])
}
