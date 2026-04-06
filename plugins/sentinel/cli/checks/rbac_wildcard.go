//go:build k8s

package checks

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// WildcardRBACCheck lista Roles/ClusterRoles com verbs ou resources wildcard (*).
type WildcardRBACCheck struct{}

func init() { Register(&WildcardRBACCheck{}) }

func (c *WildcardRBACCheck) ID() string         { return "RBAC_WILDCARD" }
func (c *WildcardRBACCheck) Name() string       { return "Permissões Wildcard" }
func (c *WildcardRBACCheck) Category() Category { return CategoryRBAC }
func (c *WildcardRBACCheck) Severity() Severity { return SeverityHigh }
func (c *WildcardRBACCheck) Description() string {
	return "Detecta Roles/ClusterRoles com verbs ou resources wildcard (*)"
}

func (c *WildcardRBACCheck) Run(ctx context.Context, client kubernetes.Interface, namespace string) ([]SecurityFinding, error) {
	var findings []SecurityFinding

	// ClusterRoles
	clusterRoles, err := client.RbacV1().ClusterRoles().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("falha ao listar ClusterRoles: %w", err)
	}
	for _, role := range clusterRoles.Items {
		for _, rule := range role.Rules {
			hasWildcard := false
			for _, verb := range rule.Verbs {
				if verb == "*" {
					hasWildcard = true
					break
				}
			}
			if !hasWildcard {
				for _, res := range rule.Resources {
					if res == "*" {
						hasWildcard = true
						break
					}
				}
			}
			if hasWildcard {
				findings = append(findings, SecurityFinding{
					CheckID:        c.ID(),
					Severity:       SeverityHigh,
					Category:       CategoryRBAC,
					Resource:       fmt.Sprintf("ClusterRole/%s", role.Name),
					Message:        fmt.Sprintf("ClusterRole '%s' possui permissões wildcard (*)", role.Name),
					Recommendation: "Substitua wildcards por verbos e recursos específicos",
					Type:           "critical",
					Description:    fmt.Sprintf("ClusterRole '%s' possui permissões wildcard (*)", role.Name),
				})
				break
			}
		}
	}

	// Roles no namespace
	roles, err := client.RbacV1().Roles(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("falha ao listar Roles: %w", err)
	}
	for _, role := range roles.Items {
		for _, rule := range role.Rules {
			hasWildcard := false
			for _, verb := range rule.Verbs {
				if verb == "*" {
					hasWildcard = true
					break
				}
			}
			if !hasWildcard {
				for _, res := range rule.Resources {
					if res == "*" {
						hasWildcard = true
						break
					}
				}
			}
			if hasWildcard {
				findings = append(findings, SecurityFinding{
					CheckID:        c.ID(),
					Severity:       SeverityHigh,
					Category:       CategoryRBAC,
					Namespace:      namespace,
					Resource:       fmt.Sprintf("Role/%s", role.Name),
					Message:        fmt.Sprintf("Role '%s' no namespace '%s' possui permissões wildcard (*)", role.Name, namespace),
					Recommendation: "Substitua wildcards por verbos e recursos específicos",
					Type:           "critical",
					Description:    fmt.Sprintf("Role '%s' no namespace '%s' possui permissões wildcard (*)", role.Name, namespace),
				})
				break
			}
		}
	}

	return findings, nil
}
