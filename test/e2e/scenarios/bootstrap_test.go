package scenarios

import (
	"strings"
	"testing"

	"github.com/casheiro/yby-cli/test/e2e/sandbox"
)

func TestBootstrap_Cluster_Offline(t *testing.T) {
	// Scenario: Bootstrap inicial com sucesso (Offline)
	// Given que estou em um diretório vazio (Sandbox)
	s := sandbox.NewSandbox(t)
	s.Start(t)
	defer s.Stop()

	// Setup: Install fake tools to bypass ensureToolsInstalled
	// We do NOT install 'git' to prove we don't use it for cloning templates
	installFakeTools(t, s)

	// And executei `yby init`
	s.RunCLI(t, "init", "--topology", "standard", "--workflow", "essential", "--git-repo", "https://github.com/test/repo", "--include-ci=false")

	// Pre-condition: Delete critical assets to force restore/ensure
	// (Actually init scaffold might create them, let's remove them to test the embed logic which is triggered by bootstrap)
	// But scaffold.Apply uses the same source? If init creates them, bootstrap just verifies.
	// To strictly test bootstrap's logic (ensureTemplateAssets), we should remove them after init.
	s.RunShell(t, "rm", "-rf", "charts/system")
	s.RunShell(t, "rm", "-rf", "manifests")

	// When executo `yby bootstrap cluster`
	// Since we have fake kubectl, it will "succeed" applying manifests.
	// We need to inject GITHUB_REPO and GITHUB_TOKEN because ensureTemplateAssets hasn't run yet to provide blueprint.
	cmd := "export GITHUB_REPO=https://github.com/test/repo && export GITHUB_TOKEN=fake && yby bootstrap cluster"
	output := s.RunShell(t, "sh", "-c", cmd)
	t.Logf("Bootstrap Output: %s", output)

	// Then a mensagem "Bootstrap do Sistema" deve ser exibida
	if !strings.Contains(output, "Fase 1: Bootstrap do Sistema") {
		t.Errorf("Expected bootstrap progress message. Got: %s", output)
	}

	// And a estrutura de arquivos infra/ deve ser restaurada (Self-Repair)
	s.AssertFileExists(t, "charts/system/Chart.yaml")
	s.AssertFileExists(t, "manifests/argocd/root-app.yaml")

	// And não deve haver tentativa de conexão com yby-template (Implícito pois git não está instalado)
	// Se tentasse rodar git clone, falharia e o CLI retornaria erro.
}

func TestContext_Management(t *testing.T) {
	// Feature: Gerenciamento de Contexto
	s := sandbox.NewSandbox(t)
	s.Start(t)
	defer s.Stop()

	// Given init with multiple envs
	s.RunCLI(t, "init", "--topology", "complete", "--workflow", "essential")

	// When use prod
	s.RunCLI(t, "context", "use", "prod")

	// Then context show returns prod
	out := s.RunCLI(t, "context", "show")
	if !contains(out, "prod") {
		t.Errorf("Context should be prod, got: %s", out)
	}

	// And we can list them
	outList := s.RunCLI(t, "context", "list")
	if !contains(outList, "local") || !contains(outList, "prod") {
		t.Errorf("Missing envs in list: %s", outList)
	}
}

func installFakeTools(t *testing.T, s *sandbox.Sandbox) {
	// Create a fake executable that always succeeds
	fakeScript := `#!/bin/sh
echo "Fake tool executed: $0 $@"
exit 0
`
	// Write to a file in workspace
	s.RunShell(t, "sh", "-c", "echo '"+fakeScript+"' > /workspace/fake_tool.sh")
	s.RunShell(t, "chmod", "+x", "/workspace/fake_tool.sh")

	// Move to /usr/local/bin as kubectl and helm
	s.RunShell(t, "cp", "/workspace/fake_tool.sh", "/usr/local/bin/kubectl")
	s.RunShell(t, "cp", "/workspace/fake_tool.sh", "/usr/local/bin/helm")
}
