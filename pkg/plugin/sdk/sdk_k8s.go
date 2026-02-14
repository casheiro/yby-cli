//go:build k8s

package sdk

import (
	"fmt"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func GetKubeClient() (*kubernetes.Clientset, error) {
	if currentContext == nil {
		return nil, fmt.Errorf("SDK not initialized or no context received. Did you call sdk.Init()?")
	}

	kubeConfigPath := currentContext.Infra.KubeConfig
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to build kube config: %w", err)
	}

	if currentContext.Infra.KubeContext != "" {
		loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		if kubeConfigPath != "" {
			loadingRules.ExplicitPath = kubeConfigPath
		}
		configOverrides := &clientcmd.ConfigOverrides{CurrentContext: currentContext.Infra.KubeContext}
		clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
		config, err = clientConfig.ClientConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to create client config for context '%s': %w", currentContext.Infra.KubeContext, err)
		}
	}

	return kubernetes.NewForConfig(config)
}
