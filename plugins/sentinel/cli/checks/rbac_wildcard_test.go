//go:build k8s

package checks

import (
	"context"
	"testing"

	"github.com/casheiro/yby-cli/v5/plugins/sentinel/cli/allowlist"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestWildcardRBACCheck_ClusterRoleComWildcard(t *testing.T) {
	client := fake.NewSimpleClientset(&rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: "super-role"},
		Rules: []rbacv1.PolicyRule{{
			Verbs:     []string{"*"},
			Resources: []string{"pods"},
			APIGroups: []string{""},
		}},
	})

	check := &WildcardRBACCheck{}
	findings, err := check.Run(doCluster(context.Background()), client, "default")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(findings) == 0 {
		t.Fatal("esperava finding para ClusterRole com verbs wildcard")
	}
	if findings[0].Severity != SeverityHigh {
		t.Errorf("esperava severity high, obteve %s", findings[0].Severity)
	}
}

func TestWildcardRBACCheck_RoleComResourceWildcard(t *testing.T) {
	client := fake.NewSimpleClientset(&rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{Name: "wide-role", Namespace: "default"},
		Rules: []rbacv1.PolicyRule{{
			Verbs:     []string{"get"},
			Resources: []string{"*"},
			APIGroups: []string{""},
		}},
	})

	check := &WildcardRBACCheck{}
	findings, err := check.Run(context.Background(), client, "default")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(findings) == 0 {
		t.Fatal("esperava finding para Role com resources wildcard")
	}
}

func TestWildcardRBACCheck_SemWildcard(t *testing.T) {
	client := fake.NewSimpleClientset(&rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: "restricted-role"},
		Rules: []rbacv1.PolicyRule{{
			Verbs:     []string{"get", "list"},
			Resources: []string{"pods"},
			APIGroups: []string{""},
		}},
	})

	check := &WildcardRBACCheck{}
	findings, err := check.Run(context.Background(), client, "default")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("esperava zero findings, obteve %d", len(findings))
	}
}

// TestWildcardRBACCheck_AllowlistPorSubject: o role curinga ligado só à SA do Argo CD fica
// de fora e é contado; o mesmo nome ligado também a uma SA qualquer (por um RoleBinding de
// outro namespace) aparece, e o nome do role não decide nada.
func TestWildcardRBACCheck_AllowlistPorSubject(t *testing.T) {
	curinga := func(nome string) *rbacv1.ClusterRole {
		return &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{Name: nome},
			Rules:      []rbacv1.PolicyRule{{Verbs: []string{"*"}, Resources: []string{"*"}, APIGroups: []string{"*"}}},
		}
	}
	argo := rbacv1.Subject{Kind: "ServiceAccount", Name: "argocd-server", Namespace: "argocd"}
	client := fake.NewSimpleClientset(
		curinga("argocd-server"),
		curinga("argocd-maligno"),
		&rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "argocd-server"},
			RoleRef:    rbacv1.RoleRef{Kind: "ClusterRole", Name: "argocd-server"},
			Subjects:   []rbacv1.Subject{argo},
		},
		&rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "argocd-maligno"},
			RoleRef:    rbacv1.RoleRef{Kind: "ClusterRole", Name: "argocd-maligno"},
			Subjects:   []rbacv1.Subject{argo},
		},
		&rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "qualquer", Namespace: "loja"},
			RoleRef:    rbacv1.RoleRef{Kind: "ClusterRole", Name: "argocd-maligno"},
			Subjects:   []rbacv1.Subject{{Kind: "ServiceAccount", Name: "app", Namespace: "loja"}},
		},
	)
	ctx, contagem := allowlist.ComContagem(doCluster(context.Background()))

	findings, err := (&WildcardRBACCheck{}).Run(ctx, client, "default")
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 || findings[0].Resource != "ClusterRole/argocd-maligno" {
		t.Errorf("esperava só o argocd-maligno: %+v", findings)
	}
	if contagem.Total() != 1 {
		t.Errorf("ignorados pela allowlist = %d, queria 1", contagem.Total())
	}
}

// TestWildcardRBACCheck_SoORBACDoNamespace: no scan de payments, o ClusterRole curinga sem
// binding e o ligado por RoleBinding de outro namespace ficam de fora; o ligado por
// RoleBinding de payments aparece, com escopo cluster.
func TestWildcardRBACCheck_SoORBACDoNamespace(t *testing.T) {
	curinga := func(nome string) *rbacv1.ClusterRole {
		return &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{Name: nome},
			Rules:      []rbacv1.PolicyRule{{Verbs: []string{"*"}, Resources: []string{"pods"}, APIGroups: []string{""}}},
		}
	}
	rb := func(ns, papel string) *rbacv1.RoleBinding {
		return &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: ns},
			RoleRef:    rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "ClusterRole", Name: papel},
			Subjects:   []rbacv1.Subject{{Kind: "ServiceAccount", Name: "app", Namespace: ns}},
		}
	}
	client := fake.NewSimpleClientset(curinga("solto"), curinga("da-loja"), curinga("do-payments"),
		rb("loja", "da-loja"), rb("payments", "do-payments"))

	findings, err := (&WildcardRBACCheck{}).Run(context.Background(), client, "payments")
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 || findings[0].Resource != "ClusterRole/do-payments" || findings[0].Scope != "cluster" {
		t.Errorf("esperava só o ClusterRole ligado em payments, com escopo cluster: %+v", findings)
	}
}
