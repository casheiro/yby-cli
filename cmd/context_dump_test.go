package cmd

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContextDumpCmd_ComVariaveis(t *testing.T) {
	t.Setenv("YBY_ENV", "local")
	t.Setenv("YBY_GIT_REPOURL", "https://github.com/test/repo")

	// contextDumpCmd usa Run (não RunE), então verificamos que não entra em pânico
	assert.NotPanics(t, func() {
		contextDumpCmd.Run(contextDumpCmd, []string{})
	})
}

func TestContextDumpCmd_SemVariaveis(t *testing.T) {
	// Limpa todas as variáveis YBY_* existentes
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "YBY_") {
			parts := strings.SplitN(e, "=", 2)
			t.Setenv(parts[0], "")
		}
	}
	// Garante que YBY_ENV está vazio
	t.Setenv("YBY_ENV", "")

	assert.NotPanics(t, func() {
		contextDumpCmd.Run(contextDumpCmd, []string{})
	})
}
