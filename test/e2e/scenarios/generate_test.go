package scenarios

import (
	"strings"
	"testing"

	"github.com/casheiro/yby-cli/test/e2e/sandbox"
)

func TestGenerate_Keda(t *testing.T) {
	s := sandbox.NewSandbox(t)
	s.Start(t)
	defer s.Stop()

	// Test default generation via flags
	output := s.RunCLI(t, "generate", "keda",
		"--name", "my-scaler",
		"--deployment", "my-app",
		"--namespace", "my-ns",
		"--replicas", "5",
	)

	// Validate Output Content (YAML)
	if !strings.Contains(output, "kind: ScaledObject") {
		t.Errorf("Output should contain kind: ScaledObject")
	}
	if !strings.Contains(output, "name: my-scaler") {
		t.Errorf("Output should contain name: my-scaler")
	}
	if !strings.Contains(output, "namespace: my-ns") {
		t.Errorf("Output should contain namespace: my-ns")
	}
	if !strings.Contains(output, "maxReplicaCount: 5") {
		t.Errorf("Output should contain maxReplicaCount: 5")
	}
	if !strings.Contains(output, "name: my-app") {
		t.Errorf("Output should contain deployment name: my-app")
	}
}
