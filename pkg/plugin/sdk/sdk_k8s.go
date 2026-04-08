//go:build k8s

package sdk

import (
	"context"
	"fmt"
	"net/http"

	"github.com/casheiro/yby-cli/pkg/cloud"
	"github.com/casheiro/yby-cli/pkg/services/shared"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// GetKubeClient retorna um clientset Kubernetes.
// Usa o kubeconfig do contexto do plugin quando disponível,
// ou o kubeconfig padrão (~/.kube/config / KUBECONFIG) como fallback.
// Quando há configuração cloud, injeta token generator com auto-refresh.
func GetKubeClient() (*kubernetes.Clientset, error) {
	var kubeConfigPath, kubeContext string

	if currentContext != nil {
		kubeConfigPath = currentContext.Infra.KubeConfig
		kubeContext = currentContext.Infra.KubeContext
	}

	// Usar regras de carregamento padrão (respeita KUBECONFIG env var e ~/.kube/config)
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeConfigPath != "" {
		loadingRules.ExplicitPath = kubeConfigPath
	}

	configOverrides := &clientcmd.ConfigOverrides{}
	if kubeContext != "" {
		configOverrides.CurrentContext = kubeContext
	}

	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	config, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("falha ao criar config kubernetes: %w", err)
	}

	// Se ambiente tem cloud config, injetar token generator com auto-refresh
	if cloudCfg := getCloudConfig(); cloudCfg != nil {
		runner := &shared.RealRunner{}
		tokenGen := getTokenGenerator(cloudCfg, runner)
		if tokenGen != nil {
			cache := &cloud.TokenCache{}
			token, err := tokenGen.GenerateToken(context.Background())
			if err == nil {
				config.BearerToken = token.Value
				config.BearerTokenFile = ""
				cache.Set(token)
				config.WrapTransport = func(rt http.RoundTripper) http.RoundTripper {
					return &cloud.AutoRefreshTransport{
						Base:      rt,
						Generator: tokenGen,
						Cache:     cache,
					}
				}
			}
		}
	}

	return kubernetes.NewForConfig(config)
}
