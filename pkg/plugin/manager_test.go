package plugin

import (
	"os"
	"path/filepath"
	"testing"

	projectContext "github.com/casheiro/yby-cli/pkg/context"
	"github.com/casheiro/yby-cli/pkg/scaffold"
)

func TestBuildPluginContext(t *testing.T) {
	// Setup temporary directory structure
	tmpDir, err := os.MkdirTemp("", "yby-test-mgr-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// .yby/environments.yaml
	envConfig := `
current: prod
environments:
  prod:
    type: remote
    kube_config: ~/.kube/config
    kube_context: prod-ctx
    namespace: backend
    values: config/values-prod.yaml
`
	if err := os.MkdirAll(filepath.Join(tmpDir, ".yby"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, ".yby", "environments.yaml"), []byte(envConfig), 0644); err != nil {
		t.Fatal(err)
	}

	// config/values-prod.yaml
	valuesConfig := `
replicas: 3
image: nginx
`
	if err := os.MkdirAll(filepath.Join(tmpDir, "config"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "config", "values-prod.yaml"), []byte(valuesConfig), 0644); err != nil {
		t.Fatal(err)
	}

	// Inputs
	coreCtx := &projectContext.CoreContext{
		ProjectName: "test-project",
		Environment: "prod",
	}
	blueprintCtx := &scaffold.BlueprintContext{
		Data: make(map[string]interface{}),
	}

	manager := NewManager() // No plugins loaded, so hooks won't do much

	// Execute
	fullCtx, values, err := manager.BuildPluginContext(coreCtx, blueprintCtx, tmpDir)
	if err != nil {
		t.Fatalf("BuildPluginContext failed: %v", err)
	}

	// Assertions
	if fullCtx.ProjectName != "test-project" {
		t.Errorf("expected ProjectName 'test-project', got '%s'", fullCtx.ProjectName)
	}
	if fullCtx.Environment != "prod" {
		t.Errorf("expected Environment 'prod', got '%s'", fullCtx.Environment)
	}

	// Infra
	if fullCtx.Infra.KubeContext != "prod-ctx" {
		t.Errorf("expected KubeContext 'prod-ctx', got '%s'", fullCtx.Infra.KubeContext)
	}
	if fullCtx.Infra.Namespace != "backend" {
		t.Errorf("expected Namespace 'backend', got '%s'", fullCtx.Infra.Namespace)
	}

	// Values
	if val, ok := values["replicas"]; !ok || val.(int) != 3 {
		t.Errorf("expected values['replicas'] = 3, got %v", val)
	}

	// Ensure values made it into fullCtx
	if val, ok := fullCtx.Values["image"]; !ok || val != "nginx" {
		t.Errorf("expected fullCtx.Values['image'] = 'nginx', got %v", val)
	}
}
