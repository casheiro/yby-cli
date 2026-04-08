//go:build aws

package cloud

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

// clusterNameHeader é o header usado pelo EKS para identificar o cluster no token presigned.
const clusterNameHeader = "x-k8s-aws-id"

// eksTokenPrefix é o prefixo padrão de tokens EKS gerados via STS presigned URL.
const eksTokenPrefix = "k8s-aws-v1."

// eksTokenTTL é o tempo de vida padrão de tokens EKS (15 minutos, mesmo do aws-iam-authenticator).
const eksTokenTTL = 15 * time.Minute

// MFATokenProvider é uma função que solicita o código MFA ao usuário.
// Permite injeção de dependência para testes.
type MFATokenProvider func() (string, error)

// defaultMFATokenProvider solicita o código MFA via terminal (stderr/stdin).
func defaultMFATokenProvider() (string, error) {
	fmt.Fprintf(os.Stderr, "Código MFA: ")
	var code string
	if _, err := fmt.Fscanln(os.Stdin, &code); err != nil {
		return "", fmt.Errorf("erro lendo código MFA: %w", err)
	}
	return strings.TrimSpace(code), nil
}

// AWSAdvancedTokenGenerator gera tokens EKS usando o SDK AWS com suporte a
// múltiplos métodos de autenticação: SSO, assume-role, web-identity (IRSA) e MFA.
type AWSAdvancedTokenGenerator struct {
	Region    string
	Cluster   string
	Profile   string
	RoleARN   string
	MFASerial string

	// MFAProvider permite injetar um provider de código MFA customizado (para testes).
	// Se nil, usa prompt interativo via terminal.
	MFAProvider MFATokenProvider

	// Stdin e Stderr permitem injeção para testes do prompt MFA.
	Stdin  io.Reader
	Stderr io.Writer
}

// GenerateToken gera um token EKS usando presigned URL do STS GetCallerIdentity.
// Suporta SSO (via profile), assume-role, IRSA (web-identity) e MFA automaticamente.
func (g *AWSAdvancedTokenGenerator) GenerateToken(ctx context.Context) (*Token, error) {
	cfg, err := g.loadAWSConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("erro carregando configuração AWS: %w", err)
	}

	// Wrap com assume-role se configurado
	if g.RoleARN != "" {
		cfg, err = g.wrapWithAssumeRole(cfg)
		if err != nil {
			return nil, fmt.Errorf("erro configurando assume-role: %w", err)
		}
	}

	return g.generatePresignedToken(ctx, cfg)
}

// loadAWSConfig carrega a configuração AWS com região e profile opcionais.
// SSO e web-identity são resolvidos automaticamente pelo SDK via shared config.
func (g *AWSAdvancedTokenGenerator) loadAWSConfig(ctx context.Context) (aws.Config, error) {
	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(g.Region),
	}

	if g.Profile != "" {
		slog.Debug("usando AWS profile para autenticação", "profile", g.Profile)
		opts = append(opts, awsconfig.WithSharedConfigProfile(g.Profile))
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		errMsg := err.Error()
		// Verifica se é erro específico de SSO com token expirado (não profile inexistente)
		isSSOError := (strings.Contains(errMsg, "SSO") || strings.Contains(errMsg, "sso")) &&
			!strings.Contains(errMsg, "failed to get shared config profile")
		if isSSOError {
			return aws.Config{}, fmt.Errorf(
				"token SSO expirado ou inválido. Execute 'aws sso login --profile %s' e tente novamente: %w",
				g.Profile, err,
			)
		}
		return aws.Config{}, err
	}

	return cfg, nil
}

// wrapWithAssumeRole configura credenciais via STS AssumeRole, opcionalmente com MFA.
func (g *AWSAdvancedTokenGenerator) wrapWithAssumeRole(cfg aws.Config) (aws.Config, error) {
	slog.Debug("configurando assume-role", "role_arn", g.RoleARN, "mfa", g.MFASerial != "")

	stsClient := sts.NewFromConfig(cfg)
	provider := stscreds.NewAssumeRoleProvider(stsClient, g.RoleARN, func(o *stscreds.AssumeRoleOptions) {
		o.RoleSessionName = "yby-cli"
		if g.MFASerial != "" {
			o.SerialNumber = &g.MFASerial
			o.TokenProvider = g.mfaTokenFunc()
		}
	})

	cfg.Credentials = aws.NewCredentialsCache(provider)
	return cfg, nil
}

// mfaTokenFunc retorna a função de provider MFA configurada ou a default.
func (g *AWSAdvancedTokenGenerator) mfaTokenFunc() func() (string, error) {
	if g.MFAProvider != nil {
		return g.MFAProvider
	}
	return defaultMFATokenProvider
}

// generatePresignedToken gera o token EKS via presigned GetCallerIdentity URL.
// Mesmo mecanismo usado internamente pelo aws-iam-authenticator.
func (g *AWSAdvancedTokenGenerator) generatePresignedToken(ctx context.Context, cfg aws.Config) (*Token, error) {
	stsClient := sts.NewFromConfig(cfg)
	presignClient := sts.NewPresignClient(stsClient)

	presigned, err := presignClient.PresignGetCallerIdentity(ctx, &sts.GetCallerIdentityInput{},
		func(o *sts.PresignOptions) {
			o.ClientOptions = append(o.ClientOptions, sts.WithAPIOptions(
				smithyhttp.AddHeaderValue(clusterNameHeader, g.Cluster),
			))
		},
	)
	if err != nil {
		return nil, fmt.Errorf("erro gerando presigned URL para token EKS: %w", err)
	}

	// Token = "k8s-aws-v1." + base64url(presigned.URL) sem padding
	tokenValue := eksTokenPrefix + base64.RawURLEncoding.EncodeToString([]byte(presigned.URL))

	return &Token{
		Value:     tokenValue,
		ExpiresAt: time.Now().Add(eksTokenTTL),
	}, nil
}
