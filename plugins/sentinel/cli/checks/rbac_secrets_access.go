//go:build k8s

package checks

import (
	"context"
	"fmt"

	"github.com/casheiro/yby-cli/plugins/sentinel/cli/allowlist"
	"github.com/casheiro/yby-cli/plugins/sentinel/cli/escopo"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// SecretsAccessCheck lista roles que permitem get/list em secrets.
type SecretsAccessCheck struct{}

func init() { Register(&SecretsAccessCheck{}) }

func (c *SecretsAccessCheck) ID() string         { return "RBAC_SECRETS_ACCESS" }
func (c *SecretsAccessCheck) Name() string       { return "Acesso a Secrets" }
func (c *SecretsAccessCheck) Category() Category { return CategoryRBAC }
func (c *SecretsAccessCheck) Severity() Severity { return SeverityHigh }
func (c *SecretsAccessCheck) Description() string {
	return "Detecta roles que permitem leitura de secrets"
}

func (c *SecretsAccessCheck) Run(ctx context.Context, client kubernetes.Interface, namespace string) ([]SecurityFinding, error) {
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
		if !recorte.TemClusterRole(role.Name) || !hasSecretsAccess(role.Rules) {
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
			Message:        fmt.Sprintf("ClusterRole '%s' permite leitura de secrets", role.Name),
			Recommendation: "Restrinja acesso a secrets apenas aos serviços que realmente precisam",
			Type:           "critical",
			Description:    fmt.Sprintf("ClusterRole '%s' permite leitura de secrets", role.Name),
		})
	}

	// Roles no namespace
	roles, err := client.RbacV1().Roles(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("falha ao listar Roles: %w", err)
	}
	for _, role := range roles.Items {
		if !hasSecretsAccess(role.Rules) {
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
			Message:        fmt.Sprintf("Role '%s' no namespace '%s' permite leitura de secrets", role.Name, namespace),
			Recommendation: "Restrinja acesso a secrets apenas aos serviços que realmente precisam",
			Type:           "critical",
			Description:    fmt.Sprintf("Role '%s' no namespace '%s' permite leitura de secrets", role.Name, namespace),
		})
	}

	return findings, nil
}

// hasSecretsAccess verifica se alguma rule permite get/list/watch em secrets.
func hasSecretsAccess(rules []rbacv1.PolicyRule) bool {
	readVerbs := map[string]bool{"get": true, "list": true, "watch": true, "*": true}
	for _, rule := range rules {
		hasSecrets := false
		for _, res := range rule.Resources {
			if res == "secrets" || res == "*" {
				hasSecrets = true
				break
			}
		}
		if !hasSecrets {
			continue
		}
		for _, verb := range rule.Verbs {
			if readVerbs[verb] {
				return true
			}
		}
	}
	return false
}
