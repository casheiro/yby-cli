package analysis

import "testing"

func TestContarMermaid(t *testing.T) {
	texto := "```\nflowchart TD\n  %% comentário\n  subgraph g[Grupo]\n    api[API] --> redis[(Redis)]\n    api -->|usa| db{{DB}}\n  end\n  classDef x fill:#fff\n  class api x\n  solo[Solo]\n  a --> b --> c\n"
	nos, arestas := ContarMermaid(texto)
	if nos != 7 || arestas != 4 {
		t.Errorf("esperava 7 nós e 4 arestas, obtido %d e %d", nos, arestas)
	}
}
