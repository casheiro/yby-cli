package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/casheiro/yby-cli/pkg/plugin"
)

// Contrato com o Sentinel: os arquivos ficam no testdata do Sentinel, que é quem escreve o
// JSON. Os pedidos são os que as ferramentas mandam; as respostas são os goldens que os
// testes TestContrato_* do Sentinel gravam rodando o hook command de verdade, com um
// cluster falso (rode go test -tags k8s ./plugins/sentinel/cli -run TestContrato -update ao
// mudar o Sentinel, e estes testes dizem se o Bard ainda entende a resposta). O Bard não
// importa o Sentinel, que só compila com a tag k8s: lê os arquivos.
var dirContratoSentinel = filepath.Join("..", "..", "sentinel", "cli", "testdata")

// lerContratoSentinel lê testdata/<nome> do Sentinel.
func lerContratoSentinel(t *testing.T, nome string) []byte {
	t.Helper()
	dados, err := os.ReadFile(filepath.Join(dirContratoSentinel, nome))
	if err != nil {
		t.Fatal(err)
	}
	return dados
}

// A ferramenta sentinel_scan manda ao Sentinel, numa sessão com cluster e namespace, o
// mesmo pedido com que o golden foi gravado.
func TestContrato_SentinelScanPedido(t *testing.T) {
	DefinirKube("kind-yby", "payments")
	defer DefinirKube("", "")
	var esperado plugin.PluginRequest
	if err := json.Unmarshal(lerContratoSentinel(t, "contrato-scan-pedido.json"), &esperado); err != nil {
		t.Fatal(err)
	}
	// Ida e volta pelo JSON, como o pedido viaja na variável do ambiente.
	var obtido plugin.PluginRequest
	dados, _ := json.Marshal(pedidoSentinelScan(map[string]string{}))
	if err := json.Unmarshal(dados, &obtido); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(obtido, esperado) {
		t.Errorf("o pedido ao Sentinel mudou: %#v, e o golden foi gravado com %#v", obtido, esperado)
	}
}

// O JSON do scan gravado pelo Sentinel passa pelo mesmo caminho da ferramenta
// (respostaDoPlugin e formatScanResults) e chega à IA resumido: a contagem por
// severidade, os achados do mais grave para o menos, com todos os recursos de um achado
// agrupado e o escopo, e o que não rodou.
func TestContrato_SentinelScan(t *testing.T) {
	resp, err := respostaDoPlugin(lerContratoSentinel(t, "contrato-scan.json"))
	if err != nil {
		t.Fatal(err)
	}
	dados, ok := resp.Data.(map[string]interface{})
	if !ok || dados["findings"] == nil {
		t.Fatalf("o relatório do scan deveria virar o Data da resposta: %#v", resp.Data)
	}
	texto := formatScanResults(resp.Data)
	for _, trecho := range []string{
		"polaris/memoryLimitsMissing",
		"Deployment/local-path-provisioner-com-nome-bem-comprido",
		"ClusterRoleBinding/ci-deployer",
		"critical",
		"rbac-wildcard",
		"sem permissão para acessar os clusterroles",
		"6 achados: 2 critical, 1 high, 1 medium, 2 low",
		"namespace payments",
		"kind-yby",
	} {
		if !strings.Contains(texto, trecho) {
			t.Errorf("o texto para a IA deveria ter %q:\n%s", trecho, texto)
		}
	}
	// Os críticos vêm antes dos de severidade baixa, na ordem em que o Sentinel os mandou.
	if strings.Index(texto, "polaris/runAsRootAllowed") > strings.Index(texto, "polaris/memoryLimitsMissing") {
		t.Errorf("os achados deveriam vir do mais grave para o menos:\n%s", texto)
	}
}

// O JSON do investigate --no-ai gravado pelo Sentinel (os dados brutos do pod) passa pelo
// caminho da ferramenta e chega à IA com o estado, os eventos e o fim dos logs.
func TestContrato_SentinelInvestigate(t *testing.T) {
	resp, err := respostaDoPlugin(lerContratoSentinel(t, "contrato-investigate.json"))
	if err != nil {
		t.Fatal(err)
	}
	texto := formatInvestigateResults(resp.Data)
	for _, trecho := range []string{
		"worker-5d8b6-q7tvn",
		"CrashLoopBackOff",
		"Back-off restarting failed container",
		"fake logs",
	} {
		if !strings.Contains(texto, trecho) {
			t.Errorf("o texto para a IA deveria ter %q:\n%s", trecho, texto)
		}
	}
}

// A ferramenta sentinel_investigate manda ao Sentinel o mesmo pedido (com --no-ai) com que o
// golden foi gravado.
func TestContrato_SentinelInvestigatePedido(t *testing.T) {
	DefinirKube("kind-yby", "payments")
	defer DefinirKube("", "")
	var esperado plugin.PluginRequest
	if err := json.Unmarshal(lerContratoSentinel(t, "contrato-investigate-pedido.json"), &esperado); err != nil {
		t.Fatal(err)
	}
	var obtido plugin.PluginRequest
	dados, _ := json.Marshal(pedidoSentinelInvestigate("worker-5d8b6-q7tvn", map[string]string{}))
	if err := json.Unmarshal(dados, &obtido); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(obtido, esperado) {
		t.Errorf("o pedido ao Sentinel mudou: %#v, e o golden foi gravado com %#v", obtido, esperado)
	}
}
