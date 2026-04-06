package analysis

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/casheiro/yby-cli/plugins/atlas/discovery"
	"github.com/casheiro/yby-cli/plugins/atlas/discovery/analyzers"
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

// componentRelPath converte o path absoluto de um componente para relativo ao root.
// Se a conversão falhar, retorna o path original.
func componentRelPath(compPath, rootPath string) string {
	if rootPath == "" {
		return compPath
	}
	rel, err := filepath.Rel(rootPath, compPath)
	if err != nil {
		return compPath
	}
	return rel
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

// DiagramStats contém estatísticas sobre o diagrama gerado.
type DiagramStats struct {
	Components    int
	Relations     int
	ValidEdges    int
	OrphanedEdges int
}

// GenerateMermaid gera um diagrama Mermaid flowchart a partir de um Blueprint.
// rootPath é o diretório base do scan, usado para normalizar os paths dos componentes
// para ficarem consistentes com os paths relativos das relações.
func GenerateMermaid(bp *discovery.Blueprint, rootPath string) string {
	if bp == nil || len(bp.Components) == 0 {
		return "flowchart TD\n"
	}

	var b strings.Builder
	b.WriteString("flowchart TD\n")

	// Coletar IDs válidos para validação de edges
	validNodeIDs := make(map[string]bool)

	// Agrupar componentes por tipo
	groups := make(map[string][]discovery.Component)
	for _, c := range bp.Components {
		groups[c.Type] = append(groups[c.Type], c)
	}

	// Gerar subgraphs por tipo
	for typ, comps := range groups {
		b.WriteString(fmt.Sprintf("  subgraph %s[%s]\n", nodeID(typ), typ))
		for _, c := range comps {
			relPath := componentRelPath(c.Path, rootPath)
			id := nodeID(relPath)
			validNodeIDs[id] = true
			label := nodeLabel(c)
			b.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", id, label))
		}
		b.WriteString("  end\n")
	}

	// Gerar edges com labels (somente edges válidas)
	for _, rel := range bp.Relations {
		from := nodeID(rel.From)
		to := nodeID(rel.To)
		if validNodeIDs[from] && validNodeIDs[to] {
			b.WriteString(fmt.Sprintf("  %s -->|%s| %s\n", from, rel.Type, to))
		}
	}

	return b.String()
}

// ValidateDiagram verifica a qualidade do diagrama gerado e retorna estatísticas.
func ValidateDiagram(bp *discovery.Blueprint, rootPath string) DiagramStats {
	stats := DiagramStats{}
	if bp == nil {
		return stats
	}

	stats.Components = len(bp.Components)
	stats.Relations = len(bp.Relations)

	validNodeIDs := make(map[string]bool)
	for _, c := range bp.Components {
		relPath := componentRelPath(c.Path, rootPath)
		validNodeIDs[nodeID(relPath)] = true
	}

	for _, rel := range bp.Relations {
		from := nodeID(rel.From)
		to := nodeID(rel.To)
		if validNodeIDs[from] && validNodeIDs[to] {
			stats.ValidEdges++
		} else {
			stats.OrphanedEdges++
		}
	}

	return stats
}

// GenerateC4 gera um diagrama C4 simplificado (Container) a partir de um Blueprint.
// rootPath é o diretório base do scan, usado para normalizar os paths dos componentes.
func GenerateC4(bp *discovery.Blueprint, rootPath string) string {
	if bp == nil || len(bp.Components) == 0 {
		return "C4Context\n"
	}

	var b strings.Builder
	b.WriteString("C4Context\n")

	validNodeIDs := make(map[string]bool)

	// Cada componente como Container
	for _, c := range bp.Components {
		tech := c.Language
		if c.Framework != "" {
			tech = c.Language + "/" + c.Framework
		}
		relPath := componentRelPath(c.Path, rootPath)
		id := nodeID(relPath)
		validNodeIDs[id] = true
		desc := fmt.Sprintf("Componente %s", c.Type)
		b.WriteString(fmt.Sprintf("  Container(%s, \"%s\", \"%s\", \"%s\")\n",
			id, c.Name, tech, desc))
	}

	// Relações (somente válidas)
	for _, rel := range bp.Relations {
		from := nodeID(rel.From)
		to := nodeID(rel.To)
		if validNodeIDs[from] && validNodeIDs[to] {
			b.WriteString(fmt.Sprintf("  Rel(%s, %s, \"%s\")\n", from, to, rel.Type))
		}
	}

	return b.String()
}

// infraCategory mapeia Kind de recurso para categoria de agrupamento no diagrama.
var infraCategory = map[string]string{
	"HelmChart": "Helm Charts", "HelmRepository": "Repositorios Externos",
	"Deployment": "Workloads", "StatefulSet": "Workloads", "DaemonSet": "Workloads",
	"Job": "Workloads", "CronJob": "Workloads",
	"Service": "Networking", "Ingress": "Networking", "NetworkPolicy": "Networking",
	"ConfigMap": "Configuracao", "Secret": "Configuracao", "Namespace": "Configuracao",
	"SealedSecret":   "Configuracao",
	"ServiceAccount": "RBAC", "Role": "RBAC", "ClusterRole": "RBAC",
	"RoleBinding": "RBAC", "ClusterRoleBinding": "RBAC",
	"Application": "GitOps", "ApplicationSet": "GitOps", "AppProject": "GitOps",
	"ComposeService": "Compose Services", "ComposeNetwork": "Compose Networks",
	"ComposeVolume": "Compose Volumes",
	"Kustomization": "Kustomize", "KustomizeRemote": "Kustomize Remoto",
	"TerraformResource": "Terraform Resources", "TerraformModule": "Terraform Modules",
	"TerraformData": "Terraform Data",
	"ClusterIssuer": "Certificados", "Certificate": "Certificados",
	"HorizontalPodAutoscaler": "Scaling",
	"ServiceMonitor":          "Observabilidade", "PrometheusRule": "Observabilidade",
	"EventSource": "Eventos", "EventBus": "Eventos", "Sensor": "Eventos",
	"Workflow": "Workflows", "WorkflowTemplate": "Workflows", "CronWorkflow": "Workflows",
}

// infraNodeID gera um ID válido para Mermaid a partir de um ID de recurso.
// Remove caracteres especiais que quebram a sintaxe Mermaid.
func infraNodeID(resourceID string) string {
	r := strings.NewReplacer(
		"/", "_", ".", "_", "-", "_", " ", "_", ":", "_",
		"{", "", "}", "", "(", "", ")", "", "[", "", "]", "",
		"<", "", ">", "", "#", "", "&", "", "|", "", "\"", "",
		"'", "", "`", "", "~", "", "!", "", "@", "", "$", "",
		"%", "", "^", "", "*", "", "=", "", "+", "", "\\", "",
		";", "", ",", "",
	)
	id := r.Replace(resourceID)
	if id == "" {
		return "unknown"
	}
	return id
}

// sanitizeMermaidLabel remove caracteres que quebram labels Mermaid.
func sanitizeMermaidLabel(label string) string {
	r := strings.NewReplacer(
		"{{", "", "}}", "",
		"{", "", "}", "",
		"\"", "", "'", "",
		"[", "", "]", "",
		"<", "", ">", "",
		"|", "",
	)
	result := r.Replace(label)
	if strings.TrimSpace(result) == "" {
		return "unnamed"
	}
	return result
}

// isValidMermaidResource verifica se um recurso pode ser representado no Mermaid.
func isValidMermaidResource(r analyzers.InfraResource) bool {
	name := strings.TrimSpace(r.Name)
	if name == "" {
		return false
	}
	// Rejeitar nomes encriptados por SOPS
	if strings.Contains(name, "ENC[") {
		return false
	}
	// Rejeitar templates Helm não renderizados
	if strings.Contains(name, "{{") {
		return false
	}
	// Rejeitar se o nome é basicamente só caracteres especiais
	cleaned := strings.ReplaceAll(strings.ReplaceAll(name, "{", ""), "}", "")
	cleaned = strings.ReplaceAll(strings.ReplaceAll(cleaned, ".", ""), " ", "")
	return len(cleaned) > 0
}

// overviewKinds define os tipos de recurso que aparecem no diagrama overview.
// São os recursos que realmente definem a topologia da infraestrutura.
var overviewKinds = map[string]bool{
	// Helm
	"HelmChart": true, "HelmRepository": true,
	// GitOps
	"Application": true, "ApplicationSet": true,
	// Workloads
	"Deployment": true, "StatefulSet": true, "DaemonSet": true,
	"CronJob": true,
	// Networking
	"Service": true, "Ingress": true,
	// Compose
	"ComposeService": true, "ComposeNetwork": true,
	// Kustomize
	"Kustomization": true,
	// Terraform
	"TerraformModule": true, "TerraformResource": true,
}

// DetailLevel define o nível de detalhamento do diagrama.
type DetailLevel string

const (
	// DetailOverview mostra só os componentes principais da topologia.
	DetailOverview DetailLevel = "overview"
	// DetailFull mostra todos os recursos descobertos.
	DetailFull DetailLevel = "full"
)

// GenerateInfraMermaid gera um diagrama Mermaid da topologia de infraestrutura.
// O nível de detalhe controla quais recursos aparecem:
//   - overview: só Helm charts, ArgoCD apps, workloads, networking, compose services
//   - full: todos os recursos (RBAC, ConfigMaps, CRDs, etc.)
func GenerateInfraMermaid(bp *discovery.InfraBlueprint, detail DetailLevel) string {
	if bp == nil || len(bp.Resources) == 0 {
		return ""
	}

	// Filtrar recursos válidos pelo nível de detalhe
	var filtered []analyzers.InfraResource
	validIDs := make(map[string]bool)

	for _, r := range bp.Resources {
		if !isValidMermaidResource(r) {
			continue
		}
		if detail == DetailOverview && !overviewKinds[r.Kind] {
			continue
		}
		filtered = append(filtered, r)
		validIDs[r.ID()] = true
	}

	if len(filtered) == 0 {
		return ""
	}

	// Pré-calcular edges válidas para saber quais nós participam
	type edge struct{ from, to, typ string }
	var validEdges []edge
	seenEdges := make(map[string]bool)
	connectedIDs := make(map[string]bool)

	for _, rel := range bp.Relations {
		if !validIDs[rel.From] || !validIDs[rel.To] {
			continue
		}
		fromID := infraNodeID(rel.From)
		toID := infraNodeID(rel.To)
		edgeKey := fmt.Sprintf("%s->%s->%s", fromID, toID, rel.Type)
		if seenEdges[edgeKey] {
			continue
		}
		seenEdges[edgeKey] = true
		validEdges = append(validEdges, edge{fromID, toID, rel.Type})
		connectedIDs[rel.From] = true
		connectedIDs[rel.To] = true
	}

	// No overview, manter apenas nós que participam de pelo menos uma edge
	var visible []analyzers.InfraResource
	for _, r := range filtered {
		if detail == DetailOverview && !connectedIDs[r.ID()] {
			continue
		}
		visible = append(visible, r)
	}

	if len(visible) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("flowchart TD\n")

	// Agrupar recursos por categoria
	groups := make(map[string][]analyzers.InfraResource)
	for _, r := range visible {
		cat, ok := infraCategory[r.Kind]
		if !ok {
			cat = "Outros"
		}
		groups[cat] = append(groups[cat], r)
	}

	// Ordenar categorias para output determinístico
	categories := make([]string, 0, len(groups))
	for cat := range groups {
		categories = append(categories, cat)
	}
	sort.Strings(categories)

	// Gerar subgraphs por categoria
	for _, cat := range categories {
		resources := groups[cat]
		subgraphID := infraNodeID(cat)
		b.WriteString(fmt.Sprintf("  subgraph %s[\"%s\"]\n", subgraphID, cat))
		for _, r := range resources {
			id := infraNodeID(r.ID())
			label := sanitizeMermaidLabel(fmt.Sprintf("%s\\n(%s)", r.Name, r.Kind))
			b.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", id, label))
		}
		b.WriteString("  end\n")
	}

	// Gerar edges
	for _, e := range validEdges {
		b.WriteString(fmt.Sprintf("  %s -->|%s| %s\n", e.from, e.typ, e.to))
	}

	return b.String()
}

// InfraStats contém estatísticas do diagrama de infraestrutura.
type InfraStats struct {
	TotalResources   int
	VisibleResources int
	Relations        int
	Analyzers        []string
	Detail           DetailLevel
}

// GetInfraStats retorna estatísticas do InfraBlueprint para um nível de detalhe.
func GetInfraStats(bp *discovery.InfraBlueprint, detail DetailLevel) InfraStats {
	if bp == nil {
		return InfraStats{}
	}
	visible := 0
	for _, r := range bp.Resources {
		if !isValidMermaidResource(r) {
			continue
		}
		if detail == DetailOverview && !overviewKinds[r.Kind] {
			continue
		}
		visible++
	}
	return InfraStats{
		TotalResources:   len(bp.Resources),
		VisibleResources: visible,
		Relations:        len(bp.Relations),
		Analyzers:        bp.Analyzers,
		Detail:           detail,
	}
}
