package analysis

import (
	"fmt"
	"strings"

	"github.com/casheiro/yby-cli/plugins/atlas/discovery"
)

// nodeID gera um identificador válido para Mermaid a partir de um path.
func nodeID(path string) string {
	r := strings.NewReplacer("/", "_", ".", "_", "-", "_", " ", "_")
	id := r.Replace(path)
	if id == "" {
		return "root"
	}
	return id
}

// nodeLabel gera o label de exibição de um componente.
func nodeLabel(c discovery.Component) string {
	parts := []string{c.Name}
	if c.Type != "" {
		parts = append(parts, c.Type)
	}
	if c.Language != "" {
		parts = append(parts, c.Language)
	}
	return strings.Join(parts, "\\n")
}

// GenerateMermaid gera um diagrama Mermaid flowchart a partir de um Blueprint.
func GenerateMermaid(bp *discovery.Blueprint) string {
	if bp == nil || len(bp.Components) == 0 {
		return "flowchart TD\n"
	}

	var b strings.Builder
	b.WriteString("flowchart TD\n")

	// Agrupar componentes por tipo
	groups := make(map[string][]discovery.Component)
	for _, c := range bp.Components {
		groups[c.Type] = append(groups[c.Type], c)
	}

	// Gerar subgraphs por tipo
	for typ, comps := range groups {
		b.WriteString(fmt.Sprintf("  subgraph %s[%s]\n", nodeID(typ), typ))
		for _, c := range comps {
			id := nodeID(c.Path)
			label := nodeLabel(c)
			b.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", id, label))
		}
		b.WriteString("  end\n")
	}

	// Gerar edges com labels
	for _, rel := range bp.Relations {
		from := nodeID(rel.From)
		to := nodeID(rel.To)
		b.WriteString(fmt.Sprintf("  %s -->|%s| %s\n", from, rel.Type, to))
	}

	return b.String()
}

// GenerateC4 gera um diagrama C4 simplificado (Container) a partir de um Blueprint.
func GenerateC4(bp *discovery.Blueprint) string {
	if bp == nil || len(bp.Components) == 0 {
		return "C4Context\n"
	}

	var b strings.Builder
	b.WriteString("C4Context\n")

	// Cada componente como Container
	for _, c := range bp.Components {
		tech := c.Language
		if c.Framework != "" {
			tech = c.Language + "/" + c.Framework
		}
		desc := fmt.Sprintf("Componente %s", c.Type)
		b.WriteString(fmt.Sprintf("  Container(%s, \"%s\", \"%s\", \"%s\")\n",
			nodeID(c.Path), c.Name, tech, desc))
	}

	// Relações
	for _, rel := range bp.Relations {
		b.WriteString(fmt.Sprintf("  Rel(%s, %s, \"%s\")\n",
			nodeID(rel.From), nodeID(rel.To), rel.Type))
	}

	return b.String()
}
