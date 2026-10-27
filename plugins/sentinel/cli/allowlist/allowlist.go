//go:build k8s

// Package allowlist é a lista única do que o Sentinel deixa de fora nas verificações de
// RBAC: os checks internos a leem em Go e o OPA a recebe como data (data.allowlist).
//
// A lista casa pelo subject (quem recebe as permissões), nunca pelo nome do binding, e
// o nome do role sozinho nunca basta: qualquer um que cria um ClusterRoleBinding escolhe
// o nome dele, então um "helm-qualquer" que dá cluster-admin a default/app aparece como
// qualquer outro, e um ClusterRoleBinding que dá edit a default/app também. Os
// labels (app.kubernetes.io/managed-by e cia.) também ficam de fora pelo mesmo motivo:
// quem cria o binding põe o label que quiser.
package allowlist

import (
	"context"
	"slices"
	"strings"
	"sync"

	rbacv1 "k8s.io/api/rbac/v1"
)

// Controlador é uma ferramenta que precisa de permissões amplas por design: as
// ServiceAccounts dela ficam num dos Namespaces e o nome começa com PrefixoSA.
type Controlador struct {
	Nome       string
	Namespaces []string
	PrefixoSA  string
}

// Lista é a allowlist de RBAC.
type Lista struct {
	// PapeisDoKubernetes são os ClusterRoles de usuário que o apiserver cria e reconcilia
	// (admin, edit, view, cluster-admin): dar acesso a secrets é o papel deles. Ligados
	// por RoleBinding (o uso normal, valem num namespace só) ficam de fora; ligados por
	// ClusterRoleBinding (valem no cluster todo), só quando quem recebe é do plano de
	// controle ou permitido.
	PapeisDoKubernetes []string
	// PrefixoDoSistema marca os ClusterRoles do Kubernetes ("system:"). Eles só ficam de
	// fora enquanto estão ligados apenas ao plano de controle (ou a ninguém).
	PrefixoDoSistema string
	// GruposAdmin podem ter cluster-admin (system:masters é o admin padrão).
	GruposAdmin []string
	// GruposDoSistema e UsuariosDoSistema são as identidades do plano de controle que
	// recebem os ClusterRoles system:*. Lista fechada: system:authenticated,
	// system:anonymous e system:serviceaccount:<ns>:<nome> não entram.
	GruposDoSistema   []string
	UsuariosDoSistema []string
	// NamespaceDoSistema é o namespace das ServiceAccounts dos controllers do Kubernetes,
	// aceitas só nos ClusterRoles system:*.
	NamespaceDoSistema string
	// Controladores são as ferramentas conhecidas.
	Controladores []Controlador
}

// Padrao é a allowlist do Sentinel, a mesma para os checks internos e para o OPA.
var Padrao = Lista{
	PapeisDoKubernetes: []string{"admin", "edit", "view", "cluster-admin"},
	PrefixoDoSistema:   "system:",
	GruposAdmin:        []string{"system:masters"},
	GruposDoSistema:    []string{"system:masters", "system:nodes"},
	UsuariosDoSistema: []string{
		"system:kube-controller-manager", "system:kube-scheduler", "system:kube-proxy",
		"system:k3s-controller", "kube-apiserver",
	},
	NamespaceDoSistema: "kube-system",
	Controladores: []Controlador{
		{Nome: "Argo CD", Namespaces: []string{"argocd"}, PrefixoSA: "argocd-"},
		{Nome: "Argo Workflows", Namespaces: []string{"argo"}, PrefixoSA: "argo"},
		{Nome: "Argo Events", Namespaces: []string{"argo-events"}, PrefixoSA: "argo-events-"},
		{Nome: "cert-manager", Namespaces: []string{"cert-manager"}, PrefixoSA: "cert-manager"},
		{Nome: "Traefik", Namespaces: []string{"kube-system", "traefik"}, PrefixoSA: "traefik"},
		{Nome: "Helm do k3s", Namespaces: []string{"kube-system"}, PrefixoSA: "helm-"},
		{Nome: "local-path-provisioner", Namespaces: []string{"kube-system", "local-path-storage"}, PrefixoSA: "local-path-provisioner"},
		{Nome: "kube-prometheus-stack", Namespaces: []string{"monitoring"}, PrefixoSA: "monitoring-"},
		{Nome: "Loki", Namespaces: []string{"loki", "monitoring"}, PrefixoSA: "loki"},
		{Nome: "CloudNativePG", Namespaces: []string{"cnpg-system"}, PrefixoSA: "cnpg-"},
		{Nome: "KEDA", Namespaces: []string{"keda"}, PrefixoSA: "keda-"},
	},
}

// Sujeito é quem recebe as permissões de um binding (rbacv1.Subject).
type Sujeito struct {
	Kind, Nome, Namespace string
	// PeloCluster diz que o subject recebe o papel por um ClusterRoleBinding, no cluster
	// todo; falso quando vem de um RoleBinding (vale só no namespace dele).
	PeloCluster bool
}

// SujeitoPermitido diz se s pode ter qualquer permissão, inclusive cluster-admin: um
// grupo admin ou uma ServiceAccount de um controller conhecido, no namespace dele.
func (l Lista) SujeitoPermitido(s Sujeito) bool {
	switch s.Kind {
	case rbacv1.GroupKind:
		return slices.Contains(l.GruposAdmin, s.Nome)
	case rbacv1.ServiceAccountKind:
		for _, c := range l.Controladores {
			if slices.Contains(c.Namespaces, s.Namespace) && strings.HasPrefix(s.Nome, c.PrefixoSA) {
				return true
			}
		}
	}
	return false
}

// doPlanoDeControle diz se s é do plano de controle: os permitidos, as identidades do
// sistema e as ServiceAccounts do namespace do sistema.
func (l Lista) doPlanoDeControle(s Sujeito) bool {
	if l.SujeitoPermitido(s) {
		return true
	}
	switch s.Kind {
	case rbacv1.GroupKind:
		return slices.Contains(l.GruposDoSistema, s.Nome)
	case rbacv1.UserKind:
		return slices.Contains(l.UsuariosDoSistema, s.Nome)
	case rbacv1.ServiceAccountKind:
		return s.Namespace == l.NamespaceDoSistema
	}
	return false
}

// ligacaoAceitaEmPapelDoKubernetes diz se s pode receber um ClusterRole de
// PapeisDoKubernetes: por RoleBinding, qualquer um; por ClusterRoleBinding, só o plano de
// controle e os permitidos.
func (l Lista) ligacaoAceitaEmPapelDoKubernetes(s Sujeito) bool {
	return !s.PeloCluster || l.doPlanoDeControle(s)
}

// IgnorarPapel diz se um Role ou ClusterRole com permissões perigosas fica de fora,
// dados os subjects de todos os bindings que apontam para ele. Fica de fora:
//   - um ClusterRole de PapeisDoKubernetes ligado por ClusterRoleBinding só ao plano de
//     controle (ou a ninguém); por RoleBinding, a qualquer um;
//   - um ClusterRole system:* ligado só ao plano de controle (ou a ninguém);
//   - qualquer role ligado a pelo menos um subject, todos eles permitidos.
//
// Um role sem binding que não é do sistema continua aparecendo, como antes.
func (l Lista) IgnorarPapel(kind, nome string, sujeitos []Sujeito) bool {
	if kind == "ClusterRole" {
		if slices.Contains(l.PapeisDoKubernetes, nome) && todos(sujeitos, l.ligacaoAceitaEmPapelDoKubernetes) {
			return true
		}
		if strings.HasPrefix(nome, l.PrefixoDoSistema) && todos(sujeitos, l.doPlanoDeControle) {
			return true
		}
	}
	return len(sujeitos) > 0 && todos(sujeitos, l.SujeitoPermitido)
}

func todos(sujeitos []Sujeito, ok func(Sujeito) bool) bool {
	for _, s := range sujeitos {
		if !ok(s) {
			return false
		}
	}
	return true
}

// ParaRego devolve a lista no formato de data.allowlist das políticas Rego.
func (l Lista) ParaRego() map[string]interface{} {
	controladores := make([]interface{}, 0, len(l.Controladores))
	for _, c := range l.Controladores {
		controladores = append(controladores, map[string]interface{}{
			"nome":       c.Nome,
			"namespaces": paraInterface(c.Namespaces),
			"prefixo_sa": c.PrefixoSA,
		})
	}
	return map[string]interface{}{
		"papeis_do_kubernetes": paraInterface(l.PapeisDoKubernetes),
		"prefixo_do_sistema":   l.PrefixoDoSistema,
		"grupos_admin":         paraInterface(l.GruposAdmin),
		"grupos_do_sistema":    paraInterface(l.GruposDoSistema),
		"usuarios_do_sistema":  paraInterface(l.UsuariosDoSistema),
		"namespace_do_sistema": l.NamespaceDoSistema,
		"controladores":        controladores,
	}
}

func paraInterface(ss []string) []interface{} {
	r := make([]interface{}, len(ss))
	for i, s := range ss {
		r[i] = s
	}
	return r
}

// SujeitosDe converte os subjects de um binding.
func SujeitosDe(subjects []rbacv1.Subject) []Sujeito {
	r := make([]Sujeito, 0, len(subjects))
	for _, s := range subjects {
		r = append(r, Sujeito{Kind: s.Kind, Nome: s.Name, Namespace: s.Namespace})
	}
	return r
}

// Vinculos guarda, para cada Role e ClusterRole, os subjects de todos os bindings que
// apontam para ele: ClusterRoleBindings e RoleBindings de qualquer namespace (um
// RoleBinding também pode apontar para um ClusterRole). Os que vêm de um
// ClusterRoleBinding saem com PeloCluster.
type Vinculos struct {
	m map[string][]Sujeito
}

// NovosVinculos monta os vínculos a partir das listas de bindings.
func NovosVinculos(crbs []rbacv1.ClusterRoleBinding, rbs []rbacv1.RoleBinding) Vinculos {
	v := Vinculos{m: map[string][]Sujeito{}}
	for _, b := range crbs {
		chave := chaveClusterRole(b.RoleRef.Name)
		for _, s := range SujeitosDe(b.Subjects) {
			s.PeloCluster = true
			v.m[chave] = append(v.m[chave], s)
		}
	}
	for _, b := range rbs {
		chave := chaveClusterRole(b.RoleRef.Name)
		if b.RoleRef.Kind == "Role" {
			chave = chaveRole(b.Namespace, b.RoleRef.Name)
		}
		v.m[chave] = append(v.m[chave], SujeitosDe(b.Subjects)...)
	}
	return v
}

// DoClusterRole devolve os subjects ligados ao ClusterRole.
func (v Vinculos) DoClusterRole(nome string) []Sujeito { return v.m[chaveClusterRole(nome)] }

// DoRole devolve os subjects ligados ao Role do namespace.
func (v Vinculos) DoRole(namespace, nome string) []Sujeito { return v.m[chaveRole(namespace, nome)] }

func chaveClusterRole(nome string) string     { return "ClusterRole/" + nome }
func chaveRole(namespace, nome string) string { return "Role/" + namespace + "/" + nome }

// Contagem junta os recursos que a allowlist deixou de fora num scan. Um recurso conta
// uma vez só, mesmo que o OPA e os checks internos (ou duas políticas) o ignorem.
type Contagem struct {
	mu     sync.Mutex
	vistos map[string]bool
}

type chaveContagem struct{}

// ComContagem devolve um contexto que leva uma Contagem nova; os backends e os checks
// registram nela o que ignoraram (Ignorar).
func ComContagem(ctx context.Context) (context.Context, *Contagem) {
	c := &Contagem{vistos: map[string]bool{}}
	return context.WithValue(ctx, chaveContagem{}, c), c
}

// Ignorar registra que a allowlist deixou o recurso (ex.: "ClusterRole/admin") de fora.
// Sem Contagem no contexto, não faz nada.
func Ignorar(ctx context.Context, recurso string) {
	c, ok := ctx.Value(chaveContagem{}).(*Contagem)
	if !ok || c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.vistos[recurso] = true
}

// Total devolve quantos recursos distintos foram ignorados.
func (c *Contagem) Total() int {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.vistos)
}
