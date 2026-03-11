package plugin

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
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

	// Values are unmarshaled as interface{}, numbers usually come as int from loose YAML or we need to handle.
	// Since we used yaml.v3 (or sigs.yaml), standard behavior varies.
	// If it was JSON, it would be float64. YAML v3 typically preserves int if it looks like int.
	// However, panic says 'interface {} is float64'.
	// This means sigs.k8s.io/yaml uses JSON unmarshal under hood or v3 behaves so?
	// Safe way: cast to int with check.
	replicas, ok := values["replicas"]
	if !ok {
		t.Errorf("expected values['replicas']")
	} else {
		// Handle potential float64/int
		var asInt int
		switch v := replicas.(type) {
		case int:
			asInt = v
		case float64:
			asInt = int(v)
		default:
			t.Errorf("expected int or float64 for replicas, got %T", v)
		}
		if asInt != 3 {
			t.Errorf("expected values['replicas'] = 3, got %d", asInt)
		}
	}

	// Ensure values made it into fullCtx
	if val, ok := fullCtx.Values["image"]; !ok || val != "nginx" {
		t.Errorf("expected fullCtx.Values['image'] = 'nginx', got %v", val)
	}
}

func TestExtractTarGzZipSlip(t *testing.T) {
	// Cria um tarball malicioso com entrada que escapa do diretório destino
	dest, err := os.MkdirTemp("", "zipslip-dest-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dest)

	// Cria o tarball em memória com entrada maliciosa "../escape"
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	// Entrada maliciosa: path com ../ que escapa do destino
	hdr := &tar.Header{
		Name:     "../escape_test_file",
		Typeflag: tar.TypeReg,
		Size:     5,
		Mode:     0644,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte("oops!")); err != nil {
		t.Fatal(err)
	}
	tw.Close()
	gw.Close()

	err = extractTarGz(&buf, dest)
	if err == nil {
		t.Error("extractTarGz deveria retornar erro para entrada com Zip Slip")
	}

	// Verificar que o arquivo de escape não foi criado
	escapePath := filepath.Join(filepath.Dir(dest), "escape_test_file")
	if _, statErr := os.Stat(escapePath); statErr == nil {
		t.Error("arquivo de escape não deveria ter sido criado fora do destino")
		os.Remove(escapePath) // limpar
	}
}
