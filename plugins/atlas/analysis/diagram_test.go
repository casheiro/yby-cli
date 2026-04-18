package analysis

import (
	"strings"
	"testing"

	"github.com/casheiro/yby-cli/plugins/atlas/discovery"
)

func TestGenerateMermaid_BlueprintVazio(t *testing.T) {
	bp := &discovery.Blueprint{Components: []discovery.Component{}}
	result := GenerateMermaid(bp, "")
	if result != "flowchart TD\n" {
		t.Errorf("esperado flowchart vazio, obtido: %s", result)
	}
}

func TestGenerateMermaid_BlueprintNil(t *testing.T) {
	result := GenerateMermaid(nil, "")
	if result != "flowchart TD\n" {
		t.Errorf("esperado flowchart vazio para nil, obtido: %s", result)
	}
}

func TestGenerateMermaid_ComComponentes(t *testing.T) {
	bp := &discovery.Blueprint{
		Components: []discovery.Component{
			{Name: "api", Type: "app", Path: "services/api", Language: "go"},
			{Name: "web", Type: "app", Path: "services/web", Language: "nodejs"},
			{Name: "api-chart", Type: "helm", Path: "charts/api"},
		},
		Relations: []discovery.Relation{
			{From: "services/api", To: "charts/api", Type: "deploys"},
		},
	}

	// Paths já relativos, rootPath vazio
	result := GenerateMermaid(bp, "")

	// Verificar que começa com flowchart
	if !strings.HasPrefix(result, "flowchart TD\n") {
		t.Error("resultado deve começar com 'flowchart TD'")
	}

	// Verificar subgraphs
	if !strings.Contains(result, "subgraph app[app]") {
		t.Error("deve conter subgraph para tipo 'app'")
	}
	if !strings.Contains(result, "subgraph helm[helm]") {
		t.Error("deve conter subgraph para tipo 'helm'")
	}

	// Verificar nós
	if !strings.Contains(result, "services_api") {
		t.Error("deve conter nó para services/api")
	}
	if !strings.Contains(result, "services_web") {
		t.Error("deve conter nó para services/web")
	}

	// Verificar labels com linguagem
	if !strings.Contains(result, "go") {
		t.Error("label deve conter linguagem 'go'")
	}

	// Verificar edge
	if !strings.Contains(result, "-->|deploys|") {
		t.Error("deve conter edge com label 'deploys'")
	}
}

func TestGenerateMermaid_PathsAbsolutos(t *testing.T) {
	// Simula o cenário real: componentes com paths absolutos, relações com relativos
	bp := &discovery.Blueprint{
		Components: []discovery.Component{
			{Name: "api", Type: "app", Path: "/home/user/project/services/api", Language: "go"},
			{Name: "web", Type: "app", Path: "/home/user/project/services/web", Language: "nodejs"},
		},
		Relations: []discovery.Relation{
			{From: "services/api", To: "services/web", Type: "imports"},
		},
	}

	result := GenerateMermaid(bp, "/home/user/project")

	// Node IDs devem usar paths relativos, batendo com as relações
	if !strings.Contains(result, "services_api") {
		t.Error("deve conter nó com path relativo services_api")
	}
	if !strings.Contains(result, "-->|imports|") {
		t.Error("deve conter edge imports (paths devem bater)")
	}
}

func TestGenerateMermaid_EdgesOrfasIgnoradas(t *testing.T) {
	bp := &discovery.Blueprint{
		Components: []discovery.Component{
			{Name: "api", Type: "app", Path: "services/api"},
		},
		Relations: []discovery.Relation{
			// Edge para nó inexistente
			{From: "services/api", To: "nao/existe", Type: "imports"},
		},
	}

	result := GenerateMermaid(bp, "")

	// Edge não deve aparecer pois o destino não existe
	if strings.Contains(result, "-->|imports|") {
		t.Error("edge para nó inexistente não deve ser incluída")
	}
}

func TestGenerateMermaid_SubgraphsAgrupadosPorTipo(t *testing.T) {
	bp := &discovery.Blueprint{
		Components: []discovery.Component{
			{Name: "svc1", Type: "app", Path: "svc1"},
			{Name: "svc2", Type: "app", Path: "svc2"},
			{Name: "infra1", Type: "infra", Path: "infra1"},
		},
	}

	result := GenerateMermaid(bp, "")

	// Contar subgraphs
	subgraphCount := strings.Count(result, "subgraph ")
	if subgraphCount != 2 {
		t.Errorf("esperado 2 subgraphs (app + infra), obtido %d", subgraphCount)
	}
}

func TestValidateDiagram(t *testing.T) {
	bp := &discovery.Blueprint{
		Components: []discovery.Component{
			{Name: "api", Type: "app", Path: "services/api"},
			{Name: "web", Type: "app", Path: "services/web"},
		},
		Relations: []discovery.Relation{
			{From: "services/api", To: "services/web", Type: "imports"},
			{From: "services/api", To: "nao/existe", Type: "broken"},
		},
	}

	stats := ValidateDiagram(bp, "")
	if stats.Components != 2 {
		t.Errorf("esperado 2 componentes, obtido %d", stats.Components)
	}
	if stats.ValidEdges != 1 {
		t.Errorf("esperado 1 edge válida, obtido %d", stats.ValidEdges)
	}
	if stats.OrphanedEdges != 1 {
		t.Errorf("esperado 1 edge órfã, obtido %d", stats.OrphanedEdges)
	}
}

func TestGenerateC4_BlueprintVazio(t *testing.T) {
	bp := &discovery.Blueprint{Components: []discovery.Component{}}
	result := GenerateC4(bp, "")
	if result != "C4Context\n" {
		t.Errorf("esperado C4Context vazio, obtido: %s", result)
	}
}

func TestGenerateC4_BlueprintNil(t *testing.T) {
	result := GenerateC4(nil, "")
	if result != "C4Context\n" {
		t.Errorf("esperado C4Context vazio para nil, obtido: %s", result)
	}
}

func TestGenerateC4_ComComponentes(t *testing.T) {
	bp := &discovery.Blueprint{
		Components: []discovery.Component{
			{Name: "api", Type: "app", Path: "services/api", Language: "go", Framework: "gin"},
			{Name: "web", Type: "app", Path: "services/web", Language: "nodejs"},
		},
		Relations: []discovery.Relation{
			{From: "services/api", To: "services/web", Type: "imports"},
		},
	}

	result := GenerateC4(bp, "")

	if !strings.HasPrefix(result, "C4Context\n") {
		t.Error("resultado deve começar com 'C4Context'")
	}

	// Verificar containers
	if !strings.Contains(result, "Container(services_api") {
		t.Error("deve conter Container para services/api")
	}
	if !strings.Contains(result, "\"go/gin\"") {
		t.Error("tech deve incluir framework: go/gin")
	}
	if !strings.Contains(result, "\"nodejs\"") {
		t.Error("tech deve incluir linguagem nodejs")
	}

	// Verificar relação
	if !strings.Contains(result, "Rel(services_api, services_web, \"imports\")") {
		t.Error("deve conter relação entre api e web")
	}
}

func TestNodeID_CaracteresEspeciais(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"services/api", "services_api"},
		{"my-app", "my_app"},
		{"pkg.utils", "pkg_utils"},
		{"", "root"},
	}

	for _, tt := range tests {
		result := nodeID(tt.input)
		if result != tt.expected {
			t.Errorf("nodeID(%q) = %q, esperado %q", tt.input, result, tt.expected)
		}
	}
}
