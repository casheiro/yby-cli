//go:build !aws && !azure && !gcp

package sdk

import (
	"github.com/casheiro/yby-cli/v5/pkg/cloud"
	"github.com/casheiro/yby-cli/v5/pkg/services/shared"
)

// cloudConfig contém a configuração cloud extraída do contexto do plugin.
type cloudConfig struct {
	Provider string
	Cluster  string
}

// getCloudConfig retorna nil quando compilado sem build tags cloud.
func getCloudConfig() *cloudConfig {
	return nil
}

// getTokenGenerator retorna nil quando compilado sem build tags cloud.
func getTokenGenerator(_ *cloudConfig, _ shared.Runner) cloud.TokenGenerator {
	return nil
}
