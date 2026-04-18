//go:build integration && azure

package cloud

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestAzureAdvanced_Integration_CLI(t *testing.T) {
	if os.Getenv("AZURE_TENANT_ID") == "" {
		t.Skip("AZURE_TENANT_ID não configurado")
	}

	gen := &AzureAdvancedTokenGenerator{
		LoginMode: "azurecli",
		TenantID:  os.Getenv("AZURE_TENANT_ID"),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	token, err := gen.GenerateToken(ctx)
	if err != nil {
		t.Fatalf("GenerateToken() com Azure CLI erro: %v", err)
	}

	if token == nil {
		t.Fatal("token não deveria ser nil")
	}
	if token.Value == "" {
		t.Error("token não deveria ser vazio")
	}
	if token.ExpiresAt.IsZero() {
		t.Error("ExpiresAt não deveria ser zero")
	}

	t.Logf("Token Azure CLI gerado (expira em %s)", time.Until(token.ExpiresAt))
}

func TestAzureAdvanced_Integration_ServicePrincipal(t *testing.T) {
	tenantID := os.Getenv("AZURE_TENANT_ID")
	clientID := os.Getenv("AZURE_CLIENT_ID")
	clientSecret := os.Getenv("AZURE_CLIENT_SECRET")

	if tenantID == "" || clientID == "" || clientSecret == "" {
		t.Skip("Credenciais Azure SPN não configuradas (AZURE_TENANT_ID, AZURE_CLIENT_ID, AZURE_CLIENT_SECRET)")
	}

	gen := &AzureAdvancedTokenGenerator{
		LoginMode: "spn",
		TenantID:  tenantID,
		ClientID:  clientID,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	token, err := gen.GenerateToken(ctx)
	if err != nil {
		t.Fatalf("GenerateToken() com Service Principal erro: %v", err)
	}

	if token == nil {
		t.Fatal("token não deveria ser nil")
	}
	if token.Value == "" {
		t.Error("token não deveria ser vazio")
	}

	t.Logf("Token Azure SPN gerado (expira em %s)", time.Until(token.ExpiresAt))
}
