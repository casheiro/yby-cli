//go:build aws

package cloud

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestAWSAdvancedTokenGenerator_AssumeRole(t *testing.T) {
	// Configura credenciais estáticas via env para o teste não depender de ~/.aws/credentials
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIAIOSFODNN7EXAMPLE")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY")
	t.Setenv("AWS_SESSION_TOKEN", "FwoGZXIvYXdzEBYaDH_test_session_token")

	gen := &AWSAdvancedTokenGenerator{
		Region:  "us-east-1",
		Cluster: "test-cluster",
		RoleARN: "arn:aws:iam::123456789012:role/TestRole",
	}

	// Verifica que o generator é criado corretamente com assume-role
	if gen.RoleARN == "" {
		t.Fatal("RoleARN não deve ser vazio")
	}
	if gen.Cluster != "test-cluster" {
		t.Errorf("Cluster = %q, want %q", gen.Cluster, "test-cluster")
	}

	// Testa loadAWSConfig não falha com credenciais estáticas
	cfg, err := gen.loadAWSConfig(context.Background())
	if err != nil {
		t.Fatalf("loadAWSConfig() error = %v", err)
	}
	if cfg.Region != "us-east-1" {
		t.Errorf("Region = %q, want %q", cfg.Region, "us-east-1")
	}

	// Testa wrapWithAssumeRole configura o provider corretamente
	wrapped, err := gen.wrapWithAssumeRole(cfg)
	if err != nil {
		t.Fatalf("wrapWithAssumeRole() error = %v", err)
	}
	if wrapped.Credentials == nil {
		t.Fatal("Credentials não deve ser nil após wrapWithAssumeRole")
	}
}

func TestAWSAdvancedTokenGenerator_IRSA(t *testing.T) {
	// Simula ambiente IRSA com web identity token file
	tmpFile, err := os.CreateTemp(t.TempDir(), "web-identity-token")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tmpFile.WriteString("eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.test"); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	t.Setenv("AWS_WEB_IDENTITY_TOKEN_FILE", tmpFile.Name())
	t.Setenv("AWS_ROLE_ARN", "arn:aws:iam::123456789012:role/IRSARole")

	gen := &AWSAdvancedTokenGenerator{
		Region:  "us-west-2",
		Cluster: "eks-irsa-cluster",
	}

	// Verifica que loadAWSConfig não falha quando web identity está configurado
	cfg, err := gen.loadAWSConfig(context.Background())
	if err != nil {
		t.Fatalf("loadAWSConfig() com IRSA error = %v", err)
	}
	if cfg.Region != "us-west-2" {
		t.Errorf("Region = %q, want %q", cfg.Region, "us-west-2")
	}
}

func TestAWSAdvancedTokenGenerator_SSO(t *testing.T) {
	// Testa que profile SSO é configurado corretamente no generator.
	// Em ambiente de teste sem shared config real, loadAWSConfig falha com
	// "failed to get shared config profile" — isso é esperado e NÃO deve
	// ser confundido com erro de token SSO expirado.
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIAIOSFODNN7EXAMPLE")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY")

	gen := &AWSAdvancedTokenGenerator{
		Region:  "us-east-1",
		Cluster: "sso-cluster",
		Profile: "my-sso-profile",
	}

	if gen.Profile != "my-sso-profile" {
		t.Errorf("Profile = %q, want %q", gen.Profile, "my-sso-profile")
	}

	// loadAWSConfig com profile inexistente deve falhar com erro genérico,
	// não com mensagem de SSO expirado
	_, err := gen.loadAWSConfig(context.Background())
	if err == nil {
		// Profile não existe no shared config, deveria falhar
		t.Log("loadAWSConfig() não falhou — profile pode existir no ambiente de teste")
		return
	}

	// Verifica que o erro NÃO é mapeado como SSO expirado
	if strings.Contains(err.Error(), "aws sso login") {
		t.Errorf("erro de profile inexistente não deve sugerir 'aws sso login': %v", err)
	}
}

func TestAWSAdvancedTokenGenerator_MFA(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIAIOSFODNN7EXAMPLE")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY")

	mfaCalled := false
	gen := &AWSAdvancedTokenGenerator{
		Region:    "us-east-1",
		Cluster:   "mfa-cluster",
		RoleARN:   "arn:aws:iam::123456789012:role/MFARole",
		MFASerial: "arn:aws:iam::123456789012:mfa/user",
		MFAProvider: func() (string, error) {
			mfaCalled = true
			return "123456", nil
		},
	}

	// Verifica configuração
	if gen.MFASerial == "" {
		t.Fatal("MFASerial não deve ser vazio")
	}

	// Carrega config base
	cfg, err := gen.loadAWSConfig(context.Background())
	if err != nil {
		t.Fatalf("loadAWSConfig() error = %v", err)
	}

	// Wrap com assume-role + MFA
	wrapped, err := gen.wrapWithAssumeRole(cfg)
	if err != nil {
		t.Fatalf("wrapWithAssumeRole() com MFA error = %v", err)
	}
	if wrapped.Credentials == nil {
		t.Fatal("Credentials não deve ser nil após wrapWithAssumeRole com MFA")
	}

	// Testa que o MFA provider customizado funciona
	mfaFunc := gen.mfaTokenFunc()
	code, err := mfaFunc()
	if err != nil {
		t.Fatalf("mfaTokenFunc()() error = %v", err)
	}
	if code != "123456" {
		t.Errorf("mfaTokenFunc()() = %q, want %q", code, "123456")
	}
	if !mfaCalled {
		t.Error("MFAProvider não foi chamado")
	}
}

func TestAWSAdvancedTokenGenerator_FallbackToBasic(t *testing.T) {
	// Sem auth config avançada, deve funcionar com credenciais padrão
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIAIOSFODNN7EXAMPLE")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY")

	gen := &AWSAdvancedTokenGenerator{
		Region:  "us-east-1",
		Cluster: "basic-cluster",
	}

	// Sem profile, role_arn ou MFA — autenticação básica
	if gen.Profile != "" {
		t.Error("Profile deve ser vazio para fallback básico")
	}
	if gen.RoleARN != "" {
		t.Error("RoleARN deve ser vazio para fallback básico")
	}
	if gen.MFASerial != "" {
		t.Error("MFASerial deve ser vazio para fallback básico")
	}

	// Deve gerar token via presigned URL com credenciais estáticas
	token, err := gen.GenerateToken(context.Background())
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	if token == nil {
		t.Fatal("GenerateToken() retornou nil")
	}
	if !strings.HasPrefix(token.Value, eksTokenPrefix) {
		t.Errorf("Token não tem prefixo %q: %q", eksTokenPrefix, token.Value[:20])
	}
	if token.ExpiresAt.IsZero() {
		t.Error("ExpiresAt não deve ser zero")
	}
}

func TestAWSAdvancedTokenGenerator_PresignedTokenFormat(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIAIOSFODNN7EXAMPLE")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY")

	gen := &AWSAdvancedTokenGenerator{
		Region:  "us-east-1",
		Cluster: "format-test-cluster",
	}

	token, err := gen.GenerateToken(context.Background())
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	// Verifica formato do token
	if !strings.HasPrefix(token.Value, "k8s-aws-v1.") {
		t.Errorf("Token deve começar com 'k8s-aws-v1.', got prefix: %q", token.Value[:15])
	}

	// Verifica que não tem padding base64 (usa RawURLEncoding)
	encoded := strings.TrimPrefix(token.Value, "k8s-aws-v1.")
	if strings.ContainsAny(encoded, "=+/") {
		t.Errorf("Token não deve conter caracteres de padding ou base64 padrão: %q", encoded[:30])
	}
}
