//go:build integration

package cloud

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/casheiro/yby-cli/pkg/services/shared"
	"github.com/casheiro/yby-cli/pkg/testutil"
)

// Testes de integração que rodam contra APIs reais (NÃO rodam no CI).
// Rodar manualmente: go test -tags integration ./pkg/cloud/... -v

func TestAWS_Integration_ValidateCredentials(t *testing.T) {
	if os.Getenv("AWS_ACCESS_KEY_ID") == "" {
		t.Skip("AWS credentials não configuradas (AWS_ACCESS_KEY_ID ausente)")
	}

	runner := &shared.RealRunner{}
	p := GetProvider(runner, "aws")
	if p == nil {
		t.Fatal("provider 'aws' não encontrado no registry")
	}

	if !p.IsAvailable(context.Background()) {
		t.Skip("AWS CLI não instalado")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	status, err := p.ValidateCredentials(ctx)
	if err != nil {
		t.Fatalf("ValidateCredentials() erro: %v", err)
	}

	if !status.Authenticated {
		t.Error("esperava Authenticated=true com credenciais válidas")
	}
	if status.Identity == "" {
		t.Error("Identity não deve estar vazio quando autenticado")
	}
	t.Logf("AWS autenticado como: %s (método: %s)", status.Identity, status.Method)
}

func TestAzure_Integration_ValidateCredentials(t *testing.T) {
	if os.Getenv("AZURE_TENANT_ID") == "" {
		t.Skip("Azure credentials não configuradas (AZURE_TENANT_ID ausente)")
	}

	runner := &shared.RealRunner{}
	p := GetProvider(runner, "azure")
	if p == nil {
		t.Fatal("provider 'azure' não encontrado no registry")
	}

	if !p.IsAvailable(context.Background()) {
		t.Skip("Azure CLI não instalado")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	status, err := p.ValidateCredentials(ctx)
	if err != nil {
		t.Fatalf("ValidateCredentials() erro: %v", err)
	}

	if !status.Authenticated {
		t.Error("esperava Authenticated=true com credenciais válidas")
	}
	if status.Identity == "" {
		t.Error("Identity não deve estar vazio quando autenticado")
	}
	t.Logf("Azure autenticado como: %s (método: %s)", status.Identity, status.Method)
}

func TestGCP_Integration_ValidateCredentials(t *testing.T) {
	if os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") == "" {
		t.Skip("GCP credentials não configuradas (GOOGLE_APPLICATION_CREDENTIALS ausente)")
	}

	runner := &shared.RealRunner{}
	p := GetProvider(runner, "gcp")
	if p == nil {
		t.Fatal("provider 'gcp' não encontrado no registry")
	}

	if !p.IsAvailable(context.Background()) {
		t.Skip("GCP CLI (gcloud) não instalado")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	status, err := p.ValidateCredentials(ctx)
	if err != nil {
		t.Fatalf("ValidateCredentials() erro: %v", err)
	}

	if !status.Authenticated {
		t.Error("esperava Authenticated=true com credenciais válidas")
	}
	if status.Identity == "" {
		t.Error("Identity não deve estar vazio quando autenticado")
	}
	t.Logf("GCP autenticado como: %s (método: %s)", status.Identity, status.Method)
}

// TestIntegration_DetectWithRealKubeconfig testa a detecção de providers
// usando o kubeconfig real da máquina.
func TestIntegration_DetectWithRealKubeconfig(t *testing.T) {
	// Pula se não houver kubeconfig configurado
	kubeconfigPath := os.Getenv("KUBECONFIG")
	if kubeconfigPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			t.Skip("não foi possível determinar home dir")
		}
		kubeconfigPath = home + "/.kube/config"
	}
	if _, err := os.Stat(kubeconfigPath); os.IsNotExist(err) {
		t.Skip("kubeconfig não encontrado: " + kubeconfigPath)
	}

	// Usa MockRunner que simula LookPath para não depender de CLIs instalados
	runner := &testutil.MockRunner{
		LookPathFunc: func(file string) (string, error) {
			// Aceita qualquer comando como "encontrado" para testar detecção
			return "/usr/local/bin/" + file, nil
		},
	}

	providers := Detect(context.Background(), runner)
	t.Logf("providers detectados no kubeconfig: %d", len(providers))
	for _, p := range providers {
		t.Logf("  - %s", p.Name())
	}
}
