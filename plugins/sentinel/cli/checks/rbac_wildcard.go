//go:build k8s

package checks

import (
	"context"
	"fmt"
	"slices"

	"github.com/casheiro/yby-cli/v5/plugins/sentinel/cli/allowlist"
	"github.com/casheiro/yby-cli/v5/plugins/sentinel/cli/escopo"
	rbacv1 "k8s.io/api/rbac/v1"
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
	// Só os ClusterRoles do recorte do escopo entram, e a allowlist decide pelos subjects
	// que os bindings do recorte ligam a cada role.
	recorte, err := escopo.Ler(ctx, client, namespace)
	if err != nil {
		return nil, err
	}
	vinculos := recorte.Vinculos()
	for _, role := range clusterRoles.Items {
		if !recorte.TemClusterRole(role.Name) || !hasWildcard(role.Rules) {
			continue
		}
		if allowlist.Padrao.IgnorarPapel("ClusterRole", role.Name, vinculos.DoClusterRole(role.Name)) {
			allowlist.Ignorar(ctx, "ClusterRole/"+role.Name)
			continue
		}
		findings = append(findings, SecurityFinding{
			CheckID:        c.ID(),
			Severity:       SeverityHigh,
			Category:       CategoryRBAC,
			Resource:       fmt.Sprintf("ClusterRole/%s", role.Name),
			Scope:          string(escopo.Cluster),
			Message:        fmt.Sprintf("ClusterRole '%s' possui permissões wildcard (*)", role.Name),
			Recommendation: "Substitua wildcards por verbos e recursos específicos",
			Type:           "critical",
			Description:    fmt.Sprintf("ClusterRole '%s' possui permissões wildcard (*)", role.Name),
		})
	}

	// Roles no namespace
	roles, err := client.RbacV1().Roles(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("falha ao listar Roles: %w", err)
	}
	for _, role := range roles.Items {
		if !hasWildcard(role.Rules) {
			continue
		}
		if allowlist.Padrao.IgnorarPapel("Role", role.Name, vinculos.DoRole(namespace, role.Name)) {
			allowlist.Ignorar(ctx, "Role/"+namespace+"/"+role.Name)
			continue
		}
		findings = append(findings, SecurityFinding{
			CheckID:        c.ID(),
			Severity:       SeverityHigh,
			Category:       CategoryRBAC,
			Namespace:      namespace,
			Resource:       fmt.Sprintf("Role/%s", role.Name),
			Scope:          string(escopo.Namespace),
			Message:        fmt.Sprintf("Role '%s' no namespace '%s' possui permissões wildcard (*)", role.Name, namespace),
			Recommendation: "Substitua wildcards por verbos e recursos específicos",
			Type:           "critical",
			Description:    fmt.Sprintf("Role '%s' no namespace '%s' possui permissões wildcard (*)", role.Name, namespace),
		})
	}

	return findings, nil
}

// hasWildcard verifica se alguma rule tem verb ou resource "*".
func hasWildcard(rules []rbacv1.PolicyRule) bool {
	for _, rule := range rules {
		if slices.Contains(rule.Verbs, "*") || slices.Contains(rule.Resources, "*") {
			return true
		}
	}
	return false
}
