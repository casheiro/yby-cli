package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const maxHistoryEntries = 20
const historyFile = ".yby/.bard_history.jsonl"

// HistoryEntry representa uma entrada no histórico de conversas.
type HistoryEntry struct {
	Role      string `json:"role"` // "user" ou "assistant"
	Content   string `json:"content"`
	Timestamp string `json:"timestamp"`
}

// loadHistory lê o arquivo de histórico JSONL e retorna as últimas
// maxHistoryEntries entradas. Se o arquivo não existir, retorna slice vazio.
func loadHistory() []HistoryEntry {
	f, err := os.Open(historyFile)
	if err != nil {
		return nil
	}
	defer f.Close()

	var entries []HistoryEntry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var entry HistoryEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		entries = append(entries, entry)
	}

	// Retornar apenas as últimas maxHistoryEntries entradas
	if len(entries) > maxHistoryEntries {
		entries = entries[len(entries)-maxHistoryEntries:]
	}

	return entries
}

// saveMessage adiciona uma entrada ao arquivo de histórico JSONL.
// Cria o diretório .yby se não existir.
func saveMessage(role, content string) {
	dir := filepath.Dir(historyFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return
	}

	f, err := os.OpenFile(historyFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer f.Close()

	entry := HistoryEntry{
		Role:      role,
		Content:   content,
		Timestamp: time.Now().Format(time.RFC3339),
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return
	}

	fmt.Fprintln(f, string(data))
}

// clearHistory remove o arquivo de histórico.
func clearHistory() {
	os.Remove(historyFile)
}

// formatHistoryContext formata as entradas de histórico como texto
// para injeção no system prompt.
func formatHistoryContext(entries []HistoryEntry) string {
	if len(entries) == 0 {
		return ""
	}

	result := "## Histórico de Conversas Anteriores\n"
	for _, e := range entries {
		label := "Usuário"
		if e.Role == "assistant" {
			label = "Assistente"
		}
		result += fmt.Sprintf("\n**%s** (%s):\n%s\n", label, e.Timestamp, e.Content)
	}

	return result
}
