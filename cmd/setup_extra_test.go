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
