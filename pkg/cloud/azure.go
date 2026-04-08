package cloud

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/casheiro/yby-cli/pkg/services/shared"
)

func init() {
	RegisterProvider(func(runner shared.Runner) CloudProvider {
		return &azureProvider{runner: runner}
	})
}

type azureProvider struct {
	runner shared.Runner
}

func (a *azureProvider) Name() string {
	return "azure"
}

// IsAvailable verifica se o CLI `az` está instalado sem fazer chamadas de rede.
func (a *azureProvider) IsAvailable(_ context.Context) bool {
	_, err := a.runner.LookPath("az")
	return err == nil
}

// CLIVersion retorna a versão do Azure CLI via `az --version`.
func (a *azureProvider) CLIVersion(ctx context.Context) (string, error) {
	out, err := a.runner.RunCombinedOutput(ctx, "az", "--version")
	if err != nil {
		return "", fmt.Errorf("az --version: %w", err)
	}
	lines := strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)
	if len(lines) > 0 {
		return strings.TrimSpace(lines[0]), nil
	}
	return strings.TrimSpace(string(out)), nil
}

// azAccountShow representa a saída de `az account show --output json`.
type azAccountShow struct {
	User struct {
		Name string `json:"name"`
		Type string `json:"type"`
	} `json:"user"`
	State string `json:"state"`
}

// ValidateCredentials verifica credenciais via `az account show`.
func (a *azureProvider) ValidateCredentials(ctx context.Context) (*CredentialStatus, error) {
	out, err := a.runner.RunCombinedOutput(ctx, "az", "account", "show", "--output", "json")
	if err != nil {
		return &CredentialStatus{Authenticated: false}, fmt.Errorf("az account show: %w", err)
	}

	var account azAccountShow
	if err := json.Unmarshal(out, &account); err != nil {
		return &CredentialStatus{Authenticated: false}, fmt.Errorf("parse account show: %w", err)
	}

	return &CredentialStatus{
		Authenticated: true,
		Identity:      account.User.Name,
		Method:        "az-cli",
	}, nil
}

// azAKSCluster representa um cluster AKS na saída de `az aks list --output json`.
type azAKSCluster struct {
	Name              string `json:"name"`
	Location          string `json:"location"`
	ResourceGroup     string `json:"resourceGroup"`
	KubernetesVersion string `json:"kubernetesVersion"`
	PowerState        struct {
		Code string `json:"code"`
	} `json:"powerState"`
}

// ListClusters lista clusters AKS via `az aks list`.
// Quando opts.Region não está vazio, filtra por localização (case-insensitive).
func (a *azureProvider) ListClusters(ctx context.Context, opts ListOptions) ([]ClusterInfo, error) {
	out, err := a.runner.RunCombinedOutput(ctx, "az", "aks", "list", "--output", "json")
	if err != nil {
		return nil, fmt.Errorf("az aks list: %w", err)
	}

	var aksClusters []azAKSCluster
	if err := json.Unmarshal(out, &aksClusters); err != nil {
		return nil, fmt.Errorf("parse aks list: %w", err)
	}

	clusters := make([]ClusterInfo, 0, len(aksClusters))
	for _, c := range aksClusters {
		if opts.Region != "" && !strings.EqualFold(c.Location, opts.Region) {
			continue
		}
		clusters = append(clusters, ClusterInfo{
			Name:          c.Name,
			Region:        c.Location,
			Provider:      "azure",
			Version:       c.KubernetesVersion,
			Status:        c.PowerState.Code,
			ResourceGroup: c.ResourceGroup,
		})
	}
	return clusters, nil
}

// ConfigureKubeconfig configura o kubeconfig via `az aks get-credentials`.
func (a *azureProvider) ConfigureKubeconfig(ctx context.Context, cluster ClusterInfo) error {
	args := []string{"aks", "get-credentials", "--name", cluster.Name, "--resource-group", cluster.ResourceGroup}
	if err := a.runner.Run(ctx, "az", args...); err != nil {
		return fmt.Errorf("az aks get-credentials: %w", err)
	}
	return nil
}

// RefreshToken renova o token de autenticação via `az account get-access-token`.
func (a *azureProvider) RefreshToken(ctx context.Context, _ ClusterInfo) error {
	if _, err := a.runner.RunCombinedOutput(ctx, "az", "account", "get-access-token", "--output", "json"); err != nil {
		return fmt.Errorf("az account get-access-token: %w", err)
	}
	return nil
}

// IsKubeloginAvailable verifica se o kubelogin está instalado.
// kubelogin é necessário para autenticação AAD/Entra ID em clusters AKS.
func (a *azureProvider) IsKubeloginAvailable(_ context.Context) bool {
	_, err := a.runner.LookPath("kubelogin")
	return err == nil
}
