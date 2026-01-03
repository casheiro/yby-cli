package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func requireIntegration(t *testing.T) {
	if os.Getenv("YBY_AI_INTEGRATION_TESTS") != "true" {
		t.Skip("⚠️ - Skipping Integration Test: Set YBY_AI_INTEGRATION_TESTS=true to run.")
	}
}

// TestAIInit_Integration verifies the "yby init --description" flow.
// It builds the CLI on the fly to ensure we test the current code.
func TestAIInit_Integration(t *testing.T) {
	requireIntegration(t)
	// 1. Build the CLI Binary
	binDir := t.TempDir()
	binaryPath := filepath.Join(binDir, "yby")

	// We assume we are running from the project root or we know where main is.
	// Adjust path relative to test file location if needed,
	// but usually "go test ./test/e2e" runs in the package dir.
	// Let's use absolute path to project root.
	wd, _ := os.Getwd()
	projectRoot := filepath.Dir(filepath.Dir(wd)) // ../../ from test/e2e

	cmdBuild := exec.Command("go", "build", "-o", binaryPath, "./cmd/yby")
	cmdBuild.Dir = projectRoot
	if out, err := cmdBuild.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build yby CLI: %v\nOutput: %s", err, string(out))
	}

	// 2. Setup Test Environment (Target Dir)
	targetDir := t.TempDir()

	// 3. Run "yby init" with AI Description (Portuguese)
	// We use "Um banco digital para adolescentes" to test language detection.
	description := "Um banco digital para adolescentes focado em educação financeira"

	cmdInit := exec.Command(binaryPath, "init",
		"--description", description,
		"--target-dir", targetDir,
		"--project-name", "test-bank",
		"--git-repo", "https://github.com/test/bank",
		"--topology", "single",
		"--workflow", "essential",
		"--offline=false", // Force online to try to hit Ollama/API
	)

	// Capture output to verify AI detection
	outputBytes, err := cmdInit.CombinedOutput()
	output := string(outputBytes)

	if err != nil {
		t.Fatalf("yby init failed: %v\nOutput: %s", err, output)
	}

	t.Logf("CLI Output:\n%s", output)

	// 4. Verify Assertions

	// A. Check if AI was detected (Ollama or Cloud) or Fallback triggered
	if strings.Contains(output, "AI Engine Detected") {
		t.Log("✅ AI Engine was successfully detected and engaged.")

		// B. Check for AI-Generated Files
		expectedFiles := []string{
			".synapstor/00_PROJECT_OVERVIEW.md",
			".synapstor/.personas/ARCHITECT_BOT.md",
		}

		for _, f := range expectedFiles {
			fullPath := filepath.Join(targetDir, f)
			if _, err := os.Stat(fullPath); os.IsNotExist(err) {
				t.Errorf("❌ Expected AI file missing: %s", f)
			} else {
				t.Logf("✅ Found file: %s", f)
			}
		}

		// Check for UKIs (Dynamic Names)
		entries, _ := os.ReadDir(filepath.Join(targetDir, ".synapstor"))
		foundUKI := false
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), "UKI_") {
				foundUKI = true
				t.Logf("✅ Found UKI: %s", e.Name())
				break
			}
		}
		if !foundUKI {
			t.Errorf("❌ No UKI files generated in .synapstor/")
		}

		// C. Verify Language Consistency (Portuguese)
		// Read Overview content
		overviewPath := filepath.Join(targetDir, ".synapstor/00_PROJECT_OVERVIEW.md")
		contentBytes, _ := os.ReadFile(overviewPath)
		content := string(contentBytes)

		// Heuristics for Portuguese
		ptKeywords := []string{"projeto", "arquitetura", "financeira", "banco", "objetivo"}
		foundPt := false
		for _, kw := range ptKeywords {
			if strings.Contains(strings.ToLower(content), kw) {
				foundPt = true
				break
			}
		}

		if !foundPt {
			t.Logf("⚠️ Content might not be in Portuguese. Preview:\n%s", content[:200])
			// Not failing hard because AI generation is non-deterministic,
			// but warning is useful.
		} else {
			t.Log("✅ Verified Portuguese content in generated files.")
		}

	} else {
		t.Log("⚠️ No AI Engine detected (Ollama offline?). Test verified Fallback to Static Templates.")
		// Check for Standard Static Files (if any were meant to be generated)
		// Since we implemented fallback, at least the directory structure should exist
		// provided the scaffold applied successfully.
		if _, err := os.Stat(filepath.Join(targetDir, "config")); os.IsNotExist(err) {
			t.Errorf("Standard scaffold failed: config/ missing")
		}
	}
}

// TestAIProviderSelection_Gemini verifies explicit provider selection.
func TestAIProviderSelection_Gemini(t *testing.T) {
	requireIntegration(t)
	// 1. Build CLI (Reuse logic if possible, or rebuild)
	binDir := t.TempDir()
	binaryPath := filepath.Join(binDir, "yby")
	wd, _ := os.Getwd()
	projectRoot := filepath.Dir(filepath.Dir(wd))
	cmdBuild := exec.Command("go", "build", "-o", binaryPath, "./cmd/yby")
	cmdBuild.Dir = projectRoot
	if out, err := cmdBuild.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build yby CLI: %v\nOutput: %s", err, string(out))
	}

	// 2. Run with --ai-provider gemini and Dummy Key
	targetDir := t.TempDir()
	cmdInit := exec.Command(binaryPath, "init",
		"--description", "Teste de Provider",
		"--ai-provider", "gemini", // Explicit selection
		"--target-dir", targetDir,
		"--project-name", "gemini-test",
		"--git-repo", "https://git.local",
		"--topology", "single",
		"--workflow", "essential",
		"--offline=false",
	)

	// Inject Dummy Key to force "Available" state
	//	cmdInit.Env = append(os.Environ(), "GEMINI_API_KEY=AIzaSyDummyKeyForTest")

	outputBytes, err := cmdInit.CombinedOutput()
	output := string(outputBytes)

	// We expect exit code 0 (CLI succeeds even if AI fails)
	if err != nil {
		t.Fatalf("yby init failed unexpectedly: %v\nOutput: %s", err, output)
	}

	t.Logf("CLI Output:\n%s", output)

	// 3. Assertions
	// It should detect Gemini
	if !strings.Contains(output, "AI Engine Detected: Google Gemini") {
		t.Errorf("❌ Expected Gemini detection. Output:\n%s", output)
	} else {
		t.Log("✅ Correctly selected Google Gemini provider.")
	}

	// It should fail generation (Invalid Key) but not crash
	if strings.Contains(output, "AI Generation failed") {
		t.Log("✅ Correctly handled generation failure (Invalid Key).")
	}
}
