//go:build k8s

package sdk

import (
	"fmt"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// GetKubeClient retorna um clientset Kubernetes.
// Usa o kubeconfig do contexto do plugin quando disponível,
// ou o kubeconfig padrão (~/.kube/config / KUBECONFIG) como fallback.
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

	return kubernetes.NewForConfig(config)
}
