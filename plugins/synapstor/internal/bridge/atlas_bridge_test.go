package bridge

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/casheiro/yby-cli/plugins/synapstor/internal/graph"
)

func criarSnapshotAtlas(t *testing.T, dir string) string {
	t.Helper()

	snapshot := AtlasSnapshot{
		Components: []AtlasComponent{
			{Name: "api-gateway", Type: "service", Path: "cmd/api", Language: "go", Framework: "gin"},
			{Name: "auth-middleware", Type: "middleware", Path: "pkg/auth", Language: "go"},
			{Name: "deploy-chart", Type: "helm", Path: "deploy/helm", Language: "yaml"},
		},
	}

	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	snapshotPath := filepath.Join(dir, ".yby", "atlas-snapshot.json")
	if err := os.MkdirAll(filepath.Dir(snapshotPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(snapshotPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	return snapshotPath
}

func TestSyncFromAtlas_CriaUKIsParaComponentesNovos(t *testing.T) {
	dir := t.TempDir()
	snapshotPath := criarSnapshotAtlas(t, dir)
	ukiDir := filepath.Join(dir, ".synapstor", ".uki")

	report, err := SyncFromAtlas(snapshotPath, ukiDir)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	if report.NewUKIs != 3 {
		t.Errorf("esperado 3 novos UKIs, obtido %d", report.NewUKIs)
	}
	if report.SkippedExisting != 0 {
		t.Errorf("esperado 0 existentes, obtido %d", report.SkippedExisting)
	}
	if report.Errors != 0 {
		t.Errorf("esperado 0 erros, obtido %d", report.Errors)
	}

	// Verificar que arquivos foram criados
	entries, _ := os.ReadDir(ukiDir)
	mdCount := 0
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".md" {
			mdCount++
		}
	}
	if mdCount != 3 {
		t.Errorf("esperado 3 arquivos .md, obtido %d", mdCount)
	}
}

func TestSyncFromAtlas_SkipExistente(t *testing.T) {
	dir := t.TempDir()
	snapshotPath := criarSnapshotAtlas(t, dir)
	ukiDir := filepath.Join(dir, ".synapstor", ".uki")

	// Primeira sync
	_, err := SyncFromAtlas(snapshotPath, ukiDir)
	if err != nil {
		t.Fatalf("erro na primeira sync: %v", err)
	}

	// Segunda sync — deve skippar todos
	report, err := SyncFromAtlas(snapshotPath, ukiDir)
	if err != nil {
		t.Fatalf("erro na segunda sync: %v", err)
	}

	if report.NewUKIs != 0 {
		t.Errorf("esperado 0 novos na segunda sync, obtido %d", report.NewUKIs)
	}
	if report.SkippedExisting != 3 {
		t.Errorf("esperado 3 existentes, obtido %d", report.SkippedExisting)
	}
}

func TestSyncFromAtlas_SnapshotInexistente(t *testing.T) {
	_, err := SyncFromAtlas("/caminho/inexistente.json", "/tmp/ukis")
	if err == nil {
		t.Error("esperado erro para snapshot inexistente")
	}
}

func TestSyncFromAtlas_SnapshotInvalido(t *testing.T) {
	dir := t.TempDir()
	snapshotPath := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(snapshotPath, []byte("não é json"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := SyncFromAtlas(snapshotPath, filepath.Join(dir, "ukis"))
	if err == nil {
		t.Error("esperado erro para snapshot inválido")
	}
}

func TestSyncFromAtlasWithGraph_AdicionaEdges(t *testing.T) {
	dir := t.TempDir()

	// Snapshot com componentes no mesmo diretório pai
	snapshot := AtlasSnapshot{
		Components: []AtlasComponent{
			{Name: "service-a", Type: "service", Path: "pkg/services/a"},
			{Name: "service-b", Type: "service", Path: "pkg/services/b"},
		},
	}
	data, _ := json.MarshalIndent(snapshot, "", "  ")
	snapshotPath := filepath.Join(dir, "snapshot.json")
	if err := os.WriteFile(snapshotPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	ukiDir := filepath.Join(dir, "ukis")
	kg := &graph.KnowledgeGraph{}

	report, err := SyncFromAtlasWithGraph(snapshotPath, ukiDir, kg)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	if report.NewUKIs != 2 {
		t.Errorf("esperado 2 novos UKIs, obtido %d", report.NewUKIs)
	}

	// Deve haver edges entre UKIs do mesmo diretório pai (pkg/services)
	if len(kg.Edges) == 0 {
		t.Error("esperado ao menos 1 edge no grafo")
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"API Gateway", "api-gateway"},
		{"auth_middleware", "auth-middleware"},
		{"pkg/auth", "pkg-auth"},
		{"service.name", "service-name"},
	}

	for _, tt := range tests {
		got := slugify(tt.input)
		if got != tt.want {
			t.Errorf("slugify(%q) = %q, esperado %q", tt.input, got, tt.want)
		}
	}
}

func TestGenerateUKIStub_ContemCampos(t *testing.T) {
	comp := AtlasComponent{
		Name:      "API Gateway",
		Type:      "service",
		Path:      "cmd/api",
		Language:  "go",
		Framework: "gin",
	}

	content := generateUKIStub(comp)

	checks := []string{
		"# API Gateway",
		"**Type:** Reference",
		"**Status:** Draft",
		"## Context",
		"- **Tipo:** service",
		"- **Caminho:** cmd/api",
		"- **Linguagem:** go",
		"- **Framework:** gin",
	}

	for _, check := range checks {
		if !containsStr(content, check) {
			t.Errorf("conteúdo deve conter %q", check)
		}
	}
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
