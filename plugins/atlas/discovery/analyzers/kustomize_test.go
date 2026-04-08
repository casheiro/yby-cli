package analyzers

import (
	"os"
	"path/filepath"
	"testing"
)

func TestKustomizeAnalyzer_Name(t *testing.T) {
	a := &KustomizeAnalyzer{}
	if a.Name() != "kustomize" {
		t.Errorf("esperado 'kustomize', obteve %q", a.Name())
	}
}

func TestKustomizeAnalyzer_Analyze_ValidFile(t *testing.T) {
	dir := t.TempDir()
	content := `
namespace: production
resources:
  - ../base
  - deployment.yaml
bases:
  - ../common
components:
  - ../components/monitoring
`
	filePath := filepath.Join(dir, "kustomization.yaml")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	a := &KustomizeAnalyzer{}
	result, err := a.Analyze(dir, []string{filePath})
	if err != nil {
		t.Fatal(err)
	}

	if result.Type != "kustomize" {
		t.Errorf("esperado tipo 'kustomize', obteve %q", result.Type)
	}

	// 1 Kustomization (o próprio)
	kustCount := 0
	for _, r := range result.Resources {
		if r.Kind == "Kustomization" {
			kustCount++
			if r.Namespace != "production" {
				t.Errorf("esperado namespace 'production', obteve %q", r.Namespace)
			}
		}
	}
	if kustCount != 1 {
		t.Errorf("esperado 1 recurso Kustomization, obteve %d", kustCount)
	}

	// 4 relações includes (base, deployment.yaml, common, monitoring)
	includesCount := 0
	for _, rel := range result.Relations {
		if rel.Type == "includes" {
			includesCount++
		}
	}
	if includesCount != 4 {
		t.Errorf("esperado 4 relações includes, obteve %d", includesCount)
	}
}

func TestKustomizeAnalyzer_Analyze_RemoteResources(t *testing.T) {
	dir := t.TempDir()
	content := `
resources:
  - https://github.com/org/repo//manifests?ref=v1.0
  - git@github.com:org/repo.git
  - ./local-overlay
`
	filePath := filepath.Join(dir, "kustomization.yaml")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	a := &KustomizeAnalyzer{}
	result, err := a.Analyze(dir, []string{filePath})
	if err != nil {
		t.Fatal(err)
	}

	remoteCount := 0
	for _, r := range result.Resources {
		if r.Kind == "KustomizeRemote" {
			remoteCount++
		}
	}
	if remoteCount != 2 {
		t.Errorf("esperado 2 recursos KustomizeRemote, obteve %d", remoteCount)
	}

	// 3 relações includes (2 remotas + 1 local)
	includesCount := 0
	for _, rel := range result.Relations {
		if rel.Type == "includes" {
			includesCount++
		}
	}
	if includesCount != 3 {
		t.Errorf("esperado 3 relações includes, obteve %d", includesCount)
	}
}

func TestKustomizeAnalyzer_Analyze_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "kustomization.yaml")
	if err := os.WriteFile(filePath, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	a := &KustomizeAnalyzer{}
	result, err := a.Analyze(dir, []string{filePath})
	if err != nil {
		t.Fatal(err)
	}

	// Mesmo vazio, cria o recurso Kustomization do diretório
	if len(result.Resources) != 1 {
		t.Errorf("esperado 1 recurso (kustomization vazia), obteve %d", len(result.Resources))
	}
	if len(result.Relations) != 0 {
		t.Errorf("esperado 0 relações, obteve %d", len(result.Relations))
	}
}

func TestKustomizeAnalyzer_Analyze_NoMatchingFiles(t *testing.T) {
	a := &KustomizeAnalyzer{}
	result, err := a.Analyze("/tmp", []string{"/tmp/main.go", "/tmp/deployment.yaml"})
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Resources) != 0 {
		t.Errorf("esperado 0 recursos, obteve %d", len(result.Resources))
	}
}

func TestKustomizeAnalyzer_Analyze_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "kustomization.yaml")
	if err := os.WriteFile(filePath, []byte(":::invalid{{{"), 0644); err != nil {
		t.Fatal(err)
	}

	a := &KustomizeAnalyzer{}
	result, err := a.Analyze(dir, []string{filePath})
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Resources) != 0 {
		t.Errorf("esperado 0 recursos para YAML inválido, obteve %d", len(result.Resources))
	}
}

func TestKustomizeAnalyzer_Analyze_YMLExtension(t *testing.T) {
	dir := t.TempDir()
	content := `
resources:
  - ../base
`
	filePath := filepath.Join(dir, "kustomization.yml")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	a := &KustomizeAnalyzer{}
	result, err := a.Analyze(dir, []string{filePath})
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Resources) != 1 {
		t.Errorf("esperado 1 recurso para .yml, obteve %d", len(result.Resources))
	}
	if len(result.Relations) != 1 {
		t.Errorf("esperado 1 relação, obteve %d", len(result.Relations))
	}
}

func TestKustomizeAnalyzer_Analyze_NestedDir(t *testing.T) {
	dir := t.TempDir()
	overlayDir := filepath.Join(dir, "overlays", "prod")
	if err := os.MkdirAll(overlayDir, 0755); err != nil {
		t.Fatal(err)
	}

	content := `
resources:
  - ../../base
`
	filePath := filepath.Join(overlayDir, "kustomization.yaml")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	a := &KustomizeAnalyzer{}
	result, err := a.Analyze(dir, []string{filePath})
	if err != nil {
		t.Fatal(err)
	}

	// O nome do recurso deve ser o path relativo do diretório
	found := false
	for _, r := range result.Resources {
		if r.Kind == "Kustomization" && r.Name == filepath.Join("overlays", "prod") {
			found = true
		}
	}
	if !found {
		t.Error("esperado recurso Kustomization com nome 'overlays/prod'")
		for _, r := range result.Resources {
			t.Logf("  recurso: %s/%s", r.Kind, r.Name)
		}
	}
}
