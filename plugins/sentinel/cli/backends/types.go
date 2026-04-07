//go:build k8s

package backends

import (
	"context"

	"k8s.io/client-go/kubernetes"
)

// Finding representa um achado de seguranca unificado de qualquer backend.
type Finding struct {
	ID             string `json:"id"`              // ex: "polaris/runAsRootAllowed", "opa/rbac_cluster_admin"
	Source         string `json:"source"`           // "polaris", "opa", "trivy"
	Severity       string `json:"severity"`         // "critical", "high", "medium", "low", "info"
	Category       string `json:"category"`         // "pod-security", "rbac", "network", "image", "config"
	Resource       string `json:"resource"`         // "Deployment/api-server"
	Namespace      string `json:"namespace"`
	Message        string `json:"message"`
	Recommendation string `json:"recommendation"`
}

// SecurityBackend e a interface que cada integracao de seguranca implementa.
type SecurityBackend interface {
	// Name retorna o identificador do backend.
	Name() string
	// IsAvailable verifica se o backend pode ser usado.
	IsAvailable() bool
	// ScanCluster escaneia um namespace de um cluster ativo.
	ScanCluster(ctx context.Context, client kubernetes.Interface, namespace string) ([]Finding, error)
}
