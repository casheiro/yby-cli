package analyzers

import (
	"os"
	"path/filepath"
	"strings"
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

	// 1 Kustomization (o próprio), com o namespace fora do ID
	kustCount := 0
	for _, r := range result.Resources {
		if r.Kind == "Kustomization" {
			kustCount++
			if r.Namespace != "" || r.Metadata["namespace"] != "production" {
				t.Errorf("esperado namespace 'production' nos metadados e fora do ID, obteve %q / %v", r.Namespace, r.Metadata)
			}
		}
	}
	if kustCount != 1 {
		t.Errorf("esperado 1 recurso Kustomization, obteve %d", kustCount)
	}

	// 3 relações includes para diretórios (base, common, monitoring); o
	// deployment.yaml é ligado pelo analyzer K8s, pelo plano
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

// escreverKustomize grava os arquivos (caminho relativo → conteúdo) em dir e
// devolve os caminhos das kustomizations.
func escreverKustomize(t *testing.T, dir string, arquivos map[string]string) []string {
	t.Helper()
	var kusts []string
	for rel, conteudo := range arquivos {
		p := writeTestFile(t, dir, filepath.FromSlash(rel), conteudo)
		if filepath.Base(p) == "kustomization.yaml" {
			kusts = append(kusts, p)
		}
	}
	return kusts
}

func TestKustomizeAnalyzer_ReferenciasPeloCaminho(t *testing.T) {
	dir := t.TempDir()
	kusts := escreverKustomize(t, dir, map[string]string{
		"apps/api/base/kustomization.yaml":          "resources: [deploy.yaml]\n",
		"apps/api/overlays/prod/kustomization.yaml": "namespace: prod\nresources: [../../base, ../../../../comum]\n",
		"kustomization.yaml":                        "resources: [apps/api/overlays/prod]\n",
		"comum/kustomization.yaml":                  "resources: []\n",
	})

	a := NewKustomizeAnalyzer()
	result, err := a.Analyze(dir, kusts)
	if err != nil {
		t.Fatal(err)
	}
	ids := make(map[string]bool)
	for _, r := range result.Resources {
		ids[r.ID()] = true
	}
	raiz := "Kustomization/" + filepath.Base(dir)
	for _, rel := range result.Relations {
		if !ids[rel.From] || !ids[rel.To] {
			t.Errorf("relação com um nó que não existe: %s -> %s", rel.From, rel.To)
		}
	}
	esperadas := map[string]bool{
		raiz + " -> Kustomization/apps/api/overlays/prod":                     true,
		"Kustomization/apps/api/overlays/prod -> Kustomization/apps/api/base": true,
		"Kustomization/apps/api/overlays/prod -> Kustomization/comum":         true,
	}
	if len(result.Relations) != len(esperadas) {
		t.Errorf("esperado %d relações, veio %v", len(esperadas), result.Relations)
	}
	for _, rel := range result.Relations {
		if !esperadas[rel.From+" -> "+rel.To] {
			t.Errorf("relação inesperada: %s -> %s", rel.From, rel.To)
		}
	}
}

func TestKustomizeAnalyzer_Plano(t *testing.T) {
	dir := t.TempDir()
	kusts := escreverKustomize(t, dir, map[string]string{
		// base sem namespace, usada por dois overlays e por um componente aninhado
		"base/kustomization.yaml": "namespace: base-ns\nresources: [deploy.yaml]\n",
		"meio/kustomization.yaml": "resources: [../base]\n",
		"dev/kustomization.yaml": "namespace: dev\nresources: [../meio, cm.yaml]\n" +
			"patches:\n  - path: patch.yaml\n  - patch: |-\n      inline\n" +
			"patchesStrategicMerge:\n  - smp.yaml\n  - |-\n    apiVersion: v1\n    kind: ConfigMap\n" +
			"patchesJson6902:\n  - path: json.yaml\n",
		"prod/kustomization.yaml": "namespace: prod\nresources: [../base]\n",
		// sozinha, sem namespace: o manifest fica com o dele
		"solta/kustomization.yaml": "resources: [svc.yaml]\n",
		// ciclo sem raiz
		"c1/kustomization.yaml": "namespace: c1\nresources: [../c2, a.yaml]\n",
		"c2/kustomization.yaml": "resources: [../c1]\n",
		// arquivo que é patch numa e recurso em outra continua recurso
		"usa-patch/kustomization.yaml": "resources: [../dev/patch.yaml]\n",
	})

	plano := NewKustomizeAnalyzer().Plano(dir, kusts)

	for _, p := range []string{"dev/smp.yaml", "dev/json.yaml"} {
		if !plano.Patches[p] {
			t.Errorf("esperado %s como patch; patches: %v", p, plano.Patches)
		}
	}
	if plano.Patches["dev/patch.yaml"] {
		t.Error("dev/patch.yaml é recurso em usa-patch e não pode ser tratado como patch")
	}
	if len(plano.Patches) != 2 {
		t.Errorf("esperado 2 patches (os inline ficam de fora), veio %v", plano.Patches)
	}

	esperados := map[string]string{
		"base/deploy.yaml": "dev,prod", // o namespace de fora prevalece sobre o base-ns
		"dev/cm.yaml":      "dev",
		"solta/svc.yaml":   "",
		"c1/a.yaml":        "c1",
		"dev/patch.yaml":   "",
	}
	for arquivo, ns := range esperados {
		if obtido := strings.Join(plano.Namespaces[arquivo], ","); obtido != ns {
			t.Errorf("namespaces de %s: esperado %q, obteve %q", arquivo, ns, obtido)
		}
	}
	if donos := strings.Join(plano.Donos["base/deploy.yaml"], ","); donos != "Kustomization/base" {
		t.Errorf("donos de base/deploy.yaml: %q", donos)
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
		if r.Kind == "Kustomization" && r.Name == "overlays/prod" {
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
