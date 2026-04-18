//go:build k8s

package checks

// Allowlists de recursos RBAC do sistema e de ferramentas conhecidas.
// Estes recursos precisam das permissões que têm por design —
// reportá-los como vulnerabilidades é falso positivo.

// systemClusterRoleBindings são bindings built-in do K8s que sempre existem.
var systemClusterRoleBindings = map[string]bool{
	"cluster-admin":                          true, // system:masters — admin padrão do K8s
	"system:basic-user":                      true,
	"system:discovery":                       true,
	"system:monitoring":                      true,
	"system:node-proxier":                    true,
	"system:public-info-viewer":              true,
	"system:volume-scheduler":                true,
	"system:kube-controller-manager":         true,
	"system:kube-dns":                        true,
	"system:kube-scheduler":                  true,
	"system:node":                            true,
	"system:controller:node-controller":      true,
	"system:controller:job-controller":       true,
	"system:controller:endpoint-controller":  true,
	"system:controller:service-account-controller": true,
}

// systemClusterRoles são roles built-in do K8s que sempre existem.
// Prefixos: "system:", "admin", "edit", "view", "cluster-admin"
var systemClusterRoles = map[string]bool{
	"admin":         true,
	"edit":          true,
	"view":          true,
	"cluster-admin": true,
}

// systemClusterRolePrefixes são prefixos de roles do sistema K8s.
var systemClusterRolePrefixes = []string{
	"system:",
}

// knownControllerRoles são roles de ferramentas conhecidas que precisam de
// permissões amplas por design. Não são vulnerabilidades.
var knownControllerRolePrefixes = []string{
	"argocd-",                         // ArgoCD precisa de acesso amplo para sync
	"cert-manager-",                   // cert-manager gerencia certificados/secrets
	"traefik-",                        // Traefik ingress controller
	"helm-",                           // Helm release management
	"k3s-",                            // K3s system controllers
	"local-path-provisioner-",         // K3s local path provisioner
	"monitoring-kube-prometheus-",     // Prometheus operator
	"monitoring-kube-state-metrics",   // kube-state-metrics
	"monitoring-grafana-",             // Grafana
	"loki-",                           // Loki log aggregator
	"cnpg-",                           // CloudNativePG operator
	"keda-",                           // KEDA autoscaler
	"argo-events-",                    // Argo Events
	"argo-aggregate-",                 // Argo Workflows aggregation roles
	"argo-cluster-",                   // Argo Workflows cluster role
	"argo-server-",                    // Argo Workflows server
}

// knownControllerBindings são bindings de ferramentas conhecidas.
var knownControllerBindingPrefixes = []string{
	"argocd-",
	"cert-manager-",
	"traefik-",
	"helm-",
	"k3s-",
	"monitoring-",
	"loki-",
	"cnpg-",
	"keda-",
	"argo-events-",
	"argo-server-",
}

// isSystemRole verifica se um ClusterRole é built-in do K8s.
func isSystemRole(name string) bool {
	if systemClusterRoles[name] {
		return true
	}
	for _, prefix := range systemClusterRolePrefixes {
		if len(name) >= len(prefix) && name[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

// isKnownControllerRole verifica se um ClusterRole pertence a uma ferramenta conhecida.
func isKnownControllerRole(name string) bool {
	for _, prefix := range knownControllerRolePrefixes {
		if len(name) >= len(prefix) && name[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

// isSystemBinding verifica se um ClusterRoleBinding é built-in do K8s.
func isSystemBinding(name string) bool {
	if systemClusterRoleBindings[name] {
		return true
	}
	for _, prefix := range systemClusterRolePrefixes {
		if len(name) >= len(prefix) && name[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

// isKnownControllerBinding verifica se um ClusterRoleBinding pertence a uma ferramenta conhecida.
func isKnownControllerBinding(name string) bool {
	for _, prefix := range knownControllerBindingPrefixes {
		if len(name) >= len(prefix) && name[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

// shouldSkipRBACResource verifica se um recurso RBAC deve ser ignorado no scan.
// Ignora recursos do sistema K8s e de controllers conhecidos.
func shouldSkipRBACResource(name string) bool {
	return isSystemRole(name) || isKnownControllerRole(name) ||
		isSystemBinding(name) || isKnownControllerBinding(name)
}
