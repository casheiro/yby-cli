package analyzers

import (
	"os"
	"path/filepath"
	"testing"
)

// helperWriteFile cria um arquivo no caminho especificado com o conteúdo fornecido.
func helperWriteFile(t *testing.T, path, content string) {
	t.Helper()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestHelmAnalyzer_Name(t *testing.T) {
	h := NewHelmAnalyzer()
	if h.Name() != "helm" {
		t.Errorf("esperado 'helm', obteve '%s'", h.Name())
	}
}

func TestHelmAnalyzer_ChartSemDependencias(t *testing.T) {
	root := t.TempDir()
	chartPath := filepath.Join(root, "mychart", "Chart.yaml")
	helperWriteFile(t, chartPath, `
apiVersion: v2
name: mychart
version: 1.0.0
description: Um chart simples
`)

	h := NewHelmAnalyzer()
	result, err := h.Analyze(root, []string{chartPath})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	if len(result.Resources) != 1 {
		t.Fatalf("esperado 1 recurso, obteve %d", len(result.Resources))
	}

	r := result.Resources[0]
	if r.Kind != "HelmChart" {
		t.Errorf("esperado Kind 'HelmChart', obteve '%s'", r.Kind)
	}
	if r.Name != "mychart" {
		t.Errorf("esperado Name 'mychart', obteve '%s'", r.Name)
	}
	if r.APIGroup != "helm" {
		t.Errorf("esperado APIGroup 'helm', obteve '%s'", r.APIGroup)
	}
	if r.Metadata["version"] != "1.0.0" {
		t.Errorf("esperado version '1.0.0', obteve '%s'", r.Metadata["version"])
	}
	if r.Metadata["description"] != "Um chart simples" {
		t.Errorf("esperado description, obteve '%s'", r.Metadata["description"])
	}

	if len(result.Relations) != 0 {
		t.Errorf("esperado 0 relações, obteve %d", len(result.Relations))
	}
}

func TestHelmAnalyzer_DependenciaLocal(t *testing.T) {
	root := t.TempDir()

	// Chart principal
	parentChart := filepath.Join(root, "parent", "Chart.yaml")
	helperWriteFile(t, parentChart, `
apiVersion: v2
name: parent-chart
version: 2.0.0
dependencies:
  - name: child-chart
    version: 1.0.0
    repository: "file://../child"
`)

	// Chart dependência local
	childChart := filepath.Join(root, "child", "Chart.yaml")
	helperWriteFile(t, childChart, `
apiVersion: v2
name: child-chart
version: 1.0.0
`)

	h := NewHelmAnalyzer()
	result, err := h.Analyze(root, []string{parentChart, childChart})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	// Deve ter 2 resources: parent + child
	if len(result.Resources) != 2 {
		t.Fatalf("esperado 2 recursos, obteve %d", len(result.Resources))
	}

	// Deve ter 1 relação depends_on
	if len(result.Relations) != 1 {
		t.Fatalf("esperado 1 relação, obteve %d", len(result.Relations))
	}

	rel := result.Relations[0]
	if rel.Type != "depends_on" {
		t.Errorf("esperado tipo 'depends_on', obteve '%s'", rel.Type)
	}
	if rel.From != "HelmChart/parent-chart" {
		t.Errorf("esperado From 'HelmChart/parent-chart', obteve '%s'", rel.From)
	}
	if rel.To != "HelmChart/child-chart" {
		t.Errorf("esperado To 'HelmChart/child-chart', obteve '%s'", rel.To)
	}
}

func TestHelmAnalyzer_DependenciaRemota(t *testing.T) {
	root := t.TempDir()
	chartPath := filepath.Join(root, "myapp", "Chart.yaml")
	helperWriteFile(t, chartPath, `
apiVersion: v2
name: myapp
version: 1.0.0
dependencies:
  - name: postgresql
    version: "12.1.0"
    repository: "https://charts.bitnami.com/bitnami"
    condition: postgresql.enabled
  - name: redis
    version: "17.0.0"
    repository: "https://charts.bitnami.com/bitnami"
`)

	h := NewHelmAnalyzer()
	result, err := h.Analyze(root, []string{chartPath})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	// 1 HelmChart + 2 HelmRepository
	if len(result.Resources) != 3 {
		t.Fatalf("esperado 3 recursos, obteve %d", len(result.Resources))
	}

	// Verificar HelmRepository resources
	repoCount := 0
	for _, r := range result.Resources {
		if r.Kind == "HelmRepository" {
			repoCount++
			if r.Metadata["repository"] != "https://charts.bitnami.com/bitnami" {
				t.Errorf("repositório inesperado: %s", r.Metadata["repository"])
			}
		}
	}
	if repoCount != 2 {
		t.Errorf("esperado 2 HelmRepository, obteve %d", repoCount)
	}

	// 2 relações depends_on
	if len(result.Relations) != 2 {
		t.Fatalf("esperado 2 relações, obteve %d", len(result.Relations))
	}

	for _, rel := range result.Relations {
		if rel.Type != "depends_on" {
			t.Errorf("esperado tipo 'depends_on', obteve '%s'", rel.Type)
		}
		if rel.From != "HelmChart/myapp" {
			t.Errorf("esperado From 'HelmChart/myapp', obteve '%s'", rel.From)
		}
	}

	// Verificar que a condição foi salva nos metadados
	found := false
	for _, r := range result.Resources {
		if r.Kind == "HelmRepository" && r.Name == "postgresql" {
			if r.Metadata["condition"] == "postgresql.enabled" {
				found = true
			}
		}
	}
	if !found {
		t.Error("condição 'postgresql.enabled' não encontrada nos metadados do postgresql")
	}
}

func TestHelmAnalyzer_ComTemplates(t *testing.T) {
	root := t.TempDir()
	chartDir := filepath.Join(root, "webapp")

	chartPath := filepath.Join(chartDir, "Chart.yaml")
	helperWriteFile(t, chartPath, `
apiVersion: v2
name: webapp
version: 1.0.0
`)

	// Template de Deployment (sem template Go)
	helperWriteFile(t, filepath.Join(chartDir, "templates", "deployment.yaml"), `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: webapp
spec:
  replicas: 1
`)

	// Template de Service (com template Go)
	helperWriteFile(t, filepath.Join(chartDir, "templates", "service.yaml"), `
apiVersion: v1
kind: Service
metadata:
  name: {{ .Release.Name }}-svc
spec:
  type: ClusterIP
`)

	// Template com múltiplos documentos
	helperWriteFile(t, filepath.Join(chartDir, "templates", "extras.yaml"), `
apiVersion: v1
kind: ConfigMap
metadata:
  name: app-config
---
apiVersion: v1
kind: Secret
metadata:
  name: app-secret
`)

	h := NewHelmAnalyzer()
	result, err := h.Analyze(root, []string{chartPath})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	// 1 HelmChart + 4 recursos K8s (Deployment, Service, ConfigMap, Secret)
	if len(result.Resources) != 5 {
		t.Fatalf("esperado 5 recursos, obteve %d", len(result.Resources))
	}

	// 4 relações deploys
	deploysCount := 0
	for _, rel := range result.Relations {
		if rel.Type == "deploys" {
			deploysCount++
		}
	}
	if deploysCount != 4 {
		t.Errorf("esperado 4 relações 'deploys', obteve %d", deploysCount)
	}

	// Verificar que os kinds foram encontrados
	kinds := make(map[string]bool)
	for _, r := range result.Resources {
		if r.Kind != "HelmChart" {
			kinds[r.Kind] = true
		}
	}
	for _, expected := range []string{"Deployment", "Service", "ConfigMap", "Secret"} {
		if !kinds[expected] {
			t.Errorf("kind '%s' não encontrado nos recursos", expected)
		}
	}
}

func TestHelmAnalyzer_YAMLInvalido(t *testing.T) {
	root := t.TempDir()
	chartPath := filepath.Join(root, "bad", "Chart.yaml")
	helperWriteFile(t, chartPath, `
isso não é yaml válido: [
  {sem fechar
`)

	h := NewHelmAnalyzer()
	result, err := h.Analyze(root, []string{chartPath})
	if err != nil {
		t.Fatalf("não deveria retornar erro fatal: %v", err)
	}

	// YAML inválido deve ser ignorado com warning, não causar erro
	if len(result.Resources) != 0 {
		t.Errorf("esperado 0 recursos para YAML inválido, obteve %d", len(result.Resources))
	}
}

func TestHelmAnalyzer_MultiploCharts(t *testing.T) {
	root := t.TempDir()

	chart1 := filepath.Join(root, "charts", "api", "Chart.yaml")
	helperWriteFile(t, chart1, `
apiVersion: v2
name: api
version: 1.0.0
dependencies:
  - name: common
    version: 0.1.0
    repository: "file://../../libs/common"
`)

	chart2 := filepath.Join(root, "charts", "web", "Chart.yaml")
	helperWriteFile(t, chart2, `
apiVersion: v2
name: web
version: 2.0.0
dependencies:
  - name: common
    version: 0.1.0
    repository: "file://../../libs/common"
`)

	chartCommon := filepath.Join(root, "libs", "common", "Chart.yaml")
	helperWriteFile(t, chartCommon, `
apiVersion: v2
name: common
version: 0.1.0
`)

	h := NewHelmAnalyzer()
	result, err := h.Analyze(root, []string{chart1, chart2, chartCommon})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	// 3 charts
	chartCount := 0
	for _, r := range result.Resources {
		if r.Kind == "HelmChart" {
			chartCount++
		}
	}
	if chartCount != 3 {
		t.Errorf("esperado 3 HelmChart, obteve %d", chartCount)
	}

	// 2 relações depends_on (api -> common, web -> common)
	depsCount := 0
	for _, rel := range result.Relations {
		if rel.Type == "depends_on" {
			depsCount++
			if rel.To != "HelmChart/common" {
				t.Errorf("esperado dependência para 'HelmChart/common', obteve '%s'", rel.To)
			}
		}
	}
	if depsCount != 2 {
		t.Errorf("esperado 2 relações depends_on, obteve %d", depsCount)
	}
}

func TestHelmAnalyzer_ArquivoInexistente(t *testing.T) {
	root := t.TempDir()

	h := NewHelmAnalyzer()
	result, err := h.Analyze(root, []string{filepath.Join(root, "naoexiste", "Chart.yaml")})
	if err != nil {
		t.Fatalf("não deveria retornar erro fatal: %v", err)
	}

	if len(result.Resources) != 0 {
		t.Errorf("esperado 0 recursos, obteve %d", len(result.Resources))
	}
}

func TestHelmAnalyzer_DependenciaLocalNaoEncontrada(t *testing.T) {
	root := t.TempDir()
	chartPath := filepath.Join(root, "app", "Chart.yaml")
	helperWriteFile(t, chartPath, `
apiVersion: v2
name: app
version: 1.0.0
dependencies:
  - name: missing-lib
    version: 0.1.0
    repository: "file://../missing"
`)

	h := NewHelmAnalyzer()
	result, err := h.Analyze(root, []string{chartPath})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	// 1 HelmChart (app) + 1 HelmChart criado para a dependência não encontrada
	if len(result.Resources) != 2 {
		t.Fatalf("esperado 2 recursos, obteve %d", len(result.Resources))
	}

	// Relação deve existir mesmo para dependência não encontrada
	if len(result.Relations) != 1 {
		t.Fatalf("esperado 1 relação, obteve %d", len(result.Relations))
	}

	rel := result.Relations[0]
	if rel.Type != "depends_on" {
		t.Errorf("esperado tipo 'depends_on', obteve '%s'", rel.Type)
	}
}

func TestHelmAnalyzer_ResultType(t *testing.T) {
	h := NewHelmAnalyzer()
	result, err := h.Analyze(t.TempDir(), []string{})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if result.Type != "helm" {
		t.Errorf("esperado Type 'helm', obteve '%s'", result.Type)
	}
}

func TestHelmAnalyzer_ImplementaInterface(t *testing.T) {
	// Garante que HelmAnalyzer implementa a interface Analyzer
	var _ Analyzer = (*HelmAnalyzer)(nil)
}
