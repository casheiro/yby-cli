package monitor

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	sigsyaml "sigs.k8s.io/yaml"
)

// resourceGVR mapeia kinds comuns para seus GroupVersionResource
var resourceGVR = map[string]schema.GroupVersionResource{
	"pod":         {Group: "", Version: "v1", Resource: "pods"},
	"service":     {Group: "", Version: "v1", Resource: "services"},
	"node":        {Group: "", Version: "v1", Resource: "nodes"},
	"configmap":   {Group: "", Version: "v1", Resource: "configmaps"},
	"deployment":  {Group: "apps", Version: "v1", Resource: "deployments"},
	"statefulset": {Group: "apps", Version: "v1", Resource: "statefulsets"},
	"job":         {Group: "batch", Version: "v1", Resource: "jobs"},
	"ingress":     {Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"},
}

// GetResourceYAML busca qualquer recurso K8s e serializa como YAML
func GetResourceYAML(config *rest.Config, kind, name, namespace string) (string, error) {
	gvr, ok := resourceGVR[strings.ToLower(kind)]
	if !ok {
		return "", fmt.Errorf("tipo de recurso não suportado: %s", kind)
	}

	dynClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return "", fmt.Errorf("falha ao criar client dinâmico: %w", err)
	}

	var resource interface{}
	if namespace != "" {
		obj, err := dynClient.Resource(gvr).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
		if err != nil {
			return "", fmt.Errorf("falha ao buscar %s '%s': %w", kind, name, err)
		}
		resource = obj.Object
	} else {
		obj, err := dynClient.Resource(gvr).Get(context.Background(), name, metav1.GetOptions{})
		if err != nil {
			return "", fmt.Errorf("falha ao buscar %s '%s': %w", kind, name, err)
		}
		resource = obj.Object
	}

	yamlBytes, err := sigsyaml.Marshal(resource)
	if err != nil {
		return "", fmt.Errorf("falha ao serializar YAML: %w", err)
	}

	return string(yamlBytes), nil
}
