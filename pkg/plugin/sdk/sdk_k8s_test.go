//go:build k8s

package sdk

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/casheiro/yby-cli/v5/pkg/plugin"
)

const kubeconfigDoisClusters = `apiVersion: v1
kind: Config
current-context: a
clusters:
- name: ca
  cluster:
    server: https://cluster-a.example:6443
- name: cb
  cluster:
    server: https://cluster-b.example:6443
users:
- name: u
  user:
    token: t
contexts:
- name: a
  context: {cluster: ca, user: u}
- name: b
  context: {cluster: cb, user: u}
`

// TestGetRestConfig_RespeitaContextoDoYby garante que o rest.Config sai do contexto
// do kubeconfig pedido pelo yby, e não do current-context do arquivo.
func TestGetRestConfig_RespeitaContextoDoYby(t *testing.T) {
	arquivo := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(arquivo, []byte(kubeconfigDoisClusters), 0o600); err != nil {
		t.Fatal(err)
	}
	antes := currentContext
	t.Cleanup(func() { currentContext = antes })

	currentContext = &plugin.PluginFullContext{}
	currentContext.Infra.KubeConfig = arquivo

	cfg, err := GetRestConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Host != "https://cluster-a.example:6443" {
		t.Errorf("sem contexto pedido, esperava o current-context (a), veio %q", cfg.Host)
	}

	currentContext.Infra.KubeContext = "b"
	cfg, err = GetRestConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Host != "https://cluster-b.example:6443" {
		t.Errorf("esperava o host do contexto b, veio %q", cfg.Host)
	}
}
