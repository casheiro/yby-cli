package cmd

import (
	"bytes"
	"io"
	"os"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVersionCmd_Estrutura(t *testing.T) {
	assert.Equal(t, "version", versionCmd.Use, "Use deveria ser 'version'")
	assert.NotEmpty(t, versionCmd.Short, "Short não deveria ser vazio")
	assert.NotEmpty(t, versionCmd.Long, "Long não deveria ser vazio")
	assert.NotNil(t, versionCmd.Run, "Run não deveria ser nil")
}

func TestVersionCmd_SaidaContemVersao(t *testing.T) {
	// Capturar stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	versionCmd.Run(versionCmd, []string{})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Deve conter a versão (variável Version, default "dev")
	assert.Contains(t, output, Version, "Saída deveria conter a versão")
}

func TestVersionCmd_SaidaContemOSArch(t *testing.T) {
	// Capturar stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	versionCmd.Run(versionCmd, []string{})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Deve conter OS e arch
	expectedOSArch := runtime.GOOS + "/" + runtime.GOARCH
	assert.Contains(t, output, expectedOSArch, "Saída deveria conter OS/ARCH")
}

func TestVersionCmd_SaidaComCommit(t *testing.T) {
	// Salvar valores originais
	originalCommit := commit
	originalDate := date
	defer func() {
		commit = originalCommit
		date = originalDate
	}()

	// Definir commit e data para teste
	commit = "abc1234"
	date = "2025-01-01"

	// Capturar stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	versionCmd.Run(versionCmd, []string{})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

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

	// Capturar stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	versionCmd.Run(versionCmd, []string{})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Quando commit é "none", não deve incluir o hash
	assert.NotContains(t, output, "(none)", "Saída não deveria conter '(none)' quando commit é 'none'")
	// Quando date é "unknown", não deve incluir data
	assert.NotContains(t, output, "compilado em unknown", "Saída não deveria conter data quando é 'unknown'")
}

func TestVersionCmd_NaoPanica(t *testing.T) {
	assert.NotPanics(t, func() {
		versionCmd.Run(versionCmd, []string{})
	}, "versionCmd.Run não deveria causar panic")
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
