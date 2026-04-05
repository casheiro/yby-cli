package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_DefaultValues(t *testing.T) {
	// Sem arquivo e sem env vars — deve retornar defaults
	ResetGlobal()
	// Limpa env vars que podem interferir
	for _, key := range []string{"YBY_AI_PROVIDER", "YBY_AI_MODEL", "YBY_AI_LANGUAGE", "YBY_LOG_LEVEL", "YBY_LOG_FORMAT", "YBY_TELEMETRY_ENABLED"} {
		t.Setenv(key, "")
		os.Unsetenv(key)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() retornou erro inesperado: %v", err)
	}

	if cfg.AI.Language != "pt-BR" {
		t.Errorf("ai.language esperado 'pt-BR', obteve '%s'", cfg.AI.Language)
	}
	if cfg.AI.Provider != "" {
		t.Errorf("ai.provider esperado vazio, obteve '%s'", cfg.AI.Provider)
	}
	if cfg.AI.Model != "" {
		t.Errorf("ai.model esperado vazio, obteve '%s'", cfg.AI.Model)
	}
	if cfg.Log.Level != "info" {
		t.Errorf("log.level esperado 'info', obteve '%s'", cfg.Log.Level)
	}
	if cfg.Log.Format != "text" {
		t.Errorf("log.format esperado 'text', obteve '%s'", cfg.Log.Format)
	}
	if !cfg.Telemetry.Enabled {
		t.Error("telemetry.enabled esperado true, obteve false")
	}
}

func TestLoad_EnvVarsOverrideDefaults(t *testing.T) {
	ResetGlobal()

	t.Setenv("YBY_AI_PROVIDER", "gemini")
	t.Setenv("YBY_AI_MODEL", "gemini-pro")
	t.Setenv("YBY_AI_LANGUAGE", "en-US")
	t.Setenv("YBY_LOG_LEVEL", "debug")
	t.Setenv("YBY_LOG_FORMAT", "json")
	t.Setenv("YBY_TELEMETRY_ENABLED", "false")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() retornou erro inesperado: %v", err)
	}

	if cfg.AI.Provider != "gemini" {
		t.Errorf("ai.provider esperado 'gemini', obteve '%s'", cfg.AI.Provider)
	}
	if cfg.AI.Model != "gemini-pro" {
		t.Errorf("ai.model esperado 'gemini-pro', obteve '%s'", cfg.AI.Model)
	}
	if cfg.AI.Language != "en-US" {
		t.Errorf("ai.language esperado 'en-US', obteve '%s'", cfg.AI.Language)
	}
	if cfg.Log.Level != "debug" {
		t.Errorf("log.level esperado 'debug', obteve '%s'", cfg.Log.Level)
	}
	if cfg.Log.Format != "json" {
		t.Errorf("log.format esperado 'json', obteve '%s'", cfg.Log.Format)
	}
	if cfg.Telemetry.Enabled {
		t.Error("telemetry.enabled esperado false, obteve true")
	}
}

func TestLoad_ConfigFileValues(t *testing.T) {
	ResetGlobal()

	// Limpa env vars
	for _, key := range []string{"YBY_AI_PROVIDER", "YBY_AI_MODEL", "YBY_AI_LANGUAGE", "YBY_LOG_LEVEL", "YBY_LOG_FORMAT", "YBY_TELEMETRY_ENABLED"} {
		os.Unsetenv(key)
	}

	// Cria diretório temporário como HOME
	tmpHome := t.TempDir()
	configDir := filepath.Join(tmpHome, ".yby")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}

	configContent := `
ai:
  provider: ollama
  model: llama3
  language: es-ES
log:
  level: warn
  format: json
telemetry:
  enabled: false
`
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(configContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Override HOME para o viper encontrar o arquivo
	t.Setenv("HOME", tmpHome)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() retornou erro inesperado: %v", err)
	}

	if cfg.AI.Provider != "ollama" {
		t.Errorf("ai.provider esperado 'ollama', obteve '%s'", cfg.AI.Provider)
	}
	if cfg.AI.Model != "llama3" {
		t.Errorf("ai.model esperado 'llama3', obteve '%s'", cfg.AI.Model)
	}
	if cfg.AI.Language != "es-ES" {
		t.Errorf("ai.language esperado 'es-ES', obteve '%s'", cfg.AI.Language)
	}
	if cfg.Log.Level != "warn" {
		t.Errorf("log.level esperado 'warn', obteve '%s'", cfg.Log.Level)
	}
	if cfg.Log.Format != "json" {
		t.Errorf("log.format esperado 'json', obteve '%s'", cfg.Log.Format)
	}
	if cfg.Telemetry.Enabled {
		t.Error("telemetry.enabled esperado false, obteve true")
	}
}

func TestLoad_EnvOverridesFile(t *testing.T) {
	ResetGlobal()

	// Cria config file com valores
	tmpHome := t.TempDir()
	configDir := filepath.Join(tmpHome, ".yby")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}

	configContent := `
ai:
  provider: ollama
  language: es-ES
log:
  level: warn
`
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(configContent), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", tmpHome)
	// Env var deve sobrescrever o arquivo
	t.Setenv("YBY_AI_PROVIDER", "openai")
	t.Setenv("YBY_AI_LANGUAGE", "fr-FR")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() retornou erro inesperado: %v", err)
	}

	// Env sobrescreve arquivo
	if cfg.AI.Provider != "openai" {
		t.Errorf("ai.provider esperado 'openai' (env), obteve '%s'", cfg.AI.Provider)
	}
	if cfg.AI.Language != "fr-FR" {
		t.Errorf("ai.language esperado 'fr-FR' (env), obteve '%s'", cfg.AI.Language)
	}
	// Arquivo permanece para campos sem env override
	if cfg.Log.Level != "warn" {
		t.Errorf("log.level esperado 'warn' (file), obteve '%s'", cfg.Log.Level)
	}
}

func TestLoad_MissingFileUsesDefaults(t *testing.T) {
	ResetGlobal()

	// Aponta HOME para diretório vazio (sem .yby/config.yaml)
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Limpa env vars
	for _, key := range []string{"YBY_AI_PROVIDER", "YBY_AI_MODEL", "YBY_AI_LANGUAGE", "YBY_LOG_LEVEL", "YBY_LOG_FORMAT", "YBY_TELEMETRY_ENABLED"} {
		os.Unsetenv(key)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() retornou erro inesperado: %v", err)
	}

	if cfg.AI.Language != "pt-BR" {
		t.Errorf("ai.language esperado 'pt-BR', obteve '%s'", cfg.AI.Language)
	}
	if cfg.Log.Level != "info" {
		t.Errorf("log.level esperado 'info', obteve '%s'", cfg.Log.Level)
	}
	if !cfg.Telemetry.Enabled {
		t.Error("telemetry.enabled esperado true")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.AI.Language != "pt-BR" {
		t.Errorf("DefaultConfig ai.language esperado 'pt-BR', obteve '%s'", cfg.AI.Language)
	}
	if cfg.Log.Level != "info" {
		t.Errorf("DefaultConfig log.level esperado 'info', obteve '%s'", cfg.Log.Level)
	}
	if cfg.Log.Format != "text" {
		t.Errorf("DefaultConfig log.format esperado 'text', obteve '%s'", cfg.Log.Format)
	}
	if !cfg.Telemetry.Enabled {
		t.Error("DefaultConfig telemetry.enabled esperado true")
	}
}
