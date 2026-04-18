//go:build !gcp

package cloud

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/casheiro/yby-cli/pkg/services/shared"
)

// GCPAdvancedTokenGenerator gera tokens GCP com suporte a múltiplos métodos de autenticação.
// Stub para builds sem a tag `gcp` — usa gcloud CLI como fallback para todos os modos.
type GCPAdvancedTokenGenerator struct {
	Runner              shared.Runner
	ProjectID           string
	Zone                string
	ServiceAccountEmail string // Email da SA para impersonation
	CredentialsFile     string // Path para configuração de workload identity federation
}

// GenerateToken gera um token GCP via gcloud CLI.
// No modo stub, CredentialsFile é ignorado (requer SDK) e SA impersonation usa flag --impersonate-service-account.
func (g *GCPAdvancedTokenGenerator) GenerateToken(ctx context.Context) (*Token, error) {
	slog.Debug("usando fallback CLI para token GCP avançado (build sem tag gcp)")

	if g.CredentialsFile != "" {
		slog.Warn("credentials_file requer build com tag gcp; usando gcloud CLI como fallback")
	}

	args := []string{"auth", "print-access-token"}
	if g.ServiceAccountEmail != "" {
		args = append(args, "--impersonate-service-account", g.ServiceAccountEmail)
	}

	out, err := g.Runner.RunCombinedOutput(ctx, "gcloud", args...)
	if err != nil {
		return nil, fmt.Errorf("gcloud auth print-access-token: %w", err)
	}

	token := strings.TrimSpace(string(out))
	if token == "" {
		return nil, fmt.Errorf("gcloud auth print-access-token: token vazio")
	}

	return &Token{Value: token, ExpiresAt: time.Now().Add(1 * time.Hour)}, nil
}

// ConnectGateway configura o kubeconfig para acessar um cluster via GKE Connect Gateway.
// Usa o membership do cluster registrado no GKE Fleet.
func (g *GCPAdvancedTokenGenerator) ConnectGateway(ctx context.Context, membership string) error {
	slog.Debug("usando fallback CLI para Connect Gateway (build sem tag gcp)")

	args := []string{"container", "fleet", "memberships", "get-credentials", membership}
	if g.ProjectID != "" {
		args = append(args, "--project", g.ProjectID)
	}

	if err := g.Runner.Run(ctx, "gcloud", args...); err != nil {
		return fmt.Errorf("falha ao conectar via GKE Connect Gateway (membership=%s): %w", membership, err)
	}

	return nil
}
