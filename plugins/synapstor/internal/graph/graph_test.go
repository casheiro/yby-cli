package graph

import (
	"os"
	"path/filepath"
	"testing"
)

// criarUKIsExemplo cria arquivos UKI de teste com links cruzados.
func criarUKIsExemplo(t *testing.T, dir string) {
	t.Helper()

	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}

	uki1 := `# Arquitetura do Plugin System
**ID:** UKI-PLUGIN-ARCH
**Type:** Reference
**Status:** Active

## Context
Documentação da arquitetura de plugins.

## Content
O sistema de plugins usa processos separados.
Veja também [Protocolo](UKI-1234-protocol.md) para detalhes do protocolo.
`

	uki2 := `# Protocolo de Comunicação
**ID:** UKI-PROTOCOL
**Type:** Guide
**Status:** Active

## Context
Define o protocolo JSON entre CLI e plugins.

## Content
Referência: UKI-PLUGIN-ARCH para contexto arquitetural.
`

	uki3 := `# Deploy em Kubernetes
**ID:** UKI-K8S-DEPLOY
**Type:** Decision
**Status:** Draft

## Context
Decisões sobre deploy K8s.

## Content
Este documento é independente dos outros.
`

	if err := os.WriteFile(filepath.Join(dir, "UKI-1234-plugin-arch.md"), []byte(uki1), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "UKI-1234-protocol.md"), []byte(uki2), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "UKI-1234-k8s-deploy.md"), []byte(uki3), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestBuildGraph_CriaNodesCorretamente(t *testing.T) {
	dir := t.TempDir()
	ukiDir := filepath.Join(dir, "ukis")
	criarUKIsExemplo(t, ukiDir)

	graph, err := BuildGraph(ukiDir, nil)
	if err != nil {
		t.Fatalf("erro ao construir grafo: %v", err)
	}

	if len(graph.Nodes) != 3 {
		t.Errorf("esperado 3 nós, obtido %d", len(graph.Nodes))
	}

	// Verificar que títulos foram extraídos
	titulos := make(map[string]bool)
	for _, n := range graph.Nodes {
		titulos[n.Title] = true
	}
	if !titulos["Arquitetura do Plugin System"] {
		t.Error("título 'Arquitetura do Plugin System' não encontrado")
	}
	if !titulos["Protocolo de Comunicação"] {
		t.Error("título 'Protocolo de Comunicação' não encontrado")
	}
}

func TestBuildGraph_ExtraiRelacoesPorLink(t *testing.T) {
	dir := t.TempDir()
	ukiDir := filepath.Join(dir, "ukis")
	criarUKIsExemplo(t, ukiDir)

	graph, err := BuildGraph(ukiDir, nil)
	if err != nil {
		t.Fatalf("erro ao construir grafo: %v", err)
	}

	// UKI-1234-plugin-arch.md tem link para UKI-1234-protocol.md
	encontrouLink := false
	for _, e := range graph.Edges {
		if e.From == "UKI-1234-plugin-arch.md" && e.To == "UKI-1234-protocol.md" && e.Relation == RelReferences {
			encontrouLink = true
			break
		}
	}
	if !encontrouLink {
		t.Error("esperado edge de referência de plugin-arch para protocol")
	}
}

func TestBuildGraph_ExtraiRelacoesPorID(t *testing.T) {
	dir := t.TempDir()
	ukiDir := filepath.Join(dir, "ukis")
	criarUKIsExemplo(t, ukiDir)

	graph, err := BuildGraph(ukiDir, nil)
	if err != nil {
		t.Fatalf("erro ao construir grafo: %v", err)
	}

	// UKI-1234-protocol.md menciona UKI-PLUGIN-ARCH (ID do primeiro)
	encontrouRef := false
	for _, e := range graph.Edges {
		if e.From == "UKI-1234-protocol.md" && e.To == "UKI-1234-plugin-arch.md" && e.Relation == RelRelatesTo {
			encontrouRef = true
			break
		}
	}
	if !encontrouRef {
		t.Error("esperado edge relates_to de protocol para plugin-arch via ID")
	}
}

func TestFindRelated_EncontraRelacionados(t *testing.T) {
	dir := t.TempDir()
	ukiDir := filepath.Join(dir, "ukis")
	criarUKIsExemplo(t, ukiDir)

	graph, err := BuildGraph(ukiDir, nil)
	if err != nil {
		t.Fatalf("erro ao construir grafo: %v", err)
	}

	related := FindRelated(graph, "UKI-1234-plugin-arch.md")
	if len(related) == 0 {
		t.Fatal("esperado ao menos um UKI relacionado")
	}

	encontrou := false
	for _, r := range related {
		if r.ID == "UKI-1234-protocol.md" {
			encontrou = true
		}
	}
	if !encontrou {
		t.Error("esperado UKI-1234-protocol.md como relacionado")
	}
}

func TestFindRelated_SemRelacionados(t *testing.T) {
	dir := t.TempDir()
	ukiDir := filepath.Join(dir, "ukis")
	criarUKIsExemplo(t, ukiDir)

	graph, err := BuildGraph(ukiDir, nil)
	if err != nil {
		t.Fatalf("erro ao construir grafo: %v", err)
	}

	related := FindRelated(graph, "UKI-1234-k8s-deploy.md")
	if len(related) != 0 {
		t.Errorf("esperado 0 relacionados para k8s-deploy, obtido %d", len(related))
	}
}

func TestSaveGraph_E_LoadGraph(t *testing.T) {
	dir := t.TempDir()
	graphPath := filepath.Join(dir, ".synapstor", ".knowledge_graph.json")

	original := &KnowledgeGraph{
		Nodes: []GraphNode{
			{ID: "UKI-1.md", Title: "Teste", Path: "/tmp/UKI-1.md", Type: "Reference"},
		},
		Edges: []GraphEdge{
			{From: "UKI-1.md", To: "UKI-2.md", Relation: RelDependsOn},
		},
	}

	if err := SaveGraph(original, graphPath); err != nil {
		t.Fatalf("erro ao salvar grafo: %v", err)
	}

	loaded, err := LoadGraph(graphPath)
	if err != nil {
		t.Fatalf("erro ao carregar grafo: %v", err)
	}

	if len(loaded.Nodes) != 1 {
		t.Errorf("esperado 1 nó, obtido %d", len(loaded.Nodes))
	}
	if len(loaded.Edges) != 1 {
		t.Errorf("esperado 1 aresta, obtido %d", len(loaded.Edges))
	}
	if loaded.Nodes[0].Title != "Teste" {
		t.Errorf("título esperado 'Teste', obtido %q", loaded.Nodes[0].Title)
	}
}

func TestBuildGraph_DiretorioVazio(t *testing.T) {
	dir := t.TempDir()

	graph, err := BuildGraph(dir, nil)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	if len(graph.Nodes) != 0 {
		t.Errorf("esperado 0 nós para diretório vazio, obtido %d", len(graph.Nodes))
	}
}

func TestBuildGraph_DiretorioInexistente(t *testing.T) {
	_, err := BuildGraph("/caminho/inexistente", nil)
	if err == nil {
		t.Error("esperado erro para diretório inexistente")
	}
}

func TestAddEdge_SemDuplicata(t *testing.T) {
	g := &KnowledgeGraph{}
	g.AddEdge("a.md", "b.md", RelReferences)
	g.AddEdge("a.md", "b.md", RelReferences)

	if len(g.Edges) != 1 {
		t.Errorf("esperado 1 aresta (sem duplicata), obtido %d", len(g.Edges))
	}
}

func TestBuildGraph_IgnoraArquivosNaoMd(t *testing.T) {
	dir := t.TempDir()
	// Criar arquivo não-markdown
	if err := os.WriteFile(filepath.Join(dir, "notas.txt"), []byte("texto"), 0644); err != nil {
		t.Fatal(err)
	}
	// Criar subdiretório
	if err := os.MkdirAll(filepath.Join(dir, "subdir"), 0755); err != nil {
		t.Fatal(err)
	}

	graph, err := BuildGraph(dir, nil)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	if len(graph.Nodes) != 0 {
		t.Errorf("esperado 0 nós (ignorar não-md), obtido %d", len(graph.Nodes))
	}
}
