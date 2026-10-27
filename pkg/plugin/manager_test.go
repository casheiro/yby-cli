package plugin

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func TestLerAmbiente(t *testing.T) {
	tmpDir := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	// .yby/environments.yaml
	envConfig := `
current: dev
environments:
  dev:
    type: local
  prod:
    type: remote
    kube_config: ~/.kube/prod
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
	if err := os.MkdirAll(filepath.Join(tmpDir, "config"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "config", "values-prod.yaml"), []byte("replicas: 3\nimage: nginx\n"), 0644); err != nil {
		t.Fatal(err)
	}

	infra, values, err := lerAmbiente(tmpDir, "prod")
	if err != nil {
		t.Fatalf("lerAmbiente falhou: %v", err)
	}
	esperado := PluginInfrastructure{
		KubeConfig:  filepath.Join(home, ".kube", "prod"),
		KubeContext: "prod-ctx",
		Namespace:   "backend",
	}
	if infra != esperado {
		t.Errorf("infra = %+v, esperado %+v", infra, esperado)
	}
	// o sigs.k8s.io/yaml passa pelo JSON: os números chegam como float64
	if v, ok := values["replicas"].(float64); !ok || v != 3 {
		t.Errorf("values['replicas'] = %v, esperado 3", values["replicas"])
	}
	if v := values["image"]; v != "nginx" {
		t.Errorf("values['image'] = %v, esperado nginx", v)
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
