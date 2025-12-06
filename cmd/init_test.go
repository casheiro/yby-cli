package cmd

import (
	"os"
	"strings"
	"testing"
)

func TestApplyPatch(t *testing.T) {
	// Setup temp dir
	tmpDir, err := os.MkdirTemp("", "yby-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Change wd to tmp for file generation
	wd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(wd)

	// Create dummy config
	initialConfig := `
global:
  domainBase: "old.com"
  environment: "dev"
git:
  repoURL: "old-repo"
deep:
  nested:
    value: false
`
	configFile := "cluster-values.yaml"
	err = os.WriteFile(configFile, []byte(initialConfig), 0644)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		path     string
		value    interface{}
		expected string
	}{
		{
			name:     "Update String",
			path:     ".global.domainBase",
			value:    "new.com",
			expected: `domainBase: "new.com"`,
		},
		{
			name:     "Update Int (Environment as string for now)",
			path:     ".global.environment",
			value:    "prod",
			expected: `environment: "prod"`,
		},
		{
			name:     "Update Bool",
			path:     ".deep.nested.value",
			value:    true,
			expected: `value: true`, // Custom yaml logic writes "true" string for bools currently in setNodeValue
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			applyPatch(configFile, tt.path, tt.value)

			// Read back
			content, err := os.ReadFile(configFile)
			if err != nil {
				t.Fatalf("failed to read file: %v", err)
			}
			sContent := string(content)

			if !strings.Contains(sContent, tt.expected) {
				t.Errorf("Expected %s to contain %s", sContent, tt.expected)
			}
		})
	}
}

func TestBlueprintLogic(t *testing.T) {
	// Future: Mock Survey and test the full Run command if possible,
	// but Survey is hard to mock without a pty.
	// For now, unit testing applyPatch covers the critical logic.
}
