package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	yerrors "github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/plugins/viz/internal/monitor"
	"github.com/charmbracelet/x/ansi"
)

// kubeconfigDeTeste tem os contextos staging (atual, namespace payments) e k3d-yby-local.
const kubeconfigDeTeste = `apiVersion: v1
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

func usarKubeconfig(t *testing.T) {
	t.Helper()
	caminho := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(caminho, []byte(kubeconfigDeTeste), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KUBECONFIG", caminho)
}

// executarComando roda o yby viz com os args, guardando a conexão pedida.
func executarComando(t *testing.T, args ...string) (monitor.Conexao, bool, string, error) {
	t.Helper()
	var pedida monitor.Conexao
	abriu := false
	cmd := novoComando(func(c monitor.Conexao) error {
		pedida, abriu = c, true
		return nil
	})
	var saida bytes.Buffer
	cmd.SetOut(&saida)
	cmd.SetErr(&saida)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return pedida, abriu, saida.String(), err
}

// TestComando_FlagsLidas: --context, --namespace e -n chegam à conexão.
func TestComando_FlagsLidas(t *testing.T) {
	casos := []struct {
		args     []string
		esperada monitor.Conexao
	}{
		{nil, monitor.Conexao{}},
		{[]string{"--context", "k3d-yby-local", "--namespace", "kube-system"}, monitor.Conexao{Contexto: "k3d-yby-local", Namespace: "kube-system"}},
		{[]string{"--context=arn:aws:eks:us-east-1:482100009310:cluster/staging", "-n", "payments"}, monitor.Conexao{Contexto: "arn:aws:eks:us-east-1:482100009310:cluster/staging", Namespace: "payments"}},
		{[]string{"-n", "kube-system"}, monitor.Conexao{Namespace: "kube-system"}},
		{[]string{"--kubeconfig", "infra/kubeconfig-staging", "--context", "yby-staging"}, monitor.Conexao{Kubeconfig: "infra/kubeconfig-staging", Contexto: "yby-staging"}},
	}
	for _, c := range casos {
		pedida, abriu, _, err := executarComando(t, c.args...)
		if err != nil || !abriu {
			t.Fatalf("%v: err %v, abriu %v", c.args, err, abriu)
		}
		if pedida != c.esperada {
			t.Errorf("%v: conexão %+v, esperada %+v", c.args, pedida, c.esperada)
		}
	}
}

// TestComando_Ajuda: --help mostra as flags no formato do core, sem abrir a TUI.
func TestComando_Ajuda(t *testing.T) {
	isolarModoVisual(t, "")
	t.Setenv("NO_COLOR", "1")
	_, abriu, saida, err := executarComando(t, "--help")
	if err != nil || abriu {
		t.Fatalf("err %v, abriu %v", err, abriu)
	}
	saida = ansi.Strip(saida)
	for _, trecho := range []string{"YBY", "viz", "--kubeconfig texto", "--context texto", "-n, --namespace texto", "contexto do kubeconfig", "yby viz --context k3d-yby-local --namespace kube-system"} {
		if !strings.Contains(saida, trecho) {
			t.Errorf("a ajuda não tem %q:\n%s", trecho, saida)
		}
	}
	if strings.Contains(saida, "Usage") || strings.Contains(saida, "Flags:") {
		t.Errorf("a ajuda saiu em inglês:\n%s", saida)
	}
}

// TestComando_FlagDesconhecida vira YbyError com a dica da ajuda.
func TestComando_FlagDesconhecida(t *testing.T) {
	_, abriu, _, err := executarComando(t, "--contexto", "x")
	var ye *yerrors.YbyError
	if !errors.As(err, &ye) || ye.Code != yerrors.ErrCodeValidation || abriu {
		t.Fatalf("err %v, abriu %v", err, abriu)
	}
	if !strings.Contains(ye.GetHint(), "yby viz --help") {
		t.Errorf("dica: %q", ye.GetHint())
	}
}

// TestPedidoDoCore: o yby viz manda os args pela env YBY_PLUGIN_REQUEST.
func TestPedidoDoCore(t *testing.T) {
	env := map[string]string{"YBY_PLUGIN_REQUEST": `{"hook":"command","args":["--context","k3d-yby-local","-n","kube-system"]}`}
	req, ok, err := pedidoDoCore(ambiente(env), nil)
	if err != nil || !ok {
		t.Fatalf("ok %v, err %v", ok, err)
	}
	if req.Hook != "command" || strings.Join(req.Args, " ") != "--context k3d-yby-local -n kube-system" {
		t.Fatalf("pedido %+v", req)
	}

	if _, ok, _ := pedidoDoCore(ambiente(nil), nil); ok {
		t.Error("sem env nem stdin, não há pedido")
	}
	if _, _, err := pedidoDoCore(ambiente(map[string]string{"YBY_PLUGIN_REQUEST": "{"}), nil); err == nil {
		t.Error("pedido inválido deveria dar erro")
	}
}

// TestConectar_NamespaceInicial: o namespace pedido vence o do contexto.
func TestConectar_NamespaceInicial(t *testing.T) {
	usarKubeconfig(t)
	c, err := conectar(monitor.Conexao{Contexto: "k3d-yby-local", Namespace: "kube-system"})
	if err != nil {
		t.Fatal(err)
	}
	if c.Contexto() != "k3d-yby-local" || c.NamespaceInicial() != "kube-system" {
		t.Fatalf("contexto %q, namespace %q", c.Contexto(), c.NamespaceInicial())
	}
}

// TestExecutar_ContextoInvalidoNaoAbreATUI: o contexto que não existe sai como erro do
// comando, com os contextos que existem na dica, e código 1.
func TestExecutar_ContextoInvalidoNaoAbreATUI(t *testing.T) {
	isolarModoVisual(t, "")
	t.Setenv("NO_COLOR", "1")
	usarKubeconfig(t)
	t.Setenv("YBY_PLUGIN_REQUEST", `{"hook":"command","args":["--context","prod"]}`)

	var saida bytes.Buffer
	if codigo := executar(&saida); codigo != 1 {
		t.Fatalf("código %d, esperado 1", codigo)
	}
	texto := ansi.Strip(saida.String())
	for _, trecho := range []string{"o contexto prod não existe no kubeconfig", "k3d-yby-local, staging", "ERR_CONFIG_INVALID"} {
		if !strings.Contains(texto, trecho) {
			t.Errorf("a saída não tem %q:\n%s", trecho, texto)
		}
	}
}
