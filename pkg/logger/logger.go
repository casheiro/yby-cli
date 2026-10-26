package logger

import (
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
)

// Level maps string level names to slog.Level
func ParseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// Config struct holds initialization parameters
type Config struct {
	Level  string
	Format string // text or json
}

// InitGlobal configures the default slog logger globally.
// This allows simple slog.Info(), slog.Error() calls to respect the CLI format.
func InitGlobal(cfg Config) {
	opts := &slog.HandlerOptions{
		Level: ParseLevel(cfg.Level),
	}

	var handler slog.Handler

	if strings.ToLower(cfg.Format) == "json" {
		handler = slog.NewJSONHandler(saida, opts)
	} else {
		handler = slog.NewTextHandler(saida, opts)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)
}

// saida é o destino do log de InitGlobal: os.Stderr, salvo durante um Desviar.
var saida = &desvio{w: os.Stderr}

type desvio struct {
	mu sync.RWMutex
	w  io.Writer
}

func (d *desvio) Write(p []byte) (int, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.w.Write(p)
}

// Desviar manda o log de InitGlobal para w até restaurar ser chamada (ex.: enquanto as
// etapas de um comando longo são redesenhadas no terminal e cada linha de log precisa
// passar por elas). O nível e o formato continuam os mesmos.
func Desviar(w io.Writer) (restaurar func()) {
	saida.mu.Lock()
	anterior := saida.w
	saida.w = w
	saida.mu.Unlock()
	var uma sync.Once
	return func() {
		uma.Do(func() {
			saida.mu.Lock()
			saida.w = anterior
			saida.mu.Unlock()
		})
	}
}
