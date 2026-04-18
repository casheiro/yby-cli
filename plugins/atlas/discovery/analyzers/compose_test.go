package analyzers

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestComposeAnalyzer_Name(t *testing.T) {
	a := &ComposeAnalyzer{}
	if a.Name() != "compose" {
		t.Errorf("esperado 'compose', obteve %q", a.Name())
	}
}

func TestComposeAnalyzer_Analyze_ValidFile(t *testing.T) {
	dir := t.TempDir()
	content := `
services:
  web:
    image: nginx:latest
    ports:
      - "80:80"
    depends_on:
      - api
    networks:
      - frontend
  api:
    build: ./api
    depends_on:
      db:
        condition: service_healthy
    networks:
      - frontend
      - backend
  db:
    image: postgres:15
    volumes:
      - pgdata:/var/lib/postgresql/data
    networks:
      backend:
        aliases:
          - database

networks:
  frontend:
  backend:

volumes:
  pgdata:
`
	filePath := filepath.Join(dir, "docker-compose.yml")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	a := &ComposeAnalyzer{}
	result, err := a.Analyze(dir, []string{filePath})
	if err != nil {
		t.Fatal(err)
	}

	if result.Type != "compose" {
		t.Errorf("esperado tipo 'compose', obteve %q", result.Type)
	}

	// 3 serviços + 2 networks + 1 volume = 6 recursos
	if len(result.Resources) != 6 {
		t.Errorf("esperado 6 recursos, obteve %d", len(result.Resources))
		for _, r := range result.Resources {
			t.Logf("  recurso: %s/%s", r.Kind, r.Name)
		}
	}

	// Verifica que existem os tipos esperados
	kinds := map[string]int{}
	for _, r := range result.Resources {
		kinds[r.Kind]++
	}
	if kinds["ComposeService"] != 3 {
		t.Errorf("esperado 3 ComposeService, obteve %d", kinds["ComposeService"])
	}
	if kinds["ComposeNetwork"] != 2 {
		t.Errorf("esperado 2 ComposeNetwork, obteve %d", kinds["ComposeNetwork"])
	}
	if kinds["ComposeVolume"] != 1 {
		t.Errorf("esperado 1 ComposeVolume, obteve %d", kinds["ComposeVolume"])
	}

	// Verifica relações depends_on
	depRelations := filterRelationsByType(result.Relations, "depends_on")
	if len(depRelations) < 2 {
		t.Errorf("esperado ao menos 2 relações depends_on, obteve %d", len(depRelations))
	}

	// Verifica relações connects (serviços na mesma rede)
	connectRelations := filterRelationsByType(result.Relations, "connects")
	if len(connectRelations) == 0 {
		t.Error("esperado ao menos 1 relação 'connects', obteve 0")
	}
}

func TestComposeAnalyzer_Analyze_DependsOnAsList(t *testing.T) {
	dir := t.TempDir()
	content := `
services:
  web:
    image: nginx
    depends_on:
      - api
      - cache
  api:
    image: node
  cache:
    image: redis
`
	filePath := filepath.Join(dir, "compose.yaml")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	a := &ComposeAnalyzer{}
	result, err := a.Analyze(dir, []string{filePath})
	if err != nil {
		t.Fatal(err)
	}

	depRelations := filterRelationsByType(result.Relations, "depends_on")
	if len(depRelations) != 2 {
		t.Errorf("esperado 2 relações depends_on, obteve %d", len(depRelations))
	}
}

func TestComposeAnalyzer_Analyze_DependsOnAsMap(t *testing.T) {
	dir := t.TempDir()
	content := `
services:
  web:
    image: nginx
    depends_on:
      api:
        condition: service_started
      db:
        condition: service_healthy
  api:
    image: node
  db:
    image: postgres
`
	filePath := filepath.Join(dir, "docker-compose.yaml")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	a := &ComposeAnalyzer{}
	result, err := a.Analyze(dir, []string{filePath})
	if err != nil {
		t.Fatal(err)
	}

	depRelations := filterRelationsByType(result.Relations, "depends_on")
	if len(depRelations) != 2 {
		t.Errorf("esperado 2 relações depends_on (mapa), obteve %d", len(depRelations))
	}
}

func TestComposeAnalyzer_Analyze_NetworksAsList(t *testing.T) {
	dir := t.TempDir()
	content := `
services:
  app:
    image: app:latest
    networks:
      - net1
      - net2
  worker:
    image: worker:latest
    networks:
      - net1
networks:
  net1:
  net2:
`
	filePath := filepath.Join(dir, "compose.yml")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	a := &ComposeAnalyzer{}
	result, err := a.Analyze(dir, []string{filePath})
	if err != nil {
		t.Fatal(err)
	}

	connectRelations := filterRelationsByType(result.Relations, "connects")
	// app e worker compartilham net1 → 1 relação connects
	if len(connectRelations) != 1 {
		t.Errorf("esperado 1 relação connects, obteve %d", len(connectRelations))
	}
}

func TestComposeAnalyzer_Analyze_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "docker-compose.yml")
	if err := os.WriteFile(filePath, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	a := &ComposeAnalyzer{}
	result, err := a.Analyze(dir, []string{filePath})
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Resources) != 0 {
		t.Errorf("esperado 0 recursos para arquivo vazio, obteve %d", len(result.Resources))
	}
}

func TestComposeAnalyzer_Analyze_NoMatchingFiles(t *testing.T) {
	a := &ComposeAnalyzer{}
	result, err := a.Analyze("/tmp", []string{"/tmp/main.go", "/tmp/Dockerfile"})
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Resources) != 0 {
		t.Errorf("esperado 0 recursos, obteve %d", len(result.Resources))
	}
}

func TestComposeAnalyzer_Analyze_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "docker-compose.yml")
	if err := os.WriteFile(filePath, []byte(":::invalid yaml{{{"), 0644); err != nil {
		t.Fatal(err)
	}

	a := &ComposeAnalyzer{}
	// Não deve retornar erro — apenas loga warning e continua
	result, err := a.Analyze(dir, []string{filePath})
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Resources) != 0 {
		t.Errorf("esperado 0 recursos para YAML inválido, obteve %d", len(result.Resources))
	}
}

func TestComposeAnalyzer_Analyze_Metadata(t *testing.T) {
	dir := t.TempDir()
	content := `
services:
  web:
    image: nginx:1.25
    build: ./web
    ports:
      - "8080:80"
      - "443:443"
`
	filePath := filepath.Join(dir, "docker-compose.yml")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	a := &ComposeAnalyzer{}
	result, err := a.Analyze(dir, []string{filePath})
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Resources) != 1 {
		t.Fatalf("esperado 1 recurso, obteve %d", len(result.Resources))
	}

	res := result.Resources[0]
	if res.Metadata["image"] != "nginx:1.25" {
		t.Errorf("esperado imagem 'nginx:1.25', obteve %q", res.Metadata["image"])
	}
	if res.Metadata["build"] != "true" {
		t.Errorf("esperado build 'true', obteve %q", res.Metadata["build"])
	}
	if res.Metadata["ports"] != "8080:80,443:443" {
		t.Errorf("esperado ports '8080:80,443:443', obteve %q", res.Metadata["ports"])
	}
}

// filterRelationsByType filtra relações por tipo.
func filterRelationsByType(relations []InfraRelation, relType string) []InfraRelation {
	var filtered []InfraRelation
	for _, r := range relations {
		if r.Type == relType {
			filtered = append(filtered, r)
		}
	}
	// Ordena para resultados determinísticos
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].From+filtered[i].To < filtered[j].From+filtered[j].To
	})
	return filtered
}
