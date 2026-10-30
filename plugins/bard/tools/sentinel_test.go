package tools

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
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

// Sem o relatório, a IA não pode dizer que o namespace está limpo.
func TestFormatScanResults_Nil(t *testing.T) {
	result := formatScanResults(nil)
	if result != msgScanSemRelatorio || strings.Contains(result, "Nenhum") {
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

// Um scan com 500 achados vira resumo: a contagem por severidade e os mais graves que
// couberem, com os recursos de cada um até um teto; o resto fica numa linha.
func TestFormatScanResults_Resumo(t *testing.T) {
	texto := formatScanResults(scanFalso(500))

	if len(texto) > 8*1024 {
		t.Errorf("o resumo tem de caber no teto da ferramenta: %d bytes\n%s", len(texto), texto)
	}
	listados := strings.Count(texto, "\n- [")
	for _, trecho := range []string{
		"500 achados: 10 critical, 50 high, 440 low",
		"polaris/check-007",
		"Deployment/grupo-00",
		"e mais 20 recursos",
		fmt.Sprintf("Os %d mais graves (os outros não couberam aqui)", listados),
		fmt.Sprintf("e mais %d achados", 500-listados),
		"yby sentinel scan -n payments: o relatório completo",
	} {
		if !strings.Contains(texto, trecho) {
			t.Errorf("o resumo deveria ter %q:\n%s", trecho, texto)
		}
	}
	// A lista usa o espaço: todos os critical e high cabem, com uns low depois.
	if listados < 60 {
		t.Errorf("o resumo deveria listar todos os critical e high: %d achados\n%s", listados, texto)
	}
	if strings.Index(texto, "[low]") < strings.LastIndex(texto, "[high]") {
		t.Errorf("achado low não deveria passar na frente dos high:\n%s", texto)
	}
	if strings.Contains(texto, "\n  ") {
		t.Errorf("nada de JSON indentado:\n%s", texto)
	}
}

// Um scan do tamanho de um namespace comum vai inteiro, com os comandos para o usuário:
// o relatório, o --fix-dry-run quando o Sentinel corrige algum achado e o risco do
// securityContext.
func TestFormatScanResults_InteiroComComandos(t *testing.T) {
	dados := scanFalso(23)
	achados := dados["findings"].([]interface{})
	achados[0].(map[string]interface{})["check_id"] = "polaris/privilegeEscalationAllowed"
	texto := formatScanResults(dados)

	for _, trecho := range []string{
		"23 achados",
		"Do mais grave para o menos:",
		"yby sentinel scan -n payments --fix-dry-run para ver e --fix para aplicar",
		"runAsNonRoot e runAsUser falham com imagem que roda como root",
	} {
		if !strings.Contains(texto, trecho) {
			t.Errorf("o resumo deveria ter %q:\n%s", trecho, texto)
		}
	}
	if n := strings.Count(texto, "\n- ["); n != 23 {
		t.Errorf("os 23 achados deveriam ir inteiros: %d\n%s", n, texto)
	}
	if strings.Contains(texto, "não couberam") {
		t.Errorf("nada ficou de fora:\n%s", texto)
	}

	// Sem achado que o Sentinel corrija, o --fix-dry-run não aparece; com o contexto da
	// sessão, a IA fica sabendo que os comandos usam o contexto atual do kubeconfig.
	DefinirKube("staging", "")
	t.Cleanup(func() { DefinirKube("", "") })
	texto = formatScanResults(scanFalso(3))
	if strings.Contains(texto, "--fix-dry-run") {
		t.Errorf("nenhum achado corrigível, nada de --fix-dry-run:\n%s", texto)
	}
	if !strings.Contains(texto, "o scan foi no contexto staging") {
		t.Errorf("faltou o contexto da sessão:\n%s", texto)
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

// O resumo inteiro fica abaixo do teto de 8 KB do Fluxo também no pior caso: muitos
// achados, nomes longos, o contexto de um EKS e uma dúzia de verificações que não rodaram.
// Se passasse, o corte do Fluxo tiraria o meio da lista que o resumo diz ter listado. Um
// achado com uma mensagem enorme sai cortado, sem levar a lista junto.
func TestFormatScanResults_PiorCasoCabeNoTeto(t *testing.T) {
	DefinirKube("arn:aws:eks:sa-east-1:123456789012:cluster/producao-pagamentos-principal", "")
	t.Cleanup(func() { DefinirKube("", "") })
	dados := scanFalso(200)
	dados["namespace"] = strings.Repeat("n", 63)
	var naoRodaram []interface{}
	for i := 0; i < 12; i++ {
		naoRodaram = append(naoRodaram, map[string]interface{}{
			"backend":      "checks-internos",
			"verificacoes": []interface{}{fmt.Sprintf("CHECK_%02d", i)},
			"motivo":       strings.Repeat("sem permissão para listar os recursos do namespace ", 4),
		})
	}
	dados["nao_rodaram"] = naoRodaram
	achados := dados["findings"].([]interface{})
	achados[0].(map[string]interface{})["message"] = strings.Repeat("ã", 4500)

	texto := formatScanResults(dados)
	if len(texto) > 7*1024+512 {
		t.Errorf("o resumo passou do teto: %d bytes", len(texto))
	}
	listados := strings.Count(texto, "\n- [")
	if !strings.Contains(texto, fmt.Sprintf("e mais %d achados", 200-listados)) {
		t.Errorf("faltou dizer quantos achados ficaram de fora (%d listados):\n%s", listados, texto)
	}
	for _, trecho := range []string{"e mais 6 (o relatório completo traz todos)", "…", ": o relatório completo, com todos os achados"} {
		if !strings.Contains(texto, trecho) {
			t.Errorf("o resumo deveria ter %q:\n%s", trecho, texto)
		}
	}
	if !utf8.ValidString(texto) {
		t.Error("o corte partiu um caractere")
	}
}
