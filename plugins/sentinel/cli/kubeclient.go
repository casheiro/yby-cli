//go:build k8s

package main

import (
	"github.com/casheiro/yby-cli/pkg/plugin/sdk"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// getKubeClient retorna o cliente Kubernetes via SDK do plugin.
func getKubeClient() (kubernetes.Interface, error) {
	return sdk.GetKubeClient()
}

// getRestConfig retorna o config do mesmo cluster de getKubeClient, para os clientes
// que o clientset tipado não cobre (o dinâmico do Polaris).
func getRestConfig() (*rest.Config, error) {
	return sdk.GetRestConfig()
}
