//go:build k8s

package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/casheiro/yby-cli/pkg/ui"
	"github.com/casheiro/yby-cli/plugins/sentinel/cli/backends"
	"github.com/muesli/termenv"
	"k8s.io/client-go/kubernetes/fake"
)

// backendsComPerfil devolve um Polaris e um OPA falsos com um achado fora do PCI-DSS (a
// priority class é confiabilidade) e dois dentro (cluster-admin e container como root).
func backendsComPerfil() []backends.SecurityBackend {
	return []backends.SecurityBackend{
		backendFalso{nome: "polaris", achados: []backends.Finding{
			{ID: "polaris/priorityClassNotSet", Severity: "low", Category: "reliability", Resource: "Deployment/api", Message: "Priority class should be set"},
			{ID: "polaris/runAsRootAllowed", Severity: "critical", Category: "pod-security", Resource: "Deployment/api", Message: "Should not be allowed to run as root"},
		}},
		backendFalso{nome: "opa", achados: []backends.Finding{
			{ID: "opa/rbac_cluster_admin", Severity: "critical", Category: "rbac", Resource: "ClusterRoleBinding/ci-deployer", Message: "cluster-admin dado a ci-deployer"},
		}},
	}
}

// O --profile filtra os achados de todos os motores, não só os dos checks internos.
func TestScan_PerfilFiltraTodosOsMotores(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
	usarDependencias(t, fake.NewSimpleClientset(), backendsComPerfil(), nil, nil)

	saida, _, err := rodar(t, "scan", "-n", "payments", "--profile", "pci-dss", "-o", "json", "--no-ai")
	if err != nil {
		t.Fatal(err)
	}
	var rel ScanReport
	if err := json.Unmarshal([]byte(saida), &rel); err != nil {
		t.Fatalf("json inválido: %v\n%s", err, saida)
	}
	var ids []string
	for _, f := range rel.Findings {
		ids = append(ids, f.CheckID)
	}
	if strings.Join(ids, ",") != "polaris/runAsRootAllowed,opa/rbac_cluster_admin" {
		t.Errorf("com --profile pci-dss só ficam os achados do perfil: %v", ids)
	}
	if rel.Perfil != "pci-dss" || rel.ForaDoPerfil != 1 {
		t.Errorf("o JSON diz o perfil e quantos achados ficaram fora: perfil=%q fora=%d\n%s", rel.Perfil, rel.ForaDoPerfil, saida)
	}

	// Sem --profile, nada é filtrado e o JSON não fala de perfil.
	saida, _, err = rodar(t, "scan", "-n", "payments", "-o", "json", "--no-ai")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(saida, `"perfil"`) || strings.Contains(saida, `"fora_do_perfil"`) || !strings.Contains(saida, "polaris/priorityClassNotSet") {
		t.Errorf("sem --profile, todos os achados e nenhum perfil:\n%s", saida)
	}
}

// A tela e o relatório dizem qual perfil foi aplicado e quantos achados ficaram de fora.
func TestScan_PerfilNaTelaENoRelatorio(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
	usarDependencias(t, fake.NewSimpleClientset(), backendsComPerfil(), nil, nil)

	saida, _, err := rodar(t, "scan", "-n", "payments", "--profile", "pci-dss", "--no-ai")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(saida, "scan · namespace payments · perfil pci-dss") || !strings.Contains(saida, "1 achado fora do perfil") {
		t.Errorf("a tela diz o perfil aplicado:\n%s", saida)
	}
	if strings.Contains(saida, "priority class") {
		t.Errorf("o achado fora do perfil não aparece na tela:\n%s", saida)
	}
	md := lerRelatorio(t)
	if !strings.Contains(string(md), "**Perfil:** pci-dss · 1 achado fora do perfil\n") {
		t.Errorf("o relatório diz o perfil aplicado:\n%s", md)
	}
}

// Com todos os achados fora do perfil, a linha final fala do perfil, não do cluster.
func TestScan_PerfilSemAchados(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
	usarDependencias(t, fake.NewSimpleClientset(), []backends.SecurityBackend{backendFalso{nome: "polaris", achados: []backends.Finding{
		{ID: "polaris/priorityClassNotSet", Severity: "low", Category: "reliability", Resource: "Deployment/api", Message: "Priority class should be set"},
	}}}, nil, nil)

	saida, _, err := rodar(t, "scan", "-n", "payments", "--profile", "pci-dss", "--no-ai")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(saida, "nenhum problema do perfil pci-dss") || !strings.Contains(saida, "1 achado fora do perfil") {
		t.Errorf("sem achados no perfil, a tela diz o perfil e o que ficou fora:\n%s", saida)
	}
}
