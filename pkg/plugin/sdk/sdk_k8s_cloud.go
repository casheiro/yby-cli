//go:build k8s && (aws || azure || gcp)

package sdk

import (
	"github.com/casheiro/yby-cli/pkg/cloud"
	"github.com/casheiro/yby-cli/pkg/services/shared"
)

// cloudConfig contém a configuração cloud extraída do contexto do plugin.
type cloudConfig struct {
	Provider string
	Cluster  string
}

// getCloudConfig extrai configuração cloud do contexto atual do plugin.
// Retorna nil se não houver configuração cloud disponível.
func getCloudConfig() *cloudConfig {
	if currentContext == nil {
		return nil
	}

	values := currentContext.Values
	if values == nil {
		return nil
	}

	provider, _ := values["cloud_provider"].(string)
	if provider == "" {
		return nil
	}

	cluster, _ := values["cloud_cluster"].(string)

	return &cloudConfig{
		Provider: provider,
		Cluster:  cluster,
	}
}

// getTokenGenerator retorna o TokenGenerator correto baseado no provider cloud.
func getTokenGenerator(cfg *cloudConfig, runner shared.Runner) cloud.TokenGenerator {
	switch cfg.Provider {
	case "aws":
		return &cloud.AWSTokenGenerator{Runner: runner, Cluster: cfg.Cluster}
	case "azure":
		return &cloud.AzureTokenGenerator{Runner: runner}
	case "gcp":
		return &cloud.GCPTokenGenerator{Runner: runner}
	default:
		return nil
	}
}
