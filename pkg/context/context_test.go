package context

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManager_LoadManifest(t *testing.T) {
	// Setup: Create temp directory
	tmpDir, err := os.MkdirTemp("", "yby-context-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create .yby directory
	ybyDir := filepath.Join(tmpDir, ".yby")
	if err := os.MkdirAll(ybyDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Test Case 1: File doesn't exist
	t.Run("FileNotExists", func(t *testing.T) {
		m := NewManager(tmpDir)
		_, err := m.LoadManifest()
		if err == nil {
			t.Error("Expected error when file doesn't exist")
		}
	})

	// Test Case 2: Valid manifest
	t.Run("ValidManifest", func(t *testing.T) {
		validYAML := `current: local
environments:
  local:
    type: local
    description: Local development
    values: config/values-local.yaml
  prod:
    type: remote
    description: Production
    values: config/values-prod.yaml
    kube_config: ~/.kube/config
    kube_context: prod-cluster
    namespace: backend
`
		envFile := filepath.Join(ybyDir, "environments.yaml")
		if err := os.WriteFile(envFile, []byte(validYAML), 0644); err != nil {
			t.Fatal(err)
		}

		m := NewManager(tmpDir)
		manifest, err := m.LoadManifest()
		if err != nil {
			t.Fatalf("LoadManifest failed: %v", err)
		}

		// Assertions
		if manifest.Current != "local" {
			t.Errorf("Expected current='local', got '%s'", manifest.Current)
		}
		if len(manifest.Environments) != 2 {
			t.Errorf("Expected 2 environments, got %d", len(manifest.Environments))
		}

		// Check prod environment
		prod, ok := manifest.Environments["prod"]
		if !ok {
			t.Fatal("Expected 'prod' environment to exist")
		}
		if prod.KubeContext != "prod-cluster" {
			t.Errorf("Expected KubeContext='prod-cluster', got '%s'", prod.KubeContext)
		}
		if prod.Namespace != "backend" {
			t.Errorf("Expected Namespace='backend', got '%s'", prod.Namespace)
		}
	})

	// Test Case 3: Invalid YAML
	t.Run("InvalidYAML", func(t *testing.T) {
		invalidYAML := `current: local
environments:
  - this is not valid yaml structure
`
		envFile := filepath.Join(ybyDir, "environments.yaml")
		if err := os.WriteFile(envFile, []byte(invalidYAML), 0644); err != nil {
			t.Fatal(err)
		}

		m := NewManager(tmpDir)
		_, err := m.LoadManifest()
		if err == nil {
			t.Error("Expected error for invalid YAML")
		}
	})
}

func TestManager_SaveManifest(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "yby-context-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	ybyDir := filepath.Join(tmpDir, ".yby")
	if err := os.MkdirAll(ybyDir, 0755); err != nil {
		t.Fatal(err)
	}

	m := NewManager(tmpDir)
	manifest := &EnvironmentsManifest{
		Current: "dev",
		Environments: map[string]Environment{
			"dev": {
				Type:        "local",
				Description: "Development",
				Values:      "config/values-dev.yaml",
			},
		},
	}

	if err := m.SaveManifest(manifest); err != nil {
		t.Fatalf("SaveManifest failed: %v", err)
	}

	// Verify file was created
	envFile := filepath.Join(ybyDir, "environments.yaml")
	if _, err := os.Stat(envFile); os.IsNotExist(err) {
		t.Error("environments.yaml was not created")
	}

	// Load it back and verify
	loaded, err := m.LoadManifest()
	if err != nil {
		t.Fatalf("Failed to reload manifest: %v", err)
	}

	if loaded.Current != "dev" {
		t.Errorf("Expected current='dev', got '%s'", loaded.Current)
	}
}

func TestManager_GetCurrent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "yby-context-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	ybyDir := filepath.Join(tmpDir, ".yby")
	if err := os.MkdirAll(ybyDir, 0755); err != nil {
		t.Fatal(err)
	}

	validYAML := `current: local
environments:
  local:
    type: local
    description: Local
    values: config/values-local.yaml
  staging:
    type: remote
    description: Staging
    values: config/values-staging.yaml
`
	envFile := filepath.Join(ybyDir, "environments.yaml")
	if err := os.WriteFile(envFile, []byte(validYAML), 0644); err != nil {
		t.Fatal(err)
	}

	m := NewManager(tmpDir)

	// Test Case 1: Get current from manifest
	t.Run("FromManifest", func(t *testing.T) {
		name, env, err := m.GetCurrent()
		if err != nil {
			t.Fatalf("GetCurrent failed: %v", err)
		}
		if name != "local" {
			t.Errorf("Expected name='local', got '%s'", name)
		}
		if env.Type != "local" {
			t.Errorf("Expected type='local', got '%s'", env.Type)
		}
	})

	// Test Case 2: Override with YBY_ENV
	t.Run("WithYBY_ENV", func(t *testing.T) {
		os.Setenv("YBY_ENV", "staging")
		defer os.Unsetenv("YBY_ENV")

		name, env, err := m.GetCurrent()
		if err != nil {
			t.Fatalf("GetCurrent failed: %v", err)
		}
		if name != "staging" {
			t.Errorf("Expected name='staging', got '%s'", name)
		}
		if env.Type != "remote" {
			t.Errorf("Expected type='remote', got '%s'", env.Type)
		}
	})

	// Test Case 3: Invalid YBY_ENV
	t.Run("InvalidYBY_ENV", func(t *testing.T) {
		os.Setenv("YBY_ENV", "nonexistent")
		defer os.Unsetenv("YBY_ENV")

		_, _, err := m.GetCurrent()
		if err == nil {
			t.Error("Expected error for invalid YBY_ENV")
		}
	})
}

func TestManager_SetCurrent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "yby-context-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	ybyDir := filepath.Join(tmpDir, ".yby")
	if err := os.MkdirAll(ybyDir, 0755); err != nil {
		t.Fatal(err)
	}

	validYAML := `current: local
environments:
  local:
    type: local
    description: Local
    values: config/values-local.yaml
  prod:
    type: remote
    description: Production
    values: config/values-prod.yaml
`
	envFile := filepath.Join(ybyDir, "environments.yaml")
	if err := os.WriteFile(envFile, []byte(validYAML), 0644); err != nil {
		t.Fatal(err)
	}

	m := NewManager(tmpDir)

	// Test Case 1: Set to valid environment
	t.Run("ValidEnvironment", func(t *testing.T) {
		if err := m.SetCurrent("prod"); err != nil {
			t.Fatalf("SetCurrent failed: %v", err)
		}

		// Verify it was saved
		name, _, err := m.GetCurrent()
		if err != nil {
			t.Fatalf("GetCurrent failed: %v", err)
		}
		if name != "prod" {
			t.Errorf("Expected current='prod', got '%s'", name)
		}
	})

	// Test Case 2: Set to invalid environment
	t.Run("InvalidEnvironment", func(t *testing.T) {
		err := m.SetCurrent("nonexistent")
		if err == nil {
			t.Error("Expected error for nonexistent environment")
		}
	})
}

func TestManager_AddEnvironment(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "yby-context-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	ybyDir := filepath.Join(tmpDir, ".yby")
	if err := os.MkdirAll(ybyDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create config directory for values files
	configDir := filepath.Join(tmpDir, "config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}

	validYAML := `current: local
environments:
  local:
    type: local
    description: Local
    values: config/values-local.yaml
`
	envFile := filepath.Join(ybyDir, "environments.yaml")
	if err := os.WriteFile(envFile, []byte(validYAML), 0644); err != nil {
		t.Fatal(err)
	}

	m := NewManager(tmpDir)

	// Test Case 1: Add new environment
	t.Run("AddNew", func(t *testing.T) {
		err := m.AddEnvironment("qa", "remote", "QA Environment")
		if err != nil {
			t.Fatalf("AddEnvironment failed: %v", err)
		}

		// Verify it was added
		manifest, err := m.LoadManifest()
		if err != nil {
			t.Fatalf("LoadManifest failed: %v", err)
		}

		qa, ok := manifest.Environments["qa"]
		if !ok {
			t.Fatal("Expected 'qa' environment to exist")
		}
		if qa.Type != "remote" {
			t.Errorf("Expected type='remote', got '%s'", qa.Type)
		}
		if qa.Description != "QA Environment" {
			t.Errorf("Expected description='QA Environment', got '%s'", qa.Description)
		}

		// Verify values file was created
		valuesFile := filepath.Join(tmpDir, "config", "values-qa.yaml")
		if _, err := os.Stat(valuesFile); os.IsNotExist(err) {
			t.Error("values-qa.yaml was not created")
		}
	})

	// Test Case 2: Add duplicate environment
	t.Run("AddDuplicate", func(t *testing.T) {
		err := m.AddEnvironment("local", "local", "Duplicate")
		if err == nil {
			t.Error("Expected error when adding duplicate environment")
		}
	})
}
