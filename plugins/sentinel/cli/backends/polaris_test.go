//go:build k8s

package backends

import (
	"reflect"
	"strings"
	"testing"

	polarisconfig "github.com/fairwindsops/polaris/pkg/config"
	"github.com/fairwindsops/polaris/pkg/validator"
)

// TestPolarisBackendName verifica que o nome do backend é "polaris".
func TestPolarisBackendName(t *testing.T) {
	backend := NewPolarisBackend()
	if backend.Name() != "polaris" {
		t.Errorf("esperado Name() = \"polaris\", obtido %q", backend.Name())
	}
}

// TestPolarisBackendIsAvailable verifica que o backend está sempre disponível.
func TestPolarisBackendIsAvailable(t *testing.T) {
	backend := NewPolarisBackend()
	if !backend.IsAvailable() {
		t.Error("esperado IsAvailable() = true, obtido false")
	}
}

// TestPolarisBackendImplementsInterface verifica que PolarisBackend
// satisfaz a interface SecurityBackend em tempo de compilação.
func TestPolarisBackendImplementsInterface(t *testing.T) {
	var _ SecurityBackend = (*PolarisBackend)(nil)
}

// TestMapPolarisSeverity verifica o mapeamento de severidades Polaris → Sentinel.
func TestMapPolarisSeverity(t *testing.T) {
	tests := []struct {
		name     string
		input    string // simula o valor da severidade
		expected string
	}{
		{"danger para critical", "danger", "critical"},
		{"warning para high", "warning", "high"},
		{"ignore para info", "ignore", "info"},
		{"vazio para info", "", "info"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// mapPolarisSeverity recebe polarisconfig.Severity que é type string
			got := mapPolarisSeverity(polarisconfig.Severity(tt.input))
			if got != tt.expected {
				t.Errorf("mapPolarisSeverity(%q) = %q, esperado %q", tt.input, got, tt.expected)
			}
		})
	}
}

// TestFormatRecommendation verifica a formatação de detalhes como recomendação.
func TestFormatRecommendation(t *testing.T) {
	tests := []struct {
		name     string
		details  []string
		expected string
	}{
		{"vazio", nil, ""},
		{"um detalhe", []string{"ajustar limites de CPU"}, "ajustar limites de CPU"},
		{"multiplos detalhes", []string{"limitar CPU", "definir memoria"}, "limitar CPU; definir memoria"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatRecommendation(tt.details)
			if got != tt.expected {
				t.Errorf("formatRecommendation(%v) = %q, esperado %q", tt.details, got, tt.expected)
			}
		})
	}
}

// resultadoPolaris monta um resultado do Polaris com um check que falhou no controller e
// outro no contêiner.
func resultadoPolaris(kind, nome, namespace string) validator.Result {
	falha := func(id string, sev polarisconfig.Severity) validator.ResultMessage {
		return validator.ResultMessage{ID: id, Message: "falhou " + id, Severity: sev}
	}
	return validator.Result{
		Kind: kind, Name: nome, Namespace: namespace,
		Results: validator.ResultSet{"deploymentMissingReplicas": falha("deploymentMissingReplicas", polarisconfig.SeverityWarning)},
		PodResult: &validator.PodResult{
			ContainerResults: []validator.ContainerResult{{
				Name:    "app",
				Results: validator.ResultSet{"runAsRootAllowed": falha("runAsRootAllowed", polarisconfig.SeverityDanger)},
			}},
		},
	}
}

// TestMapPolarisResults_SoONamespacePedidoEEmOrdem: os recursos de outros namespaces
// ficam de fora, o recurso que representa o check é o primeiro em ordem alfabética e a
// saída é a mesma em toda rodada.
func TestMapPolarisResults_SoONamespacePedidoEEmOrdem(t *testing.T) {
	audit := validator.AuditData{Results: []validator.Result{
		resultadoPolaris("Deployment", "worker", "payments"),
		resultadoPolaris("Deployment", "coredns", "kube-system"),
		resultadoPolaris("Deployment", "api", "payments"),
		resultadoPolaris("Deployment", "local-path-provisioner", "kube-system"),
		resultadoPolaris("Job", "migrate", "payments"),
	}}
	primeira := mapPolarisResults(audit, "payments")
	if len(primeira) != 2 {
		t.Fatalf("esperava 2 findings (um por check), obtido %d: %+v", len(primeira), primeira)
	}
	for _, f := range primeira {
		if f.Resource != "Deployment/api (+2)" {
			t.Errorf("%s: o recurso é o primeiro em ordem, com os outros contados, obtido %q", f.ID, f.Resource)
		}
		if strings.Contains(f.Resource, "coredns") || strings.Contains(f.Resource, "local-path") || f.Namespace != "payments" {
			t.Errorf("%s: recurso de outro namespace no scan de payments: %+v", f.ID, f)
		}
	}
	if primeira[0].ID != "polaris/deploymentMissingReplicas" || primeira[1].ID != "polaris/runAsRootAllowed" {
		t.Errorf("findings fora de ordem: %s, %s", primeira[0].ID, primeira[1].ID)
	}
	for i := 0; i < 20; i++ {
		if outra := mapPolarisResults(audit, "payments"); !reflect.DeepEqual(outra, primeira) {
			t.Fatalf("rodada %d diferente:\n%+v\n%+v", i, outra, primeira)
		}
	}
}
