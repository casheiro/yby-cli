package monitor

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// kubeconfigDeDois tem dois contextos: staging (atual, com namespace payments) e
// k3d-yby-local (sem namespace).
const kubeconfigDeDois = `apiVersion: v1
kind: Config
current-context: staging
clusters:
- name: staging
  cluster:
    server: https://127.0.0.1:6443
- name: local
  cluster:
    server: https://127.0.0.1:16443
contexts:
- name: staging
  context:
    cluster: staging
    user: eu
    namespace: payments
- name: k3d-yby-local
  context:
    cluster: local
    user: eu
users:
- name: eu
  user:
    token: falso
`

func usarKubeconfig(t *testing.T, conteudo string) string {
	t.Helper()
	caminho := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(caminho, []byte(conteudo), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KUBECONFIG", caminho)
	return caminho
}

// TestNovoK8sClient_ContextoENamespacePedidos: o contexto pedido vence o atual,
// e o namespace pedido vence o do contexto.
func TestNovoK8sClient_ContextoENamespacePedidos(t *testing.T) {
	usarKubeconfig(t, kubeconfigDeDois)

	c, err := NovoK8sClient(Conexao{Contexto: "k3d-yby-local", Namespace: "kube-system"})
	if err != nil {
		t.Fatalf("NovoK8sClient: %v", err)
	}
	if c.Contexto() != "k3d-yby-local" || c.NamespaceInicial() != "kube-system" {
		t.Fatalf("contexto %q, namespace %q", c.Contexto(), c.NamespaceInicial())
	}
	if c.RESTConfig().Host != "https://127.0.0.1:16443" {
		t.Fatalf("servidor = %q, esperado o do contexto pedido", c.RESTConfig().Host)
	}

	// Só o namespace: contexto atual, namespace pedido.
	c, err = NovoK8sClient(Conexao{Namespace: "kube-system"})
	if err != nil {
		t.Fatalf("NovoK8sClient: %v", err)
	}
	if c.Contexto() != "staging" || c.NamespaceInicial() != "kube-system" {
		t.Fatalf("contexto %q, namespace %q", c.Contexto(), c.NamespaceInicial())
	}

	// Contexto pedido sem namespace e sem --namespace: todos.
	c, err = NovoK8sClient(Conexao{Contexto: "k3d-yby-local"})
	if err != nil {
		t.Fatalf("NovoK8sClient: %v", err)
	}
	if c.NamespaceInicial() != "" {
		t.Fatalf("namespace = %q, esperado vazio", c.NamespaceInicial())
	}
}

// TestNovoK8sClient_ContextoInexistente devolve o erro com os contextos que existem.
func TestNovoK8sClient_ContextoInexistente(t *testing.T) {
	usarKubeconfig(t, kubeconfigDeDois)

	_, err := NovoK8sClient(Conexao{Contexto: "prod"})
	var inexistente *ContextoInexistenteError
	if !errors.As(err, &inexistente) {
		t.Fatalf("erro = %v, esperado ContextoInexistenteError", err)
	}
	if inexistente.Nome != "prod" || !reflect.DeepEqual(inexistente.Disponiveis, []string{"k3d-yby-local", "staging"}) {
		t.Fatalf("erro = %+v", inexistente)
	}
}

// TestNovoK8sClient_VariosArquivosNoKubeconfig: como o kubectl, KUBECONFIG pode
// juntar vários arquivos.
func TestNovoK8sClient_VariosArquivosNoKubeconfig(t *testing.T) {
	dir := t.TempDir()
	um := filepath.Join(dir, "um")
	outro := filepath.Join(dir, "outro")
	if err := os.WriteFile(um, []byte(kubeconfigDeDois), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outro, []byte(`apiVersion: v1
kind: Config
clusters:
- name: aks
  cluster:
    server: https://aks.exemplo:443
contexts:
- name: aks-dados
  context:
    cluster: aks
    user: eu
`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KUBECONFIG", um+string(os.PathListSeparator)+outro)

	c, err := NovoK8sClient(Conexao{Contexto: "aks-dados"})
	if err != nil {
		t.Fatalf("NovoK8sClient: %v", err)
	}
	if c.RESTConfig().Host != "https://aks.exemplo:443" {
		t.Fatalf("servidor = %q", c.RESTConfig().Host)
	}
}

// TestNovoK8sClient_KubeconfigPedido: o --kubeconfig lê só aquele arquivo, no lugar do
// KUBECONFIG (o kube_config próprio de um ambiente do projeto).
func TestNovoK8sClient_KubeconfigPedido(t *testing.T) {
	usarKubeconfig(t, kubeconfigDeDois)
	proprio := filepath.Join(t.TempDir(), "kubeconfig-staging")
	if err := os.WriteFile(proprio, []byte(`apiVersion: v1
kind: Config
clusters:
- name: eks
  cluster:
    server: https://eks.exemplo:443
contexts:
- name: yby-staging
  context:
    cluster: eks
    user: eu
    namespace: payments
`), 0o600); err != nil {
		t.Fatal(err)
	}

	c, err := NovoK8sClient(Conexao{Kubeconfig: proprio, Contexto: "yby-staging"})
	if err != nil {
		t.Fatalf("NovoK8sClient: %v", err)
	}
	if c.RESTConfig().Host != "https://eks.exemplo:443" || c.NamespaceInicial() != "payments" {
		t.Fatalf("servidor %q, namespace %q", c.RESTConfig().Host, c.NamespaceInicial())
	}

	// os contextos do KUBECONFIG não valem com o arquivo pedido
	_, err = NovoK8sClient(Conexao{Kubeconfig: proprio, Contexto: "staging"})
	var inexistente *ContextoInexistenteError
	if !errors.As(err, &inexistente) || !reflect.DeepEqual(inexistente.Disponiveis, []string{"yby-staging"}) {
		t.Fatalf("erro = %v", err)
	}
}

// O --kubeconfig chega ao Bard do b pela cadeia do main.go: o K8sClient guarda o arquivo
// pedido e o RetryClient em volta o repassa; sem --kubeconfig, ou com um client que não o
// conhece, vem vazio (vale o KUBECONFIG herdado).
func TestKubeconfig_PeloRetryClient(t *testing.T) {
	caminho := usarKubeconfig(t, kubeconfigDeDois)
	t.Setenv("KUBECONFIG", "")

	c, err := NovoK8sClient(Conexao{Kubeconfig: caminho})
	if err != nil {
		t.Fatalf("NovoK8sClient: %v", err)
	}
	if got := NewRetryClient(c).Kubeconfig(); got != caminho {
		t.Fatalf("RetryClient.Kubeconfig() = %q, esperado %q", got, caminho)
	}

	t.Setenv("KUBECONFIG", caminho)
	c, err = NovoK8sClient(Conexao{})
	if err != nil {
		t.Fatalf("NovoK8sClient: %v", err)
	}
	if got := NewRetryClient(c).Kubeconfig(); got != "" {
		t.Fatalf("sem --kubeconfig: %q", got)
	}
	if got := NewRetryClient(&FakeClient{}).Kubeconfig(); got != "" {
		t.Fatalf("client sem kubeconfig: %q", got)
	}
}
