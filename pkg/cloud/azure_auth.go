//go:build azure

package cloud

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/casheiro/yby-cli/pkg/services/shared"
)

// azureCredential abstrai azcore.TokenCredential para testabilidade.
type azureCredential interface {
	GetToken(ctx context.Context, options policy.TokenRequestOptions) (azcore.AccessToken, error)
}

// credentialFactory cria credenciais Azure. Substituível em testes.
type credentialFactory func(loginMode, tenantID, clientID string) (azureCredential, error)

// defaultCredentialFactory cria credenciais reais via azidentity SDK.
func defaultCredentialFactory(loginMode, tenantID, clientID string) (azureCredential, error) {
	switch loginMode {
	case "azurecli":
		return azidentity.NewAzureCLICredential(nil)
	case "spn":
		secret := os.Getenv("AZURE_CLIENT_SECRET")
		if secret == "" {
			return nil, fmt.Errorf("AZURE_CLIENT_SECRET não definida para login_mode 'spn'")
		}
		return azidentity.NewClientSecretCredential(tenantID, clientID, secret, nil)
	case "certificate":
		certPath := os.Getenv("AZURE_CLIENT_CERTIFICATE_PATH")
		if certPath == "" {
			return nil, fmt.Errorf("AZURE_CLIENT_CERTIFICATE_PATH não definida para login_mode 'certificate'")
		}
		certData, err := os.ReadFile(certPath)
		if err != nil {
			return nil, fmt.Errorf("falha ao ler certificado %q: %w", certPath, err)
		}
		certs, key, err := azidentity.ParseCertificates(certData, nil)
		if err != nil {
			return nil, fmt.Errorf("falha ao parsear certificado: %w", err)
		}
		return azidentity.NewClientCertificateCredential(tenantID, clientID, certs, key, nil)
	case "msi":
		return azidentity.NewManagedIdentityCredential(nil)
	case "devicecode":
		return azidentity.NewDeviceCodeCredential(&azidentity.DeviceCodeCredentialOptions{
			TenantID: tenantID,
			ClientID: clientID,
			UserPrompt: func(ctx context.Context, msg azidentity.DeviceCodeMessage) error {
				fmt.Fprintf(os.Stderr, "\n%s\n", msg.Message)
				return nil
			},
		})
	case "interactive":
		return azidentity.NewInteractiveBrowserCredential(&azidentity.InteractiveBrowserCredentialOptions{
			TenantID: tenantID,
			ClientID: clientID,
		})
	default:
		slog.Debug("usando DefaultAzureCredential", "login_mode", loginMode)
		return azidentity.NewDefaultAzureCredential(nil)
	}
}

// AzureAdvancedTokenGenerator gera tokens AKS com suporte a múltiplos modos de autenticação.
type AzureAdvancedTokenGenerator struct {
	Runner    shared.Runner
	LoginMode string // azurecli, spn, msi, devicecode, interactive, certificate, default
	TenantID  string
	ClientID  string

	// credFactory permite injetar fábrica de credenciais em testes.
	credFactory credentialFactory
}

// GenerateToken gera um token Azure usando o modo de autenticação configurado.
func (g *AzureAdvancedTokenGenerator) GenerateToken(ctx context.Context) (*Token, error) {
	factory := g.credFactory
	if factory == nil {
		factory = defaultCredentialFactory
	}

	loginMode := g.LoginMode
	if loginMode == "" {
		loginMode = "default"
	}

	cred, err := factory(loginMode, g.TenantID, g.ClientID)
	if err != nil {
		return nil, fmt.Errorf("falha ao criar credencial Azure (%s): %w", loginMode, err)
	}

	tok, err := cred.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: []string{aksScope},
	})
	if err != nil {
		return nil, fmt.Errorf("falha ao obter token Azure: %w", err)
	}

	return &Token{
		Value:     tok.Token,
		ExpiresAt: tok.ExpiresOn,
	}, nil
}

// Garantir que implementa a interface em tempo de compilação.
var _ TokenGenerator = (*AzureAdvancedTokenGenerator)(nil)
