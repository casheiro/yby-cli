package bootstrap

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBootstrapService_EnsureToolsInstalled_Success(t *testing.T) {
	// MockRunner.LookPath always returns /usr/bin/<tool>
	svc := NewService(&MockRunner{}, &MockFilesystem{}, &MockK8sClient{})
	err := svc.ensureToolsInstalled()
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

type lookPathFailRunner struct{ MockRunner }

func (r *lookPathFailRunner) LookPath(file string) (string, error) {
	if file == "kubectl" {
		return "", errors.New("not found")
	}
	return "/usr/bin/" + file, nil
}

func TestBootstrapService_EnsureToolsInstalled_Kubectl_NotFound(t *testing.T) {
	svc := NewService(&lookPathFailRunner{}, &MockFilesystem{}, &MockK8sClient{})
	err := svc.ensureToolsInstalled()
	if err == nil {
		t.Error("expected error when kubectl not found, got nil")
	}
}

func TestBootstrapService_CheckEnvVars_LocalContext(t *testing.T) {
	runner := &MockRunner{}
	svc := NewService(runner, &MockFilesystem{}, &MockK8sClient{})

	// local context should not require GITHUB_REPO
	os.Unsetenv("GITHUB_REPO")
	err := svc.checkEnvVars("local", "local", "")
	if err != nil {
		t.Errorf("expected no error for local context, got: %v", err)
	}
}

func TestBootstrapService_CheckEnvVars_RemoteNoRepo(t *testing.T) {
	runner := &MockRunner{}
	svc := NewService(runner, &MockFilesystem{}, &MockK8sClient{})

	os.Unsetenv("GITHUB_REPO")
	err := svc.checkEnvVars("remote", "remote", "")
	if err == nil {
		t.Error("expected error for remote context without GITHUB_REPO, got nil")
	}
}

func TestBootstrapService_CheckEnvVars_BlueprintRepo(t *testing.T) {
	runner := &MockRunner{}
	svc := NewService(runner, &MockFilesystem{}, &MockK8sClient{})

	os.Unsetenv("GITHUB_REPO")
	// blueprintRepo is set, so no error even for remote
	err := svc.checkEnvVars("remote", "remote", "https://github.com/org/repo.git")
	if err != nil {
		t.Errorf("expected no error when blueprintRepo is set, got: %v", err)
	}
}

func TestBootstrapService_CheckEnvVars_EnvVarSet(t *testing.T) {
	runner := &MockRunner{}
	svc := NewService(runner, &MockFilesystem{}, &MockK8sClient{})

	os.Setenv("GITHUB_REPO", "https://github.com/org/repo.git")
	defer os.Unsetenv("GITHUB_REPO")
	err := svc.checkEnvVars("remote", "remote", "")
	if err != nil {
		t.Errorf("expected no error when GITHUB_REPO is set, got: %v", err)
	}
}

func TestBootstrapService_GetRepoURLFromBlueprint_Success(t *testing.T) {
	blueprintYAML := `prompts:
  - id: git.repoURL
    default: https://github.com/test/repo.git
`
	fsys := &MockFilesystem{
		ReadFileFunc: func(name string) ([]byte, error) {
			return []byte(blueprintYAML), nil
		},
	}
	svc := NewService(&MockRunner{}, fsys, &MockK8sClient{})
	url := svc.getRepoURLFromBlueprint("/project")
	if url != "https://github.com/test/repo.git" {
		t.Errorf("expected URL from blueprint, got %q", url)
	}
}

func TestBootstrapService_GetRepoURLFromBlueprint_FileNotFound(t *testing.T) {
	fsys := &MockFilesystem{} // ReadFile returns ErrNotExist
	svc := NewService(&MockRunner{}, fsys, &MockK8sClient{})
	url := svc.getRepoURLFromBlueprint("/project")
	if url != "" {
		t.Errorf("expected empty URL for missing file, got %q", url)
	}
}

func TestBootstrapService_GetRepoURLFromBlueprint_InvalidYAML(t *testing.T) {
	fsys := &MockFilesystem{
		ReadFileFunc: func(name string) ([]byte, error) {
			return []byte("invalid: [yaml: :::"), nil
		},
	}
	svc := NewService(&MockRunner{}, fsys, &MockK8sClient{})
	url := svc.getRepoURLFromBlueprint("/project")
	if url != "" {
		t.Errorf("expected empty URL for invalid YAML, got %q", url)
	}
}

func TestBootstrapService_GetRepoURLFromBlueprint_NoMatchingPrompt(t *testing.T) {
	blueprintYAML := `prompts:
  - id: other.prompt
    default: some-value
`
	fsys := &MockFilesystem{
		ReadFileFunc: func(name string) ([]byte, error) {
			return []byte(blueprintYAML), nil
		},
	}
	svc := NewService(&MockRunner{}, fsys, &MockK8sClient{})
	url := svc.getRepoURLFromBlueprint("/project")
	if url != "" {
		t.Errorf("expected empty URL when prompt ID doesn't match, got %q", url)
	}
}

func TestBootstrapService_Run_MissingTools(t *testing.T) {
	runner := &lookPathFailRunner{}
	svc := NewService(runner, &MockFilesystem{}, &MockK8sClient{})

	os.Setenv("GITHUB_REPO", "https://github.com/test/repo.git")
	defer os.Unsetenv("GITHUB_REPO")

	err := svc.Run(context.Background(), BootstrapOptions{Root: "/tmp/infra"})
	if err == nil {
		t.Error("expected error when kubectl not found, got nil")
	}
}

func TestBootstrapService_PhaseConfigBootstrap_Local(t *testing.T) {
	runner := &MockRunner{}
	k8s := &MockK8sClient{}
	svc := NewService(runner, &MockFilesystem{}, k8s)

	err := svc.phaseConfigBootstrap(context.Background(), "/infra", "https://github.com/test/repo.git", "local", "local")
	if err != nil {
		t.Errorf("phaseConfigBootstrap local: unexpected error: %v", err)
	}
}

func TestBootstrapService_PhaseSecrets_Default(t *testing.T) {
	// Sem blueprint → detectSecretsStrategy retorna "sealed-secrets" (branch default)
	svc := NewService(&MockRunner{}, &MockFilesystem{}, &MockK8sClient{})
	err := svc.phaseSecrets(context.Background(), "/infra", "https://github.com/test/repo.git", false)
	if err != nil {
		t.Errorf("phaseSecrets default: erro inesperado: %v", err)
	}
}

// lookPathSelectiveRunner permite controlar quais ferramentas são encontradas via LookPath.
type lookPathSelectiveRunner struct {
	MockRunner
	available map[string]bool
}

func (r *lookPathSelectiveRunner) LookPath(file string) (string, error) {
	if r.available[file] {
		return "/usr/bin/" + file, nil
	}
	return "", errors.New(file + " não encontrado")
}

func TestBootstrapService_PhaseSecrets_SOPS_Success(t *testing.T) {
	// Blueprint retorna "sops" e ambas ferramentas estão disponíveis
	fsys := &MockFilesystem{
		ReadFileFunc: func(name string) ([]byte, error) {
			return []byte(`prompts:
  - id: secrets.strategy
    default: sops
`), nil
		},
	}
	runner := &lookPathSelectiveRunner{
		available: map[string]bool{"sops": true, "age": true, "kubectl": true, "helm": true},
	}
	svc := NewService(runner, fsys, &MockK8sClient{})

	err := svc.phaseSecrets(context.Background(), "/infra", "https://github.com/test/repo.git", false)
	if err != nil {
		t.Errorf("phaseSecrets sops sucesso: erro inesperado: %v", err)
	}
}

func TestBootstrapService_PhaseSecrets_SOPS_MissingSops(t *testing.T) {
	// Blueprint retorna "sops" mas binário sops não está disponível
	fsys := &MockFilesystem{
		ReadFileFunc: func(name string) ([]byte, error) {
			return []byte(`prompts:
  - id: secrets.strategy
    default: sops
`), nil
		},
	}
	runner := &lookPathSelectiveRunner{
		available: map[string]bool{"age": true},
	}
	svc := NewService(runner, fsys, &MockK8sClient{})

	err := svc.phaseSecrets(context.Background(), "/infra", "https://github.com/test/repo.git", false)
	if err == nil {
		t.Error("esperava erro quando sops não está instalado, mas obteve nil")
	}
}

func TestBootstrapService_PhaseSecrets_SOPS_MissingAge(t *testing.T) {
	// Blueprint retorna "sops", sops existe mas age não
	fsys := &MockFilesystem{
		ReadFileFunc: func(name string) ([]byte, error) {
			return []byte(`prompts:
  - id: secrets.strategy
    default: sops
`), nil
		},
	}
	runner := &lookPathSelectiveRunner{
		available: map[string]bool{"sops": true},
	}
	svc := NewService(runner, fsys, &MockK8sClient{})

	err := svc.phaseSecrets(context.Background(), "/infra", "https://github.com/test/repo.git", false)
	if err == nil {
		t.Error("esperava erro quando age não está instalado, mas obteve nil")
	}
}

func TestBootstrapService_PhaseSecrets_ExternalSecrets(t *testing.T) {
	// Blueprint retorna "external-secrets" → executa kubectl get crd
	fsys := &MockFilesystem{
		ReadFileFunc: func(name string) ([]byte, error) {
			return []byte(`prompts:
  - id: secrets.strategy
    default: external-secrets
`), nil
		},
	}
	var kubectlCalled bool
	runner := &MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) error {
			if name == "kubectl" && len(args) > 0 && args[0] == "get" {
				kubectlCalled = true
			}
			return nil
		},
	}
	svc := NewService(runner, fsys, &MockK8sClient{})

	err := svc.phaseSecrets(context.Background(), "/infra", "https://github.com/test/repo.git", false)
	if err != nil {
		t.Errorf("phaseSecrets external-secrets: erro inesperado: %v", err)
	}
	if !kubectlCalled {
		t.Error("esperava chamada kubectl get crd para external-secrets")
	}
}

func TestBootstrapService_PhaseSecrets_PlainSecrets_PulaVerificacao(t *testing.T) {
	// Com plainSecrets=true, phaseSecrets deve retornar nil sem verificar ferramentas
	fsys := &MockFilesystem{
		ReadFileFunc: func(name string) ([]byte, error) {
			return []byte(`prompts:
  - id: secrets.strategy
    default: sops
`), nil
		},
	}
	// Runner que falha em tudo — se phaseSecrets chamar algo, o teste quebra
	runner := &lookPathSelectiveRunner{
		available: map[string]bool{},
	}
	svc := NewService(runner, fsys, &MockK8sClient{})

	err := svc.phaseSecrets(context.Background(), "/infra", "https://github.com/test/repo.git", true)
	assert.NoError(t, err, "plainSecrets=true deve pular toda verificação de secrets")
}

func TestBootstrapService_Run_PlainSecrets_PulaPhaseSecrets(t *testing.T) {
	// Com PlainSecrets=true, Run deve pular a fase de secrets mesmo com sops configurado e ausente
	t.Setenv("GITHUB_REPO", "https://github.com/org/repo.git")

	fsys := &MockFilesystem{
		ReadFileFunc: func(name string) ([]byte, error) {
			return []byte(`prompts:
  - id: secrets.strategy
    default: sops
`), nil
		},
	}
	// sops e age ausentes — sem PlainSecrets falharia
	runner := &lookPathSelectiveRunner{
		available: map[string]bool{"kubectl": true, "helm": true},
	}
	svc := NewService(runner, fsys, &MockK8sClient{})

	err := svc.Run(context.Background(), BootstrapOptions{
		Root:         "/tmp/infra",
		Context:      "local",
		Environment:  "local",
		PlainSecrets: true,
	})
	assert.NoError(t, err, "PlainSecrets=true deve pular phaseSecrets sem erro")
}

func TestBootstrapService_DetectSecretsStrategy_FileNotFound(t *testing.T) {
	// FS retorna erro → deve retornar "sealed-secrets"
	fsys := &MockFilesystem{} // ReadFile retorna ErrNotExist por padrão
	svc := NewService(&MockRunner{}, fsys, &MockK8sClient{})

	strategy := svc.detectSecretsStrategy("/project")
	assert.Equal(t, "sealed-secrets", strategy, "arquivo inexistente deve retornar sealed-secrets")
}

func TestBootstrapService_DetectSecretsStrategy_InvalidYAML(t *testing.T) {
	fsys := &MockFilesystem{
		ReadFileFunc: func(name string) ([]byte, error) {
			return []byte("invalid: [yaml: :::"), nil
		},
	}
	svc := NewService(&MockRunner{}, fsys, &MockK8sClient{})

	strategy := svc.detectSecretsStrategy("/project")
	assert.Equal(t, "sealed-secrets", strategy, "YAML inválido deve retornar sealed-secrets")
}

func TestBootstrapService_DetectSecretsStrategy_PromptFound(t *testing.T) {
	fsys := &MockFilesystem{
		ReadFileFunc: func(name string) ([]byte, error) {
			return []byte(`prompts:
  - id: secrets.strategy
    default: sops
`), nil
		},
	}
	svc := NewService(&MockRunner{}, fsys, &MockK8sClient{})

	strategy := svc.detectSecretsStrategy("/project")
	assert.Equal(t, "sops", strategy, "deve retornar a estratégia do blueprint")
}

func TestBootstrapService_DetectSecretsStrategy_PromptNotFound(t *testing.T) {
	fsys := &MockFilesystem{
		ReadFileFunc: func(name string) ([]byte, error) {
			return []byte(`prompts:
  - id: other.setting
    default: some-value
`), nil
		},
	}
	svc := NewService(&MockRunner{}, fsys, &MockK8sClient{})

	strategy := svc.detectSecretsStrategy("/project")
	assert.Equal(t, "sealed-secrets", strategy, "prompt não encontrado deve retornar sealed-secrets")
}

func TestBootstrapService_DetectSecretsStrategy_NonStringDefault(t *testing.T) {
	// Prompt encontrado mas default não é string → deve retornar "sealed-secrets"
	fsys := &MockFilesystem{
		ReadFileFunc: func(name string) ([]byte, error) {
			return []byte(`prompts:
  - id: secrets.strategy
    default: 42
`), nil
		},
	}
	svc := NewService(&MockRunner{}, fsys, &MockK8sClient{})

	strategy := svc.detectSecretsStrategy("/project")
	assert.Equal(t, "sealed-secrets", strategy, "default não-string deve retornar sealed-secrets")
}

func TestBootstrapService_Run_CheckEnvVarsFails(t *testing.T) {
	// Contexto remoto sem GITHUB_REPO e sem blueprint → checkEnvVars deve falhar
	os.Unsetenv("GITHUB_REPO")
	fsys := &MockFilesystem{} // ReadFile retorna ErrNotExist → blueprintRepo = ""
	svc := NewService(&MockRunner{}, fsys, &MockK8sClient{})

	err := svc.Run(context.Background(), BootstrapOptions{
		Root:        "/tmp/infra",
		Context:     "remote",
		Environment: "remote",
	})
	if err == nil {
		t.Error("esperava erro de checkEnvVars para contexto remoto sem repo, mas obteve nil")
	}
}

func TestBootstrapService_Run_PhaseSecretsFails(t *testing.T) {
	// Estratégia sops mas binário sops ausente → phaseSecrets falha dentro de Run
	os.Setenv("GITHUB_REPO", "https://github.com/env/repo.git")
	defer os.Unsetenv("GITHUB_REPO")

	fsys := &MockFilesystem{
		ReadFileFunc: func(name string) ([]byte, error) {
			return []byte(`prompts:
  - id: secrets.strategy
    default: sops
`), nil
		},
	}
	runner := &lookPathSelectiveRunner{
		available: map[string]bool{"kubectl": true, "helm": true},
	}
	svc := NewService(runner, fsys, &MockK8sClient{})

	err := svc.Run(context.Background(), BootstrapOptions{Root: "/tmp/infra"})
	if err == nil {
		t.Error("esperava erro de phaseSecrets (sops ausente), mas obteve nil")
	}
}

func TestBootstrapService_Run_UsesGithubRepoEnv(t *testing.T) {
	// Testa o branch onde GITHUB_REPO substitui blueprintRepo
	os.Setenv("GITHUB_REPO", "https://github.com/env/repo.git")
	defer os.Unsetenv("GITHUB_REPO")

	fsys := &MockFilesystem{
		ReadFileFunc: func(name string) ([]byte, error) {
			return []byte("prompts: []"), nil
		},
	}
	svc := NewService(&MockRunner{}, fsys, &MockK8sClient{})

	err := svc.Run(context.Background(), BootstrapOptions{Root: "/tmp/infra"})
	if err != nil {
		t.Fatalf("Run com GITHUB_REPO: erro inesperado: %v", err)
	}
}

func TestNewService(t *testing.T) {
	svc := NewService(&MockRunner{}, &MockFilesystem{}, &MockK8sClient{})
	if svc == nil {
		t.Error("NewService returned nil")
	}
}

func TestBootstrapOptions_Fields(t *testing.T) {
	opts := BootstrapOptions{
		Root:        "/infra",
		RepoURL:     "https://github.com/test/repo.git",
		Context:     "local",
		Environment: "local",
	}
	if opts.Root != "/infra" {
		t.Errorf("Root field mismatch")
	}
	_ = filepath.Join(opts.Root, "file")
}
