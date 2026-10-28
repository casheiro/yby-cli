//go:build k8s

// Package profiles define perfis de compliance para o Sentinel.
package profiles

// ComplianceProfile define um perfil de compliance com checks associados.
//
// CheckIDs lista os controles do perfil pelos IDs dos checks internos (POD_*, RBAC_*,
// NETPOL_*); um check do Polaris ou do OPA entra no perfil pelo check interno equivalente
// (equivalentes). O controle que só o Polaris verifica entra pelo próprio ID
// (polaris/<check>).
type ComplianceProfile struct {
	Name        string
	Description string
	CheckIDs    []string
}

// Inclui informa se os achados do check (interno, do Polaris ou do OPA) entram no perfil.
// Check sem equivalente e fora da lista, como um check customizado do Polaris, fica fora.
func (p ComplianceProfile) Inclui(checkID string) bool {
	interno := equivalentes[checkID]
	for _, id := range p.CheckIDs {
		if id == checkID || (interno != "" && id == interno) {
			return true
		}
	}
	return false
}

// equivalentes dá, para cada check do Polaris e do OPA, o check interno que verifica a
// mesma coisa. Os checks de confiabilidade do Polaris (probes, réplicas, priority class,
// PDB, HPA) e os de exec/attach em RBAC não têm equivalente e ficam fora dos perfis.
var equivalentes = map[string]string{
	// Polaris · Security
	"polaris/runAsRootAllowed":                   "POD_ROOT_CONTAINER",
	"polaris/runAsPrivileged":                    "POD_PRIVILEGED",
	"polaris/privilegeEscalationAllowed":         "POD_PRIVILEGE_ESCALATION",
	"polaris/dangerousCapabilities":              "POD_CAPABILITIES",
	"polaris/insecureCapabilities":               "POD_CAPABILITIES",
	"polaris/hostIPCSet":                         "POD_HOST_NAMESPACES",
	"polaris/hostPIDSet":                         "POD_HOST_NAMESPACES",
	"polaris/hostNetworkSet":                     "POD_HOST_NAMESPACES",
	"polaris/hostPortSet":                        "POD_HOST_PORTS",
	"polaris/notReadOnlyRootFilesystem":          "POD_READONLY_ROOTFS",
	"polaris/linuxHardening":                     "POD_SECCOMP",
	"polaris/automountServiceAccountToken":       "POD_SERVICE_ACCOUNT_TOKEN",
	"polaris/sensitiveContainerEnvVar":           "POD_EXPOSED_SECRETS",
	"polaris/sensitiveConfigmapContent":          "POD_EXPOSED_SECRETS",
	"polaris/missingNetworkPolicy":               "NETPOL_COVERAGE",
	"polaris/clusterrolebindingClusterAdmin":     "RBAC_CLUSTER_ADMIN",
	"polaris/rolebindingClusterAdminClusterRole": "RBAC_CLUSTER_ADMIN",
	"polaris/rolebindingClusterAdminRole":        "RBAC_CLUSTER_ADMIN",
	// Polaris · Efficiency e Reliability que são controles dos perfis CIS
	"polaris/cpuLimitsMissing":    "POD_RESOURCE_LIMITS",
	"polaris/memoryLimitsMissing": "POD_RESOURCE_LIMITS",
	"polaris/pullPolicyNotAlways": "POD_IMAGE_PULL_POLICY",
	"polaris/tagNotSpecified":     "IMAGE_LATEST_TAG",
	// OPA
	"opa/rbac_cluster_admin":  "RBAC_CLUSTER_ADMIN",
	"opa/rbac_wildcard":       "RBAC_WILDCARD",
	"opa/rbac_secrets_access": "RBAC_SECRETS_ACCESS",
	"opa/netpol_default_deny": "NETPOL_DEFAULT_DENY",
}
