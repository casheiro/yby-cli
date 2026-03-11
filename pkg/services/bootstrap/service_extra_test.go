package bootstrap

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
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

func TestBootstrapService_PhaseSecrets(t *testing.T) {
	svc := NewService(&MockRunner{}, &MockFilesystem{}, &MockK8sClient{})
	// phaseSecrets is a placeholder, just verify it returns nil
	err := svc.phaseSecrets(context.Background(), "/infra", "https://github.com/test/repo.git")
	if err != nil {
		t.Errorf("phaseSecrets: unexpected error: %v", err)
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
