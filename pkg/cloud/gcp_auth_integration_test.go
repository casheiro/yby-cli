//go:build integration && gcp

package cloud

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/casheiro/yby-cli/pkg/testutil"
)

func TestGCPAdvanced_Integration_ADC(t *testing.T) {
	if os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") == "" {
		t.Skip("GOOGLE_APPLICATION_CREDENTIALS não configurado")
	}

	gen := &GCPAdvancedTokenGenerator{
		Runner: &testutil.MockRunner{},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	token, err := gen.GenerateToken(ctx)
	if err != nil {
		t.Fatalf("GenerateToken() com ADC erro: %v", err)
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

	t.Logf("Token GCP ADC gerado (expira em %s)", time.Until(token.ExpiresAt))
}

func TestGCPAdvanced_Integration_Impersonation(t *testing.T) {
	sa := os.Getenv("GCP_IMPERSONATE_SA")
	if sa == "" {
		t.Skip("GCP_IMPERSONATE_SA não configurado")
	}

	gen := &GCPAdvancedTokenGenerator{
		Runner:              &testutil.MockRunner{},
		ServiceAccountEmail: sa,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	token, err := gen.GenerateToken(ctx)
	if err != nil {
		t.Fatalf("GenerateToken() com SA impersonation erro: %v", err)
	}

	if token == nil {
		t.Fatal("token não deveria ser nil")
	}
	if token.Value == "" {
		t.Error("token não deveria ser vazio")
	}

	t.Logf("Token GCP impersonation gerado para %s (expira em %s)", sa, time.Until(token.ExpiresAt))
}
