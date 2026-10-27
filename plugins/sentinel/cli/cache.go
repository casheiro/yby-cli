//go:build k8s

package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const cacheTTL = 1 * time.Hour

// getCacheDir retorna o diretorio global de cache do Sentinel (~/.yby/sentinel/cache).
func getCacheDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".yby/sentinel/cache" // fallback para cwd se nao encontrar home
	}
	return filepath.Join(home, ".yby", "sentinel", "cache")
}

// CacheEntry representa uma entrada no cache de análises.
type CacheEntry struct {
	Result    AnalysisResult `json:"result"`
	Timestamp time.Time      `json:"timestamp"`
	LogsHash  string         `json:"logs_hash"`
}

// idPod identifica a instância do pod a que uma análise guardada se refere: o mesmo nome
// em outro cluster, ou depois de recriado ou reiniciado, é outro caso.
type idPod struct {
	Contexto, Namespace, Pod, UID string
	Reinicios                     int32
}

// cacheKey gera a chave de cache baseada na identidade do pod e nos logs.
func cacheKey(id idPod, logs string) string {
	// Usar os primeiros 500 caracteres dos logs para o hash
	logsSample := logs
	if len(logsSample) > 500 {
		logsSample = logsSample[:500]
	}
	h := sha256.Sum256([]byte(fmt.Sprintf("%s|%s/%s|%s|%d:%s", id.Contexto, id.Namespace, id.Pod, id.UID, id.Reinicios, logsSample)))
	return fmt.Sprintf("%x", h)
}

// loadCache tenta carregar resultado do cache.
func loadCache(id idPod, logs string) (*AnalysisResult, bool) {
	key := cacheKey(id, logs)
	path := filepath.Join(getCacheDir(), key+".json")

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}

	var entry CacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, false
	}

	// Verificar TTL
	if time.Since(entry.Timestamp) > cacheTTL {
		_ = os.Remove(path) // Cache expirado
		return nil, false
	}

	return &entry.Result, true
}

// saveCache salva resultado no cache.
func saveCache(id idPod, logs string, result AnalysisResult) {
	key := cacheKey(id, logs)
	if err := os.MkdirAll(getCacheDir(), 0755); err != nil {
		return
	}

	entry := CacheEntry{
		Result:    result,
		Timestamp: time.Now(),
		LogsHash:  key,
	}

	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return
	}

	_ = os.WriteFile(filepath.Join(getCacheDir(), key+".json"), data, 0600)
}
