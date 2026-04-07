//go:build k8s

package checks

import (
	"context"
	"testing"

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
	findings, err := check.Run(context.Background(), client, "default")
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
