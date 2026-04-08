//go:build !azure

package cloud

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

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

	out, err := g.Runner.RunCombinedOutput(ctx, "az", "account", "get-access-token", "--output", "json")
	if err != nil {
		return nil, fmt.Errorf("az account get-access-token: %w", err)
	}

	var tok azAccessToken
	if err := json.Unmarshal(out, &tok); err != nil {
		return nil, fmt.Errorf("parse azure token: %w", err)
	}

	expiresAt, err := time.Parse("2006-01-02 15:04:05.999999", tok.ExpiresOn)
	if err != nil {
		expiresAt, err = time.Parse(time.RFC3339, tok.ExpiresOn)
		if err != nil {
			return nil, fmt.Errorf("parse azure token expiresOn %q: %w", tok.ExpiresOn, err)
		}
	}

	return &Token{
		Value:     tok.AccessToken,
		ExpiresAt: expiresAt,
	}, nil
}
