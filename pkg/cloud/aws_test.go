package cloud

import (
	"context"
	"errors"
	"testing"

	"github.com/casheiro/yby-cli/pkg/testutil"
)

func TestAWSProvider_Name(t *testing.T) {
	p := &awsProvider{runner: &testutil.MockRunner{}}
	if p.Name() != "aws" {
		t.Errorf("Name() = %q, want %q", p.Name(), "aws")
	}
}

func TestAWSProvider_IsAvailable_Found(t *testing.T) {
	runner := &testutil.MockRunner{
		LookPathFunc: func(file string) (string, error) {
			if file == "aws" {
				return "/usr/bin/aws", nil
			}
			return "", errors.New("not found")
		},
	}
	p := &awsProvider{runner: runner}
	if !p.IsAvailable(context.Background()) {
		t.Error("IsAvailable() = false, want true")
	}
}

func TestAWSProvider_IsAvailable_NotFound(t *testing.T) {
	runner := &testutil.MockRunner{
		LookPathFunc: func(file string) (string, error) {
			return "", errors.New("not found")
		},
	}
	p := &awsProvider{runner: runner}
	if p.IsAvailable(context.Background()) {
		t.Error("IsAvailable() = true, want false")
	}
}

func TestAWSProvider_CLIVersion(t *testing.T) {
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return []byte("aws-cli/2.15.0 Python/3.11.6 Linux/6.1.0 botocore/2.x\n"), nil
		},
	}
	p := &awsProvider{runner: runner}
	version, err := p.CLIVersion(context.Background())
	if err != nil {
		t.Fatalf("CLIVersion() error = %v, want nil", err)
	}
	if version == "" {
		t.Error("CLIVersion() retornou string vazia")
	}
}

func TestAWSProvider_CLIVersion_Error(t *testing.T) {
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return nil, errors.New("comando não encontrado")
		},
	}
	p := &awsProvider{runner: runner}
	_, err := p.CLIVersion(context.Background())
	if err == nil {
		t.Error("CLIVersion() error = nil, want non-nil")
	}
}

func TestAWSProvider_ValidateCredentials_Valid(t *testing.T) {
	const callerJSON = `{"UserId":"AIDAI0123456789EXAMPLE","Account":"123456789012","Arn":"arn:aws:iam::123456789012:user/Alice"}`
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return []byte(callerJSON), nil
		},
	}
	p := &awsProvider{runner: runner}
	status, err := p.ValidateCredentials(context.Background())
	if err != nil {
		t.Fatalf("ValidateCredentials() error = %v, want nil", err)
	}
	if !status.Authenticated {
		t.Error("Authenticated = false, want true")
	}
	if status.Identity != "arn:aws:iam::123456789012:user/Alice" {
		t.Errorf("Identity = %q, want ARN", status.Identity)
	}
	if status.Method != "iam" {
		t.Errorf("Method = %q, want %q", status.Method, "iam")
	}
}

func TestAWSProvider_ValidateCredentials_CLIError(t *testing.T) {
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return nil, errors.New("credenciais inválidas")
		},
	}
	p := &awsProvider{runner: runner}
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

func TestAWSProvider_ValidateCredentials_InvalidJSON(t *testing.T) {
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return []byte("não é json"), nil
		},
	}
	p := &awsProvider{runner: runner}
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

func TestAWSProvider_ListClusters(t *testing.T) {
	callCount := 0
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, _ string, args ...string) ([]byte, error) {
			callCount++
			if callCount == 1 {
				return []byte(`{"clusters":["prod-cluster","staging-cluster"]}`), nil
			}
			// describe-cluster: extrai o nome do cluster dos args
			clusterName := ""
			for i, arg := range args {
				if arg == "--name" && i+1 < len(args) {
					clusterName = args[i+1]
				}
			}
			return []byte(`{"cluster":{"name":"` + clusterName + `","version":"1.29","status":"ACTIVE","endpoint":"https://` + clusterName + `.eks.amazonaws.com","arn":"arn:aws:eks:us-east-1:123456789012:cluster/` + clusterName + `"}}`), nil
		},
	}
	p := &awsProvider{runner: runner}
	clusters, err := p.ListClusters(context.Background(), ListOptions{Region: "us-east-1"})
	if err != nil {
		t.Fatalf("ListClusters() error = %v, want nil", err)
	}
	if len(clusters) != 2 {
		t.Fatalf("ListClusters() len = %d, want 2", len(clusters))
	}
	if clusters[0].Provider != "aws" {
		t.Errorf("clusters[0].Provider = %q, want %q", clusters[0].Provider, "aws")
	}
	if clusters[0].Region != "us-east-1" {
		t.Errorf("clusters[0].Region = %q, want %q", clusters[0].Region, "us-east-1")
	}
	if clusters[0].Version != "1.29" {
		t.Errorf("clusters[0].Version = %q, want %q", clusters[0].Version, "1.29")
	}
	if clusters[0].Status != "ACTIVE" {
		t.Errorf("clusters[0].Status = %q, want %q", clusters[0].Status, "ACTIVE")
	}
	if clusters[0].Endpoint != "https://prod-cluster.eks.amazonaws.com" {
		t.Errorf("clusters[0].Endpoint = %q, want %q", clusters[0].Endpoint, "https://prod-cluster.eks.amazonaws.com")
	}
	if clusters[0].ARN != "arn:aws:eks:us-east-1:123456789012:cluster/prod-cluster" {
		t.Errorf("clusters[0].ARN = %q, want %q", clusters[0].ARN, "arn:aws:eks:us-east-1:123456789012:cluster/prod-cluster")
	}
}

func TestAWSProvider_ListClusters_Empty(t *testing.T) {
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return []byte(`{"clusters":[]}`), nil
		},
	}
	p := &awsProvider{runner: runner}
	clusters, err := p.ListClusters(context.Background(), ListOptions{Region: "us-east-1"})
	if err != nil {
		t.Fatalf("ListClusters() error = %v, want nil", err)
	}
	if len(clusters) != 0 {
		t.Errorf("ListClusters() len = %d, want 0", len(clusters))
	}
}

func TestAWSProvider_ListClusters_CLIError(t *testing.T) {
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return nil, errors.New("erro de rede")
		},
	}
	p := &awsProvider{runner: runner}
	_, err := p.ListClusters(context.Background(), ListOptions{Region: "us-east-1"})
	if err == nil {
		t.Error("ListClusters() error = nil, want non-nil")
	}
}

func TestAWSProvider_ListClusters_DescribeError_FallsBack(t *testing.T) {
	callCount := 0
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			callCount++
			if callCount == 1 {
				return []byte(`{"clusters":["my-cluster"]}`), nil
			}
			return nil, errors.New("erro no describe")
		},
	}
	p := &awsProvider{runner: runner}
	clusters, err := p.ListClusters(context.Background(), ListOptions{Region: "us-east-1"})
	if err != nil {
		t.Fatalf("ListClusters() error = %v, want nil", err)
	}
	if len(clusters) != 1 {
		t.Fatalf("ListClusters() len = %d, want 1 (fallback)", len(clusters))
	}
	if clusters[0].Name != "my-cluster" {
		t.Errorf("clusters[0].Name = %q, want %q", clusters[0].Name, "my-cluster")
	}
}

func TestAWSProvider_ListClusters_NoRegion(t *testing.T) {
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, _ string, args ...string) ([]byte, error) {
			// Verifica que --region não está presente quando opts.Region está vazio
			for _, arg := range args {
				if arg == "--region" {
					return nil, errors.New("--region não deveria estar presente")
				}
			}
			return []byte(`{"clusters":[]}`), nil
		},
	}
	p := &awsProvider{runner: runner}
	_, err := p.ListClusters(context.Background(), ListOptions{})
	if err != nil {
		t.Fatalf("ListClusters() sem região error = %v, want nil", err)
	}
}

func TestAWSProvider_ConfigureKubeconfig(t *testing.T) {
	var capturedArgs []string
	runner := &testutil.MockRunner{
		RunFunc: func(_ context.Context, _ string, args ...string) error {
			capturedArgs = args
			return nil
		},
	}
	p := &awsProvider{runner: runner}
	err := p.ConfigureKubeconfig(context.Background(), ClusterInfo{Name: "prod", Region: "us-east-1"})
	if err != nil {
		t.Fatalf("ConfigureKubeconfig() error = %v, want nil", err)
	}
	// Verifica que --name prod está nos args
	nameFound := false
	for i, arg := range capturedArgs {
		if arg == "--name" && i+1 < len(capturedArgs) && capturedArgs[i+1] == "prod" {
			nameFound = true
		}
	}
	if !nameFound {
		t.Errorf("--name prod não encontrado nos args: %v", capturedArgs)
	}
	// Verifica que --region us-east-1 está nos args
	regionFound := false
	for i, arg := range capturedArgs {
		if arg == "--region" && i+1 < len(capturedArgs) && capturedArgs[i+1] == "us-east-1" {
			regionFound = true
		}
	}
	if !regionFound {
		t.Errorf("--region us-east-1 não encontrado nos args: %v", capturedArgs)
	}
}

func TestAWSProvider_ConfigureKubeconfig_Error(t *testing.T) {
	runner := &testutil.MockRunner{
		RunFunc: func(_ context.Context, _ string, _ ...string) error {
			return errors.New("acesso negado")
		},
	}
	p := &awsProvider{runner: runner}
	err := p.ConfigureKubeconfig(context.Background(), ClusterInfo{Name: "prod", Region: "us-east-1"})
	if err == nil {
		t.Error("ConfigureKubeconfig() error = nil, want non-nil")
	}
}

func TestAWSProvider_RefreshToken(t *testing.T) {
	var capturedArgs []string
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, _ string, args ...string) ([]byte, error) {
			capturedArgs = args
			return []byte(`{"kind":"ExecCredential","apiVersion":"client.authentication.k8s.io/v1beta1"}`), nil
		},
	}
	p := &awsProvider{runner: runner}
	err := p.RefreshToken(context.Background(), ClusterInfo{Name: "prod", Region: "us-east-1"})
	if err != nil {
		t.Fatalf("RefreshToken() error = %v, want nil", err)
	}
	// Verifica que --cluster-name prod está nos args
	found := false
	for i, arg := range capturedArgs {
		if arg == "--cluster-name" && i+1 < len(capturedArgs) && capturedArgs[i+1] == "prod" {
			found = true
		}
	}
	if !found {
		t.Errorf("--cluster-name prod não encontrado nos args: %v", capturedArgs)
	}
}

func TestAWSProvider_RefreshToken_Error(t *testing.T) {
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return nil, errors.New("token expirado")
		},
	}
	p := &awsProvider{runner: runner}
	err := p.RefreshToken(context.Background(), ClusterInfo{Name: "prod", Region: "us-east-1"})
	if err == nil {
		t.Error("RefreshToken() error = nil, want non-nil")
	}
}

func TestAWSProvider_RegisteredInRegistry(t *testing.T) {
	found := false
	for _, factory := range providerRegistry {
		p := factory(&testutil.MockRunner{})
		if p.Name() == "aws" {
			found = true
			break
		}
	}
	if !found {
		t.Error("provider 'aws' não encontrado no providerRegistry após init()")
	}
}
