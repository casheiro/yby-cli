package analysis

import (
	"github.com/casheiro/yby-cli/plugins/atlas/discovery"
)

// DetectCycles encontra ciclos de dependência no blueprint usando DFS com coloração.
// Retorna uma lista de ciclos, onde cada ciclo é um slice de paths formando o ciclo
// (ex: ["A", "B", "C", "A"]).
func DetectCycles(bp *discovery.Blueprint) [][]string {
	if bp == nil || len(bp.Relations) == 0 {
		return nil
	}

	// Construir grafo de adjacência
	graph := make(map[string][]string)
	nodes := make(map[string]bool)
	for _, rel := range bp.Relations {
		graph[rel.From] = append(graph[rel.From], rel.To)
		nodes[rel.From] = true
		nodes[rel.To] = true
	}

	// Coloração: 0 = branco (não visitado), 1 = cinza (em processamento), 2 = preto (finalizado)
	const (
		white = 0
		gray  = 1
		black = 2
	)

	color := make(map[string]int)
	parent := make(map[string]string)
	var cycles [][]string

	// reconstructCycle reconstrói o ciclo. start é o nó cinza encontrado novamente,
	// end é o nó atual que tem o back edge para start.
	reconstructCycle := func(start, end string) []string {
		if start == end {
			return []string{start, start}
		}
		// Remontar do end até o start seguindo os parents
		var path []string
		for node := end; node != start; node = parent[node] {
			path = append(path, node)
		}
		path = append(path, start)
		// Reverter para ordem correta (start → ... → end)
		for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
			path[i], path[j] = path[j], path[i]
		}
		// Fechar o ciclo
		path = append(path, start)
		return path
	}

	var dfs func(node string)
	dfs = func(node string) {
		color[node] = gray
		for _, neighbor := range graph[node] {
			if color[neighbor] == gray {
				// Encontrou ciclo — back edge
				cycles = append(cycles, reconstructCycle(neighbor, node))
			} else if color[neighbor] == white {
				parent[neighbor] = node
				dfs(neighbor)
			}
		}
		color[node] = black
	}

	// Executar DFS em todos os nós não visitados
	for node := range nodes {
		if color[node] == white {
			dfs(node)
		}
	}

	return cycles
}
