package cmd

import (
	"encoding/json"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// saidaDoVersion roda o version e devolve o que ele escreveu. A saída vai para
// cmd.OutOrStdout(), que outros testes podem ter desviado no rootCmd; capturarSaidaPadrao
// liga a saída do próprio versionCmd ao pipe.
func saidaDoVersion(t *testing.T) string {
	t.Helper()
	var err error
	saida := capturarSaidaPadrao(t, versionCmd, func() { err = versionCmd.RunE(versionCmd, []string{}) })
	require.NoError(t, err)
	return saida
}

func TestVersionCmd_Estrutura(t *testing.T) {
	assert.Equal(t, "version", versionCmd.Use, "Use deveria ser 'version'")
	assert.NotEmpty(t, versionCmd.Short, "Short não deveria ser vazio")
	assert.NotEmpty(t, versionCmd.Long, "Long não deveria ser vazio")
	assert.NotNil(t, versionCmd.RunE, "RunE não deveria ser nil")
}

func TestVersionCmd_SaidaContemVersao(t *testing.T) {
	output := saidaDoVersion(t)
	assert.Contains(t, output, Version, "Saída deveria conter a versão")
}

func TestVersionCmd_SaidaContemOSArch(t *testing.T) {
	// Garante que log-format seja "text" para evitar saída JSON
	_ = rootCmd.PersistentFlags().Set("log-format", "text")

	output := saidaDoVersion(t)

	expectedOSArch := runtime.GOOS + "/" + runtime.GOARCH
	assert.Contains(t, output, expectedOSArch, "Saída deveria conter OS/ARCH")
}

func TestVersionCmd_SaidaComCommit(t *testing.T) {
	originalCommit := commit
	originalDate := date
	defer func() {
		commit = originalCommit
		date = originalDate
	}()

	commit = "abc1234"
	date = "2025-01-01"

	output := saidaDoVersion(t)

	assert.Contains(t, output, "abc1234", "Saída deveria conter o hash do commit")
	assert.Contains(t, output, "2025-01-01", "Saída deveria conter a data de build")
}

func TestVersionCmd_SaidaSemCommitQuandoNone(t *testing.T) {
	originalCommit := commit
	originalDate := date
	defer func() {
		commit = originalCommit
		date = originalDate
	}()

	commit = "none"
	date = "unknown"

	output := saidaDoVersion(t)

	assert.NotContains(t, output, "(none)", "Saída não deveria conter '(none)' quando commit é 'none'")
	assert.NotContains(t, output, "compilado em unknown", "Saída não deveria conter data quando é 'unknown'")
}

func TestVersionCmd_NaoPanica(t *testing.T) {
	assert.NotPanics(t, func() {
		saidaDoVersion(t)
	}, "versionCmd.RunE não deveria causar panic")
}

func TestVersionCmd_EhSubcomandoDeRoot(t *testing.T) {
	found := false
	for _, sub := range rootCmd.Commands() {
		if sub.Name() == "version" {
			found = true
			break
		}
	}
	assert.True(t, found, "version deveria ser subcomando de rootCmd")
}

func TestVersionCmd_JSONOutput(t *testing.T) {
	// Simular flag --log-format json no root
	err := rootCmd.PersistentFlags().Set("log-format", "json")
	require.NoError(t, err)
	defer rootCmd.PersistentFlags().Set("log-format", "text")

	output := saidaDoVersion(t)

	var info map[string]string
	err = json.Unmarshal([]byte(strings.TrimSpace(output)), &info)
	require.NoError(t, err, "output deve ser JSON válido com --log-format json")

	assert.Equal(t, Version, info["version"])
	assert.Equal(t, runtime.GOOS, info["os"])
	assert.Equal(t, runtime.GOARCH, info["arch"])
}
