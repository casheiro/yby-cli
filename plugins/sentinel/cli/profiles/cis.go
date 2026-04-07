//go:build k8s

package profiles

func init() {
	register(ComplianceProfile{
		Name:        "cis-l1",
		Description: "CIS Kubernetes Benchmark Level 1 — checks básicos de segurança de pods",
		CheckIDs: []string{
			"POD_ROOT_CONTAINER",
			"POD_RESOURCE_LIMITS",
			"POD_IMAGE_PULL_POLICY",
			"POD_EXPOSED_SECRETS",
			"POD_PRIVILEGED",
			"POD_HOST_NAMESPACES",
			"POD_SERVICE_ACCOUNT_TOKEN",
		},
	})

	register(ComplianceProfile{
		Name:        "cis-l2",
		Description: "CIS Kubernetes Benchmark Level 2 — todos os checks de segurança incluindo RBAC e rede",
		CheckIDs: []string{
			// Pod Security
			"POD_ROOT_CONTAINER",
			"POD_RESOURCE_LIMITS",
			"POD_IMAGE_PULL_POLICY",
			"POD_EXPOSED_SECRETS",
			"POD_PRIVILEGE_ESCALATION",
			"POD_CAPABILITIES",
			"POD_READONLY_ROOTFS",
			"POD_HOST_NAMESPACES",
			"POD_SERVICE_ACCOUNT_TOKEN",
			"POD_SECCOMP",
			"POD_PRIVILEGED",
			"POD_HOST_PORTS",
			// RBAC
			"RBAC_CLUSTER_ADMIN",
			"RBAC_WILDCARD",
			"RBAC_SECRETS_ACCESS",
			// Network
			"NETPOL_COVERAGE",
			"NETPOL_DEFAULT_DENY",
		},
	})
}
