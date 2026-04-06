// Package graph implementa o Knowledge Graph entre UKIs do Synapstor.
// Extrai relações entre documentos UKI baseado em links markdown e referências de IDs.
package graph

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// GraphNode representa um UKI no knowledge graph.
type GraphNode struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Path  string `json:"path"`
	Type  string `json:"type"`
}

// GraphEdge representa uma relação entre dois UKIs.
type GraphEdge struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Relation string `json:"relation"`
}

// KnowledgeGraph armazena nós e arestas do grafo de conhecimento.
type KnowledgeGraph struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

// Relações válidas entre UKIs.
const (
	RelDependsOn  = "depends_on"
	RelSupersedes = "supersedes"
	RelRelatesTo  = "relates_to"
	RelReferences = "references"
)

// Regex para extrair links markdown para UKIs: [texto](UKI-*.md)
var reLinkUKI = regexp.MustCompile(`\[([^\]]*)\]\(([^)]*UKI-[^)]*\.md)\)`)

// Regex para extrair referências de ID: **ID:** UKI-*
var reIDRef = regexp.MustCompile(`\*\*ID:\*\*\s*(UKI-[^\s]+)`)

// Regex para extrair título do markdown: # Título
var reTitle = regexp.MustCompile(`(?m)^#\s+(.+)$`)

// Regex para extrair tipo: **Type:** ou **Tipo:**
var reType = regexp.MustCompile(`\*\*(?:Type|Tipo):\*\*\s*(\w+)`)

// Regex para detectar referências inline a UKI IDs
var reUKIRef = regexp.MustCompile(`UKI-[\w-]+`)

// BuildGraph escaneia o diretório de UKIs e constrói o knowledge graph.
func BuildGraph(ukiDir string) (*KnowledgeGraph, error) {
	graph := &KnowledgeGraph{}

	entries, err := os.ReadDir(ukiDir)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler diretório de UKIs: %w", err)
	}

	// Mapa de filename -> node para resolução de referências
	nodeByFile := make(map[string]*GraphNode)
	// Mapa de ID -> filename
	idToFile := make(map[string]string)
	// Conteúdo de cada arquivo para extração de relações
	contentByFile := make(map[string]string)

	// Primeira passada: criar nós
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		filePath := filepath.Join(ukiDir, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		content := string(data)
		contentByFile[entry.Name()] = content

		node := GraphNode{
			ID:   entry.Name(),
			Path: filePath,
		}

		// Extrair título
		if matches := reTitle.FindStringSubmatch(content); len(matches) > 1 {
			node.Title = strings.TrimSpace(matches[1])
		} else {
			node.Title = entry.Name()
		}

		// Extrair tipo
		if matches := reType.FindStringSubmatch(content); len(matches) > 1 {
			node.Type = matches[1]
		}

		// Extrair ID do UKI (pode ser diferente do filename)
		if matches := reIDRef.FindStringSubmatch(content); len(matches) > 1 {
			idToFile[matches[1]] = entry.Name()
		}

		graph.Nodes = append(graph.Nodes, node)
		nodeByFile[entry.Name()] = &graph.Nodes[len(graph.Nodes)-1]
	}

	// Segunda passada: extrair relações
	for fileName, content := range contentByFile {
		// Extrair links markdown para UKIs
		linkMatches := reLinkUKI.FindAllStringSubmatch(content, -1)
		for _, m := range linkMatches {
			targetFile := m[2]
			// Normalizar path relativo
			targetFile = filepath.Base(targetFile)
			if targetFile != fileName && nodeByFile[targetFile] != nil {
				graph.Edges = append(graph.Edges, GraphEdge{
					From:     fileName,
					To:       targetFile,
					Relation: RelReferences,
				})
			}
		}

		// Extrair referências de ID inline (UKI-xxx que não são o próprio ID do arquivo)
		selfID := ""
		if matches := reIDRef.FindStringSubmatch(content); len(matches) > 1 {
			selfID = matches[1]
		}

		allRefs := reUKIRef.FindAllString(content, -1)
		seen := make(map[string]bool)
		for _, ref := range allRefs {
			if ref == selfID || seen[ref] {
				continue
			}
			seen[ref] = true

			// Verificar se o ID referenciado pertence a um nó existente
			if targetFile, ok := idToFile[ref]; ok && targetFile != fileName {
				// Verificar se já existe edge via link markdown
				if !edgeExists(graph.Edges, fileName, targetFile) {
					graph.Edges = append(graph.Edges, GraphEdge{
						From:     fileName,
						To:       targetFile,
						Relation: RelRelatesTo,
					})
				}
			}
		}
	}

	return graph, nil
}

// edgeExists verifica se já existe uma aresta entre dois nós.
func edgeExists(edges []GraphEdge, from, to string) bool {
	for _, e := range edges {
		if e.From == from && e.To == to {
			return true
		}
	}
	return false
}

// FindRelated encontra todos os UKIs relacionados a um dado UKI ID.
func FindRelated(graph *KnowledgeGraph, ukiID string) []GraphNode {
	related := make(map[string]bool)

	for _, edge := range graph.Edges {
		if edge.From == ukiID {
			related[edge.To] = true
		}
		if edge.To == ukiID {
			related[edge.From] = true
		}
	}

	var result []GraphNode
	for _, node := range graph.Nodes {
		if related[node.ID] {
			result = append(result, node)
		}
	}

	return result
}

// SaveGraph persiste o knowledge graph em formato JSON.
func SaveGraph(graph *KnowledgeGraph, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("erro ao criar diretório para o grafo: %w", err)
	}

	data, err := json.MarshalIndent(graph, "", "  ")
	if err != nil {
		return fmt.Errorf("erro ao serializar o grafo: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("erro ao salvar o grafo: %w", err)
	}

	return nil
}

// LoadGraph carrega um knowledge graph de um arquivo JSON.
func LoadGraph(path string) (*KnowledgeGraph, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler o grafo: %w", err)
	}

	var graph KnowledgeGraph
	if err := json.Unmarshal(data, &graph); err != nil {
		return nil, fmt.Errorf("erro ao desserializar o grafo: %w", err)
	}

	return &graph, nil
}

// AddEdge adiciona uma aresta ao grafo se não existir duplicata.
func (g *KnowledgeGraph) AddEdge(from, to, relation string) {
	if !edgeExists(g.Edges, from, to) {
		g.Edges = append(g.Edges, GraphEdge{
			From:     from,
			To:       to,
			Relation: relation,
		})
	}
}
