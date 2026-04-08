//go:build k8s

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCacheKey_Deterministico(t *testing.T) {
	key1 := cacheKey("default", "pod-1", "log line 1")
	key2 := cacheKey("default", "pod-1", "log line 1")
	if key1 != key2 {
		t.Error("cacheKey deveria ser determinístico para os mesmos inputs")
	}
}

func TestCacheKey_InputsDiferentes(t *testing.T) {
	base := cacheKey("default", "pod-1", "log A")

	keyPodDiferente := cacheKey("default", "pod-2", "log A")
	if base == keyPodDiferente {
		t.Error("cacheKey deveria ser diferente para pods diferentes")
	}

	keyNamespaceDiferente := cacheKey("prod", "pod-1", "log A")
	if base == keyNamespaceDiferente {
		t.Error("cacheKey deveria ser diferente para namespaces diferentes")
	}

	keyLogsDiferentes := cacheKey("default", "pod-1", "log B")
	if base == keyLogsDiferentes {
		t.Error("cacheKey deveria ser diferente para logs diferentes")
	}
}

func TestCacheKey_TruncaLogsAcima500Chars(t *testing.T) {
	// Dois logs com os primeiros 500 caracteres idênticos mas que diferem após 500
	base := strings.Repeat("x", 500)
	logs1 := base + "AAA"
	logs2 := base + "BBB"
	key1 := cacheKey("default", "pod-1", logs1)
	key2 := cacheKey("default", "pod-1", logs2)
	if key1 != key2 {
		t.Error("cacheKey deveria ser igual quando logs diferem apenas após os 500 primeiros caracteres")
	}

	// Verifica que logs com primeiros 500 chars diferentes produzem chaves diferentes
	logsA := "A" + strings.Repeat("x", 499) + "cauda"
	logsB := "B" + strings.Repeat("x", 499) + "cauda"
	keyA := cacheKey("default", "pod-1", logsA)
	keyB := cacheKey("default", "pod-1", logsB)
	if keyA == keyB {
		t.Error("cacheKey deveria ser diferente quando os primeiros 500 chars diferem")
	}
}

func TestSaveAndLoadCache_RoundTrip(t *testing.T) {
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("falha ao obter diretório atual: %v", err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("falha ao mudar para tmpDir: %v", err)
	}
	defer os.Chdir(originalDir) //nolint:errcheck

	result := AnalysisResult{
		RootCause:       "OOM Killed",
		TechnicalDetail: "Container excedeu limites de memória",
		Confidence:      90,
		SuggestedFix:    "Aumentar limits.memory",
	}

	saveCache("default", "pod-roundtrip", "some logs", result)

	cached, ok := loadCache("default", "pod-roundtrip", "some logs")
	if !ok {
		t.Fatal("esperava encontrar cache após saveCache")
	}
	if cached.RootCause != result.RootCause {
		t.Errorf("RootCause: esperado %q, obtido %q", result.RootCause, cached.RootCause)
	}
	if cached.Confidence != result.Confidence {
		t.Errorf("Confidence: esperado %d, obtido %d", result.Confidence, cached.Confidence)
	}
	if cached.TechnicalDetail != result.TechnicalDetail {
		t.Errorf("TechnicalDetail: esperado %q, obtido %q", result.TechnicalDetail, cached.TechnicalDetail)
	}
}

func TestLoadCache_TTLExpirado(t *testing.T) {
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("falha ao obter diretório atual: %v", err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("falha ao mudar para tmpDir: %v", err)
	}
	defer os.Chdir(originalDir) //nolint:errcheck

	namespace := "default"
	podName := "pod-expired"
	logs := "logs do pod expirado"

	key := cacheKey(namespace, podName, logs)
	cacheFilePath := filepath.Join(getCacheDir(), key+".json")

	if err := os.MkdirAll(getCacheDir(), 0755); err != nil {
		t.Fatalf("falha ao criar diretório de cache: %v", err)
	}

	// Escrever entrada com timestamp expirado (2 horas atrás, além do TTL de 1 hora)
	entry := CacheEntry{
		Result: AnalysisResult{
			RootCause:  "expirado",
			Confidence: 50,
		},
		Timestamp: time.Now().Add(-2 * time.Hour),
		LogsHash:  key,
	}

	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		t.Fatalf("falha ao serializar entrada de cache: %v", err)
	}
	if err := os.WriteFile(cacheFilePath, data, 0600); err != nil {
		t.Fatalf("falha ao escrever arquivo de cache: %v", err)
	}

	_, ok := loadCache(namespace, podName, logs)
	if ok {
		t.Error("loadCache deveria retornar false para entrada com TTL expirado")
	}
}

func TestLoadCache_ArquivoInexistente(t *testing.T) {
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("falha ao obter diretório atual: %v", err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("falha ao mudar para tmpDir: %v", err)
	}
	defer os.Chdir(originalDir) //nolint:errcheck

	_, ok := loadCache("default", "pod-inexistente", "logs inexistentes")
	if ok {
		t.Error("loadCache deveria retornar false quando o arquivo não existe")
	}
}
