package cloud

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// maxAuditFileSize define o tamanho máximo do arquivo de auditoria antes da rotação (10MB).
const maxAuditFileSize = 10 * 1024 * 1024

// CloudAuditEvent representa um evento de auditoria de operação cloud.
type CloudAuditEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Action    string    `json:"action"`
	Provider  string    `json:"provider"`
	Identity  string    `json:"identity"`
	Role      string    `json:"role,omitempty"`
	Cluster   string    `json:"cluster,omitempty"`
	Region    string    `json:"region,omitempty"`
	Method    string    `json:"method,omitempty"`
	Success   bool      `json:"success"`
	Error     string    `json:"error,omitempty"`
	Duration  string    `json:"duration,omitempty"`
}

// AuditLogger persiste eventos de auditoria cloud em arquivo JSONL com rotação por tamanho.
type AuditLogger struct {
	filePath    string
	mu          sync.Mutex
	maxFileSize int64
}

// NewAuditLogger cria um AuditLogger com caminho padrão (~/.yby/audit.log).
func NewAuditLogger() *AuditLogger {
	home, _ := os.UserHomeDir()
	return &AuditLogger{
		filePath:    filepath.Join(home, ".yby", "audit.log"),
		maxFileSize: maxAuditFileSize,
	}
}

// NewAuditLoggerWithPath cria um AuditLogger com caminho customizado (usado em testes).
func NewAuditLoggerWithPath(path string) *AuditLogger {
	return &AuditLogger{
		filePath:    path,
		maxFileSize: maxAuditFileSize,
	}
}

// Log serializa o evento em JSON e appenda ao arquivo de auditoria.
// Rotaciona o arquivo se exceder maxFileSize.
func (a *AuditLogger) Log(event CloudAuditEvent) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(a.filePath), 0755); err != nil {
		return err
	}

	if err := a.rotateIfNeeded(); err != nil {
		return err
	}

	f, err := os.OpenFile(a.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	_, err = f.Write(data)
	return err
}

// LogAuthentication registra um evento de autenticação.
func (a *AuditLogger) LogAuthentication(provider, identity, method string, success bool, err error) error {
	event := CloudAuditEvent{
		Timestamp: time.Now(),
		Action:    "authenticate",
		Provider:  provider,
		Identity:  identity,
		Method:    method,
		Success:   success,
	}
	if err != nil {
		event.Error = err.Error()
	}
	return a.Log(event)
}

// LogRefresh registra um evento de refresh de credenciais.
func (a *AuditLogger) LogRefresh(provider, cluster string, success bool, err error) error {
	event := CloudAuditEvent{
		Timestamp: time.Now(),
		Action:    "refresh",
		Provider:  provider,
		Cluster:   cluster,
		Success:   success,
	}
	if err != nil {
		event.Error = err.Error()
	}
	return a.Log(event)
}

// LogAssumeRole registra um evento de assume-role.
func (a *AuditLogger) LogAssumeRole(provider, identity, role string, success bool, err error) error {
	event := CloudAuditEvent{
		Timestamp: time.Now(),
		Action:    "assume-role",
		Provider:  provider,
		Identity:  identity,
		Role:      role,
		Success:   success,
	}
	if err != nil {
		event.Error = err.Error()
	}
	return a.Log(event)
}

// ReadEvents lê eventos do arquivo de auditoria desde o timestamp informado.
func (a *AuditLogger) ReadEvents(since time.Time) ([]CloudAuditEvent, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	data, err := os.ReadFile(a.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var events []CloudAuditEvent
	decoder := json.NewDecoder(bytesReader(data))
	for decoder.More() {
		var event CloudAuditEvent
		if err := decoder.Decode(&event); err != nil {
			continue
		}
		if !event.Timestamp.Before(since) {
			events = append(events, event)
		}
	}

	return events, nil
}

// Export exporta eventos para o writer no formato especificado (json ou csv).
func (a *AuditLogger) Export(format string, since time.Time, w io.Writer) error {
	events, err := a.ReadEvents(since)
	if err != nil {
		return err
	}

	switch format {
	case "json":
		return a.exportJSON(events, w)
	case "csv":
		return a.exportCSV(events, w)
	default:
		return fmt.Errorf("formato de exportação não suportado: %s", format)
	}
}

func (a *AuditLogger) exportJSON(events []CloudAuditEvent, w io.Writer) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(events)
}

func (a *AuditLogger) exportCSV(events []CloudAuditEvent, w io.Writer) error {
	writer := csv.NewWriter(w)
	defer writer.Flush()

	header := []string{"timestamp", "action", "provider", "identity", "role", "cluster", "region", "method", "success", "error", "duration"}
	if err := writer.Write(header); err != nil {
		return err
	}

	for _, e := range events {
		record := []string{
			e.Timestamp.Format(time.RFC3339),
			e.Action,
			e.Provider,
			e.Identity,
			e.Role,
			e.Cluster,
			e.Region,
			e.Method,
			fmt.Sprintf("%t", e.Success),
			e.Error,
			e.Duration,
		}
		if err := writer.Write(record); err != nil {
			return err
		}
	}

	return nil
}

// rotateIfNeeded renomeia o arquivo para .1 se exceder maxFileSize.
func (a *AuditLogger) rotateIfNeeded() error {
	info, err := os.Stat(a.filePath)
	if err != nil {
		return nil
	}
	if info.Size() < a.maxFileSize {
		return nil
	}
	return os.Rename(a.filePath, a.filePath+".1")
}

// bytesReader cria um io.Reader a partir de bytes, compatível com json.NewDecoder para JSONL.
func bytesReader(data []byte) io.Reader {
	return &jsonlReader{data: data, pos: 0}
}

// jsonlReader implementa io.Reader para decodificação JSONL.
type jsonlReader struct {
	data []byte
	pos  int
}

func (r *jsonlReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}
