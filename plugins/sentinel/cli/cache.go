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
const cacheDir = ".yby/.sentinel/cache"

// CacheEntry representa uma entrada no cache de análises.
type CacheEntry struct {
	Result    AnalysisResult `json:"result"`
	Timestamp time.Time      `json:"timestamp"`
	LogsHash  string         `json:"logs_hash"`
}

// cacheKey gera a chave de cache baseada no namespace/pod e logs.
func cacheKey(namespace, podName, logs string) string {
	// Usar os primeiros 500 caracteres dos logs para o hash
	logsSample := logs
	if len(logsSample) > 500 {
		logsSample = logsSample[:500]
	}
	h := sha256.Sum256([]byte(fmt.Sprintf("%s/%s:%s", namespace, podName, logsSample)))
	return fmt.Sprintf("%x", h)
}

// loadCache tenta carregar resultado do cache.
func loadCache(namespace, podName, logs string) (*AnalysisResult, bool) {
	key := cacheKey(namespace, podName, logs)
	path := filepath.Join(cacheDir, key+".json")

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
func saveCache(namespace, podName, logs string, result AnalysisResult) {
	key := cacheKey(namespace, podName, logs)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
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

	_ = os.WriteFile(filepath.Join(cacheDir, key+".json"), data, 0644)
}
