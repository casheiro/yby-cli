package cmd

import (
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

// ========================================================
// initConfig — testes diretos (cobertura 66.7%)
// ========================================================

func TestInitConfig_ComContextFlag(t *testing.T) {
	// Salva o valor original
	originalContextFlag := contextFlag
	defer func() {
		contextFlag = originalContextFlag
		os.Unsetenv("YBY_ENV")
	}()

	// Simula a flag --context definida
	contextFlag = "staging"

	cmd := &cobra.Command{}
	initConfig(cmd, []string{})

	// Deve ter setado a variável de ambiente
	assert.Equal(t, "staging", os.Getenv("YBY_ENV"),
		"initConfig deveria setar YBY_ENV quando contextFlag está definido")
}

func TestInitConfig_SemContextFlag(t *testing.T) {
	// Salva o valor original
	originalContextFlag := contextFlag
	originalEnv := os.Getenv("YBY_ENV")
	defer func() {
		contextFlag = originalContextFlag
		if originalEnv != "" {
			os.Setenv("YBY_ENV", originalEnv)
		} else {
			os.Unsetenv("YBY_ENV")
		}
	}()

	// Limpa a variável de ambiente e a flag
	os.Unsetenv("YBY_ENV")
	contextFlag = ""

	cmd := &cobra.Command{}
	initConfig(cmd, []string{})

	// Não deve ter setado YBY_ENV
	assert.Empty(t, os.Getenv("YBY_ENV"),
		"initConfig não deveria setar YBY_ENV quando contextFlag está vazio")
}

func TestInitConfig_NiveisDeLog(t *testing.T) {
	// Salva os valores originais
	originalLogLevel := logLevelFlag
	originalLogFormat := logFormatFlag
	originalContextFlag := contextFlag
	defer func() {
		logLevelFlag = originalLogLevel
		logFormatFlag = originalLogFormat
		contextFlag = originalContextFlag
	}()

	tests := []struct {
		nome   string
		level  string
		format string
	}{
		{"debug/json", "debug", "json"},
		{"info/text", "info", "text"},
		{"warn/json", "warn", "json"},
		{"error/text", "error", "text"},
	}

	for _, tt := range tests {
		t.Run(tt.nome, func(t *testing.T) {
			logLevelFlag = tt.level
			logFormatFlag = tt.format
			contextFlag = ""

			// Não deve entrar em pânico com diferentes configurações de log
			assert.NotPanics(t, func() {
				cmd := &cobra.Command{}
				initConfig(cmd, []string{})
			})
		})
	}
}
