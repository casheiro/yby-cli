//go:build !aws

package cloud

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/casheiro/yby-cli/pkg/services/shared"
)

// AWSTokenGenerator gera tokens EKS via CLI `aws eks get-token`.
// Stub para builds sem a tag `aws` — usa o CLI como fallback.
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
	slog.Debug("usando fallback CLI para token EKS (build sem tag aws)")
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
