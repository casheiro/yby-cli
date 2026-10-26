package logger

import (
	"bytes"
	"log/slog"
	"os"
	"strings"
	"testing"
)

func TestDesviar(t *testing.T) {
	anterior := slog.Default()
	t.Cleanup(func() { slog.SetDefault(anterior) })
	InitGlobal(Config{Level: "info", Format: "text"})

	var b bytes.Buffer
	restaurar := Desviar(&b)
	slog.Info("durante as etapas", "fase", "argo cd")
	slog.Debug("abaixo do nível")
	restaurar()
	restaurar() // restaurar duas vezes é seguro
	slog.Info("depois de restaurar")

	if !strings.Contains(b.String(), `msg="durante as etapas" fase="argo cd"`) {
		t.Errorf("o log desviado deveria manter o formato: %q", b.String())
	}
	if strings.Contains(b.String(), "abaixo do nível") || strings.Contains(b.String(), "depois de restaurar") {
		t.Errorf("o desvio mantém o nível e acaba ao restaurar: %q", b.String())
	}
	if saida.w != os.Stderr {
		t.Error("restaurar deveria voltar o log para o os.Stderr")
	}
}

func TestParseLevel_Debug(t *testing.T) {
	if ParseLevel("debug") != slog.LevelDebug {
		t.Error("expected LevelDebug for 'debug'")
	}
}

func TestParseLevel_Info(t *testing.T) {
	if ParseLevel("info") != slog.LevelInfo {
		t.Error("expected LevelInfo for 'info'")
	}
}

func TestParseLevel_Warn(t *testing.T) {
	if ParseLevel("warn") != slog.LevelWarn {
		t.Error("expected LevelWarn for 'warn'")
	}
}

func TestParseLevel_Error(t *testing.T) {
	if ParseLevel("error") != slog.LevelError {
		t.Error("expected LevelError for 'error'")
	}
}

func TestParseLevel_Uppercase(t *testing.T) {
	// Case insensitive
	if ParseLevel("DEBUG") != slog.LevelDebug {
		t.Error("expected LevelDebug for 'DEBUG'")
	}
	if ParseLevel("INFO") != slog.LevelInfo {
		t.Error("expected LevelInfo for 'INFO'")
	}
}

func TestParseLevel_Default(t *testing.T) {
	// Unknown level should default to Info
	if ParseLevel("unknown") != slog.LevelInfo {
		t.Error("expected LevelInfo as default for unknown level")
	}
	if ParseLevel("") != slog.LevelInfo {
		t.Error("expected LevelInfo as default for empty string")
	}
	if ParseLevel("trace") != slog.LevelInfo {
		t.Error("expected LevelInfo as default for 'trace'")
	}
}

func TestInitGlobal_TextFormat(t *testing.T) {
	// Should not panic
	InitGlobal(Config{Level: "debug", Format: "text"})
}

func TestInitGlobal_JSONFormat(t *testing.T) {
	// Should not panic
	InitGlobal(Config{Level: "info", Format: "json"})
}

func TestInitGlobal_DefaultFormat(t *testing.T) {
	// Empty format should fallback to text
	InitGlobal(Config{Level: "warn", Format: ""})
}

func TestInitGlobal_AllLevels(t *testing.T) {
	levels := []string{"debug", "info", "warn", "error"}
	for _, lvl := range levels {
		// Should not panic for any level
		InitGlobal(Config{Level: lvl, Format: "text"})
	}
}
