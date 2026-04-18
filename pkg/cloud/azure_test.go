package cloud

import (
	"context"
	"errors"
	"testing"

	"github.com/casheiro/yby-cli/pkg/testutil"
)

func TestAzureProvider_Name(t *testing.T) {
	p := &azureProvider{runner: &testutil.MockRunner{}}
	if p.Name() != "azure" {
		t.Errorf("Name() = %q, want %q", p.Name(), "azure")
	}
}

func TestAzureProvider_IsAvailable_Found(t *testing.T) {
	runner := &testutil.MockRunner{
		LookPathFunc: func(file string) (string, error) {
			if file == "az" {
				return "/usr/bin/az", nil
			}
			return "", errors.New("não encontrado")
		},
	}
	p := &azureProvider{runner: runner}
	if !p.IsAvailable(context.Background()) {
		t.Error("IsAvailable() = false, want true")
	}
}

func TestAzureProvider_IsAvailable_NotFound(t *testing.T) {
	runner := &testutil.MockRunner{
		LookPathFunc: func(file string) (string, error) {
			return "", errors.New("não encontrado")
		},
	}
	p := &azureProvider{runner: runner}
	if p.IsAvailable(context.Background()) {
		t.Error("IsAvailable() = true, want false")
	}
}

func TestAzureProvider_CLIVersion(t *testing.T) {
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return []byte("azure-cli                         2.57.0\ncore                              2.57.0\n"), nil
		},
	}
	p := &azureProvider{runner: runner}
	version, err := p.CLIVersion(context.Background())
	if err != nil {
		t.Fatalf("CLIVersion() error = %v, want nil", err)
	}
	if version == "" {
		t.Error("CLIVersion() retornou string vazia")
	}
}

func TestAzureProvider_CLIVersion_Error(t *testing.T) {
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return nil, errors.New("comando não encontrado")
		},
	}
	p := &azureProvider{runner: runner}
	_, err := p.CLIVersion(context.Background())
	if err == nil {
		t.Error("CLIVersion() error = nil, want non-nil")
	}
}

func TestAzureProvider_ValidateCredentials_Valid(t *testing.T) {
	const accountJSON = `{"user":{"name":"alice@example.com","type":"user"},"state":"Enabled"}`
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return []byte(accountJSON), nil
		},
	}
	p := &azureProvider{runner: runner}
	status, err := p.ValidateCredentials(context.Background())
	if err != nil {
		t.Fatalf("ValidateCredentials() error = %v, want nil", err)
	}
	if !status.Authenticated {
		t.Error("Authenticated = false, want true")
	}
	if status.Identity != "alice@example.com" {
		t.Errorf("Identity = %q, want %q", status.Identity, "alice@example.com")
	}
	if status.Method != "az-cli" {
		t.Errorf("Method = %q, want %q", status.Method, "az-cli")
	}
}

func TestAzureProvider_ValidateCredentials_CLIError(t *testing.T) {
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return nil, errors.New("não autenticado")
		},
	}
	p := &azureProvider{runner: runner}
	status, err := p.ValidateCredentials(context.Background())
	if err == nil {
		t.Error("ValidateCredentials() error = nil, want non-nil")
	}
	if status == nil {
		t.Fatal("ValidateCredentials() status = nil, want non-nil CredentialStatus")
	}
	if status.Authenticated {
		t.Error("Authenticated = true, want false")
	}
}

func TestAzureProvider_ValidateCredentials_InvalidJSON(t *testing.T) {
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return []byte("não é json"), nil
		},
	}
	p := &azureProvider{runner: runner}
	status, err := p.ValidateCredentials(context.Background())
	if err == nil {
		t.Error("ValidateCredentials() error = nil, want non-nil")
	}
	if status == nil {
		t.Fatal("ValidateCredentials() status = nil, want non-nil CredentialStatus")
	}
	if status.Authenticated {
		t.Error("Authenticated = true, want false")
	}
}

func TestAzureProvider_ListClusters(t *testing.T) {
	const listJSON = `[
		{"name":"prod-aks","location":"eastus","resourceGroup":"rg-prod","kubernetesVersion":"1.29.0","powerState":{"code":"Running"}},
		{"name":"staging-aks","location":"westus2","resourceGroup":"rg-staging","kubernetesVersion":"1.28.5","powerState":{"code":"Running"}}
	]`
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return []byte(listJSON), nil
		},
	}
	p := &azureProvider{runner: runner}
	clusters, err := p.ListClusters(context.Background(), ListOptions{})
	if err != nil {
		t.Fatalf("ListClusters() error = %v, want nil", err)
	}
	if len(clusters) != 2 {
		t.Fatalf("ListClusters() len = %d, want 2", len(clusters))
	}
	if clusters[0].Provider != "azure" {
		t.Errorf("clusters[0].Provider = %q, want %q", clusters[0].Provider, "azure")
	}
	if clusters[0].Region != "eastus" {
		t.Errorf("clusters[0].Region = %q, want %q", clusters[0].Region, "eastus")
	}
	if clusters[0].Version != "1.29.0" {
		t.Errorf("clusters[0].Version = %q, want %q", clusters[0].Version, "1.29.0")
	}
	if clusters[0].Status != "Running" {
		t.Errorf("clusters[0].Status = %q, want %q", clusters[0].Status, "Running")
	}
	if clusters[0].ResourceGroup != "rg-prod" {
		t.Errorf("clusters[0].ResourceGroup = %q, want %q", clusters[0].ResourceGroup, "rg-prod")
	}
}

func TestAzureProvider_ListClusters_RegionFilter(t *testing.T) {
	const listJSON = `[
		{"name":"prod-aks","location":"eastus","resourceGroup":"rg-prod","kubernetesVersion":"1.29.0","powerState":{"code":"Running"}},
		{"name":"staging-aks","location":"westus2","resourceGroup":"rg-staging","kubernetesVersion":"1.28.5","powerState":{"code":"Running"}}
	]`
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return []byte(listJSON), nil
		},
	}
	p := &azureProvider{runner: runner}
	clusters, err := p.ListClusters(context.Background(), ListOptions{Region: "eastus"})
	if err != nil {
		t.Fatalf("ListClusters() error = %v, want nil", err)
	}
	if len(clusters) != 1 {
		t.Fatalf("ListClusters() len = %d, want 1 (filtrado por região)", len(clusters))
	}
	if clusters[0].Name != "prod-aks" {
		t.Errorf("clusters[0].Name = %q, want %q", clusters[0].Name, "prod-aks")
	}
}

func TestAzureProvider_ListClusters_Empty(t *testing.T) {
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return []byte(`[]`), nil
		},
	}
	p := &azureProvider{runner: runner}
	clusters, err := p.ListClusters(context.Background(), ListOptions{})
	if err != nil {
		t.Fatalf("ListClusters() error = %v, want nil", err)
	}
	if len(clusters) != 0 {
		t.Errorf("ListClusters() len = %d, want 0", len(clusters))
	}
}

func TestAzureProvider_ListClusters_CLIError(t *testing.T) {
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return nil, errors.New("erro de rede")
		},
	}
	p := &azureProvider{runner: runner}
	_, err := p.ListClusters(context.Background(), ListOptions{})
	if err == nil {
		t.Error("ListClusters() error = nil, want non-nil")
	}
}

func TestAzureProvider_ListClusters_InvalidJSON(t *testing.T) {
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return []byte("não é json"), nil
		},
	}
	p := &azureProvider{runner: runner}
	_, err := p.ListClusters(context.Background(), ListOptions{})
	if err == nil {
		t.Error("ListClusters() error = nil, want non-nil")
	}
}

func TestAzureProvider_ConfigureKubeconfig(t *testing.T) {
	var capturedArgs []string
	runner := &testutil.MockRunner{
		RunFunc: func(_ context.Context, _ string, args ...string) error {
			capturedArgs = args
			return nil
		},
	}
	p := &azureProvider{runner: runner}
	err := p.ConfigureKubeconfig(context.Background(), ClusterInfo{
		Name:          "prod-aks",
		ResourceGroup: "rg-prod",
	})
	if err != nil {
		t.Fatalf("ConfigureKubeconfig() error = %v, want nil", err)
	}
	// Verifica --name prod-aks
	nameFound := false
	for i, arg := range capturedArgs {
		if arg == "--name" && i+1 < len(capturedArgs) && capturedArgs[i+1] == "prod-aks" {
			nameFound = true
		}
	}
	if !nameFound {
		t.Errorf("--name prod-aks não encontrado nos args: %v", capturedArgs)
	}
	// Verifica --resource-group rg-prod
	rgFound := false
	for i, arg := range capturedArgs {
		if arg == "--resource-group" && i+1 < len(capturedArgs) && capturedArgs[i+1] == "rg-prod" {
			rgFound = true
		}
	}
	if !rgFound {
		t.Errorf("--resource-group rg-prod não encontrado nos args: %v", capturedArgs)
	}
}

func TestAzureProvider_ConfigureKubeconfig_Error(t *testing.T) {
	runner := &testutil.MockRunner{
		RunFunc: func(_ context.Context, _ string, _ ...string) error {
			return errors.New("acesso negado")
		},
	}
	p := &azureProvider{runner: runner}
	err := p.ConfigureKubeconfig(context.Background(), ClusterInfo{Name: "prod-aks", ResourceGroup: "rg-prod"})
	if err == nil {
		t.Error("ConfigureKubeconfig() error = nil, want non-nil")
	}
}

func TestAzureProvider_RefreshToken(t *testing.T) {
	var capturedCmd string
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, cmd string, _ ...string) ([]byte, error) {
			capturedCmd = cmd
			return []byte(`{"accessToken":"eyJ...","expiresOn":"2026-04-08 12:00:00.000000"}`), nil
		},
	}
	p := &azureProvider{runner: runner}
	err := p.RefreshToken(context.Background(), ClusterInfo{Name: "prod-aks"})
	if err != nil {
		t.Fatalf("RefreshToken() error = %v, want nil", err)
	}
	if capturedCmd != "az" {
		t.Errorf("cmd = %q, want %q", capturedCmd, "az")
	}
}

func TestAzureProvider_RefreshToken_Error(t *testing.T) {
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return nil, errors.New("token expirado")
		},
	}
	p := &azureProvider{runner: runner}
	err := p.RefreshToken(context.Background(), ClusterInfo{Name: "prod-aks"})
	if err == nil {
		t.Error("RefreshToken() error = nil, want non-nil")
	}
}

func TestAzureProvider_IsKubeloginAvailable_Found(t *testing.T) {
	runner := &testutil.MockRunner{
		LookPathFunc: func(file string) (string, error) {
			if file == "kubelogin" {
				return "/usr/bin/kubelogin", nil
			}
			return "", errors.New("não encontrado")
		},
	}
	p := &azureProvider{runner: runner}
	if !p.IsKubeloginAvailable(context.Background()) {
		t.Error("IsKubeloginAvailable() = false, want true")
	}
}

func TestAzureProvider_IsKubeloginAvailable_NotFound(t *testing.T) {
	runner := &testutil.MockRunner{
		LookPathFunc: func(file string) (string, error) {
			return "", errors.New("não encontrado")
		},
	}
	p := &azureProvider{runner: runner}
	if p.IsKubeloginAvailable(context.Background()) {
		t.Error("IsKubeloginAvailable() = true, want false")
	}
}

func TestAzureProvider_RegisteredInRegistry(t *testing.T) {
	found := false
	for _, factory := range providerRegistry {
		p := factory(&testutil.MockRunner{})
		if p.Name() == "azure" {
			found = true
			break
		}
	}
	if !found {
		t.Error("provider 'azure' não encontrado no providerRegistry após init()")
	}
}
