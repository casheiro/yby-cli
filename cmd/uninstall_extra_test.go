package cmd

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/casheiro/yby-cli/pkg/errors"
)

// saveUninstallMocks salva e restaura as variáveis mockáveis do uninstall (incluindo o
// prompter e a flag --yes).
func saveUninstallMocks(t *testing.T) {
	t.Helper()
	origExec := osExecutable
	origRemove := osRemove
	origPrompter := prompter
	t.Cleanup(func() {
		osExecutable = origExec
		osRemove = origRemove
		prompter = origPrompter
		_ = uninstallCmd.Flags().Set("yes", "false")
	})
}

// responderUninstall faz o Confirm do uninstall devolver resposta e err.
func responderUninstall(resposta bool, err error) {
	prompter = &mockPrompter{confirmFunc: func(string, bool) (bool, error) { return resposta, err }}
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
	responderUninstall(true, nil)

	assert.Error(t, uninstallCmd.RunE(uninstallCmd, []string{}))
}

func TestUninstallCmd_UserCancels(t *testing.T) {
	saveUninstallMocks(t)

	osExecutable = func() (string, error) { return "/usr/local/bin/yby", nil }
	osRemove = func(name string) error {
		t.Fatal("não deveria chamar Remove quando o usuário cancela")
		return nil
	}
	responderUninstall(false, nil)

	assert.NoError(t, uninstallCmd.RunE(uninstallCmd, []string{}))
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
	var perguntaPadrao *bool
	prompter = &mockPrompter{confirmFunc: func(_ string, padrao bool) (bool, error) {
		perguntaPadrao = &padrao
		return true, nil
	}}

	assert.NoError(t, uninstallCmd.RunE(uninstallCmd, []string{}))

	assert.True(t, removeCalled, "osRemove deveria ter sido chamado")
	require.NotNil(t, perguntaPadrao)
	assert.False(t, *perguntaPadrao, "a resposta padrão da pergunta é não")
}

func TestUninstallCmd_SemTerminal(t *testing.T) {
	saveUninstallMocks(t)

	osExecutable = func() (string, error) { return "/usr/local/bin/yby", nil }
	osRemove = func(name string) error {
		t.Fatal("sem terminal e sem --yes, nada deveria ser removido")
		return nil
	}
	responderUninstall(false, erroSemTerminal())

	err := uninstallCmd.RunE(uninstallCmd, []string{})
	var yerr *errors.YbyError
	require.ErrorAs(t, err, &yerr)
	assert.Contains(t, yerr.Hint, "--yes")
}

func TestUninstallCmd_FlagYes_NaoPergunta(t *testing.T) {
	saveUninstallMocks(t)

	removeCalled := false
	osExecutable = func() (string, error) { return "/usr/local/bin/yby", nil }
	osRemove = func(name string) error {
		removeCalled = true
		return nil
	}
	prompter = &mockPrompter{confirmFunc: func(string, bool) (bool, error) {
		t.Fatal("com --yes não deveria perguntar")
		return false, nil
	}}
	require.NoError(t, uninstallCmd.Flags().Set("yes", "true"))

	assert.NoError(t, uninstallCmd.RunE(uninstallCmd, []string{}))
	assert.True(t, removeCalled)
}

func TestUninstallCmd_RemovePermissionDenied(t *testing.T) {
	saveUninstallMocks(t)

	osExecutable = func() (string, error) { return "/usr/local/bin/yby", nil }
	osRemove = func(name string) error {
		return fmt.Errorf("remove %s: permission denied", name)
	}
	responderUninstall(true, nil)

	// Deve retornar erro (e imprimir a dica de sudo)
	assert.Error(t, uninstallCmd.RunE(uninstallCmd, []string{}))
}

func TestUninstallCmd_RemoveOtherError(t *testing.T) {
	saveUninstallMocks(t)

	osExecutable = func() (string, error) { return "/usr/local/bin/yby", nil }
	osRemove = func(name string) error {
		return fmt.Errorf("erro desconhecido ao remover arquivo")
	}
	responderUninstall(true, nil)

	// imprime o erro sem dica de sudo
	assert.Error(t, uninstallCmd.RunE(uninstallCmd, []string{}))
}
