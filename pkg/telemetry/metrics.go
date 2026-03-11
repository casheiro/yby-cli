package telemetry

import (
	"log/slog"
	"sync"
	"time"
)

// Event records metrics about a specific operation
type Event struct {
	Name      string
	Duration  time.Duration
	Success   bool
	Error     error
	Timestamp time.Time
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
