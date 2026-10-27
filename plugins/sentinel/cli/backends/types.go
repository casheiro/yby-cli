//go:build k8s

package backends

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/client-go/kubernetes"
)

// Finding representa um achado de seguranca unificado de qualquer backend.
type Finding struct {
	ID       string `json:"id"`       // ex: "polaris/runAsRootAllowed", "opa/rbac_cluster_admin"
	Source   string `json:"source"`   // "polaris", "opa", "trivy"
	Severity string `json:"severity"` // "critical", "high", "medium", "low", "info"
	Category string `json:"category"` // "pod-security", "rbac", "network", "secrets", "reliability", "efficiency"
	Resource string `json:"resource"` // "Deployment/api-server"; com vários, o primeiro de Resources
	// Resources lista todos os recursos afetados pelo check, em ordem alfabética; vazio
	// quando o achado é de um recurso só.
	Resources      []string `json:"resources,omitempty"`
	Namespace      string   `json:"namespace"`
	Message        string   `json:"message"`
	Recommendation string   `json:"recommendation"`
	// Scope é o escopo do recurso (escopo.Escopo): "cluster" para um ClusterRole ou um
	// ClusterRoleBinding, "namespace" para o resto.
	Scope string `json:"scope"`
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

// FalhaParcial é o erro de um backend que rodou só parte das verificações: os achados
// devolvidos junto com ele valem, e Falhas diz quais verificações ficaram de fora (o
// RBAC do usuário pode não deixar listar ClusterRoles, por exemplo). Causa é a primeira
// falha. Quando nenhuma verificação roda, o backend devolve a falha comum, não esta.
type FalhaParcial struct {
	Falhas []string
	Causa  error
}

func (f *FalhaParcial) Error() string {
	return fmt.Sprintf("%s não rodaram: %v", strings.Join(f.Falhas, ", "), f.Causa)
}

// Unwrap devolve a primeira falha.
func (f *FalhaParcial) Unwrap() error { return f.Causa }
