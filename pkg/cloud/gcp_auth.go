//go:build gcp

package cloud

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/casheiro/yby-cli/pkg/services/shared"
	"golang.org/x/oauth2/google"
)

// GCPAdvancedTokenGenerator gera tokens GCP com suporte a múltiplos métodos de autenticação:
// Workload Identity Federation, Service Account Impersonation e Application Default Credentials.
type GCPAdvancedTokenGenerator struct {
	Runner              shared.Runner
	ProjectID           string
	Zone                string
	ServiceAccountEmail string // Email da SA para impersonation
	CredentialsFile     string // Path para configuração de workload identity federation
}

// gcpCloudScope é o scope padrão para operações GCP cloud-platform.
const gcpCloudScope = "https://www.googleapis.com/auth/cloud-platform"

// GenerateToken gera um token GCP usando o método de autenticação mais específico disponível.
// Ordem de prioridade: CredentialsFile (WIF) > ServiceAccountEmail (impersonation) > ADC (default).
func (g *GCPAdvancedTokenGenerator) GenerateToken(ctx context.Context) (*Token, error) {
	// 1. Workload Identity Federation via credentials file
	if g.CredentialsFile != "" {
		return g.generateFromCredentialsFile(ctx)
	}

	// 2. Service Account Impersonation via gcloud CLI
	if g.ServiceAccountEmail != "" {
		return g.generateWithImpersonation(ctx)
	}

	// 3. Application Default Credentials (SDK)
	return g.generateFromADC(ctx)
}

// generateFromCredentialsFile gera token a partir de um arquivo de credenciais (Workload Identity Federation).
func (g *GCPAdvancedTokenGenerator) generateFromCredentialsFile(ctx context.Context) (*Token, error) {
	data, err := os.ReadFile(g.CredentialsFile)
	if err != nil {
		return nil, fmt.Errorf("falha ao ler credentials file: %w", err)
	}

	creds, err := google.CredentialsFromJSON(ctx, data, gcpCloudScope)
	if err != nil {
		return nil, fmt.Errorf("falha ao criar credenciais GCP: %w", err)
	}

	tok, err := creds.TokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("falha ao obter token GCP: %w", err)
	}

	return &Token{Value: tok.AccessToken, ExpiresAt: tok.Expiry}, nil
}

// generateWithImpersonation gera token via SA impersonation usando gcloud CLI.
func (g *GCPAdvancedTokenGenerator) generateWithImpersonation(ctx context.Context) (*Token, error) {
	out, err := g.Runner.RunCombinedOutput(ctx, "gcloud", "auth", "print-access-token",
		"--impersonate-service-account", g.ServiceAccountEmail)
	if err != nil {
		return nil, fmt.Errorf("falha ao impersonar SA %s: %w", g.ServiceAccountEmail, err)
	}

	token := strings.TrimSpace(string(out))
	if token == "" {
		return nil, fmt.Errorf("token vazio ao impersonar SA %s", g.ServiceAccountEmail)
	}

	return &Token{Value: token, ExpiresAt: time.Now().Add(1 * time.Hour)}, nil
}

// generateFromADC gera token via Application Default Credentials do SDK.
func (g *GCPAdvancedTokenGenerator) generateFromADC(ctx context.Context) (*Token, error) {
	src, err := google.DefaultTokenSource(ctx, gcpCloudScope)
	if err != nil {
		return nil, fmt.Errorf("falha ao obter default token source GCP: %w", err)
	}

	tok, err := src.Token()
	if err != nil {
		return nil, fmt.Errorf("falha ao obter token GCP: %w", err)
	}

	return &Token{Value: tok.AccessToken, ExpiresAt: tok.Expiry}, nil
}

// ConnectGateway configura o kubeconfig para acessar um cluster via GKE Connect Gateway.
// Usa o membership do cluster registrado no GKE Fleet.
func (g *GCPAdvancedTokenGenerator) ConnectGateway(ctx context.Context, membership string) error {
	args := []string{"container", "fleet", "memberships", "get-credentials", membership}
	if g.ProjectID != "" {
		args = append(args, "--project", g.ProjectID)
	}

	if err := g.Runner.Run(ctx, "gcloud", args...); err != nil {
		return fmt.Errorf("falha ao conectar via GKE Connect Gateway (membership=%s): %w", membership, err)
	}

	return nil
}

// Garantir que implementa a interface em tempo de compilação.
var _ TokenGenerator = (*GCPAdvancedTokenGenerator)(nil)
