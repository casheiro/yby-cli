package cloud

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/casheiro/yby-cli/pkg/services/shared"
	"github.com/casheiro/yby-cli/pkg/testutil"
)

// fakeProvider é uma implementação mínima de CloudProvider para testes.
type fakeProvider struct {
	name      string
	available bool
}

func (f *fakeProvider) Name() string                                 { return f.name }
func (f *fakeProvider) IsAvailable(_ context.Context) bool           { return f.available }
func (f *fakeProvider) CLIVersion(_ context.Context) (string, error) { return "1.0.0", nil }
func (f *fakeProvider) ListClusters(_ context.Context, _ ListOptions) ([]ClusterInfo, error) {
	return nil, nil
}
func (f *fakeProvider) ConfigureKubeconfig(_ context.Context, _ ClusterInfo) error { return nil }
func (f *fakeProvider) ValidateCredentials(_ context.Context) (*CredentialStatus, error) {
	return &CredentialStatus{Authenticated: f.available}, nil
}
func (f *fakeProvider) RefreshToken(_ context.Context, _ ClusterInfo) error { return nil }

// cleanRegistry substitui providerRegistry por uma lista vazia durante o teste
// e restaura o original ao final.
func cleanRegistry(t *testing.T) func() {
	t.Helper()
	original := providerRegistry
	providerRegistry = nil
	return func() { providerRegistry = original }
}

// writeKubeconfig escreve um kubeconfig YAML num arquivo temporário e retorna o caminho.
func writeKubeconfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("erro ao escrever kubeconfig: %v", err)
	}
	return path
}

func TestMatchCommand(t *testing.T) {
	tests := []struct {
		cmd  string
		want string
	}{
		{"aws", "aws"},
		{"AWS", "aws"},
		{"aws-iam-authenticator", "aws"},
		{"az", "azure"},
		{"kubelogin", "azure"},
		{"gcloud", "gcp"},
		{"gke-gcloud-auth-plugin", "gcp"},
		{"kubectl", ""},
		{"unknown-tool", ""},
		{"", ""},
	}

	for _, tc := range tests {
		got := matchCommand(tc.cmd)
		if got != tc.want {
			t.Errorf("matchCommand(%q) = %q, want %q", tc.cmd, got, tc.want)
		}
	}
}

func TestMatchCommandAbsolutePath(t *testing.T) {
	// Usa filepath.Base para obter apenas o nome do comando de um caminho absoluto
	got := matchCommand(filepath.Base("/usr/local/bin/aws"))
	if got != "aws" {
		t.Errorf("matchCommand(base do caminho absoluto) = %q, want %q", got, "aws")
	}
}

func TestDetectFromKubeconfig_NoFile(t *testing.T) {
	// Neutraliza o fallback ~/.kube/config apontando HOME para um diretório temporário
	// sem subdiretório .kube/, evitando que máquinas com kubeconfig real tornem o teste flaky.
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("KUBECONFIG", "/nao/existe/config")
	got := detectFromKubeconfig()
	if len(got) != 0 {
		t.Errorf("esperava mapa vazio com kubeconfig inexistente, got %v", got)
	}
}

func TestDetectFromKubeconfig_InvalidYAML(t *testing.T) {
	path := writeKubeconfig(t, ":::invalid yaml:::")
	t.Setenv("KUBECONFIG", path)
	got := detectFromKubeconfig()
	if len(got) != 0 {
		t.Errorf("esperava mapa vazio com YAML inválido, got %v", got)
	}
}

func TestDetectFromKubeconfig_AWSPattern(t *testing.T) {
	kubeYAML := `
users:
  - name: arn:aws:eks:us-east-1:123:cluster/prod
    user:
      exec:
        command: aws
`
	path := writeKubeconfig(t, kubeYAML)
	t.Setenv("KUBECONFIG", path)

	got := detectFromKubeconfig()
	if _, ok := got["aws"]; !ok {
		t.Errorf("esperava provider 'aws' detectado, got %v", got)
	}
}

func TestDetectFromKubeconfig_AWSIAMAuthenticator(t *testing.T) {
	kubeYAML := `
users:
  - name: eks-user
    user:
      exec:
        command: aws-iam-authenticator
`
	path := writeKubeconfig(t, kubeYAML)
	t.Setenv("KUBECONFIG", path)

	got := detectFromKubeconfig()
	if _, ok := got["aws"]; !ok {
		t.Errorf("esperava provider 'aws' para aws-iam-authenticator, got %v", got)
	}
}

func TestDetectFromKubeconfig_AzurePatterns(t *testing.T) {
	tests := []struct {
		name    string
		command string
	}{
		{"kubelogin", "kubelogin"},
		{"az", "az"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			kubeYAML := "users:\n  - name: aks-user\n    user:\n      exec:\n        command: " + tc.command + "\n"
			path := writeKubeconfig(t, kubeYAML)
			t.Setenv("KUBECONFIG", path)

			got := detectFromKubeconfig()
			if _, ok := got["azure"]; !ok {
				t.Errorf("esperava provider 'azure' para comando %q, got %v", tc.command, got)
			}
		})
	}
}

func TestDetectFromKubeconfig_GCPPatterns(t *testing.T) {
	tests := []struct {
		name    string
		command string
	}{
		{"gke-plugin", "gke-gcloud-auth-plugin"},
		{"gcloud", "gcloud"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			kubeYAML := "users:\n  - name: gke-user\n    user:\n      exec:\n        command: " + tc.command + "\n"
			path := writeKubeconfig(t, kubeYAML)
			t.Setenv("KUBECONFIG", path)

			got := detectFromKubeconfig()
			if _, ok := got["gcp"]; !ok {
				t.Errorf("esperava provider 'gcp' para comando %q, got %v", tc.command, got)
			}
		})
	}
}

func TestDetectFromKubeconfig_NoExecPlugin(t *testing.T) {
	kubeYAML := `
users:
  - name: plain-user
    user:
      token: meu-token-estatico
`
	path := writeKubeconfig(t, kubeYAML)
	t.Setenv("KUBECONFIG", path)

	got := detectFromKubeconfig()
	if len(got) != 0 {
		t.Errorf("esperava mapa vazio para usuário sem exec plugin, got %v", got)
	}
}

func TestDetectFromKubeconfig_MultipleUsers(t *testing.T) {
	kubeYAML := `
users:
  - name: eks-user
    user:
      exec:
        command: aws
  - name: aks-user
    user:
      exec:
        command: kubelogin
  - name: plain-user
    user:
      token: abc123
`
	path := writeKubeconfig(t, kubeYAML)
	t.Setenv("KUBECONFIG", path)

	got := detectFromKubeconfig()
	if _, ok := got["aws"]; !ok {
		t.Errorf("esperava 'aws' no mapa, got %v", got)
	}
	if _, ok := got["azure"]; !ok {
		t.Errorf("esperava 'azure' no mapa, got %v", got)
	}
	if len(got) != 2 {
		t.Errorf("esperava exatamente 2 providers, got %d: %v", len(got), got)
	}
}

func TestDetect_EmptyRegistry(t *testing.T) {
	restore := cleanRegistry(t)
	defer restore()

	runner := &testutil.MockRunner{}
	result := Detect(context.Background(), runner)
	if len(result) != 0 {
		t.Errorf("Detect com registry vazio deve retornar slice vazio, got %v", result)
	}
}

func TestDetect_ProviderInKubeconfigAndAvailable(t *testing.T) {
	restore := cleanRegistry(t)
	defer restore()

	RegisterProvider(func(_ shared.Runner) CloudProvider {
		return &fakeProvider{name: "aws", available: true}
	})

	kubeYAML := `
users:
  - name: eks-user
    user:
      exec:
        command: aws
`
	path := writeKubeconfig(t, kubeYAML)
	t.Setenv("KUBECONFIG", path)

	runner := &testutil.MockRunner{}
	result := Detect(context.Background(), runner)
	if len(result) != 1 {
		t.Fatalf("esperava 1 provider, got %d", len(result))
	}
	if result[0].Name() != "aws" {
		t.Errorf("provider name = %q, want %q", result[0].Name(), "aws")
	}
}

func TestDetect_ProviderAvailableButNotInKubeconfig(t *testing.T) {
	restore := cleanRegistry(t)
	defer restore()

	RegisterProvider(func(_ shared.Runner) CloudProvider {
		return &fakeProvider{name: "aws", available: true}
	})

	// Kubeconfig sem exec plugin
	kubeYAML := `
users:
  - name: plain-user
    user:
      token: abc
`
	path := writeKubeconfig(t, kubeYAML)
	t.Setenv("KUBECONFIG", path)

	runner := &testutil.MockRunner{}
	result := Detect(context.Background(), runner)
	// CLI disponível mesmo sem estar no kubeconfig → incluído
	if len(result) != 1 {
		t.Errorf("esperava 1 provider (CLI disponível), got %d", len(result))
	}
}

func TestDetect_ProviderInKubeconfigButCLINotInstalled(t *testing.T) {
	restore := cleanRegistry(t)
	defer restore()

	RegisterProvider(func(_ shared.Runner) CloudProvider {
		return &fakeProvider{name: "aws", available: false}
	})

	kubeYAML := `
users:
  - name: eks-user
    user:
      exec:
        command: aws
`
	path := writeKubeconfig(t, kubeYAML)
	t.Setenv("KUBECONFIG", path)

	runner := &testutil.MockRunner{
		LookPathFunc: func(file string) (string, error) {
			return "", errors.New("not found")
		},
	}
	result := Detect(context.Background(), runner)
	// Está no kubeconfig mas IsAvailable=false → ainda incluído (kubeconfig tem precedência)
	if len(result) != 1 {
		t.Errorf("esperava 1 provider (presente no kubeconfig), got %d", len(result))
	}
}

func TestDetect_NoDuplicates(t *testing.T) {
	restore := cleanRegistry(t)
	defer restore()

	// Registra o mesmo provider duas vezes (simulando importação duplicada acidental)
	for i := 0; i < 2; i++ {
		RegisterProvider(func(_ shared.Runner) CloudProvider {
			return &fakeProvider{name: "aws", available: true}
		})
	}

	runner := &testutil.MockRunner{}
	result := Detect(context.Background(), runner)
	if len(result) != 1 {
		t.Errorf("Detect deve deduplicar por Name(), got %d providers", len(result))
	}
}

func TestActiveKubeconfigPath_EnvVar(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "custom-config")
	if err := os.WriteFile(path, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KUBECONFIG", path)

	got := activeKubeconfigPath()
	if got != path {
		t.Errorf("activeKubeconfigPath() = %q, want %q", got, path)
	}
}

func TestActiveKubeconfigPath_EnvVarFirstExisting(t *testing.T) {
	dir := t.TempDir()
	exists := filepath.Join(dir, "exists")
	if err := os.WriteFile(exists, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(dir, "missing")
	// missing não existe, exists sim — deve retornar o primeiro existente
	t.Setenv("KUBECONFIG", missing+string(filepath.ListSeparator)+exists)

	got := activeKubeconfigPath()
	if got != exists {
		t.Errorf("activeKubeconfigPath() = %q, want %q (primeiro existente)", got, exists)
	}
}
