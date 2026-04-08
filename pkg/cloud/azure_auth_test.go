//go:build azure

package cloud

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/casheiro/yby-cli/pkg/testutil"
)

// mockCredential implementa azureCredential para testes.
type mockCredential struct {
	token azcore.AccessToken
	err   error
}

func (m *mockCredential) GetToken(_ context.Context, _ policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return m.token, m.err
}

// mockFactory retorna uma credentialFactory que captura o loginMode e retorna a credencial fornecida.
func mockFactory(cred azureCredential, err error, capturedMode *string) credentialFactory {
	return func(loginMode, tenantID, clientID string) (azureCredential, error) {
		if capturedMode != nil {
			*capturedMode = loginMode
		}
		if err != nil {
			return nil, err
		}
		return cred, nil
	}
}

func TestAzureAdvancedTokenGenerator_DefaultCredential(t *testing.T) {
	var capturedMode string
	expiry := time.Now().Add(1 * time.Hour)
	cred := &mockCredential{
		token: azcore.AccessToken{Token: "tok-default", ExpiresOn: expiry},
	}

	gen := &AzureAdvancedTokenGenerator{
		Runner:      &testutil.MockRunner{},
		LoginMode:   "default",
		credFactory: mockFactory(cred, nil, &capturedMode),
	}

	tok, err := gen.GenerateToken(context.Background())
	if err != nil {
		t.Fatalf("GenerateToken() error = %v, want nil", err)
	}
	if tok.Value != "tok-default" {
		t.Errorf("Value = %q, want %q", tok.Value, "tok-default")
	}
	if capturedMode != "default" {
		t.Errorf("loginMode = %q, want %q", capturedMode, "default")
	}
}

func TestAzureAdvancedTokenGenerator_AzureCLI(t *testing.T) {
	var capturedMode string
	expiry := time.Now().Add(1 * time.Hour)
	cred := &mockCredential{
		token: azcore.AccessToken{Token: "tok-cli", ExpiresOn: expiry},
	}

	gen := &AzureAdvancedTokenGenerator{
		Runner:      &testutil.MockRunner{},
		LoginMode:   "azurecli",
		credFactory: mockFactory(cred, nil, &capturedMode),
	}

	tok, err := gen.GenerateToken(context.Background())
	if err != nil {
		t.Fatalf("GenerateToken() error = %v, want nil", err)
	}
	if tok.Value != "tok-cli" {
		t.Errorf("Value = %q, want %q", tok.Value, "tok-cli")
	}
	if capturedMode != "azurecli" {
		t.Errorf("loginMode = %q, want %q", capturedMode, "azurecli")
	}
}

func TestAzureAdvancedTokenGenerator_ServicePrincipal(t *testing.T) {
	var capturedMode string
	expiry := time.Now().Add(1 * time.Hour)
	cred := &mockCredential{
		token: azcore.AccessToken{Token: "tok-spn", ExpiresOn: expiry},
	}

	gen := &AzureAdvancedTokenGenerator{
		Runner:      &testutil.MockRunner{},
		LoginMode:   "spn",
		TenantID:    "tenant-123",
		ClientID:    "client-456",
		credFactory: mockFactory(cred, nil, &capturedMode),
	}

	tok, err := gen.GenerateToken(context.Background())
	if err != nil {
		t.Fatalf("GenerateToken() error = %v, want nil", err)
	}
	if tok.Value != "tok-spn" {
		t.Errorf("Value = %q, want %q", tok.Value, "tok-spn")
	}
	if capturedMode != "spn" {
		t.Errorf("loginMode = %q, want %q", capturedMode, "spn")
	}
}

func TestAzureAdvancedTokenGenerator_ManagedIdentity(t *testing.T) {
	var capturedMode string
	expiry := time.Now().Add(1 * time.Hour)
	cred := &mockCredential{
		token: azcore.AccessToken{Token: "tok-msi", ExpiresOn: expiry},
	}

	gen := &AzureAdvancedTokenGenerator{
		Runner:      &testutil.MockRunner{},
		LoginMode:   "msi",
		credFactory: mockFactory(cred, nil, &capturedMode),
	}

	tok, err := gen.GenerateToken(context.Background())
	if err != nil {
		t.Fatalf("GenerateToken() error = %v, want nil", err)
	}
	if tok.Value != "tok-msi" {
		t.Errorf("Value = %q, want %q", tok.Value, "tok-msi")
	}
	if capturedMode != "msi" {
		t.Errorf("loginMode = %q, want %q", capturedMode, "msi")
	}
}

func TestAzureAdvancedTokenGenerator_DeviceCode(t *testing.T) {
	var capturedMode string
	expiry := time.Now().Add(1 * time.Hour)
	cred := &mockCredential{
		token: azcore.AccessToken{Token: "tok-device", ExpiresOn: expiry},
	}

	gen := &AzureAdvancedTokenGenerator{
		Runner:      &testutil.MockRunner{},
		LoginMode:   "devicecode",
		TenantID:    "tenant-123",
		ClientID:    "client-456",
		credFactory: mockFactory(cred, nil, &capturedMode),
	}

	tok, err := gen.GenerateToken(context.Background())
	if err != nil {
		t.Fatalf("GenerateToken() error = %v, want nil", err)
	}
	if tok.Value != "tok-device" {
		t.Errorf("Value = %q, want %q", tok.Value, "tok-device")
	}
	if capturedMode != "devicecode" {
		t.Errorf("loginMode = %q, want %q", capturedMode, "devicecode")
	}
}

func TestAzureAdvancedTokenGenerator_InvalidLoginMode(t *testing.T) {
	var capturedMode string
	expiry := time.Now().Add(1 * time.Hour)
	cred := &mockCredential{
		token: azcore.AccessToken{Token: "tok-fallback", ExpiresOn: expiry},
	}

	gen := &AzureAdvancedTokenGenerator{
		Runner:      &testutil.MockRunner{},
		LoginMode:   "modo-invalido",
		credFactory: mockFactory(cred, nil, &capturedMode),
	}

	tok, err := gen.GenerateToken(context.Background())
	if err != nil {
		t.Fatalf("GenerateToken() error = %v, want nil", err)
	}
	if tok.Value != "tok-fallback" {
		t.Errorf("Value = %q, want %q", tok.Value, "tok-fallback")
	}
	// Modo inválido deve cair no default
	if capturedMode != "modo-invalido" {
		t.Errorf("loginMode = %q, want %q", capturedMode, "modo-invalido")
	}
}

func TestAzureAdvancedTokenGenerator_EmptyLoginMode(t *testing.T) {
	var capturedMode string
	expiry := time.Now().Add(1 * time.Hour)
	cred := &mockCredential{
		token: azcore.AccessToken{Token: "tok-empty", ExpiresOn: expiry},
	}

	gen := &AzureAdvancedTokenGenerator{
		Runner:      &testutil.MockRunner{},
		LoginMode:   "",
		credFactory: mockFactory(cred, nil, &capturedMode),
	}

	tok, err := gen.GenerateToken(context.Background())
	if err != nil {
		t.Fatalf("GenerateToken() error = %v, want nil", err)
	}
	if tok.Value != "tok-empty" {
		t.Errorf("Value = %q, want %q", tok.Value, "tok-empty")
	}
	if capturedMode != "default" {
		t.Errorf("loginMode = %q, want %q (vazio deve virar default)", capturedMode, "default")
	}
}

func TestAzureAdvancedTokenGenerator_CredentialFactoryError(t *testing.T) {
	gen := &AzureAdvancedTokenGenerator{
		Runner:      &testutil.MockRunner{},
		LoginMode:   "spn",
		credFactory: mockFactory(nil, fmt.Errorf("credencial inválida"), nil),
	}

	_, err := gen.GenerateToken(context.Background())
	if err == nil {
		t.Fatal("GenerateToken() error = nil, want non-nil")
	}
}

func TestAzureAdvancedTokenGenerator_GetTokenError(t *testing.T) {
	cred := &mockCredential{
		err: fmt.Errorf("token expirado"),
	}

	gen := &AzureAdvancedTokenGenerator{
		Runner:      &testutil.MockRunner{},
		LoginMode:   "azurecli",
		credFactory: mockFactory(cred, nil, nil),
	}

	_, err := gen.GenerateToken(context.Background())
	if err == nil {
		t.Fatal("GenerateToken() error = nil, want non-nil")
	}
}

func TestAzureAdvancedTokenGenerator_ExpiresAtPreserved(t *testing.T) {
	expiry := time.Date(2026, 12, 25, 10, 30, 0, 0, time.UTC)
	cred := &mockCredential{
		token: azcore.AccessToken{Token: "tok", ExpiresOn: expiry},
	}

	gen := &AzureAdvancedTokenGenerator{
		Runner:      &testutil.MockRunner{},
		LoginMode:   "default",
		credFactory: mockFactory(cred, nil, nil),
	}

	tok, err := gen.GenerateToken(context.Background())
	if err != nil {
		t.Fatalf("GenerateToken() error = %v, want nil", err)
	}
	if !tok.ExpiresAt.Equal(expiry) {
		t.Errorf("ExpiresAt = %v, want %v", tok.ExpiresAt, expiry)
	}
}
