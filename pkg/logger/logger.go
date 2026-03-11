package logger

import (
	"log/slog"
	"os"
	"strings"
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
		handler = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		handler = slog.NewTextHandler(os.Stderr, opts)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)
}
