//go:build k8s

package checks

import (
	"testing"
)

func TestGetAll_RetornaChecksRegistrados(t *testing.T) {
	all := GetAll()
	if len(all) < 4 {
		t.Errorf("esperava pelo menos 4 checks registrados (via init), obteve %d", len(all))
	}
}

func TestGetByCategory_FiltraPorCategoria(t *testing.T) {
	podSec := GetByCategory(CategoryPodSecurity)
	if len(podSec) < 3 {
		t.Errorf("esperava pelo menos 3 checks pod-security, obteve %d", len(podSec))
	}

	secrets := GetByCategory(CategorySecrets)
	if len(secrets) < 1 {
		t.Errorf("esperava pelo menos 1 check secrets, obteve %d", len(secrets))
	}
}

func TestGetByIDs_FiltraPorIDs(t *testing.T) {
	result := GetByIDs([]string{"POD_ROOT_CONTAINER", "POD_EXPOSED_SECRETS"})
	if len(result) != 2 {
		t.Errorf("esperava 2 checks filtrados por ID, obteve %d", len(result))
	}

	ids := make(map[string]bool)
	for _, c := range result {
		ids[c.ID()] = true
	}
	if !ids["POD_ROOT_CONTAINER"] || !ids["POD_EXPOSED_SECRETS"] {
		t.Error("checks retornados não correspondem aos IDs solicitados")
	}
}

func TestGetByIDs_IDInexistente(t *testing.T) {
	result := GetByIDs([]string{"INEXISTENTE"})
	if len(result) != 0 {
		t.Errorf("esperava 0 checks para ID inexistente, obteve %d", len(result))
	}
}
