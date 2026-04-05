package setup

import (
	"context"
	"fmt"
	"testing"

	"github.com/casheiro/yby-cli/pkg/testutil"
	"github.com/stretchr/testify/assert"
)

// ========================================================
// SystemToolChecker
// ========================================================

func TestSystemToolChecker_IsInstalled_Encontrado(t *testing.T) {
	runner := &testutil.MockRunner{
		LookPathFunc: func(file string) (string, error) {
			return "/usr/bin/" + file, nil
		},
	}
	checker := &SystemToolChecker{Runner: runner}

	path, err := checker.IsInstalled("kubectl")
	assert.NoError(t, err)
	assert.Equal(t, "/usr/bin/kubectl", path)
}

func TestSystemToolChecker_IsInstalled_NaoEncontrado(t *testing.T) {
	runner := &testutil.MockRunner{
		LookPathFunc: func(file string) (string, error) {
			return "", fmt.Errorf("não encontrado: %s", file)
		},
	}
	checker := &SystemToolChecker{Runner: runner}

	_, err := checker.IsInstalled("kubectl")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "não encontrado")
}

// ========================================================
// SystemPackageManager — goos
// ========================================================

func TestSystemPackageManager_goos_Injetado(t *testing.T) {
	pkg := &SystemPackageManager{Runner: &testutil.MockRunner{}, GOOS: "darwin"}
	assert.Equal(t, "darwin", pkg.goos())
}

func TestSystemPackageManager_goos_Default(t *testing.T) {
	pkg := &SystemPackageManager{Runner: &testutil.MockRunner{}}
	// Sem GOOS definido, deve retornar runtime.GOOS (não vazio)
	assert.NotEmpty(t, pkg.goos())
}

// ========================================================
// SystemPackageManager — Detect
// ========================================================

func TestSystemPackageManager_Detect_Brew(t *testing.T) {
	runner := &testutil.MockRunner{
		LookPathFunc: func(file string) (string, error) {
			if file == "brew" {
				return "/opt/homebrew/bin/brew", nil
			}
			return "", fmt.Errorf("não encontrado")
		},
	}
	pkg := &SystemPackageManager{Runner: runner}

	assert.Equal(t, "brew", pkg.Detect())
}

func TestSystemPackageManager_Detect_AptLinux(t *testing.T) {
	runner := &testutil.MockRunner{
		LookPathFunc: func(file string) (string, error) {
			if file == "apt-get" {
				return "/usr/bin/apt-get", nil
			}
			return "", fmt.Errorf("não encontrado")
		},
	}
	pkg := &SystemPackageManager{Runner: runner, GOOS: "linux"}

	assert.Equal(t, "apt", pkg.Detect())
}

func TestSystemPackageManager_Detect_SnapLinux(t *testing.T) {
	runner := &testutil.MockRunner{
		LookPathFunc: func(file string) (string, error) {
			if file == "snap" {
				return "/usr/bin/snap", nil
			}
			return "", fmt.Errorf("não encontrado")
		},
	}
	pkg := &SystemPackageManager{Runner: runner, GOOS: "linux"}

	assert.Equal(t, "snap", pkg.Detect())
}

func TestSystemPackageManager_Detect_NenhumEncontrado(t *testing.T) {
	runner := &testutil.MockRunner{
		LookPathFunc: func(file string) (string, error) {
			return "", fmt.Errorf("não encontrado")
		},
	}
	pkg := &SystemPackageManager{Runner: runner, GOOS: "linux"}

	assert.Equal(t, "", pkg.Detect())
}

func TestSystemPackageManager_Detect_AptIgnoradoEmDarwin(t *testing.T) {
	runner := &testutil.MockRunner{
		LookPathFunc: func(file string) (string, error) {
			if file == "apt-get" {
				return "/usr/bin/apt-get", nil
			}
			return "", fmt.Errorf("não encontrado")
		},
	}
	// Em darwin, apt-get não deve ser considerado
	pkg := &SystemPackageManager{Runner: runner, GOOS: "darwin"}

	assert.Equal(t, "", pkg.Detect())
}

// ========================================================
// SystemPackageManager — Install
// ========================================================

func TestSystemPackageManager_Install_Brew(t *testing.T) {
	ctx := context.Background()
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, name string, args ...string) ([]byte, error) {
			assert.Equal(t, "brew", name)
			assert.Equal(t, []string{"install", "kubectl"}, args)
			return []byte("instalado"), nil
		},
	}
	pkg := &SystemPackageManager{Runner: runner}

	out, err := pkg.Install(ctx, "kubectl", "brew")
	assert.NoError(t, err)
	assert.Equal(t, "instalado", string(out))
}

func TestSystemPackageManager_Install_Apt(t *testing.T) {
	ctx := context.Background()
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, name string, args ...string) ([]byte, error) {
			assert.Equal(t, "sudo", name)
			assert.Equal(t, []string{"apt-get", "install", "-y", "helm"}, args)
			return []byte("instalado"), nil
		},
	}
	pkg := &SystemPackageManager{Runner: runner}

	out, err := pkg.Install(ctx, "helm", "apt")
	assert.NoError(t, err)
	assert.Equal(t, "instalado", string(out))
}

func TestSystemPackageManager_Install_Snap(t *testing.T) {
	ctx := context.Background()
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, name string, args ...string) ([]byte, error) {
			assert.Equal(t, "sudo", name)
			assert.Equal(t, []string{"snap", "install", "k3d"}, args)
			return []byte("instalado"), nil
		},
	}
	pkg := &SystemPackageManager{Runner: runner}

	out, err := pkg.Install(ctx, "k3d", "snap")
	assert.NoError(t, err)
	assert.Equal(t, "instalado", string(out))
}

func TestSystemPackageManager_Install_Default(t *testing.T) {
	ctx := context.Background()
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, name string, args ...string) ([]byte, error) {
			assert.Equal(t, "echo", name)
			assert.Equal(t, []string{"noop"}, args)
			return []byte("noop"), nil
		},
	}
	pkg := &SystemPackageManager{Runner: runner}

	out, err := pkg.Install(ctx, "tool", "desconhecido")
	assert.NoError(t, err)
	assert.Equal(t, "noop", string(out))
}

func TestSystemPackageManager_Install_Erro(t *testing.T) {
	ctx := context.Background()
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, name string, args ...string) ([]byte, error) {
			return nil, fmt.Errorf("falha na instalação")
		},
	}
	pkg := &SystemPackageManager{Runner: runner}

	_, err := pkg.Install(ctx, "kubectl", "brew")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "falha na instalação")
}
