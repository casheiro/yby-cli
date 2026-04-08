package cloud

import (
	"context"
	"errors"
	"testing"

	"github.com/casheiro/yby-cli/pkg/testutil"
)

func TestGCPProvider_Name(t *testing.T) {
	p := &gcpProvider{runner: &testutil.MockRunner{}}
	if p.Name() != "gcp" {
		t.Errorf("Name() = %q, want %q", p.Name(), "gcp")
	}
}

func TestGCPProvider_IsAvailable_Found(t *testing.T) {
	runner := &testutil.MockRunner{
		LookPathFunc: func(file string) (string, error) {
			if file == "gcloud" {
				return "/usr/bin/gcloud", nil
			}
			return "", errors.New("não encontrado")
		},
	}
	p := &gcpProvider{runner: runner}
	if !p.IsAvailable(context.Background()) {
		t.Error("IsAvailable() = false, want true")
	}
}

func TestGCPProvider_IsAvailable_NotFound(t *testing.T) {
	runner := &testutil.MockRunner{
		LookPathFunc: func(file string) (string, error) {
			return "", errors.New("não encontrado")
		},
	}
	p := &gcpProvider{runner: runner}
	if p.IsAvailable(context.Background()) {
		t.Error("IsAvailable() = true, want false")
	}
}

func TestGCPProvider_CLIVersion(t *testing.T) {
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return []byte("Google Cloud SDK 461.0.0\nbq 2.0.101\ncore 2024.02.09\n"), nil
		},
	}
	p := &gcpProvider{runner: runner}
	version, err := p.CLIVersion(context.Background())
	if err != nil {
		t.Fatalf("CLIVersion() error = %v, want nil", err)
	}
	if version == "" {
		t.Error("CLIVersion() retornou string vazia")
	}
}

func TestGCPProvider_CLIVersion_Error(t *testing.T) {
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return nil, errors.New("comando não encontrado")
		},
	}
	p := &gcpProvider{runner: runner}
	_, err := p.CLIVersion(context.Background())
	if err == nil {
		t.Error("CLIVersion() error = nil, want non-nil")
	}
}

func TestGCPProvider_ValidateCredentials_Active(t *testing.T) {
	const authJSON = `[{"account":"alice@example.com","status":"ACTIVE"},{"account":"bob@example.com","status":""}]`
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return []byte(authJSON), nil
		},
	}
	p := &gcpProvider{runner: runner}
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
	if status.Method != "gcloud-cli" {
		t.Errorf("Method = %q, want %q", status.Method, "gcloud-cli")
	}
}

func TestGCPProvider_ValidateCredentials_NoActive(t *testing.T) {
	const authJSON = `[{"account":"alice@example.com","status":""}]`
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return []byte(authJSON), nil
		},
	}
	p := &gcpProvider{runner: runner}
	status, err := p.ValidateCredentials(context.Background())
	if err != nil {
		t.Fatalf("ValidateCredentials() error = %v, want nil", err)
	}
	if status.Authenticated {
		t.Error("Authenticated = true, want false (nenhuma conta ativa)")
	}
}

func TestGCPProvider_ValidateCredentials_Empty(t *testing.T) {
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return []byte(`[]`), nil
		},
	}
	p := &gcpProvider{runner: runner}
	status, err := p.ValidateCredentials(context.Background())
	if err != nil {
		t.Fatalf("ValidateCredentials() error = %v, want nil", err)
	}
	if status.Authenticated {
		t.Error("Authenticated = true, want false (lista vazia)")
	}
}

func TestGCPProvider_ValidateCredentials_CLIError(t *testing.T) {
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return nil, errors.New("não autenticado")
		},
	}
	p := &gcpProvider{runner: runner}
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

func TestGCPProvider_ValidateCredentials_InvalidJSON(t *testing.T) {
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return []byte("não é json"), nil
		},
	}
	p := &gcpProvider{runner: runner}
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

func TestGCPProvider_ListClusters(t *testing.T) {
	const listJSON = `[
		{"name":"prod-gke","location":"us-central1","currentMasterVersion":"1.29.1-gke.1000","status":"RUNNING"},
		{"name":"staging-gke","location":"us-east1","currentMasterVersion":"1.28.5-gke.1000","status":"RUNNING"}
	]`
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return []byte(listJSON), nil
		},
	}
	p := &gcpProvider{runner: runner}
	clusters, err := p.ListClusters(context.Background(), ListOptions{Project: "my-project"})
	if err != nil {
		t.Fatalf("ListClusters() error = %v, want nil", err)
	}
	if len(clusters) != 2 {
		t.Fatalf("ListClusters() len = %d, want 2", len(clusters))
	}
	if clusters[0].Provider != "gcp" {
		t.Errorf("clusters[0].Provider = %q, want %q", clusters[0].Provider, "gcp")
	}
	if clusters[0].Region != "us-central1" {
		t.Errorf("clusters[0].Region = %q, want %q", clusters[0].Region, "us-central1")
	}
	if clusters[0].Version != "1.29.1-gke.1000" {
		t.Errorf("clusters[0].Version = %q, want %q", clusters[0].Version, "1.29.1-gke.1000")
	}
	if clusters[0].Status != "RUNNING" {
		t.Errorf("clusters[0].Status = %q, want %q", clusters[0].Status, "RUNNING")
	}
	if clusters[0].ProjectID != "my-project" {
		t.Errorf("clusters[0].ProjectID = %q, want %q", clusters[0].ProjectID, "my-project")
	}
}

func TestGCPProvider_ListClusters_WithRegion(t *testing.T) {
	var capturedArgs []string
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, _ string, args ...string) ([]byte, error) {
			capturedArgs = args
			return []byte(`[]`), nil
		},
	}
	p := &gcpProvider{runner: runner}
	_, err := p.ListClusters(context.Background(), ListOptions{Project: "proj", Region: "us-central1"})
	if err != nil {
		t.Fatalf("ListClusters() error = %v, want nil", err)
	}
	// Verifica --region us-central1
	regionFound := false
	for i, arg := range capturedArgs {
		if arg == "--region" && i+1 < len(capturedArgs) && capturedArgs[i+1] == "us-central1" {
			regionFound = true
		}
	}
	if !regionFound {
		t.Errorf("--region us-central1 não encontrado nos args: %v", capturedArgs)
	}
	// Verifica --project proj
	projectFound := false
	for i, arg := range capturedArgs {
		if arg == "--project" && i+1 < len(capturedArgs) && capturedArgs[i+1] == "proj" {
			projectFound = true
		}
	}
	if !projectFound {
		t.Errorf("--project proj não encontrado nos args: %v", capturedArgs)
	}
}

func TestGCPProvider_ListClusters_NoProjectNoRegion(t *testing.T) {
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, _ string, args ...string) ([]byte, error) {
			// Verifica que --project e --region não estão presentes
			for _, arg := range args {
				if arg == "--project" || arg == "--region" {
					return nil, errors.New("flag inesperada: " + arg)
				}
			}
			return []byte(`[]`), nil
		},
	}
	p := &gcpProvider{runner: runner}
	_, err := p.ListClusters(context.Background(), ListOptions{})
	if err != nil {
		t.Fatalf("ListClusters() sem projeto/região error = %v, want nil", err)
	}
}

func TestGCPProvider_ListClusters_Empty(t *testing.T) {
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return []byte(`[]`), nil
		},
	}
	p := &gcpProvider{runner: runner}
	clusters, err := p.ListClusters(context.Background(), ListOptions{})
	if err != nil {
		t.Fatalf("ListClusters() error = %v, want nil", err)
	}
	if len(clusters) != 0 {
		t.Errorf("ListClusters() len = %d, want 0", len(clusters))
	}
}

func TestGCPProvider_ListClusters_CLIError(t *testing.T) {
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return nil, errors.New("erro de rede")
		},
	}
	p := &gcpProvider{runner: runner}
	_, err := p.ListClusters(context.Background(), ListOptions{})
	if err == nil {
		t.Error("ListClusters() error = nil, want non-nil")
	}
}

func TestGCPProvider_ConfigureKubeconfig(t *testing.T) {
	var capturedArgs []string
	runner := &testutil.MockRunner{
		RunFunc: func(_ context.Context, _ string, args ...string) error {
			capturedArgs = args
			return nil
		},
	}
	p := &gcpProvider{runner: runner}
	err := p.ConfigureKubeconfig(context.Background(), ClusterInfo{
		Name:      "prod-gke",
		Region:    "us-central1",
		ProjectID: "my-project",
	})
	if err != nil {
		t.Fatalf("ConfigureKubeconfig() error = %v, want nil", err)
	}
	// Verifica nome do cluster nos args
	nameFound := false
	for _, arg := range capturedArgs {
		if arg == "prod-gke" {
			nameFound = true
		}
	}
	if !nameFound {
		t.Errorf("prod-gke não encontrado nos args: %v", capturedArgs)
	}
	// Verifica --region us-central1
	regionFound := false
	for i, arg := range capturedArgs {
		if arg == "--region" && i+1 < len(capturedArgs) && capturedArgs[i+1] == "us-central1" {
			regionFound = true
		}
	}
	if !regionFound {
		t.Errorf("--region us-central1 não encontrado nos args: %v", capturedArgs)
	}
	// Verifica --project my-project
	projectFound := false
	for i, arg := range capturedArgs {
		if arg == "--project" && i+1 < len(capturedArgs) && capturedArgs[i+1] == "my-project" {
			projectFound = true
		}
	}
	if !projectFound {
		t.Errorf("--project my-project não encontrado nos args: %v", capturedArgs)
	}
}

func TestGCPProvider_ConfigureKubeconfig_Error(t *testing.T) {
	runner := &testutil.MockRunner{
		RunFunc: func(_ context.Context, _ string, _ ...string) error {
			return errors.New("acesso negado")
		},
	}
	p := &gcpProvider{runner: runner}
	err := p.ConfigureKubeconfig(context.Background(), ClusterInfo{Name: "prod-gke", Region: "us-central1"})
	if err == nil {
		t.Error("ConfigureKubeconfig() error = nil, want non-nil")
	}
}

func TestGCPProvider_RefreshToken(t *testing.T) {
	var capturedCmd string
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, cmd string, _ ...string) ([]byte, error) {
			capturedCmd = cmd
			return []byte("ya29.a0...token"), nil
		},
	}
	p := &gcpProvider{runner: runner}
	err := p.RefreshToken(context.Background(), ClusterInfo{Name: "prod-gke"})
	if err != nil {
		t.Fatalf("RefreshToken() error = %v, want nil", err)
	}
	if capturedCmd != "gcloud" {
		t.Errorf("cmd = %q, want %q", capturedCmd, "gcloud")
	}
}

func TestGCPProvider_RefreshToken_Error(t *testing.T) {
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return nil, errors.New("token expirado")
		},
	}
	p := &gcpProvider{runner: runner}
	err := p.RefreshToken(context.Background(), ClusterInfo{Name: "prod-gke"})
	if err == nil {
		t.Error("RefreshToken() error = nil, want non-nil")
	}
}

func TestGCPProvider_RegisteredInRegistry(t *testing.T) {
	found := false
	for _, factory := range providerRegistry {
		p := factory(&testutil.MockRunner{})
		if p.Name() == "gcp" {
			found = true
			break
		}
	}
	if !found {
		t.Error("provider 'gcp' não encontrado no providerRegistry após init()")
	}
}
