//go:build k8s

package main

import (
	"os"
	"strings"
	"testing"
)

func TestExportMarkdown_ContemSecoes(t *testing.T) {
	patch := "kubectl patch..."
	result := AnalysisResult{
		RootCause:       "CrashLoop",
		TechnicalDetail: "Falha de inicialização",
		Confidence:      70,
		SuggestedFix:    "Verificar configuração",
		KubectlPatch:    &patch,
	}
	content := exportMarkdown(result, "pod-crash", "production")
	if !strings.Contains(content, "# Relatório Sentinel") {
		t.Error("markdown deveria conter título")
	}
	if !strings.Contains(content, "CrashLoop") {
		t.Error("markdown deveria conter causa raiz")
	}
	if !strings.Contains(content, "kubectl patch") {
		t.Error("markdown deveria conter comando sugerido")
	}
}

func TestExportMarkdown_SemComandoOpcional(t *testing.T) {
	result := AnalysisResult{
		RootCause:       "Config inválida",
		TechnicalDetail: "Variável de ambiente ausente",
		Confidence:      90,
		SuggestedFix:    "Adicionar env var",
		KubectlPatch:    nil,
	}
	content := exportMarkdown(result, "pod-config", "default")
	if strings.Contains(content, "Comando Sugerido") {
		t.Error("markdown não deveria conter seção de comando quando KubectlPatch é nil")
	}
}

func TestCacheKey_DiferenteParaInputsDiferentes(t *testing.T) {
	key1 := cacheKey("default", "pod-1", "log A")
	key2 := cacheKey("default", "pod-2", "log A")
	if key1 == key2 {
		t.Error("cacheKey deveria ser diferente para pods diferentes")
	}
}

func TestSaveAndLoadCache(t *testing.T) {
	// Override cacheDir usando chdir para diretório temporário
	originalDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(originalDir)

	result := AnalysisResult{
		RootCause:  "teste",
		Confidence: 80,
	}
	saveCache("default", "pod-test", "logs here", result)
	cached, ok := loadCache("default", "pod-test", "logs here")
	if !ok {
		t.Fatal("esperava encontrar cache")
	}
	if cached.RootCause != "teste" {
		t.Errorf("causa raiz esperada 'teste', obtida %q", cached.RootCause)
	}
}
