//go:build k8s

// Package escopo diz que parte do RBAC do cluster um scan olha, para os checks internos,
// o OPA e o Polaris recortarem do mesmo jeito.
//
// No escopo namespace (o padrão do scan) só entra o RBAC que vale no namespace pedido:
//   - os Roles e os RoleBindings dele;
//   - os ClusterRoles a que os RoleBindings dele apontam;
//   - os ClusterRoleBindings que dão o papel a uma ServiceAccount do namespace, com os
//     subjects reduzidos aos do namespace (o resto é assunto do cluster), e os ClusterRoles
//     a que eles apontam.
//
// O resto do cluster (um cluster-admin dado a um usuário, um ClusterRole curinga sem
// binding no namespace) fica para o escopo cluster (--scope cluster), em que entra o RBAC
// do cluster todo. Sem isso, os mesmos achados se repetiam no scan de todo namespace.
package escopo

import (
	"context"
	"fmt"
	"strings"

	"github.com/casheiro/yby-cli/v5/plugins/sentinel/cli/allowlist"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Escopo é o alcance de um scan ou de um achado: "namespace" ou "cluster".
type Escopo string

const (
	// Namespace é o RBAC que vale no namespace pedido (e, num achado, um recurso do
	// namespace).
	Namespace Escopo = "namespace"
	// Cluster é o RBAC do cluster todo (e, num achado, um recurso sem namespace, como um
	// ClusterRole ou um ClusterRoleBinding).
	Cluster Escopo = "cluster"
)

type chave struct{}

// Com devolve um contexto que leva o escopo do scan para os backends e os checks.
func Com(ctx context.Context, e Escopo) context.Context {
	return context.WithValue(ctx, chave{}, e)
}

// De devolve o escopo do contexto; sem escopo, é Namespace.
func De(ctx context.Context) Escopo {
	if e, ok := ctx.Value(chave{}).(Escopo); ok && e == Cluster {
		return Cluster
	}
	return Namespace
}

// DoKind dá o escopo de um recurso de RBAC pelo kind: ClusterRole e ClusterRoleBinding
// valem no cluster todo; o resto, no namespace.
func DoKind(kind string) Escopo {
	switch kind {
	case "ClusterRole", "ClusterRoleBinding":
		return Cluster
	}
	return Namespace
}

// RBAC é o recorte dos bindings que o scan olha (ver o comentário do pacote).
type RBAC struct {
	escopo Escopo
	crbs   []rbacv1.ClusterRoleBinding
	rbs    []rbacv1.RoleBinding
	// clusterRoles e bindings são os ClusterRoles e os ClusterRoleBindings do recorte, no
	// escopo namespace.
	clusterRoles map[string]bool
	bindings     map[string]bool
}

// Recortar monta o recorte do escopo e a partir das listas de bindings. rbs pode trazer
// RoleBindings de qualquer namespace: no escopo namespace, só os do namespace contam.
func Recortar(e Escopo, namespace string, crbs []rbacv1.ClusterRoleBinding, rbs []rbacv1.RoleBinding) RBAC {
	if e == Cluster {
		return RBAC{escopo: Cluster, crbs: crbs, rbs: rbs}
	}
	r := RBAC{escopo: Namespace, clusterRoles: map[string]bool{}, bindings: map[string]bool{}}
	for _, b := range rbs {
		if b.Namespace != namespace {
			continue
		}
		r.rbs = append(r.rbs, b)
		if b.RoleRef.Kind == "ClusterRole" {
			r.clusterRoles[b.RoleRef.Name] = true
		}
	}
	for _, b := range crbs {
		var doNamespace []rbacv1.Subject
		for _, s := range b.Subjects {
			if SujeitoDoNamespace(s, namespace) {
				doNamespace = append(doNamespace, s)
			}
		}
		if len(doNamespace) == 0 {
			continue
		}
		b.Subjects = doNamespace
		r.crbs = append(r.crbs, b)
		r.bindings[b.Name] = true
		r.clusterRoles[b.RoleRef.Name] = true
	}
	return r
}

// SujeitoDoNamespace diz se s é uma ServiceAccount do namespace: pelo kind, pelo usuário
// system:serviceaccount:<ns>:<nome> ou pelo grupo system:serviceaccounts:<ns>, que junta
// todas as dele.
func SujeitoDoNamespace(s rbacv1.Subject, namespace string) bool {
	switch s.Kind {
	case rbacv1.ServiceAccountKind:
		return s.Namespace == namespace
	case rbacv1.UserKind:
		return strings.HasPrefix(s.Name, "system:serviceaccount:"+namespace+":")
	case rbacv1.GroupKind:
		return s.Name == "system:serviceaccounts:"+namespace
	}
	return false
}

// Ler lista os bindings e monta o recorte do escopo do contexto. No escopo namespace,
// basta ler os RoleBindings do namespace; no escopo cluster, os de todos.
func Ler(ctx context.Context, client kubernetes.Interface, namespace string) (RBAC, error) {
	e := De(ctx)
	crbs, err := client.RbacV1().ClusterRoleBindings().List(ctx, metav1.ListOptions{})
	if err != nil {
		return RBAC{}, fmt.Errorf("falha ao listar ClusterRoleBindings: %w", err)
	}
	nsDosRoleBindings := namespace
	if e == Cluster {
		nsDosRoleBindings = metav1.NamespaceAll
	}
	rbs, err := client.RbacV1().RoleBindings(nsDosRoleBindings).List(ctx, metav1.ListOptions{})
	if err != nil {
		return RBAC{}, fmt.Errorf("falha ao listar RoleBindings: %w", err)
	}
	return Recortar(e, namespace, crbs.Items, rbs.Items), nil
}

// ClusterRoleBindings devolve os ClusterRoleBindings do recorte; no escopo namespace, só
// com os subjects do namespace.
func (r RBAC) ClusterRoleBindings() []rbacv1.ClusterRoleBinding { return r.crbs }

// Vinculos devolve os subjects ligados a cada papel pelos bindings do recorte, para a
// allowlist: no escopo namespace, um ClusterRole é julgado por quem o recebe no namespace.
func (r RBAC) Vinculos() allowlist.Vinculos { return allowlist.NovosVinculos(r.crbs, r.rbs) }

// TemClusterRole diz se o ClusterRole entra no recorte.
func (r RBAC) TemClusterRole(nome string) bool {
	return r.escopo == Cluster || r.clusterRoles[nome]
}

// Inclui diz se um recurso sem namespace entra no recorte: um ClusterRole ou um
// ClusterRoleBinding do recorte. No escopo namespace, os outros recursos sem namespace
// ficam de fora.
func (r RBAC) Inclui(kind, nome string) bool {
	if r.escopo == Cluster {
		return true
	}
	switch kind {
	case "ClusterRole":
		return r.clusterRoles[nome]
	case "ClusterRoleBinding":
		return r.bindings[nome]
	}
	return false
}
