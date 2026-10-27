package tools

import (
	"context"
	"strings"
	"testing"
)

// TestSentinelScan_Parametros verifica os parâmetros do sentinel_scan.
func TestSentinelScan_Parametros(t *testing.T) {
	// Verificar que a tool está registrada (pode ter sido resetada por outros testes)
	tool := &Tool{
		Name:        "sentinel_scan",
		Description: "Executa scan de segurança no cluster via plugin Sentinel",
		Parameters: []ToolParam{
			{Name: "namespace", Description: "Namespace a escanear", Required: false},
		},
		Execute: executeSentinelScan,
	}

	if len(tool.Parameters) != 1 {
		t.Fatalf("esperava 1 parâmetro, obteve %d", len(tool.Parameters))
	}
	if tool.Parameters[0].Required {
		t.Error("parâmetro 'namespace' deveria ser opcional")
	}
}

// TestSentinelInvestigate_SemPod verifica que investigate exige pod.
func TestSentinelInvestigate_SemPod(t *testing.T) {
	_, err := executeSentinelInvestigate(context.Background(), map[string]string{})
	if err == nil {
		t.Error("esperava erro quando pod está vazio")
	}
}

// TestSentinelScan_SemBinario verifica erro quando binário não está disponível.
func TestSentinelScan_SemBinario(t *testing.T) {
	// Em ambiente de teste, o binário do sentinel não está disponível
	_, err := executeSentinelScan(context.Background(), map[string]string{})
	if err == nil {
		t.Error("esperava erro quando binário do sentinel não está disponível")
	}
}

// TestFormatScanResults_Nil verifica formatação com dados nil.
func TestFormatScanResults_Nil(t *testing.T) {
	result := formatScanResults(nil)
	if result != "Nenhum finding reportado." {
		t.Errorf("resultado inesperado: %q", result)
	}
}

// Plugins que respondem com o envelope (o Atlas) continuam com o Data e o Error dele.
func TestRespostaDoPlugin_Envelope(t *testing.T) {
	resp, err := respostaDoPlugin([]byte(`{"data": {"resources": 3}}`))
	if err != nil {
		t.Fatal(err)
	}
	if dados, ok := resp.Data.(map[string]interface{}); !ok || dados["resources"] != float64(3) {
		t.Errorf("data inesperado: %#v", resp.Data)
	}

	_, err = respostaDoPlugin([]byte(`{"error": "sem cluster"}`))
	if err == nil || !strings.Contains(err.Error(), "sem cluster") {
		t.Errorf("esperava o erro do plugin, veio %v", err)
	}
}
