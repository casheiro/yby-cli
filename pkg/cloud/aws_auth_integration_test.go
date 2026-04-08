//go:build integration && aws

package cloud

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestAWSAdvanced_Integration_AssumeRole(t *testing.T) {
	roleARN := os.Getenv("AWS_ROLE_ARN")
	if roleARN == "" {
		t.Skip("AWS_ROLE_ARN não configurado")
	}
	if os.Getenv("AWS_ACCESS_KEY_ID") == "" {
		t.Skip("AWS_ACCESS_KEY_ID não configurado")
	}

	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = "us-east-1"
	}

	gen := &AWSAdvancedTokenGenerator{
		Region:  region,
		Cluster: "integration-test-cluster",
		RoleARN: roleARN,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	token, err := gen.GenerateToken(ctx)
	if err != nil {
		t.Fatalf("GenerateToken() com assume-role erro: %v", err)
	}

	if token == nil {
		t.Fatal("token não deveria ser nil")
	}
	if !strings.HasPrefix(token.Value, eksTokenPrefix) {
		t.Errorf("token deveria ter prefixo %q, obtido: %q", eksTokenPrefix, token.Value[:20])
	}
	if token.ExpiresAt.IsZero() {
		t.Error("ExpiresAt não deveria ser zero")
	}

	t.Logf("Token gerado com assume-role para %s (expira em %s)", roleARN, time.Until(token.ExpiresAt))
}

func TestAWSAdvanced_Integration_SSO(t *testing.T) {
	profile := os.Getenv("AWS_SSO_PROFILE")
	if profile == "" {
		t.Skip("AWS_SSO_PROFILE não configurado")
	}

	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = "us-east-1"
	}

	gen := &AWSAdvancedTokenGenerator{
		Region:  region,
		Cluster: "sso-integration-cluster",
		Profile: profile,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	token, err := gen.GenerateToken(ctx)
	if err != nil {
		t.Fatalf("GenerateToken() com SSO profile erro: %v", err)
	}

	if token == nil {
		t.Fatal("token não deveria ser nil")
	}
	if !strings.HasPrefix(token.Value, eksTokenPrefix) {
		t.Errorf("token deveria ter prefixo %q", eksTokenPrefix)
	}

	t.Logf("Token SSO gerado com profile %s (expira em %s)", profile, time.Until(token.ExpiresAt))
}
