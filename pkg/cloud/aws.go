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
		return &awsProvider{runner: runner}
	})
}

type awsProvider struct {
	runner shared.Runner
}

func (a *awsProvider) Name() string {
	return "aws"
}

// IsAvailable verifica se o CLI `aws` está instalado sem fazer chamadas de rede.
func (a *awsProvider) IsAvailable(_ context.Context) bool {
	_, err := a.runner.LookPath("aws")
	return err == nil
}

// CLIVersion retorna a versão do AWS CLI via `aws --version`.
func (a *awsProvider) CLIVersion(ctx context.Context) (string, error) {
	out, err := a.runner.RunCombinedOutput(ctx, "aws", "--version")
	if err != nil {
		return "", fmt.Errorf("aws --version: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// awsCallerIdentity representa a saída de `aws sts get-caller-identity --output json`.
type awsCallerIdentity struct {
	UserId  string `json:"UserId"`
	Account string `json:"Account"`
	Arn     string `json:"Arn"`
}

// ValidateCredentials verifica credenciais via `aws sts get-caller-identity`.
func (a *awsProvider) ValidateCredentials(ctx context.Context) (*CredentialStatus, error) {
	out, err := a.runner.RunCombinedOutput(ctx, "aws", "sts", "get-caller-identity", "--output", "json")
	if err != nil {
		return &CredentialStatus{Authenticated: false}, fmt.Errorf("aws sts get-caller-identity: %w", err)
	}

	var identity awsCallerIdentity
	if err := json.Unmarshal(out, &identity); err != nil {
		return &CredentialStatus{Authenticated: false}, fmt.Errorf("parse caller identity: %w", err)
	}

	return &CredentialStatus{
		Authenticated: true,
		Identity:      identity.Arn,
		Method:        "iam",
	}, nil
}

// awsClusterList representa a saída de `aws eks list-clusters --output json`.
type awsClusterList struct {
	Clusters []string `json:"clusters"`
}

// awsClusterDetail representa a saída de `aws eks describe-cluster --output json`.
type awsClusterDetail struct {
	Cluster struct {
		Name     string `json:"name"`
		Version  string `json:"version"`
		Status   string `json:"status"`
		Endpoint string `json:"endpoint"`
		Arn      string `json:"arn"`
	} `json:"cluster"`
}

// ListClusters lista clusters EKS via `aws eks list-clusters` + `describe-cluster` para cada.
func (a *awsProvider) ListClusters(ctx context.Context, opts ListOptions) ([]ClusterInfo, error) {
	args := []string{"eks", "list-clusters", "--output", "json"}
	if opts.Region != "" {
		args = append(args, "--region", opts.Region)
	}

	out, err := a.runner.RunCombinedOutput(ctx, "aws", args...)
	if err != nil {
		return nil, fmt.Errorf("aws eks list-clusters: %w", err)
	}

	var list awsClusterList
	if err := json.Unmarshal(out, &list); err != nil {
		return nil, fmt.Errorf("parse list-clusters: %w", err)
	}

	clusters := make([]ClusterInfo, 0, len(list.Clusters))
	for _, name := range list.Clusters {
		if name == "" {
			continue
		}
		info, descErr := a.describeCluster(ctx, name, opts.Region)
		if descErr != nil {
			// Fallback com dados mínimos para não bloquear listagem parcial
			clusters = append(clusters, ClusterInfo{
				Name:     name,
				Region:   opts.Region,
				Provider: "aws",
			})
			continue
		}
		clusters = append(clusters, *info)
	}
	return clusters, nil
}

// describeCluster busca detalhes de um cluster EKS via `aws eks describe-cluster`.
func (a *awsProvider) describeCluster(ctx context.Context, name, region string) (*ClusterInfo, error) {
	args := []string{"eks", "describe-cluster", "--name", name, "--output", "json"}
	if region != "" {
		args = append(args, "--region", region)
	}

	out, err := a.runner.RunCombinedOutput(ctx, "aws", args...)
	if err != nil {
		return nil, fmt.Errorf("aws eks describe-cluster %s: %w", name, err)
	}

	var desc awsClusterDetail
	if err := json.Unmarshal(out, &desc); err != nil {
		return nil, fmt.Errorf("parse describe-cluster %s: %w", name, err)
	}

	return &ClusterInfo{
		Name:     desc.Cluster.Name,
		Region:   region,
		Provider: "aws",
		Version:  desc.Cluster.Version,
		Status:   desc.Cluster.Status,
		Endpoint: desc.Cluster.Endpoint,
		ARN:      desc.Cluster.Arn,
	}, nil
}

// ConfigureKubeconfig configura o kubeconfig via `aws eks update-kubeconfig`.
func (a *awsProvider) ConfigureKubeconfig(ctx context.Context, cluster ClusterInfo) error {
	args := []string{"eks", "update-kubeconfig", "--name", cluster.Name}
	if cluster.Region != "" {
		args = append(args, "--region", cluster.Region)
	}
	if err := a.runner.Run(ctx, "aws", args...); err != nil {
		return fmt.Errorf("aws eks update-kubeconfig: %w", err)
	}
	return nil
}

// RefreshToken renova o token de autenticação via `aws eks get-token`.
func (a *awsProvider) RefreshToken(ctx context.Context, cluster ClusterInfo) error {
	args := []string{"eks", "get-token", "--cluster-name", cluster.Name}
	if cluster.Region != "" {
		args = append(args, "--region", cluster.Region)
	}
	if _, err := a.runner.RunCombinedOutput(ctx, "aws", args...); err != nil {
		return fmt.Errorf("aws eks get-token: %w", err)
	}
	return nil
}
