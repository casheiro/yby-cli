package config

import (
	"os"
	"testing"
)

func TestLoadSave(t *testing.T) {
	// Cleanup
	defer os.RemoveAll(StateDir)

	// 1. Load empty/new
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Failed to load: %v", err)
	}
	if cfg.CurrentContext != "" {
		t.Errorf("Expected empty context, got %s", cfg.CurrentContext)
	}

	// 2. Save
	cfg.CurrentContext = "staging"
	if err := cfg.Save(); err != nil {
		t.Fatalf("Failed to save: %v", err)
	}

	// 3. Load again
	cfg2, err := Load()
	if err != nil {
		t.Fatalf("Failed to reload: %v", err)
	}
	if cfg2.CurrentContext != "staging" {
		t.Errorf("Expected staging, got %s", cfg2.CurrentContext)
	}
}
