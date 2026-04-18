//go:build !gcp

package cloud

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/casheiro/yby-cli/pkg/services/shared"
)

// GCPTokenGenerator gera tokens GCP via CLI `gcloud auth print-access-token`.
type GCPTokenGenerator struct {
	Runner shared.Runner
}

// GenerateToken gera um token GCP executando `gcloud auth print-access-token` via CLI.
// Como o comando não retorna expiração explícita, usa TTL de 1 hora.
func (g *GCPTokenGenerator) GenerateToken(ctx context.Context) (*Token, error) {
	out, err := g.Runner.RunCombinedOutput(ctx, "gcloud", "auth", "print-access-token")
	if err != nil {
		return nil, fmt.Errorf("gcloud auth print-access-token: %w", err)
	}

	token := strings.TrimSpace(string(out))
	if token == "" {
		return nil, fmt.Errorf("gcloud auth print-access-token: token vazio")
	}

	return &Token{
		Value:     token,
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}, nil
}
