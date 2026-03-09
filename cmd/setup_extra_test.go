package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ========================================================
// attemptInstall — cenários adicionais (cobertura 36.4%)
// ========================================================

func TestAttemptInstall_ComBrew(t *testing.T) {
	originalLookPath := lookPath
	originalExecCommand := execCommand
	defer func() {
		lookPath = originalLookPath
		execCommand = originalExecCommand
	}()

	// Simula brew disponível como gerenciador de pacotes
	lookPath = func(file string) (string, error) {
		if file == "brew" {
			return "/usr/local/bin/brew", nil
		}
		return "", fmt.Errorf("not found: %s", file)
	}

	// Registra quais comandos foram chamados
	comandosChamados := []string{}
	execCommand = func(name string, arg ...string) *exec.Cmd {
		comandosChamados = append(comandosChamados, name)
		// Retorna um comando que simplesmente imprime sucesso
		cs := []string{"-test.run=TestHelperProcess", "--", "echo", "ok"}
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
		return cmd
	}

	capturarStdout(t, func() {
		attemptInstall([]string{"kubectl", "helm"})
	})

	// Deve ter chamado execCommand pelo menos 2 vezes (uma por ferramenta)
	assert.GreaterOrEqual(t, len(comandosChamados), 2,
		"deveria chamar execCommand para cada ferramenta")
}

func TestAttemptInstall_ComApt(t *testing.T) {
	originalLookPath := lookPath
	originalExecCommand := execCommand
	defer func() {
		lookPath = originalLookPath
		execCommand = originalExecCommand
	}()

	// Simula apt disponível (brew não disponível)
	lookPath = func(file string) (string, error) {
		if file == "apt-get" {
			return "/usr/bin/apt-get", nil
		}
		return "", fmt.Errorf("not found: %s", file)
	}

	comandosChamados := []string{}
	execCommand = func(name string, arg ...string) *exec.Cmd {
		comandosChamados = append(comandosChamados, name)
		cs := []string{"-test.run=TestHelperProcess", "--", "echo", "ok"}
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
		return cmd
	}

	capturarStdout(t, func() {
		attemptInstall([]string{"kubectl"})
	})

	assert.GreaterOrEqual(t, len(comandosChamados), 1,
		"deveria chamar execCommand para a ferramenta via apt")
}

func TestAttemptInstall_ComSnap(t *testing.T) {
	originalLookPath := lookPath
	originalExecCommand := execCommand
	defer func() {
		lookPath = originalLookPath
		execCommand = originalExecCommand
	}()

	// Simula snap disponível (brew e apt não disponíveis)
	lookPath = func(file string) (string, error) {
		if file == "snap" {
			return "/usr/bin/snap", nil
		}
		return "", fmt.Errorf("not found: %s", file)
	}

	comandosChamados := []string{}
	execCommand = func(name string, arg ...string) *exec.Cmd {
		comandosChamados = append(comandosChamados, name)
		cs := []string{"-test.run=TestHelperProcess", "--", "echo", "ok"}
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
		return cmd
	}

	capturarStdout(t, func() {
		attemptInstall([]string{"helm"})
	})

	assert.GreaterOrEqual(t, len(comandosChamados), 1,
		"deveria chamar execCommand para a ferramenta via snap")
}

func TestAttemptInstall_FalhaInstalacao(t *testing.T) {
	originalLookPath := lookPath
	originalExecCommand := execCommand
	defer func() {
		lookPath = originalLookPath
		execCommand = originalExecCommand
	}()

	lookPath = func(file string) (string, error) {
		if file == "brew" {
			return "/usr/local/bin/brew", nil
		}
		return "", fmt.Errorf("not found: %s", file)
	}

	// Simula falha na instalação
	execCommand = func(name string, arg ...string) *exec.Cmd {
		// Retorna um comando que falha
		cmd := exec.Command("false")
		return cmd
	}

	// Não deve entrar em pânico mesmo com falha
	assert.NotPanics(t, func() {
		capturarStdout(t, func() {
			attemptInstall([]string{"ferramenta-inexistente"})
		})
	})
}

// TestSetupCmd_DevProfile_FerramentasFaltando testa o caminho com ferramentas ausentes (sem prompt interativo)
func TestSetupCmd_DevProfile_FerramentasFaltando(t *testing.T) {
	originalLookPath := lookPath
	originalExecCommand := execCommand
	defer func() {
		lookPath = originalLookPath
		execCommand = originalExecCommand
	}()

	// Simula k3d e direnv ausentes
	lookPath = func(file string) (string, error) {
		if file == "k3d" || file == "direnv" {
			return "", fmt.Errorf("not found: %s", file)
		}
		return "/usr/bin/" + file, nil
	}

	execCommand = func(name string, arg ...string) *exec.Cmd {
		cs := []string{"-test.run=TestHelperProcess", "--", "echo", "ok"}
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
		return cmd
	}

	setupCmd.Flags().Set("profile", "dev")

	// O comando não entra em pânico; survey.AskOne falha silenciosamente sem stdin
	capturarStdout(t, func() {
		err := setupCmd.RunE(setupCmd, []string{})
		assert.NoError(t, err)
	})
}

// TestSetupCmd_ServerProfile_FerramentasFaltando testa o perfil server com ferramentas ausentes
func TestSetupCmd_ServerProfile_FerramentasFaltando(t *testing.T) {
	originalLookPath := lookPath
	originalExecCommand := execCommand
	defer func() {
		lookPath = originalLookPath
		execCommand = originalExecCommand
	}()

	lookPath = func(file string) (string, error) {
		if file == "helm" {
			return "", fmt.Errorf("not found: %s", file)
		}
		return "/usr/bin/" + file, nil
	}

	execCommand = func(name string, arg ...string) *exec.Cmd {
		cs := []string{"-test.run=TestHelperProcess", "--", "echo", "ok"}
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
		return cmd
	}

	setupCmd.Flags().Set("profile", "server")
	defer setupCmd.Flags().Set("profile", "dev")

	capturarStdout(t, func() {
		err := setupCmd.RunE(setupCmd, []string{})
		assert.NoError(t, err)
	})
}

// TestConfigureDirenv_CriaEnvrc testa a criação do .envrc
func TestConfigureDirenv_CriaEnvrc(t *testing.T) {
	originalExecCommand := execCommand
	defer func() { execCommand = originalExecCommand }()

	execCommand = func(name string, arg ...string) *exec.Cmd {
		cs := []string{"-test.run=TestHelperProcess", "--", "echo", "ok"}
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
		return cmd
	}

	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	capturarStdout(t, func() {
		configureDirenv()
	})

	// Verifica que o .envrc foi criado
	assert.FileExists(t, dir+"/.envrc")
}

// TestConfigureDirenv_EnvrcJaExiste testa quando .envrc já existe
func TestConfigureDirenv_EnvrcJaExiste(t *testing.T) {
	originalExecCommand := execCommand
	defer func() { execCommand = originalExecCommand }()

	execCommand = func(name string, arg ...string) *exec.Cmd {
		cs := []string{"-test.run=TestHelperProcess", "--", "echo", "ok"}
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
		return cmd
	}

	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	// Cria .envrc antes
	os.WriteFile(dir+"/.envrc", []byte("existente"), 0644)

	capturarStdout(t, func() {
		configureDirenv()
	})

	// Verifica que o conteúdo original foi preservado
	data, _ := os.ReadFile(dir + "/.envrc")
	assert.Equal(t, "existente", string(data))
}

func TestAttemptInstall_ListaVazia(t *testing.T) {
	originalLookPath := lookPath
	defer func() { lookPath = originalLookPath }()

	lookPath = func(file string) (string, error) {
		if file == "brew" {
			return "/usr/local/bin/brew", nil
		}
		return "", fmt.Errorf("not found: %s", file)
	}

	// Não deve entrar em pânico com lista vazia
	assert.NotPanics(t, func() {
		capturarStdout(t, func() {
			attemptInstall([]string{})
		})
	})
}
