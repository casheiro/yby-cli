//go:build aws

package cloud

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/casheiro/yby-cli/pkg/services/shared"
)

// AWSTokenGenerator gera tokens EKS via CLI `aws eks get-token`.
// Decisão de design: aws-iam-authenticator foi excluído como dependência direta
// por causar conflitos de versão com client-go. Ambas as build tags (aws e !aws)
// usam o CLI como implementação. A tag `aws` existe para futura diferenciação
// caso uma lib alternativa seja adotada.
type AWSTokenGenerator struct {
	Runner  shared.Runner
	Cluster string
}

// awsEKSToken representa a saída de `aws eks get-token --output json`.
type awsEKSToken struct {
	Status struct {
		Token               string    `json:"token"`
		ExpirationTimestamp time.Time `json:"expirationTimestamp"`
	} `json:"status"`
}

// GenerateToken gera um token EKS executando `aws eks get-token` via CLI.
func (g *AWSTokenGenerator) GenerateToken(ctx context.Context) (*Token, error) {
	out, err := g.Runner.RunCombinedOutput(ctx, "aws", "eks", "get-token", "--cluster-name", g.Cluster, "--output", "json")
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
