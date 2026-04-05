package setup

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"testing"

	"github.com/casheiro/yby-cli/pkg/testutil"
	"github.com/stretchr/testify/assert"
)

// --- Mocks internos ---

type mockToolChecker struct {
	installed map[string]string // cmd -> path
}

func (m *mockToolChecker) IsInstalled(tool string) (string, error) {
	if path, ok := m.installed[tool]; ok {
		return path, nil
	}
	return "", fmt.Errorf("não encontrado: %s", tool)
}

type mockPackageManager struct {
	detected   string
	installErr map[string]error // tool -> erro
}

func (m *mockPackageManager) Detect() string {
	return m.detected
}

func (m *mockPackageManager) Install(_ context.Context, tool, _ string) ([]byte, error) {
	if m.installErr != nil {
		if err, ok := m.installErr[tool]; ok {
			return []byte("falha na instalação"), err
		}
	}
	return []byte("instalado com sucesso"), nil
}

// --- Testes CheckTools ---

func TestCheckTools_PerfilDev_TodasInstaladas(t *testing.T) {
	checker := &mockToolChecker{
		installed: map[string]string{
			"kubectl": "/usr/bin/kubectl",
			"helm":    "/usr/bin/helm",
			"k3d":     "/usr/bin/k3d",
			"direnv":  "/usr/bin/direnv",
		},
	}
	svc := NewService(checker, &mockPackageManager{}, &testutil.MockRunner{}, &testutil.MockFilesystem{})

	result, err := svc.CheckTools("dev")
	assert.NoError(t, err)
	assert.Len(t, result.Tools, 4)
	assert.Empty(t, result.Missing)

	for _, ts := range result.Tools {
		assert.True(t, ts.Installed, "ferramenta %s deveria estar instalada", ts.Name)
	}
}

func TestCheckTools_PerfilDev_AlgumasFaltando(t *testing.T) {
	checker := &mockToolChecker{
		installed: map[string]string{
			"kubectl": "/usr/bin/kubectl",
			"helm":    "/usr/bin/helm",
		},
	}
	svc := NewService(checker, &mockPackageManager{}, &testutil.MockRunner{}, &testutil.MockFilesystem{})

	result, err := svc.CheckTools("dev")
	assert.NoError(t, err)
	assert.Len(t, result.Tools, 4)
	assert.Equal(t, []string{"k3d", "direnv"}, result.Missing)
}

func TestCheckTools_PerfilServer(t *testing.T) {
	checker := &mockToolChecker{
		installed: map[string]string{
			"kubectl": "/usr/bin/kubectl",
			"helm":    "/usr/bin/helm",
		},
	}
	svc := NewService(checker, &mockPackageManager{}, &testutil.MockRunner{}, &testutil.MockFilesystem{})

	result, err := svc.CheckTools("server")
	assert.NoError(t, err)
	assert.Len(t, result.Tools, 2)
	assert.Empty(t, result.Missing)
}

func TestCheckTools_PerfilServer_FaltandoHelm(t *testing.T) {
	checker := &mockToolChecker{
		installed: map[string]string{
			"kubectl": "/usr/bin/kubectl",
		},
	}
	svc := NewService(checker, &mockPackageManager{}, &testutil.MockRunner{}, &testutil.MockFilesystem{})

	result, err := svc.CheckTools("server")
	assert.NoError(t, err)
	assert.Len(t, result.Tools, 2)
	assert.Equal(t, []string{"helm"}, result.Missing)
}

func TestCheckTools_PerfilInvalido(t *testing.T) {
	svc := NewService(&mockToolChecker{}, &mockPackageManager{}, &testutil.MockRunner{}, &testutil.MockFilesystem{})

	result, err := svc.CheckTools("invalido")
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "perfil desconhecido")
}

// --- Testes InstallMissing ---

func TestInstallMissing_ComBrew(t *testing.T) {
	pkg := &mockPackageManager{detected: "brew"}
	svc := NewService(&mockToolChecker{}, pkg, &testutil.MockRunner{}, &testutil.MockFilesystem{})

	results := svc.InstallMissing(context.Background(), []string{"kubectl", "helm"})
	assert.Len(t, results, 2)
	for _, r := range results {
		assert.True(t, r.Success, "ferramenta %s deveria ser instalada com sucesso", r.Tool)
		assert.Contains(t, r.Output, "instalado com sucesso")
		assert.NoError(t, r.Error)
	}
}

func TestInstallMissing_ComApt(t *testing.T) {
	pkg := &mockPackageManager{detected: "apt"}
	svc := NewService(&mockToolChecker{}, pkg, &testutil.MockRunner{}, &testutil.MockFilesystem{})

	results := svc.InstallMissing(context.Background(), []string{"kubectl"})
	assert.Len(t, results, 1)
	assert.True(t, results[0].Success)
	assert.Equal(t, "kubectl", results[0].Tool)
}

func TestInstallMissing_ComApt_FalhaInstalacao(t *testing.T) {
	pkg := &mockPackageManager{
		detected:   "apt",
		installErr: map[string]error{"k3d": fmt.Errorf("pacote não encontrado")},
	}
	svc := NewService(&mockToolChecker{}, pkg, &testutil.MockRunner{}, &testutil.MockFilesystem{})

	results := svc.InstallMissing(context.Background(), []string{"kubectl", "k3d"})
	assert.Len(t, results, 2)

	assert.True(t, results[0].Success)
	assert.False(t, results[1].Success)
	assert.Error(t, results[1].Error)
}

func TestInstallMissing_SemPackageManager(t *testing.T) {
	pkg := &mockPackageManager{detected: ""}
	svc := NewService(&mockToolChecker{}, pkg, &testutil.MockRunner{}, &testutil.MockFilesystem{})

	results := svc.InstallMissing(context.Background(), []string{"kubectl", "helm"})
	assert.Len(t, results, 2)
	for _, r := range results {
		assert.False(t, r.Success, "sem pkg manager, instalação deveria falhar para %s", r.Tool)
		assert.Contains(t, r.Output, "nenhum gerenciador de pacotes")
	}
}

func TestInstallMissing_ListaVazia(t *testing.T) {
	svc := NewService(&mockToolChecker{}, &mockPackageManager{detected: "brew"}, &testutil.MockRunner{}, &testutil.MockFilesystem{})

	results := svc.InstallMissing(context.Background(), []string{})
	assert.Empty(t, results)
}

// --- Testes ConfigureDirenv ---

func TestConfigureDirenv_EnvrcNaoExiste(t *testing.T) {
	var arquivoCriado string
	var conteudoEscrito []byte
	var permissaoUsada fs.FileMode

	mockFs := &testutil.MockFilesystem{
		StatFunc: func(name string) (fs.FileInfo, error) {
			return nil, os.ErrNotExist
		},
		WriteFileFunc: func(name string, data []byte, perm fs.FileMode) error {
			arquivoCriado = name
			conteudoEscrito = data
			permissaoUsada = perm
			return nil
		},
	}

	mockRunner := &testutil.MockRunner{
		RunFunc: func(_ context.Context, name string, args ...string) error {
			assert.Equal(t, "direnv", name)
			assert.Equal(t, []string{"allow"}, args)
			return nil
		},
	}

	svc := NewService(&mockToolChecker{}, &mockPackageManager{}, mockRunner, mockFs)

	err := svc.ConfigureDirenv("/projeto")
	assert.NoError(t, err)
	assert.Equal(t, "/projeto/.envrc", arquivoCriado)
	assert.Contains(t, string(conteudoEscrito), "KUBECONFIG")
	assert.Equal(t, fs.FileMode(0600), permissaoUsada)
}

func TestConfigureDirenv_EnvrcJaExiste(t *testing.T) {
	writeFileChamado := false

	mockFs := &testutil.MockFilesystem{
		StatFunc: func(name string) (fs.FileInfo, error) {
			// Simula que o arquivo já existe
			return nil, nil
		},
		WriteFileFunc: func(name string, data []byte, perm fs.FileMode) error {
			writeFileChamado = true
			return nil
		},
	}

	direnvChamado := false
	mockRunner := &testutil.MockRunner{
		RunFunc: func(_ context.Context, name string, args ...string) error {
			if name == "direnv" {
				direnvChamado = true
			}
			return nil
		},
	}

	svc := NewService(&mockToolChecker{}, &mockPackageManager{}, mockRunner, mockFs)

	err := svc.ConfigureDirenv("/projeto")
	assert.NoError(t, err)
	assert.False(t, writeFileChamado, "não deveria criar .envrc quando já existe")
	assert.True(t, direnvChamado, "deveria executar direnv allow mesmo quando .envrc já existe")
}

func TestConfigureDirenv_FalhaAoEscrever(t *testing.T) {
	mockFs := &testutil.MockFilesystem{
		StatFunc: func(name string) (fs.FileInfo, error) {
			return nil, os.ErrNotExist
		},
		WriteFileFunc: func(name string, data []byte, perm fs.FileMode) error {
			return fmt.Errorf("permissão negada")
		},
	}

	svc := NewService(&mockToolChecker{}, &mockPackageManager{}, &testutil.MockRunner{}, mockFs)

	err := svc.ConfigureDirenv("/projeto")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "permissão negada")
}
