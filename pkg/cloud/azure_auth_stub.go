//go:build !azure

package cloud

import (
	"context"
	"log/slog"

	"github.com/casheiro/yby-cli/pkg/services/shared"
)

// AzureAdvancedTokenGenerator gera tokens AKS via CLI `az account get-access-token` (fallback sem SDK).
type AzureAdvancedTokenGenerator struct {
	Runner    shared.Runner
	LoginMode string
	TenantID  string
	ClientID  string
}

// GenerateToken gera um token Azure via CLI como fallback (build sem tag azure).
func (g *AzureAdvancedTokenGenerator) GenerateToken(ctx context.Context) (*Token, error) {
	slog.Debug("usando fallback CLI para token Azure avançado (build sem tag azure)", "login_mode", g.LoginMode)

	return tokenAKSPeloCLI(ctx, g.Runner)
}
