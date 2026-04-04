//go:build k8s

package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExportJSON_FormatoValido(t *testing.T) {
	patch := "kubectl set resources..."
	result := AnalysisResult{
		RootCause:       "OOM",
		TechnicalDetail: "Container excedeu limites de memória",
		Confidence:      85,
		SuggestedFix:    "Aumentar limits.memory",
		KubectlPatch:    &patch,
	}
	content, err := exportJSON(result, "meu-pod", "default")
	if err != nil {
		t.Fatalf("exportJSON falhou: %v", err)
	}
	var report FullReport
	if err := json.Unmarshal([]byte(content), &report); err != nil {
		t.Fatalf("JSON inválido: %v", err)
	}
	if report.Metadata.Pod != "meu-pod" {
		t.Errorf("pod esperado 'meu-pod', obtido %q", report.Metadata.Pod)
	}
	if report.Analysis.Confidence != 85 {
		t.Errorf("confiança esperada 85, obtida %d", report.Analysis.Confidence)
	}
}

func TestExportJSON_MetadadosCorretos(t *testing.T) {
	result := AnalysisResult{
		RootCause:  "Crash",
		Confidence: 70,
	}
	before := time.Now()
	content, err := exportJSON(result, "pod-meta", "prod")
	after := time.Now()
	if err != nil {
		t.Fatalf("exportJSON falhou: %v", err)
	}
	var report FullReport
	if err := json.Unmarshal([]byte(content), &report); err != nil {
		t.Fatalf("JSON inválido: %v", err)
	}
	if report.Metadata.Pod != "pod-meta" {
		t.Errorf("pod esperado 'pod-meta', obtido %q", report.Metadata.Pod)
	}
	if report.Metadata.Namespace != "prod" {
		t.Errorf("namespace esperado 'prod', obtido %q", report.Metadata.Namespace)
	}
	if report.Metadata.Timestamp.Before(before) || report.Metadata.Timestamp.After(after) {
		t.Errorf("timestamp fora do intervalo esperado: %v", report.Metadata.Timestamp)
	}
}

func TestExportMarkdown_SecoesObrigatorias(t *testing.T) {
	patch := "kubectl patch deployment meu-app..."
	result := AnalysisResult{
		RootCause:       "OOMKilled",
		TechnicalDetail: "Memória insuficiente",
		Confidence:      80,
		SuggestedFix:    "Aumentar limits.memory",
		KubectlPatch:    &patch,
	}
	content := exportMarkdown(result, "pod-abc", "staging")

	secoes := []string{
		"# Relatório Sentinel",
		"## Causa Raiz",
		"## Detalhe Técnico",
		"## Confiança",
		"## Sugestão de Correção",
	}
	for _, secao := range secoes {
		if !strings.Contains(content, secao) {
			t.Errorf("seção obrigatória ausente: %q", secao)
		}
	}
}

func TestExportMarkdown_KubectlPatchNil(t *testing.T) {
	result := AnalysisResult{
		RootCause:       "Config inválida",
		TechnicalDetail: "Variável de ambiente ausente",
		Confidence:      90,
		SuggestedFix:    "Adicionar env var",
		KubectlPatch:    nil,
	}
	content := exportMarkdown(result, "pod-nil", "default")
	if strings.Contains(content, "Comando Sugerido") {
		t.Error("não deveria gerar seção 'Comando Sugerido' quando KubectlPatch é nil")
	}
}

func TestExportMarkdown_KubectlPatchNone(t *testing.T) {
	none := "none"
	result := AnalysisResult{
		RootCause:       "Config inválida",
		TechnicalDetail: "Variável de ambiente ausente",
		Confidence:      90,
		SuggestedFix:    "Adicionar env var",
		KubectlPatch:    &none,
	}
	content := exportMarkdown(result, "pod-none", "default")
	if strings.Contains(content, "Comando Sugerido") {
		t.Error("não deveria gerar seção 'Comando Sugerido' quando KubectlPatch é 'none'")
	}
}

func TestWriteReport_ParaArquivo(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "report.json")
	err := writeReport(`{"test": true}`, filePath)
	if err != nil {
		t.Fatalf("writeReport falhou: %v", err)
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("falha ao ler arquivo: %v", err)
	}
	if string(data) != `{"test": true}` {
		t.Errorf("conteúdo inesperado: %s", data)
	}
}

func TestWriteReport_ParaStdout(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("falha ao criar pipe: %v", err)
	}
	original := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = original }()

	const conteudo = "saída para stdout de teste"
	if writeErr := writeReport(conteudo, ""); writeErr != nil {
		w.Close()
		r.Close()
		t.Fatalf("writeReport falhou: %v", writeErr)
	}
	w.Close()

	saida, err := io.ReadAll(r)
	r.Close()
	if err != nil {
		t.Fatalf("falha ao ler pipe: %v", err)
	}

	if string(saida) != conteudo {
		t.Errorf("saída esperada %q, obtida %q", conteudo, string(saida))
	}
}
