package bootstrap

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ---- ensureToolsInstalled ----

type helmFailRunner struct{ MockRunner }

func (r *helmFailRunner) LookPath(file string) (string, error) {
	if file == "helm" {
		return "", errors.New("not found")
	}
	return "/usr/bin/" + file, nil
}

func TestBootstrapService_EnsureToolsInstalled_HelmNaoEncontrado(t *testing.T) {
	svc := NewService(&helmFailRunner{}, &MockFilesystem{}, &MockK8sClient{})
	err := svc.ensureToolsInstalled()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "helm não encontrado")
}

// ---- checkEnvVars ----

func TestBootstrapService_CheckEnvVars_LocalPeloContextFlag(t *testing.T) {
	svc := NewService(&MockRunner{}, &MockFilesystem{}, &MockK8sClient{})
	t.Setenv("GITHUB_REPO", "")

	// context=local, env=qualquer -> não requer GITHUB_REPO
	err := svc.checkEnvVars("local", "production", "")
	assert.NoError(t, err)
}

func TestBootstrapService_CheckEnvVars_LocalPeloEnvEnv(t *testing.T) {
	svc := NewService(&MockRunner{}, &MockFilesystem{}, &MockK8sClient{})
	t.Setenv("GITHUB_REPO", "")

	// context=qualquer, env=local -> não requer GITHUB_REPO
	err := svc.checkEnvVars("staging", "local", "")
	assert.NoError(t, err)
}

// ---- phaseSystemBootstrap com contexto curto para evitar retry longo ----

func shortCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 500*time.Millisecond)
}

func TestBootstrapService_PhaseSystemBootstrap_HelmRepoFalha(t *testing.T) {
	ctx, cancel := shortCtx()
	defer cancel()

	runner := &MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) error {
			if name == "helm" && len(args) > 0 && args[0] == "repo" {
				return errors.New("helm repo add falhou")
			}
			return nil
		},
	}
	svc := NewService(runner, &MockFilesystem{}, &MockK8sClient{})
	err := svc.phaseSystemBootstrap(ctx, "/infra", "argo/argo-cd", "5.51.6")
	assert.Error(t, err)
}

func TestBootstrapService_PhaseSystemBootstrap_NamespaceFalha(t *testing.T) {
	ctx, cancel := shortCtx()
	defer cancel()

	k8s := &MockK8sClient{
		CreateNamespaceFunc: func(ctx context.Context, ns string) error {
			return errors.New("falha ao criar namespace")
		},
	}
	svc := NewService(&MockRunner{}, &MockFilesystem{}, k8s)
	err := svc.phaseSystemBootstrap(ctx, "/infra", "argo/argo-cd", "5.51.6")
	assert.Error(t, err)
}

func TestBootstrapService_PhaseSystemBootstrap_HelmInstallFalha(t *testing.T) {
	ctx, cancel := shortCtx()
	defer cancel()

	runner := &MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) error {
			if name == "helm" && len(args) > 0 && args[0] == "upgrade" {
				return errors.New("helm install falhou")
			}
			return nil
		},
	}
	svc := NewService(runner, &MockFilesystem{}, &MockK8sClient{})
	err := svc.phaseSystemBootstrap(ctx, "/infra", "argo/argo-cd", "5.51.6")
	assert.Error(t, err)
}

// ---- phaseConfigBootstrap ----

func TestBootstrapService_PhaseConfigBootstrap_Remote(t *testing.T) {
	// context!=local e env!=local -> não aplica patch
	applyCalled := false
	k8s := &MockK8sClient{
		ApplyManifestFunc: func(ctx context.Context, path string, namespace string) error {
			applyCalled = true
			return nil
		},
	}
	svc := NewService(&MockRunner{}, &MockFilesystem{}, k8s)
	err := svc.phaseConfigBootstrap(context.Background(), "/infra", "https://github.com/test/repo.git", "remote", "production")
	assert.NoError(t, err)
	assert.True(t, applyCalled)
}

func TestBootstrapService_PhaseConfigBootstrap_ApplyFalha(t *testing.T) {
	ctx, cancel := shortCtx()
	defer cancel()

	k8s := &MockK8sClient{
		ApplyManifestFunc: func(ctx context.Context, path string, namespace string) error {
			return errors.New("falha ao aplicar manifesto")
		},
	}
	svc := NewService(&MockRunner{}, &MockFilesystem{}, k8s)
	err := svc.phaseConfigBootstrap(ctx, "/infra", "https://github.com/test/repo.git", "local", "local")
	assert.Error(t, err)
}

// ---- Run completo ----

func TestBootstrapService_Run_ComEnvVarGithubRepo(t *testing.T) {
	t.Setenv("GITHUB_REPO", "https://github.com/org/repo.git")

	fsys := &MockFilesystem{
		ReadFileFunc: func(name string) ([]byte, error) {
			return nil, os.ErrNotExist // Sem blueprint
		},
	}
	svc := NewService(&MockRunner{}, fsys, &MockK8sClient{})
	err := svc.Run(context.Background(), BootstrapOptions{
		Root:        "/infra",
		Context:     "remote",
		Environment: "production",
	})
	assert.NoError(t, err)
}

func TestBootstrapService_Run_CheckEnvVarsFalha(t *testing.T) {
	t.Setenv("GITHUB_REPO", "")

	fsys := &MockFilesystem{
		ReadFileFunc: func(name string) ([]byte, error) {
			return nil, os.ErrNotExist
		},
	}
	svc := NewService(&MockRunner{}, fsys, &MockK8sClient{})
	err := svc.Run(context.Background(), BootstrapOptions{
		Root:        "/infra",
		Context:     "remote",
		Environment: "production",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "GITHUB_REPO")
}

// ---- phaseConfigBootstrap com PatchApplication falhando ----

type patchFailK8s struct {
	MockK8sClient
}

func (k *patchFailK8s) PatchApplication(ctx context.Context, name, ns, patch string) error {
	return errors.New("falha no patch")
}

func TestBootstrapService_PhaseConfigBootstrap_PatchFalha(t *testing.T) {
	ctx, cancel := shortCtx()
	defer cancel()

	k8s := &patchFailK8s{}
	svc := NewService(&MockRunner{}, &MockFilesystem{}, k8s)
	// local=true dispara PatchApplication
	err := svc.phaseConfigBootstrap(ctx, "/infra", "https://github.com/test/repo.git", "local", "local")
	assert.Error(t, err)
}

// ---- Run com phaseSystemBootstrap falhando (cobre linha 57 do Run) ----

func TestBootstrapService_Run_PhaseSystemFalha(t *testing.T) {
	ctx, cancel := shortCtx()
	defer cancel()

	t.Setenv("GITHUB_REPO", "https://github.com/org/repo.git")

	runner := &MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) error {
			return errors.New("falha no helm")
		},
	}
	fsys := &MockFilesystem{
		ReadFileFunc: func(name string) ([]byte, error) {
			return nil, os.ErrNotExist
		},
	}
	svc := NewService(runner, fsys, &MockK8sClient{})
	err := svc.Run(ctx, BootstrapOptions{Root: "/infra", Context: "remote", Environment: "production"})
	assert.Error(t, err)
}

// ---- Run com phaseConfigBootstrap falhando (cobre linha 67 do Run) ----

func TestBootstrapService_Run_PhaseConfigFalha(t *testing.T) {
	ctx, cancel := shortCtx()
	defer cancel()

	t.Setenv("GITHUB_REPO", "https://github.com/org/repo.git")

	k8s := &MockK8sClient{
		ApplyManifestFunc: func(ctx context.Context, path string, namespace string) error {
			return errors.New("falha ao aplicar manifesto")
		},
	}
	fsys := &MockFilesystem{
		ReadFileFunc: func(name string) ([]byte, error) {
			return nil, os.ErrNotExist
		},
	}
	svc := NewService(&MockRunner{}, fsys, k8s)
	err := svc.Run(ctx, BootstrapOptions{Root: "/infra", Context: "remote", Environment: "production"})
	assert.Error(t, err)
}

// ---- getRepoURLFromBlueprint ----

func TestBootstrapService_GetRepoURLFromBlueprint_DefaultNaoString(t *testing.T) {
	// default é int em vez de string
	fsys := &MockFilesystem{
		ReadFileFunc: func(name string) ([]byte, error) {
			return []byte(`prompts:
  - id: git.repoURL
    default: 42
`), nil
		},
	}
	svc := NewService(&MockRunner{}, fsys, &MockK8sClient{})
	url := svc.getRepoURLFromBlueprint("/project")
	assert.Empty(t, url) // 42 não é string, retorna vazio
}

func TestBootstrapService_GetRepoURLFromBlueprint_MultiplePrompts(t *testing.T) {
	fsys := &MockFilesystem{
		ReadFileFunc: func(name string) ([]byte, error) {
			return []byte(`prompts:
  - id: project.name
    default: meu-projeto
  - id: git.repoURL
    default: https://github.com/org/repo.git
  - id: other
    default: valor
`), nil
		},
	}
	svc := NewService(&MockRunner{}, fsys, &MockK8sClient{})
	url := svc.getRepoURLFromBlueprint("/project")
	assert.Equal(t, "https://github.com/org/repo.git", url)
}
