package cmd

import (
	"context"
	"fmt"
	"testing"

	"github.com/casheiro/yby-cli/pkg/services/setup"
	"github.com/casheiro/yby-cli/pkg/services/shared"
	"github.com/casheiro/yby-cli/pkg/testutil"
	"github.com/stretchr/testify/assert"
)

// ========================================================
// Testes adicionais do cmd/setup usando newSetupService
// ========================================================

// mockSetupSvc implementa setup.Service para testes do cmd
type mockSetupSvc struct {
	checkToolsResult *setup.SetupResult
	checkToolsErr    error
	installResults   []setup.InstallResult
	direnvErr        error
	direnvCalled     bool
}

func (m *mockSetupSvc) CheckTools(profile string) (*setup.SetupResult, error) {
	return m.checkToolsResult, m.checkToolsErr
}

func (m *mockSetupSvc) InstallMissing(_ context.Context, tools []string) []setup.InstallResult {
	return m.installResults
}

func (m *mockSetupSvc) ConfigureDirenv(workDir string) error {
	m.direnvCalled = true
	return m.direnvErr
}

func TestSetupCmd_DevProfile_FerramentasFaltando(t *testing.T) {
	original := newSetupService
	defer func() { newSetupService = original }()

	mockSvc := &mockSetupSvc{
		checkToolsResult: &setup.SetupResult{
			Tools: []setup.ToolStatus{
				{Name: "kubectl", Installed: true, Path: "/usr/bin/kubectl"},
				{Name: "helm", Installed: true, Path: "/usr/bin/helm"},
				{Name: "k3d", Installed: false},
				{Name: "direnv", Installed: false},
			},
			Missing: []string{"k3d", "direnv"},
		},
	}

	newSetupService = func(r shared.Runner, fs shared.Filesystem) setup.Service {
		return mockSvc
	}

	setupCmd.Flags().Set("profile", "dev")

	// O survey.AskOne vai falhar silenciosamente sem stdin interativo
	capturarStdout(t, func() {
		err := setupCmd.RunE(setupCmd, []string{})
		assert.NoError(t, err)
	})
}

func TestSetupCmd_ServerProfile_FerramentasFaltando(t *testing.T) {
	original := newSetupService
	defer func() { newSetupService = original }()

	mockSvc := &mockSetupSvc{
		checkToolsResult: &setup.SetupResult{
			Tools: []setup.ToolStatus{
				{Name: "kubectl", Installed: true, Path: "/usr/bin/kubectl"},
				{Name: "helm", Installed: false},
			},
			Missing: []string{"helm"},
		},
	}

	newSetupService = func(r shared.Runner, fs shared.Filesystem) setup.Service {
		return mockSvc
	}

	setupCmd.Flags().Set("profile", "server")
	defer setupCmd.Flags().Set("profile", "dev")

	capturarStdout(t, func() {
		err := setupCmd.RunE(setupCmd, []string{})
		assert.NoError(t, err)
	})
}

func TestSetupCmd_DevProfile_TodasInstaladas_ConfiguraDirenv(t *testing.T) {
	original := newSetupService
	defer func() { newSetupService = original }()

	mockSvc := &mockSetupSvc{
		checkToolsResult: &setup.SetupResult{
			Tools: []setup.ToolStatus{
				{Name: "kubectl", Installed: true, Path: "/usr/bin/kubectl"},
				{Name: "helm", Installed: true, Path: "/usr/bin/helm"},
				{Name: "k3d", Installed: true, Path: "/usr/bin/k3d"},
				{Name: "direnv", Installed: true, Path: "/usr/bin/direnv"},
			},
			Missing: []string{},
		},
	}

	newSetupService = func(r shared.Runner, fs shared.Filesystem) setup.Service {
		return mockSvc
	}

	setupCmd.Flags().Set("profile", "dev")

	capturarStdout(t, func() {
		err := setupCmd.RunE(setupCmd, []string{})
		assert.NoError(t, err)
	})

	assert.True(t, mockSvc.direnvCalled, "deveria chamar ConfigureDirenv no perfil dev")
}

func TestSetupCmd_ServerProfile_NaoConfiguraDirenv(t *testing.T) {
	original := newSetupService
	defer func() { newSetupService = original }()

	mockSvc := &mockSetupSvc{
		checkToolsResult: &setup.SetupResult{
			Tools: []setup.ToolStatus{
				{Name: "kubectl", Installed: true, Path: "/usr/bin/kubectl"},
				{Name: "helm", Installed: true, Path: "/usr/bin/helm"},
			},
			Missing: []string{},
		},
	}

	newSetupService = func(r shared.Runner, fs shared.Filesystem) setup.Service {
		return mockSvc
	}

	setupCmd.Flags().Set("profile", "server")
	defer setupCmd.Flags().Set("profile", "dev")

	capturarStdout(t, func() {
		err := setupCmd.RunE(setupCmd, []string{})
		assert.NoError(t, err)
	})

	assert.False(t, mockSvc.direnvCalled, "não deveria chamar ConfigureDirenv no perfil server")
}

func TestSetupCmd_CheckToolsFalha(t *testing.T) {
	original := newSetupService
	defer func() { newSetupService = original }()

	newSetupService = func(r shared.Runner, fs shared.Filesystem) setup.Service {
		return &mockSetupSvc{
			checkToolsErr: fmt.Errorf("erro inesperado"),
		}
	}

	setupCmd.Flags().Set("profile", "dev")

	capturarStdout(t, func() {
		err := setupCmd.RunE(setupCmd, []string{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "falha ao verificar ferramentas")
	})
}

func TestSetupCmd_DirenvFalhaLogWarning(t *testing.T) {
	original := newSetupService
	defer func() { newSetupService = original }()

	mockSvc := &mockSetupSvc{
		checkToolsResult: &setup.SetupResult{
			Tools: []setup.ToolStatus{
				{Name: "kubectl", Installed: true},
				{Name: "helm", Installed: true},
				{Name: "k3d", Installed: true},
				{Name: "direnv", Installed: true},
			},
			Missing: []string{},
		},
		direnvErr: fmt.Errorf("falha ao criar .envrc"),
	}

	newSetupService = func(r shared.Runner, fs shared.Filesystem) setup.Service {
		return mockSvc
	}

	setupCmd.Flags().Set("profile", "dev")

	// Não deve retornar erro — o erro de direnv é apenas warning
	capturarStdout(t, func() {
		err := setupCmd.RunE(setupCmd, []string{})
		assert.NoError(t, err)
	})
}

// --- Testes dos adapters via cmd ---

func TestSystemToolChecker_Instalado(t *testing.T) {
	runner := &testutil.MockRunner{
		LookPathFunc: func(file string) (string, error) {
			return "/usr/bin/" + file, nil
		},
	}
	checker := &setup.SystemToolChecker{Runner: runner}

	path, err := checker.IsInstalled("kubectl")
	assert.NoError(t, err)
	assert.Equal(t, "/usr/bin/kubectl", path)
}

func TestSystemToolChecker_NaoInstalado(t *testing.T) {
	runner := &testutil.MockRunner{
		LookPathFunc: func(file string) (string, error) {
			return "", fmt.Errorf("não encontrado: %s", file)
		},
	}
	checker := &setup.SystemToolChecker{Runner: runner}

	_, err := checker.IsInstalled("kubectl")
	assert.Error(t, err)
}

func TestSystemPackageManager_DetectBrew(t *testing.T) {
	runner := &testutil.MockRunner{
		LookPathFunc: func(file string) (string, error) {
			if file == "brew" {
				return "/usr/local/bin/brew", nil
			}
			return "", fmt.Errorf("não encontrado")
		},
	}
	pkg := &setup.SystemPackageManager{Runner: runner}

	assert.Equal(t, "brew", pkg.Detect())
}

func TestSystemPackageManager_DetectApt(t *testing.T) {
	runner := &testutil.MockRunner{
		LookPathFunc: func(file string) (string, error) {
			if file == "apt-get" {
				return "/usr/bin/apt-get", nil
			}
			return "", fmt.Errorf("não encontrado")
		},
	}
	pkg := &setup.SystemPackageManager{Runner: runner, GOOS: "linux"}

	assert.Equal(t, "apt", pkg.Detect())
}

func TestSystemPackageManager_DetectSnap(t *testing.T) {
	runner := &testutil.MockRunner{
		LookPathFunc: func(file string) (string, error) {
			if file == "snap" {
				return "/usr/bin/snap", nil
			}
			return "", fmt.Errorf("não encontrado")
		},
	}
	pkg := &setup.SystemPackageManager{Runner: runner, GOOS: "linux"}

	assert.Equal(t, "snap", pkg.Detect())
}

func TestSystemPackageManager_DetectNenhum(t *testing.T) {
	runner := &testutil.MockRunner{
		LookPathFunc: func(file string) (string, error) {
			return "", fmt.Errorf("não encontrado")
		},
	}
	pkg := &setup.SystemPackageManager{Runner: runner}

	assert.Equal(t, "", pkg.Detect())
}

func TestSystemPackageManager_AptNaoDetectaForaLinux(t *testing.T) {
	runner := &testutil.MockRunner{
		LookPathFunc: func(file string) (string, error) {
			if file == "apt-get" {
				return "/usr/bin/apt-get", nil
			}
			return "", fmt.Errorf("não encontrado")
		},
	}
	pkg := &setup.SystemPackageManager{Runner: runner, GOOS: "darwin"}

	assert.Equal(t, "", pkg.Detect())
}
