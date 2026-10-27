//go:build k8s

package main

import (
	"github.com/casheiro/yby-cli/pkg/plugin/sdk"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// contextoDoKube é o nome do contexto do kubeconfig que getKubeClient usa: o do ambiente
// do yby, quando ele diz um, ou o contexto atual do kubeconfig. Vazio se nenhum dos dois
// diz (o relatório então fica sem contexto).
func contextoDoKube() string {
	var kubeconfig string
	if fc := sdk.GetFullContext(); fc != nil {
		if fc.Infra.KubeContext != "" {
			return fc.Infra.KubeContext
		}
		kubeconfig = fc.Infra.KubeConfig
	}
	regras := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfig != "" {
		regras.ExplicitPath = kubeconfig
	}
	cfg, err := regras.Load()
	if err != nil {
		return ""
	}
	return cfg.CurrentContext
}

// getKubeClient retorna o cliente Kubernetes via SDK do plugin.
func getKubeClient() (kubernetes.Interface, error) {
	return sdk.GetKubeClient()
}

// getRestConfig retorna o config do mesmo cluster de getKubeClient, para os clientes
// que o clientset tipado não cobre (o dinâmico do Polaris).
func getRestConfig() (*rest.Config, error) {
	return sdk.GetRestConfig()
}
