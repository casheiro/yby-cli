//go:build k8s

package backends

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/open-policy-agent/opa/v1/rego"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// regoClusterAdminPolicy detecta ClusterRoleBindings que concedem cluster-admin
// a subjects não autorizados, excluindo bindings do sistema e controllers conhecidos.
const regoClusterAdminPolicy = `
package sentinel

import rego.v1

# Prefixos de controllers conhecidos que devem ser ignorados
known_prefixes := [
	"argocd-", "cert-manager-", "helm-", "k3s-", "traefik-",
	"monitoring-", "loki-", "cnpg-", "keda-", "argo-events-", "argo-server-",
]

is_known_controller(name) if {
	some prefix in known_prefixes
	startswith(name, prefix)
}

should_skip(binding) if {
	startswith(binding.metadata.name, "system:")
}

should_skip(binding) if {
	binding.metadata.name == "cluster-admin"
}

should_skip(binding) if {
	is_known_controller(binding.metadata.name)
}

is_system_masters(subject) if {
	subject.kind == "Group"
	subject.name == "system:masters"
}

violation contains result if {
	binding := input
	binding.roleRef.name == "cluster-admin"
	not should_skip(binding)
	some subject in binding.subjects
	not is_system_masters(subject)
	result := {
		"msg": sprintf("ClusterRoleBinding '%s' concede cluster-admin a %s '%s'", [binding.metadata.name, subject.kind, subject.name]),
		"severity": "critical",
	}
}
`

// regoSecretsAccessPolicy detecta ClusterRoles/Roles que permitem leitura de secrets,
// excluindo roles do sistema e controllers conhecidos.
const regoSecretsAccessPolicy = `
package sentinel

import rego.v1

known_prefixes := [
	"argocd-", "cert-manager-", "helm-", "k3s-", "traefik-",
	"monitoring-", "loki-", "cnpg-", "keda-", "argo-events-", "argo-server-",
]

builtin_roles := {"admin", "edit", "view", "cluster-admin"}

is_known_controller(name) if {
	some prefix in known_prefixes
	startswith(name, prefix)
}

should_skip(role) if {
	startswith(role.metadata.name, "system:")
}

should_skip(role) if {
	builtin_roles[role.metadata.name]
}

should_skip(role) if {
	is_known_controller(role.metadata.name)
}

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
	not should_skip(role)
	has_secrets_access(role.rules)
	result := {
		"msg": sprintf("%s '%s' permite leitura de secrets", [role.kind, role.metadata.name]),
		"severity": "high",
	}
}
`

// regoWildcardPolicy detecta ClusterRoles com permissoes wildcard (*),
// excluindo roles do sistema e controllers conhecidos.
const regoWildcardPolicy = `
package sentinel

import rego.v1

known_prefixes := [
	"argocd-", "cert-manager-", "helm-", "k3s-", "traefik-",
	"monitoring-", "loki-", "cnpg-", "keda-", "argo-events-", "argo-server-",
]

builtin_roles := {"admin", "edit", "view", "cluster-admin"}

is_known_controller(name) if {
	some prefix in known_prefixes
	startswith(name, prefix)
}

should_skip(role) if {
	startswith(role.metadata.name, "system:")
}

should_skip(role) if {
	builtin_roles[role.metadata.name]
}

should_skip(role) if {
	is_known_controller(role.metadata.name)
}

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
	not should_skip(role)
	has_wildcard(role.rules)
	result := {
		"msg": sprintf("ClusterRole '%s' possui permissões wildcard (*)", [role.metadata.name]),
		"severity": "high",
	}
}
`

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

// ScanCluster escaneia o cluster avaliando políticas Rego contra recursos K8s.
func (o *OPABackend) ScanCluster(ctx context.Context, client kubernetes.Interface, namespace string) ([]Finding, error) {
	var findings []Finding

	// 1. Avaliar política de cluster-admin em ClusterRoleBindings
	clusterAdminFindings, err := o.scanClusterAdmin(ctx, client)
	if err != nil {
		slog.Warn("opa: falha ao escanear cluster-admin", "error", err)
	} else {
		findings = append(findings, clusterAdminFindings...)
	}

	// 2. Avaliar política de acesso a secrets em ClusterRoles e Roles
	secretsFindings, err := o.scanSecretsAccess(ctx, client, namespace)
	if err != nil {
		slog.Warn("opa: falha ao escanear acesso a secrets", "error", err)
	} else {
		findings = append(findings, secretsFindings...)
	}

	// 3. Avaliar política de wildcard em ClusterRoles
	wildcardFindings, err := o.scanWildcard(ctx, client)
	if err != nil {
		slog.Warn("opa: falha ao escanear wildcards", "error", err)
	} else {
		findings = append(findings, wildcardFindings...)
	}

	// 4. Avaliar política de default-deny em NetworkPolicies
	defaultDenyFindings, err := o.scanDefaultDeny(ctx, client, namespace)
	if err != nil {
		slog.Warn("opa: falha ao escanear default-deny", "error", err)
	} else {
		findings = append(findings, defaultDenyFindings...)
	}

	return findings, nil
}

// scanClusterAdmin avalia a política de cluster-admin contra todos os ClusterRoleBindings.
func (o *OPABackend) scanClusterAdmin(ctx context.Context, client kubernetes.Interface) ([]Finding, error) {
	bindings, err := client.RbacV1().ClusterRoleBindings().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("falha ao listar ClusterRoleBindings: %w", err)
	}

	var findings []Finding
	for _, binding := range bindings.Items {
		input := bindingToInput(binding)

		violations, err := evaluatePolicy(ctx, regoClusterAdminPolicy, input)
		if err != nil {
			slog.Warn("opa: falha ao avaliar política cluster-admin", "binding", binding.Name, "error", err)
			continue
		}

		for _, v := range violations {
			findings = append(findings, Finding{
				ID:             "opa/rbac_cluster_admin",
				Source:         "opa",
				Severity:       v.severity,
				Category:       "rbac",
				Resource:       fmt.Sprintf("ClusterRoleBinding/%s", binding.Name),
				Message:        v.msg,
				Recommendation: "Substitua cluster-admin por roles mais restritivos seguindo o princípio do menor privilégio",
			})
		}
	}

	return findings, nil
}

// scanSecretsAccess avalia a política de acesso a secrets contra ClusterRoles e Roles.
func (o *OPABackend) scanSecretsAccess(ctx context.Context, client kubernetes.Interface, namespace string) ([]Finding, error) {
	var findings []Finding

	// ClusterRoles
	clusterRoles, err := client.RbacV1().ClusterRoles().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("falha ao listar ClusterRoles: %w", err)
	}

	for _, role := range clusterRoles.Items {
		input := roleToInput("ClusterRole", role.Name, "", role.Rules)
		violations, err := evaluatePolicy(ctx, regoSecretsAccessPolicy, input)
		if err != nil {
			slog.Warn("opa: falha ao avaliar política secrets", "role", role.Name, "error", err)
			continue
		}
		for _, v := range violations {
			findings = append(findings, Finding{
				ID:             "opa/rbac_secrets_access",
				Source:         "opa",
				Severity:       v.severity,
				Category:       "rbac",
				Resource:       fmt.Sprintf("ClusterRole/%s", role.Name),
				Message:        v.msg,
				Recommendation: "Restrinja acesso a secrets apenas aos serviços que realmente precisam",
			})
		}
	}

	// Roles no namespace
	roles, err := client.RbacV1().Roles(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("falha ao listar Roles: %w", err)
	}

	for _, role := range roles.Items {
		input := roleToInput("Role", role.Name, namespace, role.Rules)
		violations, err := evaluatePolicy(ctx, regoSecretsAccessPolicy, input)
		if err != nil {
			slog.Warn("opa: falha ao avaliar política secrets", "role", role.Name, "error", err)
			continue
		}
		for _, v := range violations {
			findings = append(findings, Finding{
				ID:             "opa/rbac_secrets_access",
				Source:         "opa",
				Severity:       v.severity,
				Category:       "rbac",
				Namespace:      namespace,
				Resource:       fmt.Sprintf("Role/%s", role.Name),
				Message:        v.msg,
				Recommendation: "Restrinja acesso a secrets apenas aos serviços que realmente precisam",
			})
		}
	}

	return findings, nil
}

// scanWildcard avalia a política de wildcard contra ClusterRoles.
func (o *OPABackend) scanWildcard(ctx context.Context, client kubernetes.Interface) ([]Finding, error) {
	clusterRoles, err := client.RbacV1().ClusterRoles().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("falha ao listar ClusterRoles: %w", err)
	}

	var findings []Finding
	for _, role := range clusterRoles.Items {
		input := roleToInput("ClusterRole", role.Name, "", role.Rules)
		violations, err := evaluatePolicy(ctx, regoWildcardPolicy, input)
		if err != nil {
			slog.Warn("opa: falha ao avaliar política wildcard", "role", role.Name, "error", err)
			continue
		}
		for _, v := range violations {
			findings = append(findings, Finding{
				ID:             "opa/rbac_wildcard",
				Source:         "opa",
				Severity:       v.severity,
				Category:       "rbac",
				Resource:       fmt.Sprintf("ClusterRole/%s", role.Name),
				Message:        v.msg,
				Recommendation: "Substitua wildcards por verbos e recursos específicos",
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
		})
	}

	return findings, nil
}

// violation representa uma violação retornada por uma política Rego.
type opaViolation struct {
	msg      string
	severity string
}

// evaluatePolicy avalia uma política Rego contra o input fornecido e retorna as violações.
func evaluatePolicy(ctx context.Context, policy string, input interface{}) ([]opaViolation, error) {
	r := rego.New(
		rego.Query("data.sentinel.violation"),
		rego.Module("policy.rego", policy),
		rego.Input(input),
	)

	rs, err := r.Eval(ctx)
	if err != nil {
		return nil, fmt.Errorf("falha ao avaliar política Rego: %w", err)
	}

	if len(rs) == 0 || len(rs[0].Expressions) == 0 {
		return nil, nil
	}

	return parseViolations(rs[0].Expressions[0].Value), nil
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

// roleToInput converte um Role/ClusterRole para o formato de input do OPA.
func roleToInput(kind, name, namespace string, rules []rbacv1.PolicyRule) map[string]interface{} {
	metadata := map[string]interface{}{
		"name": name,
	}
	if namespace != "" {
		metadata["namespace"] = namespace
	}

	return map[string]interface{}{
		"kind":     kind,
		"metadata": metadata,
		"rules":    rulesToInput(rules),
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
