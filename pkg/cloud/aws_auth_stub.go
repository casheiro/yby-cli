//go:build !aws

package cloud

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"

	"github.com/casheiro/yby-cli/pkg/services/shared"
)

// MFATokenProvider é uma função que solicita o código MFA ao usuário.
type MFATokenProvider func() (string, error)

// AWSAdvancedTokenGenerator gera tokens EKS via CLI `aws eks get-token` com suporte
// a profile e role-arn. Stub para builds sem a tag `aws`.
type AWSAdvancedTokenGenerator struct {
	Runner    shared.Runner
	Region    string
	Cluster   string
	Profile   string
	RoleARN   string
	MFASerial string

	// MFAProvider não é usado no stub CLI mas mantém compatibilidade de interface.
	MFAProvider MFATokenProvider

	// Stdin e Stderr não são usados no stub CLI.
	Stdin  io.Reader
	Stderr io.Writer
}

// GenerateToken gera um token EKS via CLI `aws eks get-token` com flags --profile e --role-arn.
func (g *AWSAdvancedTokenGenerator) GenerateToken(ctx context.Context) (*Token, error) {
	slog.Debug("usando fallback CLI para token EKS avançado (build sem tag aws)")

	args := []string{"eks", "get-token", "--cluster-name", g.Cluster, "--output", "json"}
	if g.Region != "" {
		args = append(args, "--region", g.Region)
	}
	if g.Profile != "" {
		args = append(args, "--profile", g.Profile)
	}
	if g.RoleARN != "" {
		args = append(args, "--role-arn", g.RoleARN)
	}

	out, err := g.Runner.RunCombinedOutput(ctx, "aws", args...)
	if err != nil {
		return nil, fmt.Errorf("aws eks get-token: %w", err)
	}

	var eksToken awsEKSToken
	if err := json.Unmarshal(out, &eksToken); err != nil {
		return nil, fmt.Errorf("parse eks token: %w", err)
	}

	return &Token{
		Value:     eksToken.Status.Token,
		ExpiresAt: eksToken.Status.ExpirationTimestamp,
	}, nil
}
