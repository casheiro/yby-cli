//go:build k8s

package profiles

func init() {
	register(ComplianceProfile{
		Name:        "soc2",
		Description: "SOC2 — foco em controle de acesso, segurança de rede e hardening de pods",
		CheckIDs: []string{
			// Controle de acesso
			"RBAC_CLUSTER_ADMIN",
			"RBAC_WILDCARD",
			"RBAC_SECRETS_ACCESS",
			"POD_SERVICE_ACCOUNT_TOKEN",
			// Segurança de rede
			"NETPOL_COVERAGE",
			"NETPOL_DEFAULT_DENY",
			// Hardening
			"POD_ROOT_CONTAINER",
			"POD_PRIVILEGED",
			"POD_PRIVILEGE_ESCALATION",
			"POD_CAPABILITIES",
			"POD_EXPOSED_SECRETS",
		},
	})
}
