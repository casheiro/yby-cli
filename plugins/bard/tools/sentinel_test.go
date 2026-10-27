package tools

import (
	"context"
	"fmt"
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

// scanFalso monta o Data de um scan com n achados: um critical a cada 50, um high a cada
// 10 e o resto low; o primeiro critical agrupa 30 recursos.
func scanFalso(n int) map[string]interface{} {
	var achados []interface{}
	for i := 0; i < n; i++ {
		sev := "low"
		switch {
		case i%50 == 7:
			sev = "critical"
		case i%10 == 3:
			sev = "high"
		}
		achado := map[string]interface{}{
			"check_id": fmt.Sprintf("polaris/check-%03d", i),
			"severity": sev,
			"type":     sev,
			"resource": fmt.Sprintf("Deployment/app-%03d", i),
			"message":  fmt.Sprintf("mensagem do achado %03d", i),
			"scope":    "namespace",
		}
		if i == 7 {
			var recursos []interface{}
			for r := 0; r < 30; r++ {
				recursos = append(recursos, fmt.Sprintf("Deployment/grupo-%02d", r))
			}
			achado["resources"] = recursos
		}
		achados = append(achados, achado)
	}
	return map[string]interface{}{"namespace": "payments", "scope": "namespace", "findings": achados}
}

// Um scan com 500 achados vira resumo: a contagem por severidade e os mais graves, com os
// recursos de cada um até um teto; o resto fica numa linha.
func TestFormatScanResults_Resumo(t *testing.T) {
	texto := formatScanResults(scanFalso(500))

	if len(texto) > 6*1024 {
		t.Errorf("o resumo deveria ser curto: %d bytes\n%s", len(texto), texto)
	}
	for _, trecho := range []string{
		"500 achados: 10 critical, 50 high, 440 low",
		"polaris/check-007",
		"Deployment/grupo-00",
		"e mais 20 recursos",
		fmt.Sprintf("e mais %d achados", 500-maxAchadosResumo),
	} {
		if !strings.Contains(texto, trecho) {
			t.Errorf("o resumo deveria ter %q:\n%s", trecho, texto)
		}
	}
	if strings.Count(texto, "\n- [") != maxAchadosResumo {
		t.Errorf("o resumo deveria listar %d achados:\n%s", maxAchadosResumo, texto)
	}
	// Os mais graves primeiro: com 10 critical, nenhum low entra na lista.
	if strings.Contains(texto, "[low]") {
		t.Errorf("achado low não deveria passar na frente dos críticos:\n%s", texto)
	}
	if strings.Contains(texto, "\n  ") {
		t.Errorf("nada de JSON indentado:\n%s", texto)
	}
}

// O que não é um relatório de scan reconhecível vai como JSON compacto.
func TestFormatScanResults_ForaDoFormatoCompacto(t *testing.T) {
	texto := formatScanResults(map[string]interface{}{"outra": map[string]interface{}{"coisa": 1}})
	if texto != `{"outra":{"coisa":1}}` {
		t.Errorf("esperava JSON compacto: %q", texto)
	}
}

// O investigate vai compacto, com os logs no fim, em linhas, para o corte do Fluxo deixar
// as últimas.
func TestFormatInvestigateResults_Compacto(t *testing.T) {
	texto := formatInvestigateResults(map[string]interface{}{
		"healthy": false,
		"phase":   "Running",
		"logs":    "linha 1\nlinha 2\n",
	})
	if !strings.HasPrefix(texto, `{"healthy":false,"phase":"Running"}`) {
		t.Errorf("esperava JSON compacto sem os logs: %q", texto)
	}
	if !strings.HasSuffix(texto, "linha 1\nlinha 2\n") {
		t.Errorf("os logs deveriam vir no fim, em linhas: %q", texto)
	}
}

// Sem dados, a IA fica sabendo que faltou o que a ferramenta coleta (estado, eventos e
// logs), não "informação de segurança", que é do modelo antigo do investigate.
func TestFormatInvestigateResults_SemDados(t *testing.T) {
	texto := formatInvestigateResults(nil)
	if strings.Contains(texto, "segurança") || !strings.Contains(texto, "estado, eventos e logs") {
		t.Errorf("mensagem inesperada sem dados: %q", texto)
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
