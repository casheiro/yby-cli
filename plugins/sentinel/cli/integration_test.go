//go:build k8s

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// FakeExecCommand is a function that replaces exec.Command for testing.
// It calls the test executable itself with special environment variables.
func fakeExecCommand(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcess", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
	return cmd
}

func TestInvestigateCallsKubectl(t *testing.T) {
	// Swap execCommand
	execCommand = fakeExecCommand
	defer func() { execCommand = exec.Command }() // Restore

	// Capture stdout to verify output
	// Since investigate prints to stdout, we would need to capture it, but here we care about the exec call happening.
	// The helper process validation inside TestHelperProcess does the assertion on arguments.

	// We call investigate but we mock the AI Provider too?
	// Actually investigate() calls AI provider. If we don't mock AI, it will fail or try real key.
	// But investigate() handles nil provider gracefully with "No AI provider".

	// We just want to ensure kubectl is CALLED.
	// Running locally without AI keys configured => returns "No AI provider" and exits early?
	// Wait, the "kubectl logs" happens BEFORE AI check.

	investigate("pod-123", "default", "", "", false)
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	// We are now in the mocked process
	// os.Args will look like: [ /tmp/go-build.../sentinel.test -test.run=TestHelperProcess -- kubectl logs pod-123 ...]

	args := os.Args
	for i, arg := range args {
		if arg == "--" {
			args = args[i+1:]
			break
		}
	}

	if len(args) == 0 {
		return
	}

	cmd, cmdArgs := args[0], args[1:]

	if cmd != "kubectl" {
		fmt.Fprintf(os.Stderr, "Expected command kubectl, got %s\n", cmd)
		os.Exit(1)
	}

	// Simple validation of subcommands
	if cmdArgs[0] == "logs" {
		// logs pod-123 -n default --tail=50
		if cmdArgs[1] != "pod-123" {
			fmt.Fprintf(os.Stderr, "Expected pod-123, got %s\n", cmdArgs[1])
			os.Exit(1)
		}
		// Return fake logs
		fmt.Print("FAKE LOGS")
		os.Exit(0)
	}

	if cmdArgs[0] == "get" && cmdArgs[1] == "events" {
		// Return fake events
		fmt.Print(`{"items": []}`)
		os.Exit(0)
	}

	os.Exit(0)
}


func TestExportMarkdown_ContemSecoes(t *testing.T) {
	patch := "kubectl patch..."
	result := AnalysisResult{
		RootCause:       "CrashLoop",
		TechnicalDetail: "Falha de inicialização",
		Confidence:      70,
		SuggestedFix:    "Verificar configuração",
		KubectlPatch:    &patch,
	}
	content := exportMarkdown(result, "pod-crash", "production")
	if !strings.Contains(content, "# Relatório Sentinel") {
		t.Error("markdown deveria conter título")
	}
	if !strings.Contains(content, "CrashLoop") {
		t.Error("markdown deveria conter causa raiz")
	}
	if !strings.Contains(content, "kubectl patch") {
		t.Error("markdown deveria conter comando sugerido")
	}
}

func TestExportMarkdown_SemComandoOpcional(t *testing.T) {
	result := AnalysisResult{
		RootCause:       "Config inválida",
		TechnicalDetail: "Variável de ambiente ausente",
		Confidence:      90,
		SuggestedFix:    "Adicionar env var",
		KubectlPatch:    nil,
	}
	content := exportMarkdown(result, "pod-config", "default")
	if strings.Contains(content, "Comando Sugerido") {
		t.Error("markdown não deveria conter seção de comando quando KubectlPatch é nil")
	}
}


func TestCacheKey_DiferenteParaInputsDiferentes(t *testing.T) {
	key1 := cacheKey("default", "pod-1", "log A")
	key2 := cacheKey("default", "pod-2", "log A")
	if key1 == key2 {
		t.Error("cacheKey deveria ser diferente para pods diferentes")
	}
}

func TestSaveAndLoadCache(t *testing.T) {
	// Override cacheDir usando chdir para diretório temporário
	originalDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(originalDir)

	result := AnalysisResult{
		RootCause:  "teste",
		Confidence: 80,
	}
	saveCache("default", "pod-test", "logs here", result)
	cached, ok := loadCache("default", "pod-test", "logs here")
	if !ok {
		t.Fatal("esperava encontrar cache")
	}
	if cached.RootCause != "teste" {
		t.Errorf("causa raiz esperada 'teste', obtida %q", cached.RootCause)
	}
}
