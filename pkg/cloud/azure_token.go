//go:build azure

package cloud

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/casheiro/yby-cli/pkg/services/shared"
)

// aksScope é o scope de autenticação para o AKS.
const aksScope = "6dae42f8-4368-4678-94ff-3960e28e3630/.default"

// AzureTokenGenerator gera tokens AKS via Azure SDK (azidentity).
type AzureTokenGenerator struct {
	Runner shared.Runner
}

// GenerateToken gera um token Azure usando DefaultAzureCredential do SDK.
func (g *AzureTokenGenerator) GenerateToken(ctx context.Context) (*Token, error) {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("azure default credential: %w", err)
	}

	tok, err := cred.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: []string{aksScope},
	})
	if err != nil {
		return nil, fmt.Errorf("azure get token: %w", err)
	}

	return &Token{
		Value:     tok.Token,
		ExpiresAt: tok.ExpiresOn,
	}, nil
}

// azAccessToken não é usado na versão SDK mas mantém compatibilidade de tipo.
type azAccessToken struct {
	AccessToken string `json:"accessToken"`
	ExpiresOn   string `json:"expiresOn"`
}

// Garantir que implementa a interface em tempo de compilação.
var _ TokenGenerator = (*AzureTokenGenerator)(nil)

// Unused but needed for type compatibility.
func init() {
	_ = time.RFC3339
}
