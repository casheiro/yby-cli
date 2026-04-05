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
	SessionID string `json:"session_id,omitempty"` // identificador da sessão
}

// SessionSummary contém o resumo de uma sessão de conversas.
type SessionSummary struct {
	SessionID    string
	MessageCount int
	FirstMessage time.Time
	LastMessage  time.Time
}

// loadAllEntries lê todas as entradas do arquivo de histórico JSONL.
// Entries sem session_id (histórico antigo) recebem session_id "legacy".
func loadAllEntries() ([]HistoryEntry, error) {
	f, err := os.Open(historyFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
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
		if entry.SessionID == "" {
			entry.SessionID = "legacy"
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

// loadHistory lê o arquivo de histórico JSONL e retorna as últimas
// maxHistoryEntries entradas. Se o arquivo não existir, retorna slice vazio.
func loadHistory() []HistoryEntry {
	entries, _ := loadAllEntries()

	if len(entries) > maxHistoryEntries {
		entries = entries[len(entries)-maxHistoryEntries:]
	}

	return entries
}

// loadSessionHistory filtra entries pela sessão especificada e retorna
// no máximo maxEntries entradas (as mais recentes).
func loadSessionHistory(entries []HistoryEntry, sessionID string, maxEntries int) []HistoryEntry {
	var filtered []HistoryEntry
	for _, e := range entries {
		if e.SessionID == sessionID {
			filtered = append(filtered, e)
		}
	}

	if len(filtered) > maxEntries {
		filtered = filtered[len(filtered)-maxEntries:]
	}

	return filtered
}

// listSessions agrupa entries por SessionID e retorna um resumo de cada sessão.
func listSessions(entries []HistoryEntry) []SessionSummary {
	summaryMap := make(map[string]*SessionSummary)
	var order []string

	for _, e := range entries {
		sid := e.SessionID
		s, exists := summaryMap[sid]
		if !exists {
			s = &SessionSummary{SessionID: sid}
			summaryMap[sid] = s
			order = append(order, sid)
		}
		s.MessageCount++

		ts, err := time.Parse(time.RFC3339, e.Timestamp)
		if err != nil {
			continue
		}
		if s.FirstMessage.IsZero() || ts.Before(s.FirstMessage) {
			s.FirstMessage = ts
		}
		if ts.After(s.LastMessage) {
			s.LastMessage = ts
		}
	}

	var result []SessionSummary
	for _, sid := range order {
		result = append(result, *summaryMap[sid])
	}

	return result
}

// saveMessage adiciona uma entrada ao arquivo de histórico JSONL.
// Cria o diretório .yby se não existir.
func saveMessage(role, content, sessionID string) {
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
		SessionID: sessionID,
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
