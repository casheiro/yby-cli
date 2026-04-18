//go:build k8s

package checks

import (
	"context"
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestSecretsAccessCheck_ComAcesso(t *testing.T) {
	client := fake.NewSimpleClientset(&rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: "secrets-reader"},
		Rules: []rbacv1.PolicyRule{{
			Verbs:     []string{"get", "list"},
			Resources: []string{"secrets"},
			APIGroups: []string{""},
		}},
	})

	check := &SecretsAccessCheck{}
	findings, err := check.Run(context.Background(), client, "default")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(findings) == 0 {
		t.Fatal("esperava finding para role com acesso a secrets")
	}
	if findings[0].Severity != SeverityHigh {
		t.Errorf("esperava severity high, obteve %s", findings[0].Severity)
	}
}

func TestSecretsAccessCheck_SemAcesso(t *testing.T) {
	client := fake.NewSimpleClientset(&rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: "pods-reader"},
		Rules: []rbacv1.PolicyRule{{
			Verbs:     []string{"get", "list"},
			Resources: []string{"pods"},
			APIGroups: []string{""},
		}},
	})

	check := &SecretsAccessCheck{}
	findings, err := check.Run(context.Background(), client, "default")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("esperava zero findings, obteve %d", len(findings))
	}
}

func TestSecretsAccessCheck_RoleNoNamespace(t *testing.T) {
	client := fake.NewSimpleClientset(&rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{Name: "ns-secrets-reader", Namespace: "default"},
		Rules: []rbacv1.PolicyRule{{
			Verbs:     []string{"get"},
			Resources: []string{"secrets"},
			APIGroups: []string{""},
		}},
	})

	check := &SecretsAccessCheck{}
	findings, err := check.Run(context.Background(), client, "default")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(findings) == 0 {
		t.Fatal("esperava finding para Role com acesso a secrets no namespace")
	}
}

func TestHasSecretsAccess_WildcardVerb(t *testing.T) {
	rules := []rbacv1.PolicyRule{{
		Verbs:     []string{"*"},
		Resources: []string{"secrets"},
		APIGroups: []string{""},
	}}
	if !hasSecretsAccess(rules) {
		t.Error("esperava true para wildcard verb em secrets")
	}
}

func TestHasSecretsAccess_WildcardResource(t *testing.T) {
	rules := []rbacv1.PolicyRule{{
		Verbs:     []string{"get"},
		Resources: []string{"*"},
		APIGroups: []string{""},
	}}
	if !hasSecretsAccess(rules) {
		t.Error("esperava true para wildcard resource com get")
	}
}
