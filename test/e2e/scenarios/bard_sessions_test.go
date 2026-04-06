//go:build e2e

package scenarios

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// bardHistoryEntry espelha a struct HistoryEntry do plugin bard
// para leitura/escrita do arquivo JSONL nos testes.
type bardHistoryEntry struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp"`
	SessionID string `json:"session_id,omitempty"`
}

// readBardHistory lê o arquivo de histórico JSONL do bard e retorna as entries.
func readBardHistory(t *testing.T, workDir string) []bardHistoryEntry {
	t.Helper()

	historyPath := filepath.Join(workDir, ".yby", ".bard_history.jsonl")
	f, err := os.Open(historyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("Falha ao abrir histórico: %v", err)
	}
	defer f.Close()

	var entries []bardHistoryEntry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var entry bardHistoryEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Logf("Linha inválida no histórico: %s", line)
			continue
		}
		entries = append(entries, entry)
	}

	return entries
}

// writeBardHistory escreve entries no arquivo de histórico JSONL.
func writeBardHistory(t *testing.T, workDir string, entries []bardHistoryEntry) {
	t.Helper()

	ybyDir := filepath.Join(workDir, ".yby")
	if err := os.MkdirAll(ybyDir, 0755); err != nil {
		t.Fatalf("Falha ao criar diretório .yby: %v", err)
	}

	historyPath := filepath.Join(ybyDir, ".bard_history.jsonl")
	f, err := os.Create(historyPath)
	if err != nil {
		t.Fatalf("Falha ao criar arquivo de histórico: %v", err)
	}
	defer f.Close()

	for _, entry := range entries {
		data, _ := json.Marshal(entry)
		fmt.Fprintln(f, string(data))
	}
}

// TestBardSessions_NewSessionPerInvocation verifica que o formato de session_id
// é baseado em timestamp e que invocações separadas gerariam IDs diferentes.
// Como o modo batch não salva histórico, testamos a lógica de sessões
// simulando o formato de arquivo JSONL que o bard produz.
func TestBardSessions_NewSessionPerInvocation(t *testing.T) {
	workDir := t.TempDir()

	// Simular duas sessões com timestamps diferentes
	session1 := time.Now().Format("20060102-150405")
	time.Sleep(1100 * time.Millisecond)
	session2 := time.Now().Format("20060102-150405")

	if session1 == session2 {
		t.Fatal("Session IDs devem ser diferentes para timestamps separados por >1s")
	}

	entries := []bardHistoryEntry{
		{Role: "user", Content: "Pergunta 1", Timestamp: time.Now().Format(time.RFC3339), SessionID: session1},
		{Role: "assistant", Content: "Resposta 1", Timestamp: time.Now().Format(time.RFC3339), SessionID: session1},
		{Role: "user", Content: "Pergunta 2", Timestamp: time.Now().Format(time.RFC3339), SessionID: session2},
		{Role: "assistant", Content: "Resposta 2", Timestamp: time.Now().Format(time.RFC3339), SessionID: session2},
	}

	writeBardHistory(t, workDir, entries)

	// Ler de volta e verificar
	readEntries := readBardHistory(t, workDir)
	if len(readEntries) != 4 {
		t.Fatalf("Esperava 4 entries, obteve %d", len(readEntries))
	}

	// Verificar que há 2 session IDs distintos
	sessionIDs := make(map[string]int)
	for _, e := range readEntries {
		sessionIDs[e.SessionID]++
	}

	if len(sessionIDs) != 2 {
		t.Errorf("Esperava 2 session IDs distintos, encontrou %d: %v", len(sessionIDs), sessionIDs)
	}

	// Cada sessão deve ter exatamente 2 mensagens
	for sid, count := range sessionIDs {
		if count != 2 {
			t.Errorf("Sessão %s deveria ter 2 mensagens, tem %d", sid, count)
		}
	}

	t.Logf("Session IDs verificados: %v", sessionIDs)
}

// TestBardSessions_BackwardCompat verifica que entries sem session_id (formato antigo)
// são parseadas corretamente e coexistem com entries novas que têm session_id.
func TestBardSessions_BackwardCompat(t *testing.T) {
	workDir := t.TempDir()
	ybyDir := filepath.Join(workDir, ".yby")
	if err := os.MkdirAll(ybyDir, 0755); err != nil {
		t.Fatalf("Falha ao criar diretório .yby: %v", err)
	}

	// Criar arquivo JSONL com mix de entries antigas (sem session_id) e novas
	historyPath := filepath.Join(ybyDir, ".bard_history.jsonl")
	f, err := os.Create(historyPath)
	if err != nil {
		t.Fatalf("Falha ao criar arquivo de histórico: %v", err)
	}

	// Entries no formato antigo (sem session_id)
	oldEntries := []map[string]string{
		{"role": "user", "content": "Pergunta antiga 1", "timestamp": "2025-01-01T10:00:00Z"},
		{"role": "assistant", "content": "Resposta antiga 1", "timestamp": "2025-01-01T10:00:05Z"},
	}
	for _, entry := range oldEntries {
		data, _ := json.Marshal(entry)
		fmt.Fprintln(f, string(data))
	}

	// Entries no formato novo (com session_id)
	newEntries := []bardHistoryEntry{
		{Role: "user", Content: "Pergunta nova", Timestamp: "2025-06-01T10:00:00Z", SessionID: "20250601-100000"},
		{Role: "assistant", Content: "Resposta nova", Timestamp: "2025-06-01T10:00:05Z", SessionID: "20250601-100000"},
	}
	for _, entry := range newEntries {
		data, _ := json.Marshal(entry)
		fmt.Fprintln(f, string(data))
	}
	f.Close()

	// Ler todas as entries
	entries := readBardHistory(t, workDir)
	if len(entries) != 4 {
		t.Fatalf("Esperava 4 entries, obteve %d", len(entries))
	}

	// Entries antigas devem ter session_id vazio (o bard interno atribui "legacy")
	for i, e := range entries[:2] {
		if e.SessionID != "" {
			t.Errorf("Entry antiga %d deveria ter session_id vazio no JSON, obteve %q", i, e.SessionID)
		}
		if e.Role == "" || e.Content == "" {
			t.Errorf("Entry antiga %d com campos vazios: role=%q content=%q", i, e.Role, e.Content)
		}
	}

	// Entries novas devem ter session_id preenchido
	for i, e := range entries[2:] {
		if e.SessionID == "" {
			t.Errorf("Entry nova %d deveria ter session_id, está vazio", i)
		}
		if e.SessionID != "20250601-100000" {
			t.Errorf("Entry nova %d session_id inesperado: %q", i, e.SessionID)
		}
	}

	// Simular o mapeamento "legacy" que o bard faz em loadAllEntries
	for i := range entries {
		if entries[i].SessionID == "" {
			entries[i].SessionID = "legacy"
		}
	}

	// Após mapeamento, deve ter 2 sessões: "legacy" e "20250601-100000"
	sessionIDs := make(map[string]int)
	for _, e := range entries {
		sessionIDs[e.SessionID]++
	}

	if len(sessionIDs) != 2 {
		t.Errorf("Esperava 2 sessões (legacy + nova), encontrou %d: %v", len(sessionIDs), sessionIDs)
	}
	if sessionIDs["legacy"] != 2 {
		t.Errorf("Sessão 'legacy' deveria ter 2 entries, tem %d", sessionIDs["legacy"])
	}
	if sessionIDs["20250601-100000"] != 2 {
		t.Errorf("Sessão '20250601-100000' deveria ter 2 entries, tem %d", sessionIDs["20250601-100000"])
	}

	t.Logf("Backward compat OK: sessões %v", sessionIDs)
}

// TestBardSessions_ListSessions verifica que é possível agrupar entries por sessão
// e produzir um resumo correto, replicando a lógica de listSessions do bard.
func TestBardSessions_ListSessions(t *testing.T) {
	workDir := t.TempDir()

	// Criar histórico com 3 sessões
	entries := []bardHistoryEntry{
		{Role: "user", Content: "S1 Q1", Timestamp: "2025-01-01T10:00:00Z", SessionID: "20250101-100000"},
		{Role: "assistant", Content: "S1 A1", Timestamp: "2025-01-01T10:00:05Z", SessionID: "20250101-100000"},
		{Role: "user", Content: "S1 Q2", Timestamp: "2025-01-01T10:01:00Z", SessionID: "20250101-100000"},
		{Role: "assistant", Content: "S1 A2", Timestamp: "2025-01-01T10:01:05Z", SessionID: "20250101-100000"},

		{Role: "user", Content: "S2 Q1", Timestamp: "2025-02-01T10:00:00Z", SessionID: "20250201-100000"},
		{Role: "assistant", Content: "S2 A1", Timestamp: "2025-02-01T10:00:05Z", SessionID: "20250201-100000"},

		{Role: "user", Content: "S3 Q1", Timestamp: "2025-03-01T10:00:00Z", SessionID: "20250301-100000"},
	}

	writeBardHistory(t, workDir, entries)

	// Reler e agrupar por sessão (replicando lógica de listSessions)
	readEntries := readBardHistory(t, workDir)
	if len(readEntries) != 7 {
		t.Fatalf("Esperava 7 entries, obteve %d", len(readEntries))
	}

	// Agrupar por session_id
	sessionMap := make(map[string][]bardHistoryEntry)
	var sessionOrder []string
	for _, e := range readEntries {
		sid := e.SessionID
		if _, exists := sessionMap[sid]; !exists {
			sessionOrder = append(sessionOrder, sid)
		}
		sessionMap[sid] = append(sessionMap[sid], e)
	}

	if len(sessionMap) != 3 {
		t.Errorf("Esperava 3 sessões, encontrou %d", len(sessionMap))
	}

	// Verificar contagens
	expectedCounts := map[string]int{
		"20250101-100000": 4,
		"20250201-100000": 2,
		"20250301-100000": 1,
	}
	for sid, expected := range expectedCounts {
		actual := len(sessionMap[sid])
		if actual != expected {
			t.Errorf("Sessão %s: esperava %d entries, obteve %d", sid, expected, actual)
		}
	}

	// Verificar ordem de aparição
	if len(sessionOrder) >= 3 {
		if sessionOrder[0] != "20250101-100000" || sessionOrder[1] != "20250201-100000" || sessionOrder[2] != "20250301-100000" {
			t.Errorf("Ordem das sessões inesperada: %v", sessionOrder)
		}
	}

	t.Logf("Sessões listadas com sucesso: %v", sessionOrder)
}

// TestBardSessions_FilterBySession verifica que é possível filtrar entries
// de uma sessão específica, replicando a lógica de loadSessionHistory.
func TestBardSessions_FilterBySession(t *testing.T) {
	workDir := t.TempDir()

	entries := []bardHistoryEntry{
		{Role: "user", Content: "S1 Q1", Timestamp: "2025-01-01T10:00:00Z", SessionID: "sessao-a"},
		{Role: "assistant", Content: "S1 A1", Timestamp: "2025-01-01T10:00:05Z", SessionID: "sessao-a"},
		{Role: "user", Content: "S2 Q1", Timestamp: "2025-02-01T10:00:00Z", SessionID: "sessao-b"},
		{Role: "assistant", Content: "S2 A1", Timestamp: "2025-02-01T10:00:05Z", SessionID: "sessao-b"},
		{Role: "user", Content: "S1 Q2", Timestamp: "2025-01-01T10:02:00Z", SessionID: "sessao-a"},
	}

	writeBardHistory(t, workDir, entries)
	allEntries := readBardHistory(t, workDir)

	// Filtrar por sessao-a
	var sessionA []bardHistoryEntry
	for _, e := range allEntries {
		if e.SessionID == "sessao-a" {
			sessionA = append(sessionA, e)
		}
	}

	if len(sessionA) != 3 {
		t.Errorf("Sessão 'sessao-a' deveria ter 3 entries, tem %d", len(sessionA))
	}

	// Filtrar por sessao-b
	var sessionB []bardHistoryEntry
	for _, e := range allEntries {
		if e.SessionID == "sessao-b" {
			sessionB = append(sessionB, e)
		}
	}

	if len(sessionB) != 2 {
		t.Errorf("Sessão 'sessao-b' deveria ter 2 entries, tem %d", len(sessionB))
	}

	// Filtrar por sessão inexistente
	var sessionX []bardHistoryEntry
	for _, e := range allEntries {
		if e.SessionID == "inexistente" {
			sessionX = append(sessionX, e)
		}
	}

	if len(sessionX) != 0 {
		t.Errorf("Sessão inexistente deveria ter 0 entries, tem %d", len(sessionX))
	}
}
