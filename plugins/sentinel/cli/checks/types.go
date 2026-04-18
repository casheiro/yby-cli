//go:build k8s

// Package checks define os security checks do Sentinel e o registro central.
package checks

import (
	"context"

	"k8s.io/client-go/kubernetes"
)

// Severity representa o nível de severidade de um finding.
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
	SeverityInfo     Severity = "info"
)

// Category representa a categoria de um security check.
type Category string

const (
	CategoryPodSecurity Category = "pod-security"
	CategoryRBAC        Category = "rbac"
	CategoryNetwork     Category = "network"
	CategorySecrets     Category = "secrets"
	CategorySupplyChain Category = "supply-chain"
)

// SecurityFinding representa uma vulnerabilidade encontrada por um check.
type SecurityFinding struct {
	CheckID        string   `json:"check_id"`
	Severity       Severity `json:"severity"`
	Category       Category `json:"category"`
	Pod            string   `json:"pod,omitempty"`
	Container      string   `json:"container,omitempty"`
	Namespace      string   `json:"namespace"`
	Resource       string   `json:"resource"`
	Message        string   `json:"message"`
	Recommendation string   `json:"recommendation,omitempty"`
	// Type mantém compatibilidade com o formato antigo (mapeia para severity).
	Type string `json:"type"`
	// Description mantém compatibilidade com o formato antigo (mapeia para message).
	Description string `json:"description"`
}

// SecurityCheck é a interface que todo check de segurança deve implementar.
type SecurityCheck interface {
	ID() string
	Name() string
	Category() Category
	Severity() Severity
	Description() string
	Run(ctx context.Context, client kubernetes.Interface, namespace string) ([]SecurityFinding, error)
}
