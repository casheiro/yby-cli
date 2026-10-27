//go:build k8s

package profiles

import (
	"testing"

	"github.com/casheiro/yby-cli/plugins/sentinel/cli/checks"
)

func TestGetProfile_CISLevel1(t *testing.T) {
	p, ok := GetProfile("cis-l1")
	if !ok {
		t.Fatal("esperava encontrar perfil cis-l1")
	}
	if len(p.CheckIDs) == 0 {
		t.Error("perfil cis-l1 deveria ter check IDs")
	}
}

func TestGetProfile_CISLevel2(t *testing.T) {
	p, ok := GetProfile("cis-l2")
	if !ok {
		t.Fatal("esperava encontrar perfil cis-l2")
	}
	if len(p.CheckIDs) < len(mustGetProfile(t, "cis-l1").CheckIDs) {
		t.Error("cis-l2 deveria ter mais checks que cis-l1")
	}
}

func TestGetProfile_PCIDSS(t *testing.T) {
	p, ok := GetProfile("pci-dss")
	if !ok {
		t.Fatal("esperava encontrar perfil pci-dss")
	}
	// PCI-DSS deve incluir checks de secrets e RBAC
	ids := make(map[string]bool)
	for _, id := range p.CheckIDs {
		ids[id] = true
	}
	if !ids["POD_EXPOSED_SECRETS"] {
		t.Error("pci-dss deveria incluir POD_EXPOSED_SECRETS")
	}
	if !ids["RBAC_CLUSTER_ADMIN"] {
		t.Error("pci-dss deveria incluir RBAC_CLUSTER_ADMIN")
	}
}

func TestGetProfile_SOC2(t *testing.T) {
	_, ok := GetProfile("soc2")
	if !ok {
		t.Fatal("esperava encontrar perfil soc2")
	}
}

func TestGetProfile_Inexistente(t *testing.T) {
	_, ok := GetProfile("inexistente")
	if ok {
		t.Error("não deveria encontrar perfil inexistente")
	}
}

func TestListProfiles(t *testing.T) {
	all := ListProfiles()
	if len(all) < 4 {
		t.Errorf("esperava pelo menos 4 perfis, obteve %d", len(all))
	}
}

func mustGetProfile(t *testing.T, name string) ComplianceProfile {
	t.Helper()
	p, ok := GetProfile(name)
	if !ok {
		t.Fatalf("perfil '%s' não encontrado", name)
	}
	return p
}

// O perfil vale para os achados de todos os motores: o check do Polaris ou do OPA entra
// quando o check interno equivalente está no perfil.
func TestInclui_TodosOsMotores(t *testing.T) {
	casos := []struct {
		perfil, check string
		inclui        bool
	}{
		{"pci-dss", "opa/rbac_cluster_admin", true},
		{"pci-dss", "polaris/clusterrolebindingClusterAdmin", true},
		{"pci-dss", "polaris/runAsRootAllowed", true},
		{"pci-dss", "polaris/sensitiveContainerEnvVar", true},
		{"pci-dss", "polaris/tlsSettingsMissing", true},
		{"pci-dss", "POD_PRIVILEGED", true},
		{"pci-dss", "polaris/priorityClassNotSet", false},
		{"pci-dss", "polaris/cpuLimitsMissing", false},
		{"pci-dss", "POD_RESOURCE_LIMITS", false},
		{"cis-l1", "polaris/memoryLimitsMissing", true},
		{"cis-l1", "polaris/hostPIDSet", true},
		{"cis-l1", "opa/rbac_wildcard", false},
		{"cis-l2", "opa/rbac_wildcard", true},
		{"cis-l2", "opa/netpol_default_deny", true},
		{"cis-l2", "polaris/missingNetworkPolicy", true},
		{"cis-l2", "polaris/hostPathSet", true},
		{"soc2", "polaris/automountServiceAccountToken", true},
		{"soc2", "polaris/livenessProbeMissing", false},
		// check desconhecido (um check customizado do Polaris) não entra em perfil nenhum
		{"cis-l2", "polaris/meuCheckCustomizado", false},
	}
	for _, c := range casos {
		if got := mustGetProfile(t, c.perfil).Inclui(c.check); got != c.inclui {
			t.Errorf("%s inclui %s = %v, esperava %v", c.perfil, c.check, got, c.inclui)
		}
	}
}

// A tabela de equivalências só aponta para checks internos que existem.
func TestEquivalentes_ApontamParaChecksInternos(t *testing.T) {
	for externo, interno := range equivalentes {
		if len(checks.GetByIDs([]string{interno})) != 1 {
			t.Errorf("%s aponta para %s, que não é um check interno", externo, interno)
		}
	}
}
