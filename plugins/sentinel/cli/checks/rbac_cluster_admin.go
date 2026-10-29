//go:build k8s

package checks

import (
	"context"
	"fmt"

	"github.com/casheiro/yby-cli/v5/plugins/sentinel/cli/allowlist"
	"github.com/casheiro/yby-cli/v5/plugins/sentinel/cli/escopo"
	"k8s.io/client-go/kubernetes"
)

// ClusterAdminCheck lista ClusterRoleBindings que referenciam cluster-admin.
type ClusterAdminCheck struct{}

func init() { Register(&ClusterAdminCheck{}) }

func (c *ClusterAdminCheck) ID() string         { return "RBAC_CLUSTER_ADMIN" }
func (c *ClusterAdminCheck) Name() string       { return "Bindings cluster-admin" }
func (c *ClusterAdminCheck) Category() Category { return CategoryRBAC }
func (c *ClusterAdminCheck) Severity() Severity { return SeverityCritical }
func (c *ClusterAdminCheck) Description() string {
	return "Detecta ClusterRoleBindings que referenciam o role cluster-admin"
}

// Run olha os ClusterRoleBindings do recorte do escopo (escopo.Ler): no escopo namespace,
// só os que dão o papel a uma ServiceAccount do namespace, com os subjects dele.
func (c *ClusterAdminCheck) Run(ctx context.Context, client kubernetes.Interface, namespace string) ([]SecurityFinding, error) {
	recorte, err := escopo.Ler(ctx, client, namespace)
	if err != nil {
		return nil, err
	}

	var findings []SecurityFinding
	for _, binding := range recorte.ClusterRoleBindings() {
		if binding.RoleRef.Name != "cluster-admin" {
			continue
		}
		antes := len(findings)
		for _, subject := range allowlist.SujeitosDe(binding.Subjects) {
			// A allowlist casa pelo subject (system:masters, SAs de controllers
			// conhecidos), nunca pelo nome do binding.
			if allowlist.Padrao.SujeitoPermitido(subject) {
				continue
			}
			findings = append(findings, SecurityFinding{
				CheckID:        c.ID(),
				Severity:       SeverityCritical,
				Category:       CategoryRBAC,
				Namespace:      subject.Namespace,
				Resource:       binding.Name,
				Message:        fmt.Sprintf("ClusterRoleBinding '%s' concede cluster-admin a %s '%s'", binding.Name, subject.Kind, subject.Nome),
				Recommendation: "Substitua cluster-admin por roles mais restritivos seguindo o princípio do menor privilégio",
				Scope:          string(escopo.Cluster),
				Type:           "critical",
				Description:    fmt.Sprintf("ClusterRoleBinding '%s' concede cluster-admin a %s '%s'", binding.Name, subject.Kind, subject.Nome),
			})
		}
		if len(findings) == antes && len(binding.Subjects) > 0 {
			allowlist.Ignorar(ctx, "ClusterRoleBinding/"+binding.Name)
		}
	}
	return findings, nil
}
