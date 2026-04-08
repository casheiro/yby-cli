//go:build !azure

package cloud

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/casheiro/yby-cli/pkg/services/shared"
)

// AzureTokenGenerator gera tokens AKS via CLI `az account get-access-token`.
type AzureTokenGenerator struct {
	Runner shared.Runner
}

// azAccessToken representa a saída de `az account get-access-token --output json`.
type azAccessToken struct {
	AccessToken string `json:"accessToken"`
	ExpiresOn   string `json:"expiresOn"`
}

// GenerateToken gera um token Azure executando `az account get-access-token` via CLI.
func (g *AzureTokenGenerator) GenerateToken(ctx context.Context) (*Token, error) {
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
		// Fallback: tentar formato ISO 8601
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
