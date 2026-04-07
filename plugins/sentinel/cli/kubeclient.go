//go:build k8s

package main

import (
	"github.com/casheiro/yby-cli/pkg/plugin/sdk"
	"k8s.io/client-go/kubernetes"
)

// getKubeClient retorna o cliente Kubernetes via SDK do plugin.
func getKubeClient() (kubernetes.Interface, error) {
	return sdk.GetKubeClient()
}
