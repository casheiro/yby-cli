//go:build k8s

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/casheiro/yby-cli/v5/pkg/plugin"
	"github.com/casheiro/yby-cli/v5/pkg/ui"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// kubeconfigComNamespaces é o kubeconfigDeDoisClusters com o namespace de cada contexto:
// dev (o current-context) em pagamentos, staging em loja.
var kubeconfigComNamespaces = strings.NewReplacer(
	"    cluster: dev\n", "    cluster: dev\n    namespace: pagamentos\n",
	"    cluster: staging\n", "    cluster: staging\n    namespace: loja\n",
).Replace(kubeconfigDeDoisClusters)

func usarKubeconfig(t *testing.T, conteudo string) string {
	t.Helper()
	caminho := filepath.Join(t.TempDir(), "config")
	require.NoError(t, os.WriteFile(caminho, []byte(conteudo), 0o600))
	t.Setenv("KUBECONFIG", caminho)
	return caminho
}

// TestNamespaceDoKube: sem -n, o Sentinel começa no namespace do ambiente pedido; sem ele,
// no do contexto do kubeconfig que o cliente usa (como o kubectl e o Viz); sem nenhum,
// em default.
func TestNamespaceDoKube(t *testing.T) {
	t.Run("sem ambiente, o namespace do current-context", func(t *testing.T) {
		usarKubeconfig(t, kubeconfigComNamespaces)
		usarContextoDoYby(t, nil)
		assert.Equal(t, "pagamentos", namespaceDoKube())
	})

	t.Run("contexto sem namespace, default", func(t *testing.T) {
		usarKubeconfig(t, kubeconfigDeDoisClusters)
		usarContextoDoYby(t, nil)
		assert.Equal(t, "default", namespaceDoKube())
	})

	t.Run("sem kubeconfig, default", func(t *testing.T) {
		t.Setenv("KUBECONFIG", filepath.Join(t.TempDir(), "nao-existe"))
		usarContextoDoYby(t, nil)
		assert.Equal(t, "default", namespaceDoKube())
	})

	t.Run("o namespace do ambiente vence o do kubeconfig", func(t *testing.T) {
		usarKubeconfig(t, kubeconfigComNamespaces)
		usarContextoDoYby(t, &plugin.PluginFullContext{Infra: plugin.PluginInfrastructure{Namespace: "checkout"}})
		assert.Equal(t, "checkout", namespaceDoKube())
	})

	t.Run("ambiente sem namespace, o do contexto do ambiente", func(t *testing.T) {
		outro := filepath.Join(t.TempDir(), "staging.yaml")
		require.NoError(t, os.WriteFile(outro, []byte(kubeconfigComNamespaces), 0o600))
		usarKubeconfig(t, kubeconfigDeDoisClusters)
		usarContextoDoYby(t, &plugin.PluginFullContext{Infra: plugin.PluginInfrastructure{KubeConfig: outro, KubeContext: "staging"}})
		assert.Equal(t, "loja", namespaceDoKube())
	})
}

// TestScan_SemNamespaceUsaOPadrao: sem -n, o scan examina o namespace padrão, não default.
func TestScan_SemNamespaceUsaOPadrao(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
	client := fake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "pagamentos"}})
	usarDependencias(t, client, nil, nil, &provedorFalso{resposta: "nada"})
	namespacePadrao = func() string { return "pagamentos" }

	saida, _, err := rodar(t, "scan")
	require.NoError(t, err)
	assert.Contains(t, saida, "scan · namespace pagamentos")
}
