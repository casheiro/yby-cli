package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

const maxHistoryEntries = 20
const historyFile = ".yby/.bard_history.jsonl"
const maxHistoryAge = 30 * 24 * time.Hour

// secretPatterns contém expressões regulares para detectar secrets comuns.
var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`ghp_[A-Za-z0-9_]{36,}`),                        // GitHub personal access token
	regexp.MustCompile(`gho_[A-Za-z0-9_]{36,}`),                        // GitHub OAuth token
	regexp.MustCompile(`ghu_[A-Za-z0-9_]{36,}`),                        // GitHub user-to-server token
	regexp.MustCompile(`ghs_[A-Za-z0-9_]{36,}`),                        // GitHub server-to-server token
	regexp.MustCompile(`github_pat_[A-Za-z0-9_]{22,}`),                 // GitHub fine-grained PAT
	regexp.MustCompile(`sk-[A-Za-z0-9]{20,}`),                          // OpenAI / Stripe secret key
	regexp.MustCompile(`AKIA[0-9A-Z]{16}`),                             // AWS access key ID
	regexp.MustCompile(`(?i)password\s*[=:]\s*\S+`),                    // password=xxx ou password: xxx
	regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9\-._~+/]+=*`),           // Bearer tokens
	regexp.MustCompile(`(?i)api[_-]?key\s*[=:]\s*\S+`),                 // api_key=xxx ou api-key: xxx
	regexp.MustCompile(`(?i)secret[_-]?key\s*[=:]\s*\S+`),              // secret_key=xxx
	regexp.MustCompile(`xox[bporas]-[A-Za-z0-9\-]+`),                   // Slack tokens
	regexp.MustCompile(`(?i)token\s*[=:]\s*[A-Za-z0-9\-._~+/]{20,}=*`), // token genérico longo
}

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

// sanitizeContent substitui patterns de secrets conhecidos por "[REDACTED]".
func sanitizeContent(content string) string {
	for _, pattern := range secretPatterns {
		content = pattern.ReplaceAllString(content, "[REDACTED]")
	}
	return content
}

// loadAllEntries lê todas as entradas do arquivo de histórico JSONL.
// Entries sem session_id (histórico antigo) recebem session_id "legacy".
// Entries com timestamp mais antigo que maxHistoryAge são descartadas.
func loadAllEntries() ([]HistoryEntry, error) {
	f, err := os.Open(historyFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	cutoff := time.Now().Add(-maxHistoryAge)

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

		// Filtrar entries mais antigas que maxHistoryAge
		if ts, err := time.Parse(time.RFC3339, entry.Timestamp); err == nil {
			if ts.Before(cutoff) {
				continue
			}
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
// Cria o diretório .yby se não existir. O conteúdo é sanitizado
// para remover secrets antes de persistir.
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
		Content:   sanitizeContent(content),
		Timestamp: time.Now().Format(time.RFC3339),
		SessionID: sessionID,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return
	}

	fmt.Fprintln(f, string(data))
}

// PurgeOldEntries reescreve o arquivo de histórico removendo entries
// com timestamp mais antigo que maxHistoryAge.
func PurgeOldEntries() error {
	entries, err := loadAllEntriesRaw()
	if err != nil {
		return err
	}
	if entries == nil {
		return nil
	}

	cutoff := time.Now().Add(-maxHistoryAge)
	var kept []HistoryEntry
	for _, e := range entries {
		if ts, err := time.Parse(time.RFC3339, e.Timestamp); err == nil {
			if ts.Before(cutoff) {
				continue
			}
		}
		kept = append(kept, e)
	}

	// Reescrever o arquivo com apenas as entries válidas
	dir := filepath.Dir(historyFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	f, err := os.Create(historyFile)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, e := range kept {
		data, err := json.Marshal(e)
		if err != nil {
			continue
		}
		fmt.Fprintln(f, string(data))
	}

	return nil
}

// loadAllEntriesRaw lê todas as entradas do arquivo sem aplicar filtro de idade.
// Usada internamente por PurgeOldEntries para evitar dupla filtragem.
func loadAllEntriesRaw() ([]HistoryEntry, error) {
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
