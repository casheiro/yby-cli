//go:build k8s

package escopo

import (
	"context"
	"slices"
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDe_PadraoENamespace(t *testing.T) {
	if De(context.Background()) != Namespace {
		t.Error("sem escopo no contexto, vale o namespace")
	}
	if De(Com(context.Background(), Cluster)) != Cluster {
		t.Error("o escopo cluster do contexto se perdeu")
	}
}

// Uma ServiceAccount do namespace é reconhecida pelo kind, pelo usuário
// system:serviceaccount:<ns>:<nome> e pelo grupo system:serviceaccounts:<ns>.
func TestSujeitoDoNamespace(t *testing.T) {
	casos := []struct {
		s    rbacv1.Subject
		quer bool
	}{
		{rbacv1.Subject{Kind: "ServiceAccount", Name: "x", Namespace: "payments"}, true},
		{rbacv1.Subject{Kind: "ServiceAccount", Name: "x", Namespace: "other"}, false},
		{rbacv1.Subject{Kind: "User", Name: "system:serviceaccount:payments:x"}, true},
		{rbacv1.Subject{Kind: "User", Name: "system:serviceaccount:payments-2:x"}, false},
		{rbacv1.Subject{Kind: "Group", Name: "system:serviceaccounts:payments"}, true},
		{rbacv1.Subject{Kind: "Group", Name: "system:serviceaccounts"}, false},
		{rbacv1.Subject{Kind: "User", Name: "bob"}, false},
	}
	for _, c := range casos {
		if got := SujeitoDoNamespace(c.s, "payments"); got != c.quer {
			t.Errorf("%+v: %v, queria %v", c.s, got, c.quer)
		}
	}
}

// No escopo namespace, o recorte guarda os RoleBindings do namespace, os
// ClusterRoleBindings com subjects dele (reduzidos a eles) e os ClusterRoles a que uns e
// outros apontam; no escopo cluster, tudo.
func TestRecortar(t *testing.T) {
	sa := func(ns string) rbacv1.Subject {
		return rbacv1.Subject{Kind: "ServiceAccount", Name: "app", Namespace: ns}
	}
	crbs := []rbacv1.ClusterRoleBinding{
		{ObjectMeta: metav1.ObjectMeta{Name: "misto"}, RoleRef: rbacv1.RoleRef{Kind: "ClusterRole", Name: "papel-misto"},
			Subjects: []rbacv1.Subject{{Kind: "User", Name: "bob"}, sa("payments")}},
		{ObjectMeta: metav1.ObjectMeta{Name: "de-outro"}, RoleRef: rbacv1.RoleRef{Kind: "ClusterRole", Name: "papel-de-outro"},
			Subjects: []rbacv1.Subject{sa("other")}},
	}
	rbs := []rbacv1.RoleBinding{
		{ObjectMeta: metav1.ObjectMeta{Name: "rb", Namespace: "payments"}, RoleRef: rbacv1.RoleRef{Kind: "ClusterRole", Name: "papel-do-rb"},
			Subjects: []rbacv1.Subject{sa("payments")}},
		{ObjectMeta: metav1.ObjectMeta{Name: "rb", Namespace: "loja"}, RoleRef: rbacv1.RoleRef{Kind: "ClusterRole", Name: "papel-da-loja"},
			Subjects: []rbacv1.Subject{sa("loja")}},
	}

	r := Recortar(Namespace, "payments", crbs, rbs)
	if got := r.ClusterRoleBindings(); len(got) != 1 || got[0].Name != "misto" || !slices.Equal(got[0].Subjects, []rbacv1.Subject{sa("payments")}) {
		t.Errorf("ClusterRoleBindings do recorte: %+v", got)
	}
	if len(crbs[0].Subjects) != 2 {
		t.Error("o recorte alterou a lista original")
	}
	for nome, quer := range map[string]bool{"papel-misto": true, "papel-do-rb": true, "papel-de-outro": false, "papel-da-loja": false} {
		if r.TemClusterRole(nome) != quer || r.Inclui("ClusterRole", nome) != quer {
			t.Errorf("ClusterRole %s no recorte: %v, queria %v", nome, r.TemClusterRole(nome), quer)
		}
	}
	if !r.Inclui("ClusterRoleBinding", "misto") || r.Inclui("ClusterRoleBinding", "de-outro") || r.Inclui("Node", "n1") {
		t.Error("Inclui: só os ClusterRoleBindings do recorte e nenhum outro recurso sem namespace")
	}
	if s := r.Vinculos().DoClusterRole("papel-misto"); len(s) != 1 || !s[0].PeloCluster {
		t.Errorf("os vínculos do papel-misto só têm a SA do namespace, pelo cluster: %+v", s)
	}

	todos := Recortar(Cluster, "payments", crbs, rbs)
	if len(todos.ClusterRoleBindings()) != 2 || !todos.TemClusterRole("papel-da-loja") || !todos.Inclui("Node", "n1") {
		t.Error("no escopo cluster, entra o RBAC do cluster todo")
	}
}
