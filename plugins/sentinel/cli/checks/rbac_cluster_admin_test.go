//go:build k8s

package checks

import (
	"context"
	"testing"

	"github.com/casheiro/yby-cli/plugins/sentinel/cli/allowlist"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestClusterAdminCheck_ComBinding(t *testing.T) {
	client := fake.NewSimpleClientset(&rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "admin-binding"},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "cluster-admin",
		},
		Subjects: []rbacv1.Subject{{
			Kind: "User",
			Name: "admin-user",
		}},
	})

	check := &ClusterAdminCheck{}
	findings, err := check.Run(context.Background(), client, "default")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(findings) == 0 {
		t.Fatal("esperava finding para binding cluster-admin")
	}
	if findings[0].Severity != SeverityCritical {
		t.Errorf("esperava severity critical, obteve %s", findings[0].Severity)
	}
}

func TestClusterAdminCheck_SemBinding(t *testing.T) {
	client := fake.NewSimpleClientset(&rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "view-binding"},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "view",
		},
		Subjects: []rbacv1.Subject{{
			Kind: "User",
			Name: "viewer",
		}},
	})

	check := &ClusterAdminCheck{}
	findings, err := check.Run(context.Background(), client, "default")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("esperava zero findings, obteve %d", len(findings))
	}
}

// TestClusterAdminCheck_AllowlistPorSubject: um binding com nome de ferramenta conhecida
// ("helm-qualquer") que dá cluster-admin a default/app aparece; o binding da SA do Argo CD
// e o cluster-admin padrão ficam de fora e entram na contagem.
func TestClusterAdminCheck_AllowlistPorSubject(t *testing.T) {
	binding := func(nome string, s rbacv1.Subject) *rbacv1.ClusterRoleBinding {
		return &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: nome},
			RoleRef:    rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "ClusterRole", Name: "cluster-admin"},
			Subjects:   []rbacv1.Subject{s},
		}
	}
	client := fake.NewSimpleClientset(
		binding("helm-qualquer", rbacv1.Subject{Kind: "ServiceAccount", Name: "app", Namespace: "default"}),
		binding("argocd-application-controller", rbacv1.Subject{Kind: "ServiceAccount", Name: "argocd-application-controller", Namespace: "argocd"}),
		binding("cluster-admin", rbacv1.Subject{Kind: "Group", Name: "system:masters"}),
	)
	ctx, contagem := allowlist.ComContagem(context.Background())

	findings, err := (&ClusterAdminCheck{}).Run(ctx, client, "default")
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 || findings[0].Resource != "helm-qualquer" || findings[0].Severity != SeverityCritical {
		t.Errorf("esperava só o helm-qualquer, crítico: %+v", findings)
	}
	if contagem.Total() != 2 {
		t.Errorf("ignorados pela allowlist = %d, queria 2", contagem.Total())
	}
}
