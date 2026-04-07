//go:build k8s

package profiles

import (
	"testing"
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
