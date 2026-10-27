//go:build k8s

package allowlist

import (
	"context"
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func sa(ns, nome string) Sujeito { return Sujeito{Kind: "ServiceAccount", Nome: nome, Namespace: ns} }

// peloCluster marca o subject como vindo de um ClusterRoleBinding.
func peloCluster(s Sujeito) Sujeito {
	s.PeloCluster = true
	return s
}

func TestSujeitoPermitido(t *testing.T) {
	casos := []struct {
		nome string
		s    Sujeito
		quer bool
	}{
		{"system:masters", Sujeito{Kind: "Group", Nome: "system:masters"}, true},
		{"controller do Argo CD no namespace dele", sa("argocd", "argocd-application-controller"), true},
		{"SA do Helm do k3s", sa("kube-system", "helm-traefik"), true},
		{"nome de controller fora do namespace dele", sa("default", "argocd-application-controller"), false},
		{"SA qualquer", sa("default", "app"), false},
		{"SA qualquer do kube-system", sa("kube-system", "default"), false},
		{"system:authenticated", Sujeito{Kind: "Group", Nome: "system:authenticated"}, false},
		{"SA como usuário", Sujeito{Kind: "User", Nome: "system:serviceaccount:argocd:argocd-server"}, false},
		{"usuário do controller-manager não leva cluster-admin", Sujeito{Kind: "User", Nome: "system:kube-controller-manager"}, false},
	}
	for _, c := range casos {
		if got := Padrao.SujeitoPermitido(c.s); got != c.quer {
			t.Errorf("%s: SujeitoPermitido(%+v) = %v, queria %v", c.nome, c.s, got, c.quer)
		}
	}
}

func TestIgnorarPapel(t *testing.T) {
	casos := []struct {
		nome       string
		kind, papl string
		sujeitos   []Sujeito
		quer       bool
	}{
		{"admin do Kubernetes por RoleBinding", "ClusterRole", "admin", []Sujeito{sa("default", "app")}, true},
		{"admin do Kubernetes sem binding", "ClusterRole", "admin", nil, true},
		{"edit no cluster todo para uma SA qualquer", "ClusterRole", "edit", []Sujeito{peloCluster(sa("default", "app"))}, false},
		{"admin no cluster todo para system:authenticated", "ClusterRole", "admin", []Sujeito{peloCluster(Sujeito{Kind: "Group", Nome: "system:authenticated"})}, false},
		{"edit por RoleBinding e no cluster todo para uma SA qualquer", "ClusterRole", "edit", []Sujeito{sa("loja", "app"), peloCluster(sa("default", "app"))}, false},
		{"cluster-admin no cluster todo para system:masters", "ClusterRole", "cluster-admin", []Sujeito{peloCluster(Sujeito{Kind: "Group", Nome: "system:masters"})}, true},
		{"edit no cluster todo para uma SA do kube-system", "ClusterRole", "edit", []Sujeito{peloCluster(sa("kube-system", "x"))}, true},
		{"edit no cluster todo para o Argo CD, e por RoleBinding a uma SA qualquer", "ClusterRole", "edit", []Sujeito{peloCluster(sa("argocd", "argocd-server")), sa("loja", "app")}, true},
		{"Role chamado admin não é o do Kubernetes", "Role", "admin", nil, false},
		{"system: sem binding", "ClusterRole", "system:aggregate-to-edit", nil, true},
		{"system: ligado ao kube-system", "ClusterRole", "system:controller:generic-garbage-collector", []Sujeito{sa("kube-system", "generic-garbage-collector")}, true},
		{"system: ligado a uma SA qualquer", "ClusterRole", "system:x", []Sujeito{sa("default", "app")}, false},
		{"system: ligado a system:authenticated", "ClusterRole", "system:x", []Sujeito{{Kind: "Group", Nome: "system:authenticated"}}, false},
		{"role ligado só ao cert-manager, qualquer nome", "ClusterRole", "qualquer", []Sujeito{sa("cert-manager", "cert-manager")}, true},
		{"nome de controller ligado a uma SA qualquer", "ClusterRole", "argocd-server", []Sujeito{sa("argocd", "argocd-server"), sa("default", "app")}, false},
		{"nome de controller sem binding", "ClusterRole", "cert-manager-controller", nil, false},
		{"Role ligado ao Argo CD", "Role", "argocd-x", []Sujeito{sa("argocd", "argocd-server")}, true},
	}
	for _, c := range casos {
		if got := Padrao.IgnorarPapel(c.kind, c.papl, c.sujeitos); got != c.quer {
			t.Errorf("%s: IgnorarPapel = %v, queria %v", c.nome, got, c.quer)
		}
	}
}

func TestLerVinculos(t *testing.T) {
	client := fake.NewSimpleClientset(
		&rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "a"},
			RoleRef:    rbacv1.RoleRef{Kind: "ClusterRole", Name: "leitor"},
			Subjects:   []rbacv1.Subject{{Kind: "ServiceAccount", Name: "argocd-server", Namespace: "argocd"}},
		},
		&rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "b", Namespace: "loja"},
			RoleRef:    rbacv1.RoleRef{Kind: "ClusterRole", Name: "leitor"},
			Subjects:   []rbacv1.Subject{{Kind: "ServiceAccount", Name: "app", Namespace: "loja"}},
		},
		&rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "c", Namespace: "loja"},
			RoleRef:    rbacv1.RoleRef{Kind: "Role", Name: "local"},
			Subjects:   []rbacv1.Subject{{Kind: "User", Name: "ana"}},
		},
	)
	v, err := LerVinculos(context.Background(), client)
	if err != nil {
		t.Fatal(err)
	}
	// O RoleBinding de outro namespace que aponta para o ClusterRole conta: com ele, o
	// "leitor" deixa de estar ligado só ao Argo CD.
	got := v.DoClusterRole("leitor")
	if len(got) != 2 || Padrao.IgnorarPapel("ClusterRole", "leitor", got) {
		t.Errorf("subjects do ClusterRole leitor: %+v", got)
	}
	// Só o subject do ClusterRoleBinding vale no cluster todo.
	for _, s := range got {
		if s.PeloCluster != (s.Namespace == "argocd") {
			t.Errorf("PeloCluster de %+v: queria %v", s, s.Namespace == "argocd")
		}
	}
	if got := v.DoRole("loja", "local"); len(got) != 1 || got[0].Nome != "ana" || got[0].PeloCluster {
		t.Errorf("subjects do Role loja/local: %+v", got)
	}
	if got := v.DoRole("outro", "local"); len(got) != 0 {
		t.Errorf("Role de outro namespace não tem os subjects: %+v", got)
	}
}

func TestContagem(t *testing.T) {
	Ignorar(context.Background(), "ClusterRole/admin") // sem contagem no contexto: nada
	ctx, c := ComContagem(context.Background())
	Ignorar(ctx, "ClusterRole/admin")
	Ignorar(ctx, "ClusterRole/admin")
	Ignorar(ctx, "ClusterRoleBinding/cluster-admin")
	if c.Total() != 2 {
		t.Errorf("Total = %d, queria 2 (cada recurso conta uma vez)", c.Total())
	}
	var nada *Contagem
	if nada.Total() != 0 {
		t.Error("Contagem nula conta 0")
	}
}
