package context

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectContexts(t *testing.T) {
	// Setup temp dir
	tmpDir, err := os.MkdirTemp("", "yby-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create .env (default)
	if err := os.WriteFile(filepath.Join(tmpDir, ".env"), []byte("FOO=default"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create .env.staging
	if err := os.WriteFile(filepath.Join(tmpDir, ".env.staging"), []byte("FOO=staging"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create local/k3d-config.yaml
	if err := os.Mkdir(filepath.Join(tmpDir, "local"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "local/k3d-config.yaml"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	manager := NewManager(tmpDir)
	contexts, err := manager.DetectContexts()
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}

	// Expect: default, staging, local
	// Map validation
	found := make(map[string]bool)
	for _, c := range contexts {
		found[c.Name] = true
	}

	if !found["default"] {
		t.Error("Missing default context")
	}
	if !found["staging"] {
		t.Error("Missing staging context")
	}
	if !found["local"] {
		t.Error("Missing local context")
	}
}

func TestStrictIsolation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "yby-iso")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// .env has SENSITIVE_PROD_VAR
	os.WriteFile(filepath.Join(tmpDir, ".env"), []byte("SENSITIVE_PROD_VAR=exposed"), 0644)

	// .env.staging has SAFE_VAR
	os.WriteFile(filepath.Join(tmpDir, ".env.staging"), []byte("SAFE_VAR=ok"), 0644)

	manager := NewManager(tmpDir)

	// 1. Load Staging
	// Ensure Env is clean
	os.Unsetenv("SENSITIVE_PROD_VAR")
	os.Unsetenv("SAFE_VAR")

	err = manager.LoadContext("staging")
	if err != nil {
		t.Fatalf("Failed to load staging: %v", err)
	}

	if os.Getenv("SAFE_VAR") != "ok" {
		t.Error("Staging var not loaded")
	}
	if os.Getenv("SENSITIVE_PROD_VAR") != "" {
		t.Error("STRICT ISOLATION BROKEN: .env leaked into staging!")
	}
}
