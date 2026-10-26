package cloud

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"
)

// Validade da credencial de cada CLI, lida depois que a credencial foi confirmada. É uma
// chamada a mais por provider (não por cluster) e nunca derruba a validação: se o CLI não
// informar a validade (versão antiga, chave de longa duração), o ExpiresAt fica nil.

// validadeAWS lê a expiração das credenciais temporárias (SSO, assume-role) via
// `aws configure export-credentials` (AWS CLI v2.9+). Chave de IAM fixa não expira: nil.
func validadeAWS(ctx context.Context, a *awsProvider) *time.Time {
	out, err := a.runner.RunCombinedOutput(ctx, "aws", "configure", "export-credentials", "--format", "process")
	if err != nil {
		slog.Debug("cloud.aws.validade", "erro", err)
		return nil
	}
	var cred struct {
		Expiration string `json:"Expiration"`
	}
	if json.Unmarshal(out, &cred) != nil || cred.Expiration == "" {
		return nil
	}
	return lerInstante(cred.Expiration, time.UTC)
}

// validadeAzure lê a expiração do token de acesso do az. O `expires_on` (segundos desde a
// época) vem nas versões novas; o `expiresOn` antigo é a hora local, sem fuso.
func validadeAzure(ctx context.Context, a *azureProvider) *time.Time {
	out, err := a.runner.RunCombinedOutput(ctx, "az", "account", "get-access-token", "--output", "json")
	if err != nil {
		slog.Debug("cloud.azure.validade", "erro", err)
		return nil
	}
	var tok struct {
		ExpiresOnEpoch int64  `json:"expires_on"`
		ExpiresOn      string `json:"expiresOn"`
	}
	if json.Unmarshal(out, &tok) != nil {
		return nil
	}
	if tok.ExpiresOnEpoch > 0 {
		t := time.Unix(tok.ExpiresOnEpoch, 0)
		return &t
	}
	return lerInstante(tok.ExpiresOn, time.Local)
}

// validadeGCP lê a expiração do token de acesso do gcloud via `gcloud config
// config-helper`, que traz o token_expiry da credencial ativa.
func validadeGCP(ctx context.Context, g *gcpProvider) *time.Time {
	out, err := g.runner.RunCombinedOutput(ctx, "gcloud", "config", "config-helper", "--format", "json")
	if err != nil {
		slog.Debug("cloud.gcp.validade", "erro", err)
		return nil
	}
	var ajuda struct {
		Credential struct {
			TokenExpiry string `json:"token_expiry"`
		} `json:"credential"`
	}
	if json.Unmarshal(out, &ajuda) != nil {
		return nil
	}
	return lerInstante(ajuda.Credential.TokenExpiry, time.UTC)
}

// lerInstante aceita RFC 3339 e o formato "2006-01-02 15:04:05.999999" (no fuso dado).
func lerInstante(s string, fuso *time.Location) *time.Time {
	if s == "" {
		return nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return &t
	}
	if t, err := time.ParseInLocation("2006-01-02 15:04:05.999999", s, fuso); err == nil {
		return &t
	}
	return nil
}
