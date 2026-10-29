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
	findings, err := check.Run(doCluster(context.Background()), client, "default")
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

// TestSecretsAccessCheck_RoleDoNamespaceLigadoAoControlador: um Role do namespace que lê
// secrets e está ligado só a uma SA de controller conhecido fica de fora e é contado.
func TestSecretsAccessCheck_RoleDoNamespaceLigadoAoControlador(t *testing.T) {
	client := fake.NewSimpleClientset(
		&rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{Name: "leitor", Namespace: "cert-manager"},
			Rules:      []rbacv1.PolicyRule{{Verbs: []string{"get"}, Resources: []string{"secrets"}, APIGroups: []string{""}}},
		},
		&rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "leitor", Namespace: "cert-manager"},
			RoleRef:    rbacv1.RoleRef{Kind: "Role", Name: "leitor"},
			Subjects:   []rbacv1.Subject{{Kind: "ServiceAccount", Name: "cert-manager", Namespace: "cert-manager"}},
		},
	)
	ctx, contagem := allowlist.ComContagem(context.Background())

	findings, err := (&SecretsAccessCheck{}).Run(ctx, client, "cert-manager")
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 || contagem.Total() != 1 {
		t.Errorf("esperava o Role ignorado e contado: %+v, %d ignorados", findings, contagem.Total())
	}
}

// TestSecretsAccessCheck_PapelDoKubernetesNoClusterTodo: edit ligado por RoleBinding (vale
// num namespace só) fica de fora e é contado; o mesmo edit dado a default/app por um
// ClusterRoleBinding deixa a SA ler os secrets do cluster todo e aparece, sem contar.
func TestSecretsAccessCheck_PapelDoKubernetesNoClusterTodo(t *testing.T) {
	edit := func() *rbacv1.ClusterRole {
		return &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{Name: "edit"},
			Rules:      []rbacv1.PolicyRule{{Verbs: []string{"get", "list", "watch"}, Resources: []string{"secrets"}, APIGroups: []string{""}}},
		}
	}
	app := rbacv1.Subject{Kind: "ServiceAccount", Name: "app", Namespace: "default"}
	porRoleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "app-edit", Namespace: "default"},
		RoleRef:    rbacv1.RoleRef{Kind: "ClusterRole", Name: "edit"},
		Subjects:   []rbacv1.Subject{app},
	}

	t.Run("por RoleBinding", func(t *testing.T) {
		ctx, contagem := allowlist.ComContagem(context.Background())
		findings, err := (&SecretsAccessCheck{}).Run(ctx, fake.NewSimpleClientset(edit(), porRoleBinding), "default")
		if err != nil {
			t.Fatal(err)
		}
		if len(findings) != 0 || contagem.Total() != 1 {
			t.Errorf("esperava o edit ignorado e contado: %+v, %d ignorados", findings, contagem.Total())
		}
	})

	t.Run("por ClusterRoleBinding", func(t *testing.T) {
		client := fake.NewSimpleClientset(edit(), porRoleBinding, &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "app-edit-tudo"},
			RoleRef:    rbacv1.RoleRef{Kind: "ClusterRole", Name: "edit"},
			Subjects:   []rbacv1.Subject{app},
		})
		ctx, contagem := allowlist.ComContagem(context.Background())
		findings, err := (&SecretsAccessCheck{}).Run(ctx, client, "default")
		if err != nil {
			t.Fatal(err)
		}
		if len(findings) != 1 || findings[0].Resource != "ClusterRole/edit" || contagem.Total() != 0 {
			t.Errorf("esperava achado no ClusterRole/edit e nada ignorado: %+v, %d ignorados", findings, contagem.Total())
		}
	})
}
