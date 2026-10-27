//go:build k8s

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/casheiro/yby-cli/pkg/plugin"
	"github.com/casheiro/yby-cli/pkg/plugin/sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes"
)

// kubeconfigDeDoisClusters tem dev (o current-context) e staging, cada um num servidor.
const kubeconfigDeDoisClusters = `apiVersion: v1
kind: Config
current-context: dev
clusters:
- name: dev
  cluster:
    server: https://dev.invalid:6443
- name: staging
  cluster:
    server: https://staging.invalid:6443
contexts:
- name: dev
  context:
    cluster: dev
    user: eu
- name: staging
  context:
    cluster: staging
    user: eu
users:
- name: eu
  user:
    token: falso
`

// TestKubeClient_UsaOInfraDoAmbientePedido: com yby -c staging sentinel …, o core manda
// o infra do ambiente no pedido, e o client do Sentinel (pelo sdk) vai ao kube_config e
// ao kube_context dele, não ao current-context.
func TestKubeClient_UsaOInfraDoAmbientePedido(t *testing.T) {
	pasta := t.TempDir()
	kubeconfig := filepath.Join(pasta, "staging.yaml")
	require.NoError(t, os.WriteFile(kubeconfig, []byte(kubeconfigDeDoisClusters), 0o600))
	// sem o infra, o client cairia aqui, e não no ~/.kube real
	t.Setenv("KUBECONFIG", filepath.Join(pasta, "nao-existe"))

	contexto := plugin.PluginFullContext{
		Environment: "staging",
		Infra: plugin.PluginInfrastructure{
			KubeConfig:  kubeconfig,
			KubeContext: "staging",
			Namespace:   "staging-ns",
		},
	}
	var ctxMap map[string]interface{}
	b, _ := json.Marshal(contexto)
	require.NoError(t, json.Unmarshal(b, &ctxMap))
	pedido, err := json.Marshal(plugin.PluginRequest{Hook: "command", Args: []string{"scan"}, Context: ctxMap})
	require.NoError(t, err)

	t.Setenv("YBY_PLUGIN_REQUEST", string(pedido))
	t.Cleanup(func() {
		// o sdk guarda o pedido: os outros testes não podem herdar este contexto
		os.Setenv("YBY_PLUGIN_REQUEST", `{"hook":""}`)
		_ = sdk.Init()
	})
	require.NoError(t, sdk.Init())

	cliente, err := getKubeClient()
	require.NoError(t, err)
	cs, ok := cliente.(*kubernetes.Clientset)
	require.True(t, ok)
	assert.Equal(t, "staging.invalid:6443", cs.CoreV1().RESTClient().Get().URL().Host)
	assert.Equal(t, "staging-ns", sdk.GetFullContext().Infra.Namespace)
}
