package telemetry

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// maxTelemetryFileSize define o tamanho máximo do arquivo de telemetria antes da rotação (5MB).
const maxTelemetryFileSize = 5 * 1024 * 1024

// Event records metrics about a specific operation
type Event struct {
	Name      string
	Duration  time.Duration
	Success   bool
	Error     error
	Timestamp time.Time
}

// jsonEvent é a representação JSON de um evento de telemetria para persistência em JSONL.
type jsonEvent struct {
	Name       string `json:"name"`
	DurationMs int64  `json:"duration_ms"`
	Success    bool   `json:"success"`
	Error      string `json:"error,omitempty"`
	Timestamp  string `json:"timestamp"`
}

var (
	events []Event
	mu     sync.Mutex
)

// Record logs the completion of an event/operation
func Record(name string, duration time.Duration, err error) {
	mu.Lock()
	defer mu.Unlock()

	events = append(events, Event{
		Name:      name,
		Duration:  duration,
		Success:   err == nil,
		Error:     err,
		Timestamp: time.Now(),
	})
}

// Track execution easily: defer telemetry.Track("my_operation", time.Now(), &err)
func Track(name string, start time.Time, err *error) {
	var finalErr error
	if err != nil {
		finalErr = *err
	}
	Record(name, time.Since(start), finalErr)
}

// Flush outputs the tracked telemetry to debug logs
func Flush() {
	mu.Lock()
	defer mu.Unlock()

	if len(events) == 0 {
		return
	}

	slog.Debug("=============================")
	slog.Debug("🚀 CLI Telemetry Summary")
	slog.Debug("=============================")
	for _, e := range events {
		status := "✅ SUCCESS"
		errStr := ""
		if !e.Success {
			status = "❌ FAILED"
			if e.Error != nil {
				errStr = e.Error.Error()
			}
		}

		slog.Debug("Metric",
			"operation", e.Name,
			"status", status,
			"duration_ms", e.Duration.Milliseconds(),
			"error", errStr,
		)
	}
	slog.Debug("=============================")
}

// FlushToFile persiste os eventos de telemetria em arquivo JSONL (~/.yby/telemetry.jsonl).
// Se enabled for false, não faz nada. Não limpa os eventos (Flush faz slog separadamente).
func FlushToFile(enabled bool) error {
	if !enabled {
		return nil
	}

	mu.Lock()
	snapshot := make([]Event, len(events))
	copy(snapshot, events)
	mu.Unlock()

	if len(snapshot) == 0 {
		return nil
	}

	path, err := telemetryFilePath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	if err := rotateIfNeeded(path, maxTelemetryFileSize); err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, e := range snapshot {
		je := toJSONEvent(e)
		data, err := json.Marshal(je)
		if err != nil {
			continue
		}
		data = append(data, '\n')
		if _, err := f.Write(data); err != nil {
			return err
		}
	}

	return nil
}

// FlushToFilePath persiste os eventos em um caminho específico.
// Usado internamente para testes e cenários com path customizado.
func FlushToFilePath(enabled bool, path string) error {
	if !enabled {
		return nil
	}

	mu.Lock()
	snapshot := make([]Event, len(events))
	copy(snapshot, events)
	mu.Unlock()

	if len(snapshot) == 0 {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	if err := rotateIfNeeded(path, maxTelemetryFileSize); err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, e := range snapshot {
		je := toJSONEvent(e)
		data, err := json.Marshal(je)
		if err != nil {
			continue
		}
		data = append(data, '\n')
		if _, err := f.Write(data); err != nil {
			return err
		}
	}

	return nil
}

// rotateIfNeeded renomeia o arquivo para path.1 se exceder maxBytes.
func rotateIfNeeded(path string, maxBytes int64) error {
	info, err := os.Stat(path)
	if err != nil {
		// Arquivo não existe — nada a rotacionar
		return nil
	}

	if info.Size() < maxBytes {
		return nil
	}

	return os.Rename(path, path+".1")
}

// ExportEvents lê e retorna o conteúdo bruto do arquivo de telemetria.
// Retorna nil, nil se o arquivo não existir.
func ExportEvents(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return data, nil
}

// TelemetryFilePath retorna o caminho padrão do arquivo de telemetria.
func TelemetryFilePath() (string, error) {
	return telemetryFilePath()
}

func telemetryFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".yby", "telemetry.jsonl"), nil
}

func toJSONEvent(e Event) jsonEvent {
	je := jsonEvent{
		Name:       e.Name,
		DurationMs: e.Duration.Milliseconds(),
		Success:    e.Success,
		Timestamp:  e.Timestamp.Format(time.RFC3339),
	}
	if e.Error != nil {
		je.Error = e.Error.Error()
	}
	return je
}
