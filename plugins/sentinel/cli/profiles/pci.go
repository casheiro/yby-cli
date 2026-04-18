//go:build k8s

package profiles

func init() {
	register(ComplianceProfile{
		Name:        "pci-dss",
		Description: "PCI-DSS — foco em proteção de secrets, controle de acesso e segmentação de rede",
		CheckIDs: []string{
			// Secrets
			"POD_EXPOSED_SECRETS",
			// RBAC
			"RBAC_CLUSTER_ADMIN",
			"RBAC_WILDCARD",
			"RBAC_SECRETS_ACCESS",
			// Network
			"NETPOL_COVERAGE",
			"NETPOL_DEFAULT_DENY",
			// Pod Security
			"POD_PRIVILEGED",
			"POD_ROOT_CONTAINER",
			"POD_SERVICE_ACCOUNT_TOKEN",
		},
	})
}
