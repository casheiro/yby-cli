//go:build gcp

package cloud

import (
	"context"
	"fmt"

	"github.com/casheiro/yby-cli/pkg/services/shared"
	"golang.org/x/oauth2/google"
)

// gcpCloudPlatformScope é o scope de autenticação para GCP cloud-platform.
const gcpCloudPlatformScope = "https://www.googleapis.com/auth/cloud-platform"

// GCPTokenGenerator gera tokens GCP via google.DefaultTokenSource do SDK.
type GCPTokenGenerator struct {
	Runner shared.Runner
}

// GenerateToken gera um token GCP usando Application Default Credentials.
func (g *GCPTokenGenerator) GenerateToken(ctx context.Context) (*Token, error) {
	src, err := google.DefaultTokenSource(ctx, gcpCloudPlatformScope)
	if err != nil {
		return nil, fmt.Errorf("gcp default token source: %w", err)
	}

	tok, err := src.Token()
	if err != nil {
		return nil, fmt.Errorf("gcp get token: %w", err)
	}

	return &Token{
		Value:     tok.AccessToken,
		ExpiresAt: tok.Expiry,
	}, nil
}

// Garantir que implementa a interface em tempo de compilação.
var _ TokenGenerator = (*GCPTokenGenerator)(nil)
