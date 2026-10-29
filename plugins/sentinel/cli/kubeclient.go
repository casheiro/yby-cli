//go:build k8s

package main

import (
	"github.com/casheiro/yby-cli/v5/pkg/plugin/sdk"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// contextoDoKube é o nome do contexto do kubeconfig que getKubeClient usa: o do ambiente
// do yby, quando ele diz um, ou o contexto atual do kubeconfig. Vazio se nenhum dos dois
// diz (o relatório então fica sem contexto).
func contextoDoKube() string {
	if fc := sdk.GetFullContext(); fc != nil && fc.Infra.KubeContext != "" {
		return fc.Infra.KubeContext
	}
	cfg := kubeconfigDoCliente()
	if cfg == nil {
		return ""
	}
	return cfg.CurrentContext
}

// namespaceDoKube é o namespace em que o Sentinel começa sem -n: o do ambiente do yby,
// quando ele diz um; senão o do contexto do kubeconfig que getKubeClient usa (onde o
// kubectl e o Viz começam); senão default.
func namespaceDoKube() string {
	if fc := sdk.GetFullContext(); fc != nil && fc.Infra.Namespace != "" {
		return fc.Infra.Namespace
	}
	if cfg := kubeconfigDoCliente(); cfg != nil {
		if c, ok := cfg.Contexts[contextoDoKube()]; ok && c.Namespace != "" {
			return c.Namespace
		}
	}
	return "default"
}

// kubeconfigDoCliente carrega o kubeconfig que getKubeClient usa: o do ambiente do yby,
// quando ele diz um, ou o padrão (KUBECONFIG ou ~/.kube/config). Nil se não carrega.
func kubeconfigDoCliente() *clientcmdapi.Config {
	regras := clientcmd.NewDefaultClientConfigLoadingRules()
	if fc := sdk.GetFullContext(); fc != nil && fc.Infra.KubeConfig != "" {
		regras.ExplicitPath = fc.Infra.KubeConfig
	}
	cfg, err := regras.Load()
	if err != nil {
		return nil
	}
	return cfg
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
