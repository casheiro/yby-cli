//go:build k8s

package checks

import (
	"context"
	"fmt"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// NetpolDefaultDenyCheck verifica se existe uma NetworkPolicy default-deny no namespace.
type NetpolDefaultDenyCheck struct{}

func init() { Register(&NetpolDefaultDenyCheck{}) }

func (c *NetpolDefaultDenyCheck) ID() string         { return "NETPOL_DEFAULT_DENY" }
func (c *NetpolDefaultDenyCheck) Name() string       { return "Default Deny NetworkPolicy" }
func (c *NetpolDefaultDenyCheck) Category() Category { return CategoryNetwork }
func (c *NetpolDefaultDenyCheck) Severity() Severity { return SeverityMedium }
func (c *NetpolDefaultDenyCheck) Description() string {
	return "Verifica se existe NetworkPolicy default-deny no namespace"
}

func (c *NetpolDefaultDenyCheck) Run(ctx context.Context, client kubernetes.Interface, namespace string) ([]SecurityFinding, error) {
	netpols, err := client.NetworkingV1().NetworkPolicies(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("falha ao listar NetworkPolicies: %w", err)
	}

	hasDefaultDenyIngress := false
	hasDefaultDenyEgress := false

	for _, np := range netpols.Items {
		// Default deny = podSelector vazio (seleciona todos)
		if len(np.Spec.PodSelector.MatchLabels) == 0 && len(np.Spec.PodSelector.MatchExpressions) == 0 {
			for _, pt := range np.Spec.PolicyTypes {
				if pt == networkingv1.PolicyTypeIngress && len(np.Spec.Ingress) == 0 {
					hasDefaultDenyIngress = true
				}
				if pt == networkingv1.PolicyTypeEgress && len(np.Spec.Egress) == 0 {
					hasDefaultDenyEgress = true
				}
			}
		}
	}

	var findings []SecurityFinding
	if !hasDefaultDenyIngress {
		findings = append(findings, SecurityFinding{
			CheckID:        c.ID(),
			Severity:       SeverityMedium,
			Category:       CategoryNetwork,
			Namespace:      namespace,
			Resource:       namespace,
			Message:        fmt.Sprintf("Namespace '%s' não possui NetworkPolicy default-deny para Ingress", namespace),
			Recommendation: "Crie uma NetworkPolicy com podSelector vazio e policyTypes [Ingress] sem regras de ingress",
			Type:           "warning",
			Description:    fmt.Sprintf("Namespace '%s' não possui NetworkPolicy default-deny para Ingress", namespace),
		})
	}
	if !hasDefaultDenyEgress {
		findings = append(findings, SecurityFinding{
			CheckID:        c.ID(),
			Severity:       SeverityMedium,
			Category:       CategoryNetwork,
			Namespace:      namespace,
			Resource:       namespace,
			Message:        fmt.Sprintf("Namespace '%s' não possui NetworkPolicy default-deny para Egress", namespace),
			Recommendation: "Crie uma NetworkPolicy com podSelector vazio e policyTypes [Egress] sem regras de egress",
			Type:           "warning",
			Description:    fmt.Sprintf("Namespace '%s' não possui NetworkPolicy default-deny para Egress", namespace),
		})
	}

	return findings, nil
}
