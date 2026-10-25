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
// expires_on é o instante em epoch (UTC); expiresOn, a hora local, sem fuso.
type azAccessToken struct {
	AccessToken    string `json:"accessToken"`
	ExpiresOn      string `json:"expiresOn"`
	ExpiresOnEpoch int64  `json:"expires_on"`
}

// GenerateToken gera um token Azure executando `az account get-access-token` via CLI.
func (g *AzureTokenGenerator) GenerateToken(ctx context.Context) (*Token, error) {
	return tokenAKSPeloCLI(ctx, g.Runner)
}

// tokenAKSPeloCLI pede ao az um token para o servidor de API do AKS (sem --resource, o
// token sai para o Azure Resource Manager e o AKS com Entra ID o recusa).
func tokenAKSPeloCLI(ctx context.Context, runner shared.Runner) (*Token, error) {
	out, err := runner.RunCombinedOutput(ctx, "az", "account", "get-access-token", "--resource", aksServidor, "--output", "json")
	if err != nil {
		return nil, erroDoCLI("az account get-access-token", out, err)
	}

	var tok azAccessToken
	if err := json.Unmarshal(out, &tok); err != nil {
		return nil, fmt.Errorf("parse azure token: %w", err)
	}

	var expiresAt *time.Time
	if tok.ExpiresOnEpoch > 0 {
		t := time.Unix(tok.ExpiresOnEpoch, 0)
		expiresAt = &t
	} else {
		expiresAt = lerInstante(tok.ExpiresOn, time.Local)
	}
	if expiresAt == nil {
		return nil, fmt.Errorf("parse azure token expiresOn %q: formato desconhecido", tok.ExpiresOn)
	}

	return &Token{
		Value:     tok.AccessToken,
		ExpiresAt: *expiresAt,
	}, nil
}
