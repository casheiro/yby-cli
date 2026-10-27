//go:build k8s

package backends

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/casheiro/yby-cli/plugins/sentinel/cli/allowlist"
	"github.com/casheiro/yby-cli/plugins/sentinel/cli/escopo"
	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/open-policy-agent/opa/v1/storage/inmem"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// regoAllowlist são as regras da allowlist, coladas no fim das políticas de RBAC. A lista
// vem de data.allowlist (allowlist.Padrao.ParaRego), a mesma que os checks internos leem
// em Go (allowlist.Lista): casa pelo subject, nunca pelo nome do binding ou do role.
const regoAllowlist = `
# sujeito_permitido: pode ter qualquer permissão (grupo admin ou SA de um controller
# conhecido, no namespace dele).
sujeito_permitido(s) if {
	s.kind == "Group"
	s.name in data.allowlist.grupos_admin
}

sujeito_permitido(s) if {
	s.kind == "ServiceAccount"
	some c in data.allowlist.controladores
	s.namespace in c.namespaces
	startswith(s.name, c.prefixo_sa)
}

# do_plano_de_controle: os permitidos, as identidades do sistema e as SAs do kube-system.
do_plano_de_controle(s) if sujeito_permitido(s)

do_plano_de_controle(s) if {
	s.kind == "Group"
	s.name in data.allowlist.grupos_do_sistema
}

do_plano_de_controle(s) if {
	s.kind == "User"
	s.name in data.allowlist.usuarios_do_sistema
}

do_plano_de_controle(s) if {
	s.kind == "ServiceAccount"
	s.namespace == data.allowlist.namespace_do_sistema
}

sujeitos(papel) := object.get(papel, "subjects", [])

# ligacao_aceita_em_papel_do_kubernetes: admin, edit, view e cluster-admin por
# RoleBinding (vale num namespace só) a qualquer um; por ClusterRoleBinding
# (pelo_cluster), só ao plano de controle.
ligacao_aceita_em_papel_do_kubernetes(s) if not s.pelo_cluster

ligacao_aceita_em_papel_do_kubernetes(s) if do_plano_de_controle(s)

# papel_permitido: ClusterRole do Kubernetes sem ClusterRoleBinding para fora do plano de
# controle, ClusterRole system:* ligado só ao plano de controle (ou a ninguém), ou role
# ligado só a subjects permitidos.
papel_permitido(papel) if {
	papel.kind == "ClusterRole"
	papel.metadata.name in data.allowlist.papeis_do_kubernetes
	every s in sujeitos(papel) {
		ligacao_aceita_em_papel_do_kubernetes(s)
	}
}

papel_permitido(papel) if {
	papel.kind == "ClusterRole"
	startswith(papel.metadata.name, data.allowlist.prefixo_do_sistema)
	every s in sujeitos(papel) {
		do_plano_de_controle(s)
	}
}

papel_permitido(papel) if {
	count(sujeitos(papel)) > 0
	every s in sujeitos(papel) {
		sujeito_permitido(s)
	}
}
`

// regoClusterAdminPolicy detecta ClusterRoleBindings que concedem cluster-admin a
// subjects fora da allowlist. ignorado diz que o binding só dá cluster-admin a subjects
// da allowlist.
const regoClusterAdminPolicy = `
package sentinel

import rego.v1

violation contains result if {
	binding := input
	binding.roleRef.name == "cluster-admin"
	some subject in binding.subjects
	not sujeito_permitido(subject)
	result := {
		"msg": sprintf("ClusterRoleBinding '%s' concede cluster-admin a %s '%s'", [binding.metadata.name, subject.kind, subject.name]),
		"severity": "critical",
	}
}

ignorado if {
	input.roleRef.name == "cluster-admin"
	count(input.subjects) > 0
	every subject in input.subjects {
		sujeito_permitido(subject)
	}
}
` + regoAllowlist

// regoSecretsAccessPolicy detecta ClusterRoles/Roles que permitem leitura de secrets,
// fora da allowlist. O input traz em subjects quem está ligado ao role.
const regoSecretsAccessPolicy = `
package sentinel

import rego.v1

read_verbs := {"get", "list", "watch", "*"}

has_secrets_access(rules) if {
	some rule in rules
	some res in rule.resources
	res in {"secrets", "*"}
	some verb in rule.verbs
	verb in read_verbs
}

violation contains result if {
	role := input
	not papel_permitido(role)
	has_secrets_access(role.rules)
	result := {
		"msg": sprintf("%s '%s' permite leitura de secrets", [role.kind, role.metadata.name]),
		"severity": "high",
	}
}

ignorado if {
	papel_permitido(input)
	has_secrets_access(input.rules)
}
` + regoAllowlist

// regoWildcardPolicy detecta Roles/ClusterRoles com permissões wildcard (*), fora da
// allowlist.
const regoWildcardPolicy = `
package sentinel

import rego.v1

has_wildcard(rules) if {
	some rule in rules
	some verb in rule.verbs
	verb == "*"
}

has_wildcard(rules) if {
	some rule in rules
	some res in rule.resources
	res == "*"
}

violation contains result if {
	role := input
	not papel_permitido(role)
	has_wildcard(role.rules)
	result := {
		"msg": sprintf("%s '%s' possui permissões wildcard (*)", [role.kind, role.metadata.name]),
		"severity": "high",
	}
}

ignorado if {
	papel_permitido(input)
	has_wildcard(input.rules)
}
` + regoAllowlist

// regoDefaultDenyPolicy detecta namespaces sem NetworkPolicy default-deny.
// Input: objeto com "namespace" (string) e "policies" (lista de NetworkPolicies).
const regoDefaultDenyPolicy = `
package sentinel

import rego.v1

is_default_deny_ingress(np) if {
	count(np.spec.podSelector.matchLabels) == 0
	not np.spec.podSelector.matchExpressions
	some pt in np.spec.policyTypes
	pt == "Ingress"
	count(np.spec.ingress) == 0
}

is_default_deny_ingress(np) if {
	count(np.spec.podSelector.matchLabels) == 0
	np.spec.podSelector.matchExpressions
	count(np.spec.podSelector.matchExpressions) == 0
	some pt in np.spec.policyTypes
	pt == "Ingress"
	count(np.spec.ingress) == 0
}

is_default_deny_egress(np) if {
	count(np.spec.podSelector.matchLabels) == 0
	not np.spec.podSelector.matchExpressions
	some pt in np.spec.policyTypes
	pt == "Egress"
	count(np.spec.egress) == 0
}

is_default_deny_egress(np) if {
	count(np.spec.podSelector.matchLabels) == 0
	np.spec.podSelector.matchExpressions
	count(np.spec.podSelector.matchExpressions) == 0
	some pt in np.spec.policyTypes
	pt == "Egress"
	count(np.spec.egress) == 0
}

has_ingress_deny if {
	some np in input.policies
	is_default_deny_ingress(np)
}

has_egress_deny if {
	some np in input.policies
	is_default_deny_egress(np)
}

violation contains result if {
	not has_ingress_deny
	result := {
		"msg": sprintf("Namespace '%s' não possui NetworkPolicy default-deny para Ingress", [input.namespace]),
		"severity": "medium",
	}
}

violation contains result if {
	not has_egress_deny
	result := {
		"msg": sprintf("Namespace '%s' não possui NetworkPolicy default-deny para Egress", [input.namespace]),
		"severity": "medium",
	}
}
`

// OPABackend implementa SecurityBackend usando Open Policy Agent para
// avaliar políticas Rego embarcadas contra recursos Kubernetes.
type OPABackend struct{}

// NewOPABackend cria uma nova instância do OPABackend.
func NewOPABackend() *OPABackend {
	return &OPABackend{}
}

// Name retorna o identificador do backend.
func (o *OPABackend) Name() string {
	return "opa"
}

// IsAvailable retorna true pois o OPA é embarcado (não depende de binário externo).
func (o *OPABackend) IsAvailable() bool {
	return true
}

// ScanCluster escaneia o cluster avaliando políticas Rego contra recursos K8s. Quando
// nenhuma política consegue ler o cluster (fora do ar, sem permissão), devolve a primeira
// falha; quando só algumas falham, devolve os achados das outras e uma *FalhaParcial.
// As falhas vão para o log de depuração, nunca para a tela: quem chama decide como
// mostrá-las. O RBAC olhado é o do escopo do contexto (escopo.De): no escopo namespace,
// só o que vale no namespace.
func (o *OPABackend) ScanCluster(ctx context.Context, client kubernetes.Interface, namespace string) ([]Finding, error) {
	clusterRoles := novaListaClusterRoles(ctx, client)
	// O recorte do RBAC (escopo.Ler) diz quais bindings e ClusterRoles entram: no escopo
	// namespace, só o que vale no namespace; os subjects de cada papel, para a allowlist,
	// vêm dos bindings do recorte.
	recorte := sync.OnceValues(func() (escopo.RBAC, error) { return escopo.Ler(ctx, client, namespace) })
	roles := sync.OnceValues(func() ([]rbacv1.Role, error) {
		lista, err := client.RbacV1().Roles(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("falha ao listar Roles: %w", err)
		}
		return lista.Items, nil
	})
	papeis := sync.OnceValues(func() ([]papelParaAvaliar, error) {
		return papeisDoRecorte(namespace, clusterRoles, roles, recorte)
	})
	politicas := []struct {
		nome  string
		rodar func() ([]Finding, error)
	}{
		{"cluster-admin", func() ([]Finding, error) { return o.scanClusterAdmin(ctx, recorte) }},
		{"acesso a secrets", func() ([]Finding, error) {
			return avaliarPapeis(ctx, regoSecretsAccessPolicy, papeis, "opa/rbac_secrets_access",
				"Restrinja acesso a secrets apenas aos serviços que realmente precisam")
		}},
		{"wildcards", func() ([]Finding, error) {
			return avaliarPapeis(ctx, regoWildcardPolicy, papeis, "opa/rbac_wildcard",
				"Substitua wildcards por verbos e recursos específicos")
		}},
		{"default-deny", func() ([]Finding, error) { return o.scanDefaultDeny(ctx, client, namespace) }},
	}

	var findings []Finding
	var falhas []string
	var primeira error
	for _, p := range politicas {
		achados, err := p.rodar()
		if err != nil {
			slog.Debug("opa: a política não rodou", "politica", p.nome, "erro", err)
			falhas = append(falhas, p.nome)
			if primeira == nil {
				primeira = err
			}
			continue
		}
		findings = append(findings, achados...)
	}
	switch {
	case len(falhas) == len(politicas):
		return nil, primeira
	case len(falhas) > 0:
		return findings, &FalhaParcial{Falhas: falhas, Causa: primeira}
	}
	return findings, nil
}

// scanClusterAdmin avalia a política de cluster-admin contra os ClusterRoleBindings do
// recorte (no escopo namespace, só com os subjects do namespace).
func (o *OPABackend) scanClusterAdmin(ctx context.Context, recorte func() (escopo.RBAC, error)) ([]Finding, error) {
	rec, err := recorte()
	if err != nil {
		return nil, err
	}

	var findings []Finding
	for _, binding := range rec.ClusterRoleBindings() {
		input := bindingToInput(binding)

		r, err := avaliarPolitica(ctx, regoClusterAdminPolicy, input)
		if err != nil {
			slog.Debug("opa: falha ao avaliar política cluster-admin", "binding", binding.Name, "error", err)
			continue
		}
		if r.ignorado {
			allowlist.Ignorar(ctx, "ClusterRoleBinding/"+binding.Name)
		}

		for _, v := range r.violacoes {
			findings = append(findings, Finding{
				ID:             "opa/rbac_cluster_admin",
				Source:         "opa",
				Severity:       v.severity,
				Category:       "rbac",
				Resource:       fmt.Sprintf("ClusterRoleBinding/%s", binding.Name),
				Message:        v.msg,
				Recommendation: "Substitua cluster-admin por roles mais restritivos seguindo o princípio do menor privilégio",
				Scope:          string(escopo.Cluster),
			})
		}
	}

	return findings, nil
}

// papeisDoRecorte junta os papéis que as políticas de secrets e de wildcard avaliam: os
// ClusterRoles do recorte e os Roles do namespace, cada um com os subjects ligados a ele
// pelos bindings do recorte.
func papeisDoRecorte(namespace string, clusterRoles listaClusterRoles, roles func() ([]rbacv1.Role, error), recorte func() (escopo.RBAC, error)) ([]papelParaAvaliar, error) {
	itensCR, err := clusterRoles()
	if err != nil {
		return nil, err
	}
	r, err := recorte()
	if err != nil {
		return nil, err
	}
	itensRoles, err := roles()
	if err != nil {
		return nil, err
	}
	v := r.Vinculos()

	var papeis []papelParaAvaliar
	for _, role := range itensCR {
		if r.TemClusterRole(role.Name) {
			papeis = append(papeis, papelParaAvaliar{"ClusterRole", role.Name, "", role.Rules, v.DoClusterRole(role.Name)})
		}
	}
	for _, role := range itensRoles {
		papeis = append(papeis, papelParaAvaliar{"Role", role.Name, namespace, role.Rules, v.DoRole(namespace, role.Name)})
	}
	return papeis, nil
}

// papelParaAvaliar é um Role ou ClusterRole com os subjects ligados a ele.
type papelParaAvaliar struct {
	kind, nome, namespace string
	rules                 []rbacv1.PolicyRule
	sujeitos              []allowlist.Sujeito
}

// avaliarPapeis avalia a política contra cada papel: as violações viram achados com o id
// e a recomendação dados, e o que a allowlist deixou de fora entra na contagem do
// contexto.
func avaliarPapeis(ctx context.Context, policy string, lerPapeis func() ([]papelParaAvaliar, error), id, recomendacao string) ([]Finding, error) {
	papeis, err := lerPapeis()
	if err != nil {
		return nil, err
	}
	var findings []Finding
	for _, p := range papeis {
		recurso := p.kind + "/" + p.nome
		r, err := avaliarPolitica(ctx, policy, roleToInput(p.kind, p.nome, p.namespace, p.rules, p.sujeitos))
		if err != nil {
			slog.Debug("opa: falha ao avaliar política", "id", id, "role", recurso, "error", err)
			continue
		}
		if r.ignorado {
			chave := recurso
			if p.namespace != "" {
				chave = p.kind + "/" + p.namespace + "/" + p.nome
			}
			allowlist.Ignorar(ctx, chave)
		}
		for _, v := range r.violacoes {
			findings = append(findings, Finding{
				ID:             id,
				Source:         "opa",
				Severity:       v.severity,
				Category:       "rbac",
				Namespace:      p.namespace,
				Resource:       recurso,
				Message:        v.msg,
				Recommendation: recomendacao,
				Scope:          string(escopo.DoKind(p.kind)),
			})
		}
	}
	return findings, nil
}

// scanDefaultDeny avalia a política de default-deny contra NetworkPolicies do namespace.
func (o *OPABackend) scanDefaultDeny(ctx context.Context, client kubernetes.Interface, namespace string) ([]Finding, error) {
	netpols, err := client.NetworkingV1().NetworkPolicies(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("falha ao listar NetworkPolicies: %w", err)
	}

	// Converter NetworkPolicies para formato genérico do OPA
	var policies []map[string]interface{}
	for _, np := range netpols.Items {
		policies = append(policies, netpolToInput(np.Spec))
	}

	input := map[string]interface{}{
		"namespace": namespace,
		"policies":  policies,
	}

	violations, err := evaluatePolicy(ctx, regoDefaultDenyPolicy, input)
	if err != nil {
		return nil, fmt.Errorf("falha ao avaliar política default-deny: %w", err)
	}

	var findings []Finding
	for _, v := range violations {
		findings = append(findings, Finding{
			ID:             "opa/netpol_default_deny",
			Source:         "opa",
			Severity:       v.severity,
			Category:       "network",
			Namespace:      namespace,
			Resource:       namespace,
			Message:        v.msg,
			Recommendation: "Crie NetworkPolicies default-deny com podSelector vazio para Ingress e Egress",
			Scope:          string(escopo.Namespace),
		})
	}

	return findings, nil
}

// violation representa uma violação retornada por uma política Rego.
type opaViolation struct {
	msg      string
	severity string
}

// resultadoRego é o que uma política devolve para um recurso: as violações e se a
// allowlist deixou o recurso de fora (a regra ignorado).
type resultadoRego struct {
	violacoes []opaViolation
	ignorado  bool
}

// politicasPreparadas guarda cada política já compilada, indexada pelo texto Rego:
// compilar o módulo custa muito mais que avaliá-lo, e o mesmo módulo avalia todos os
// recursos de um scan.
var politicasPreparadas = struct {
	sync.Mutex
	m map[string]*rego.PreparedEvalQuery
}{m: map[string]*rego.PreparedEvalQuery{}}

// compilacoesRego conta as compilações de módulos Rego (os testes conferem o cache).
var compilacoesRego atomic.Int64

// prepararPolitica compila a política na primeira vez e devolve a consulta preparada
// nas seguintes. A allowlist entra como data.allowlist.
func prepararPolitica(ctx context.Context, policy string) (*rego.PreparedEvalQuery, error) {
	politicasPreparadas.Lock()
	defer politicasPreparadas.Unlock()
	if q, ok := politicasPreparadas.m[policy]; ok {
		return q, nil
	}
	compilacoesRego.Add(1)
	q, err := rego.New(
		rego.Query("data.sentinel"),
		rego.Module("policy.rego", policy),
		rego.Store(inmem.NewFromObject(map[string]interface{}{"allowlist": allowlist.Padrao.ParaRego()})),
	).PrepareForEval(ctx)
	if err != nil {
		return nil, err
	}
	politicasPreparadas.m[policy] = &q
	return &q, nil
}

// evaluatePolicy avalia uma política Rego contra o input fornecido e retorna as violações.
func evaluatePolicy(ctx context.Context, policy string, input interface{}) ([]opaViolation, error) {
	r, err := avaliarPolitica(ctx, policy, input)
	return r.violacoes, err
}

// avaliarPolitica avalia a política contra o input e devolve as violações e a regra
// ignorado.
func avaliarPolitica(ctx context.Context, policy string, input interface{}) (resultadoRego, error) {
	q, err := prepararPolitica(ctx, policy)
	if err != nil {
		return resultadoRego{}, fmt.Errorf("falha ao avaliar política Rego: %w", err)
	}

	rs, err := q.Eval(ctx, rego.EvalInput(input))
	if err != nil {
		return resultadoRego{}, fmt.Errorf("falha ao avaliar política Rego: %w", err)
	}

	if len(rs) == 0 || len(rs[0].Expressions) == 0 {
		return resultadoRego{}, nil
	}
	doc, _ := rs[0].Expressions[0].Value.(map[string]interface{})
	ignorado, _ := doc["ignorado"].(bool)
	return resultadoRego{violacoes: parseViolations(doc["violation"]), ignorado: ignorado}, nil
}

// parseViolations extrai violações do resultado da avaliação Rego.
// O OPA pode retornar um set como []interface{} ou map[string]interface{}.
func parseViolations(value interface{}) []opaViolation {
	var violations []opaViolation

	switch v := value.(type) {
	case []interface{}:
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				if viol := extractViolation(m); viol != nil {
					violations = append(violations, *viol)
				}
			}
		}
	case map[string]interface{}:
		// Conjunto unitário retornado como mapa
		if viol := extractViolation(v); viol != nil {
			violations = append(violations, *viol)
		}
	}

	return violations
}

// extractViolation extrai msg e severity de um mapa de violação.
func extractViolation(m map[string]interface{}) *opaViolation {
	msg, _ := m["msg"].(string)
	severity, _ := m["severity"].(string)
	if msg == "" {
		return nil
	}
	if severity == "" {
		severity = "medium"
	}
	return &opaViolation{msg: msg, severity: severity}
}

// bindingToInput converte um ClusterRoleBinding para o formato de input do OPA.
func bindingToInput(binding rbacv1.ClusterRoleBinding) map[string]interface{} {
	subjects := make([]map[string]interface{}, 0, len(binding.Subjects))
	for _, s := range binding.Subjects {
		subjects = append(subjects, map[string]interface{}{
			"kind":      s.Kind,
			"name":      s.Name,
			"namespace": s.Namespace,
		})
	}

	return map[string]interface{}{
		"metadata": map[string]interface{}{
			"name": binding.Name,
		},
		"roleRef": map[string]interface{}{
			"name":     binding.RoleRef.Name,
			"kind":     binding.RoleRef.Kind,
			"apiGroup": binding.RoleRef.APIGroup,
		},
		"subjects": subjects,
	}
}

// roleToInput converte um Role/ClusterRole para o formato de input do OPA, com os
// subjects de todos os bindings que apontam para ele.
func roleToInput(kind, name, namespace string, rules []rbacv1.PolicyRule, sujeitos []allowlist.Sujeito) map[string]interface{} {
	metadata := map[string]interface{}{
		"name": name,
	}
	if namespace != "" {
		metadata["namespace"] = namespace
	}

	subjects := make([]map[string]interface{}, 0, len(sujeitos))
	for _, s := range sujeitos {
		subjects = append(subjects, map[string]interface{}{
			"kind":         s.Kind,
			"name":         s.Nome,
			"namespace":    s.Namespace,
			"pelo_cluster": s.PeloCluster,
		})
	}

	return map[string]interface{}{
		"kind":     kind,
		"metadata": metadata,
		"rules":    rulesToInput(rules),
		"subjects": subjects,
	}
}

// rulesToInput converte PolicyRules do K8s para o formato genérico do OPA.
func rulesToInput(rules []rbacv1.PolicyRule) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(rules))
	for _, rule := range rules {
		result = append(result, map[string]interface{}{
			"verbs":     stringSliceToInterface(rule.Verbs),
			"resources": stringSliceToInterface(rule.Resources),
			"apiGroups": stringSliceToInterface(rule.APIGroups),
		})
	}
	return result
}

// netpolToInput converte um NetworkPolicySpec para o formato genérico do OPA.
func netpolToInput(spec networkingv1.NetworkPolicySpec) map[string]interface{} {
	// Converter matchLabels
	matchLabels := make(map[string]interface{})
	for k, v := range spec.PodSelector.MatchLabels {
		matchLabels[k] = v
	}

	// Converter matchExpressions
	var matchExpressions []interface{}
	for _, expr := range spec.PodSelector.MatchExpressions {
		matchExpressions = append(matchExpressions, map[string]interface{}{
			"key":      expr.Key,
			"operator": string(expr.Operator),
			"values":   stringSliceToInterface(expr.Values),
		})
	}

	podSelector := map[string]interface{}{
		"matchLabels": matchLabels,
	}
	if matchExpressions != nil {
		podSelector["matchExpressions"] = matchExpressions
	}

	// Converter policyTypes
	policyTypes := make([]interface{}, 0, len(spec.PolicyTypes))
	for _, pt := range spec.PolicyTypes {
		policyTypes = append(policyTypes, string(pt))
	}

	// Converter ingress (simplificado — só precisamos do count)
	ingress := make([]interface{}, 0, len(spec.Ingress))
	for range spec.Ingress {
		ingress = append(ingress, map[string]interface{}{})
	}

	// Converter egress (simplificado — só precisamos do count)
	egress := make([]interface{}, 0, len(spec.Egress))
	for range spec.Egress {
		egress = append(egress, map[string]interface{}{})
	}

	return map[string]interface{}{
		"spec": map[string]interface{}{
			"podSelector": podSelector,
			"policyTypes": policyTypes,
			"ingress":     ingress,
			"egress":      egress,
		},
	}
}

// stringSliceToInterface converte []string para []interface{} para o OPA.
func stringSliceToInterface(ss []string) []interface{} {
	result := make([]interface{}, len(ss))
	for i, s := range ss {
		result[i] = s
	}
	return result
}

// listaClusterRoles devolve os ClusterRoles do cluster; a primeira chamada lista e as
// seguintes reaproveitam o resultado (ou a falha).
type listaClusterRoles func() ([]rbacv1.ClusterRole, error)

// novaListaClusterRoles lista os ClusterRoles uma única vez por scan, para as políticas de
// secrets e de wildcard. Se a listagem falhar, as duas recebem a mesma falha e continuam
// aparecendo em FalhaParcial.
func novaListaClusterRoles(ctx context.Context, client kubernetes.Interface) listaClusterRoles {
	var (
		itens []rbacv1.ClusterRole
		err   error
		feito bool
	)
	return func() ([]rbacv1.ClusterRole, error) {
		if !feito {
			feito = true
			lista, e := client.RbacV1().ClusterRoles().List(ctx, metav1.ListOptions{})
			if e != nil {
				err = fmt.Errorf("falha ao listar ClusterRoles: %w", e)
			} else {
				itens = lista.Items
			}
		}
		return itens, err
	}
}
