package scenarios

import (
	"strings"
	"testing"

	"github.com/casheiro/yby-cli/test/e2e/sandbox"
)

func TestSanity_Doctor(t *testing.T) {
	s := sandbox.NewSandbox(t)
	s.Start(t)
	defer s.Stop()

	// Run doctor
	output := s.RunCLI(t, "doctor")

	// Even if tools are missing, it should run and report status
	if !strings.Contains(output, "Yby Doctor") {
		t.Errorf("Doctor output missing header")
	}
	if !strings.Contains(output, "Ferramentas Essenciais") {
		t.Errorf("Doctor output missing tools section")
	}

	// Since we are in Alpine without tools by default (unless upgraded),
	// we expect at least check execution.
	// We can check for "kubectl" text in output
	if !strings.Contains(output, "kubectl") {
		t.Errorf("Doctor should check kubectl")
	}
}
