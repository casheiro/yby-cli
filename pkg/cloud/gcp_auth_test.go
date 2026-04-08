//go:build gcp

package cloud

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/casheiro/yby-cli/pkg/testutil"
)

func TestGCPAdvancedTokenGenerator_DefaultCredentials(t *testing.T) {
	// ADC requer ambiente configurado; verifica que o método é chamado sem erro de campo.
	// Em CI sem credenciais reais, o erro esperado é sobre token source.
	gen := &GCPAdvancedTokenGenerator{
		Runner: &testutil.MockRunner{},
	}

	_, err := gen.GenerateToken(context.Background())
	if err == nil {
		t.Log("ADC disponível no ambiente de teste")
		return
	}

	// Deve falhar com erro de default token source (não pânico ou outro erro).
	if !strings.Contains(err.Error(), "token source") && !strings.Contains(err.Error(), "token GCP") {
		t.Errorf("erro inesperado para ADC: %v", err)
	}
}

func TestGCPAdvancedTokenGenerator_WorkloadIdentityFederation(t *testing.T) {
	// Cria um credentials file temporário com JSON inválido para testar o fluxo.
	tmpDir := t.TempDir()
	credFile := filepath.Join(tmpDir, "wif-config.json")

	// JSON válido mas sem campos necessários para WIF — deve falhar na criação de credenciais.
	if err := os.WriteFile(credFile, []byte(`{"type": "external_account"}`), 0600); err != nil {
		t.Fatalf("falha ao criar arquivo de credenciais temporário: %v", err)
	}

	gen := &GCPAdvancedTokenGenerator{
		Runner:          &testutil.MockRunner{},
		CredentialsFile: credFile,
	}

	_, err := gen.GenerateToken(context.Background())
	if err == nil {
		t.Fatal("esperava erro para credentials file incompleto")
	}

	// Deve falhar ao criar credenciais, não ao ler o arquivo.
	if strings.Contains(err.Error(), "falha ao ler credentials file") {
		t.Errorf("erro não deveria ser de leitura do arquivo: %v", err)
	}
}

func TestGCPAdvancedTokenGenerator_WorkloadIdentityFederation_FileNotFound(t *testing.T) {
	gen := &GCPAdvancedTokenGenerator{
		Runner:          &testutil.MockRunner{},
		CredentialsFile: "/caminho/inexistente/wif.json",
	}

	_, err := gen.GenerateToken(context.Background())
	if err == nil {
		t.Fatal("esperava erro para arquivo inexistente")
	}

	if !strings.Contains(err.Error(), "falha ao ler credentials file") {
		t.Errorf("erro deveria ser de leitura do arquivo: %v", err)
	}
}

func TestGCPAdvancedTokenGenerator_SAImpersonation(t *testing.T) {
	fakeToken := "ya29.impersonated-token-abc123"
	sa := "my-sa@my-project.iam.gserviceaccount.com"

	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			if name != "gcloud" {
				t.Errorf("esperava gcloud, recebeu %s", name)
			}

			expectedArgs := []string{"auth", "print-access-token", "--impersonate-service-account", sa}
			if len(args) != len(expectedArgs) {
				t.Errorf("args inesperados: %v", args)
			}
			for i, a := range expectedArgs {
				if i < len(args) && args[i] != a {
					t.Errorf("arg[%d]: esperava %q, recebeu %q", i, a, args[i])
				}
			}

			return []byte(fakeToken + "\n"), nil
		},
	}

	gen := &GCPAdvancedTokenGenerator{
		Runner:              runner,
		ServiceAccountEmail: sa,
	}

	tok, err := gen.GenerateToken(context.Background())
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	if tok.Value != fakeToken {
		t.Errorf("token: esperava %q, recebeu %q", fakeToken, tok.Value)
	}

	if tok.ExpiresAt.IsZero() {
		t.Error("ExpiresAt não deveria ser zero")
	}
}

func TestGCPAdvancedTokenGenerator_SAImpersonation_EmptyToken(t *testing.T) {
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return []byte("  \n"), nil
		},
	}

	gen := &GCPAdvancedTokenGenerator{
		Runner:              runner,
		ServiceAccountEmail: "sa@project.iam.gserviceaccount.com",
	}

	_, err := gen.GenerateToken(context.Background())
	if err == nil {
		t.Fatal("esperava erro para token vazio")
	}

	if !strings.Contains(err.Error(), "token vazio") {
		t.Errorf("erro deveria mencionar token vazio: %v", err)
	}
}

func TestGCPAdvancedTokenGenerator_ConnectGateway(t *testing.T) {
	membership := "my-cluster-membership"
	projectID := "my-gcp-project"

	var capturedArgs []string
	runner := &testutil.MockRunner{
		RunFunc: func(_ context.Context, name string, args ...string) error {
			capturedArgs = append([]string{name}, args...)
			return nil
		},
	}

	gen := &GCPAdvancedTokenGenerator{
		Runner:    runner,
		ProjectID: projectID,
	}

	err := gen.ConnectGateway(context.Background(), membership)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	expected := []string{"gcloud", "container", "fleet", "memberships", "get-credentials", membership, "--project", projectID}
	if len(capturedArgs) != len(expected) {
		t.Fatalf("args: esperava %v, recebeu %v", expected, capturedArgs)
	}
	for i, e := range expected {
		if capturedArgs[i] != e {
			t.Errorf("arg[%d]: esperava %q, recebeu %q", i, e, capturedArgs[i])
		}
	}
}

func TestGCPAdvancedTokenGenerator_ConnectGateway_SemProjeto(t *testing.T) {
	membership := "my-membership"

	var capturedArgs []string
	runner := &testutil.MockRunner{
		RunFunc: func(_ context.Context, name string, args ...string) error {
			capturedArgs = append([]string{name}, args...)
			return nil
		},
	}

	gen := &GCPAdvancedTokenGenerator{
		Runner: runner,
	}

	err := gen.ConnectGateway(context.Background(), membership)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	// Sem ProjectID, não deve ter --project
	expected := []string{"gcloud", "container", "fleet", "memberships", "get-credentials", membership}
	if len(capturedArgs) != len(expected) {
		t.Fatalf("args: esperava %v, recebeu %v", expected, capturedArgs)
	}
}
