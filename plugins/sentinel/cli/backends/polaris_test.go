//go:build k8s

package backends

import (
	"testing"

	polarisconfig "github.com/fairwindsops/polaris/pkg/config"
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
