package cmd

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// saveUninstallMocks salva e restaura as variáveis mockáveis do uninstall.
func saveUninstallMocks(t *testing.T) {
	t.Helper()
	origExec := osExecutable
	origRemove := osRemove
	origStdin := stdinReader
	t.Cleanup(func() {
		osExecutable = origExec
		osRemove = origRemove
		stdinReader = origStdin
	})
}

func TestUninstallCmd_ExecutableError(t *testing.T) {
	saveUninstallMocks(t)

	osExecutable = func() (string, error) {
		return "", fmt.Errorf("falha ao localizar binário")
	}
	osRemove = func(name string) error {
		t.Fatal("não deveria chamar Remove quando Executable falha")
		return nil
	}
	stdinReader = strings.NewReader("")

	// Não deve entrar em pânico; apenas imprime o erro e retorna
	assert.NotPanics(t, func() {
		uninstallCmd.Run(uninstallCmd, []string{})
	})
}

func TestUninstallCmd_UserCancels(t *testing.T) {
	saveUninstallMocks(t)

	osExecutable = func() (string, error) { return "/usr/local/bin/yby", nil }
	osRemove = func(name string) error {
		t.Fatal("não deveria chamar Remove quando o usuário cancela")
		return nil
	}
	stdinReader = strings.NewReader("n\n")

	assert.NotPanics(t, func() {
		uninstallCmd.Run(uninstallCmd, []string{})
	})
}

func TestUninstallCmd_UserConfirmsYes(t *testing.T) {
	saveUninstallMocks(t)

	// Cria um arquivo temporário para simular o binário
	tmpFile, err := os.CreateTemp("", "yby-test-*")
	assert.NoError(t, err)
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath) // limpeza caso osRemove falhe

	removeCalled := false
	osExecutable = func() (string, error) { return tmpPath, nil }
	osRemove = func(name string) error {
		assert.Equal(t, tmpPath, name, "deveria remover o caminho correto")
		removeCalled = true
		return nil
	}
	stdinReader = strings.NewReader("y\n")

	uninstallCmd.Run(uninstallCmd, []string{})

	assert.True(t, removeCalled, "osRemove deveria ter sido chamado")
}

func TestUninstallCmd_UserConfirmsSim(t *testing.T) {
	saveUninstallMocks(t)

	removeCalled := false
	osExecutable = func() (string, error) { return "/usr/local/bin/yby", nil }
	osRemove = func(name string) error {
		removeCalled = true
		return nil
	}
	stdinReader = strings.NewReader("sim\n")

	uninstallCmd.Run(uninstallCmd, []string{})

	assert.True(t, removeCalled, "osRemove deveria aceitar 'sim' como confirmação PT-BR")
}

func TestUninstallCmd_RemovePermissionDenied(t *testing.T) {
	saveUninstallMocks(t)

	osExecutable = func() (string, error) { return "/usr/local/bin/yby", nil }
	osRemove = func(name string) error {
		return fmt.Errorf("remove %s: permission denied", name)
	}
	stdinReader = strings.NewReader("y\n")

	// Não deve entrar em pânico; imprime dica sobre sudo
	assert.NotPanics(t, func() {
		uninstallCmd.Run(uninstallCmd, []string{})
	})
}

func TestUninstallCmd_RemoveOtherError(t *testing.T) {
	saveUninstallMocks(t)

	osExecutable = func() (string, error) { return "/usr/local/bin/yby", nil }
	osRemove = func(name string) error {
		return fmt.Errorf("erro desconhecido ao remover arquivo")
	}
	stdinReader = strings.NewReader("y\n")

	// Não deve entrar em pânico; imprime o erro sem dica de sudo
	assert.NotPanics(t, func() {
		uninstallCmd.Run(uninstallCmd, []string{})
	})
}

func TestUninstallCmd_UserConfirmsS(t *testing.T) {
	saveUninstallMocks(t)

	removeCalled := false
	osExecutable = func() (string, error) { return "/usr/local/bin/yby", nil }
	osRemove = func(name string) error {
		removeCalled = true
		return nil
	}
	// Testa a resposta "s" (atalho PT-BR)
	stdinReader = strings.NewReader("s\n")

	uninstallCmd.Run(uninstallCmd, []string{})

	assert.True(t, removeCalled, "osRemove deveria aceitar 's' como confirmação")
}

func TestUninstallCmd_UserConfirmsYES_CaseInsensitive(t *testing.T) {
	saveUninstallMocks(t)

	removeCalled := false
	osExecutable = func() (string, error) { return "/usr/local/bin/yby", nil }
	osRemove = func(name string) error {
		removeCalled = true
		return nil
	}
	// Testa maiúsculo — o código faz ToLower
	stdinReader = strings.NewReader("YES\n")

	uninstallCmd.Run(uninstallCmd, []string{})

	assert.True(t, removeCalled, "osRemove deveria aceitar 'YES' (case insensitive)")
}
