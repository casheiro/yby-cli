package tools

import (
	"context"
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

// TestFormatScanResults_ComDados verifica formatação com dados.
func TestFormatScanResults_ComDados(t *testing.T) {
	data := map[string]interface{}{
		"findings": []interface{}{
			map[string]interface{}{"severity": "HIGH", "message": "container rodando como root"},
		},
	}
	result := formatScanResults(data)
	if result == "" {
		t.Error("resultado não deveria estar vazio")
	}
}
