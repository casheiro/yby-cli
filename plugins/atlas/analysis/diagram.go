package analysis

import (
	"fmt"
	"path/filepath"
	"regexp"
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

// overviewIsoladosFicam são os kinds que ficam no overview mesmo sem nenhuma
// relação: sem eles, um compose sem depends_on, um chart solto ou um módulo do
// Terraform sumiriam do diagrama. Os demais (Service, Ingress, redes,
// repositórios, Kustomizations e recursos do Terraform) só aparecem ligados.
var overviewIsoladosFicam = map[string]bool{
	"Deployment": true, "StatefulSet": true, "DaemonSet": true, "CronJob": true,
	"HelmChart": true, "Application": true, "ApplicationSet": true,
	"ComposeService": true, "TerraformModule": true,
}

// MaxNosOverview é o teto de nós do overview. Acima dele, os kinds mais
// numerosos viram um nó só, até o diagrama caber.
const MaxNosOverview = 25

// DetailLevel define o nível de detalhamento do diagrama.
type DetailLevel string

const (
	// DetailOverview mostra só os componentes principais da topologia.
	DetailOverview DetailLevel = "overview"
	// DetailFull mostra todos os recursos descobertos.
	DetailFull DetailLevel = "full"
)

// infraNo é um nó desenhado: um recurso ou um grupo de recursos do mesmo kind.
type infraNo struct {
	id, label, categoria string
}

// infraAresta é uma aresta desenhada, já com os IDs Mermaid dos nós.
type infraAresta struct{ from, to, typ string }

// infraDesenho é o que vai para o diagrama. O GenerateInfraMermaid desenha e o
// GetInfraStats conta a partir dele, então os números batem com o desenho.
type infraDesenho struct {
	nos       []infraNo
	arestas   []infraAresta
	recursos  int // recursos representados, inclusive os agrupados
	agrupados int // recursos que entraram num nó agrupado
}

// montarInfra filtra os recursos pelo nível de detalhe, tira do overview os
// isolados que não definem a topologia e, acima do teto, agrupa por kind.
func montarInfra(bp *discovery.InfraBlueprint, detail DetailLevel) infraDesenho {
	var d infraDesenho
	if bp == nil {
		return d
	}
	overview := detail == DetailOverview

	// Filtrar recursos válidos pelo nível de detalhe
	var filtered []analyzers.InfraResource
	validIDs := make(map[string]bool)
	for _, r := range bp.Resources {
		if !isValidMermaidResource(r) {
			continue
		}
		if overview && !overviewKinds[r.Kind] {
			continue
		}
		filtered = append(filtered, r)
		validIDs[r.ID()] = true
	}

	// Relações entre recursos filtrados, para saber quem está ligado
	var relacoes []analyzers.InfraRelation
	connectedIDs := make(map[string]bool)
	for _, rel := range bp.Relations {
		if !validIDs[rel.From] || !validIDs[rel.To] {
			continue
		}
		relacoes = append(relacoes, rel)
		connectedIDs[rel.From] = true
		connectedIDs[rel.To] = true
	}

	// No overview, o isolado só fica se for de um kind que define a topologia.
	// Se nada sobrar (Terraform só com recursos soltos), ficam todos.
	visible := filtered
	if overview {
		visible = nil
		for _, r := range filtered {
			if connectedIDs[r.ID()] || overviewIsoladosFicam[r.Kind] {
				visible = append(visible, r)
			}
		}
		if len(visible) == 0 {
			visible = filtered
		}
	}

	// Teto do overview: agrupar os kinds mais numerosos até caber
	agrupar := make(map[string]bool)
	if overview && len(visible) > MaxNosOverview {
		contagem := make(map[string]int)
		for _, r := range visible {
			contagem[r.Kind]++
		}
		kinds := make([]string, 0, len(contagem))
		for k := range contagem {
			kinds = append(kinds, k)
		}
		sort.Slice(kinds, func(i, j int) bool {
			if contagem[kinds[i]] != contagem[kinds[j]] {
				return contagem[kinds[i]] > contagem[kinds[j]]
			}
			return kinds[i] < kinds[j]
		})
		total := len(visible)
		for _, k := range kinds {
			if total <= MaxNosOverview || contagem[k] < 2 {
				break
			}
			agrupar[k] = true
			total -= contagem[k] - 1
		}
	}

	// Nós: o grupo entra no lugar do primeiro recurso do kind
	noDe := make(map[string]string) // ID do recurso -> ID Mermaid do nó
	grupoDesenhado := make(map[string]bool)
	porKind := make(map[string]int)
	for _, r := range visible {
		porKind[r.Kind]++
	}
	for _, r := range visible {
		cat, ok := infraCategory[r.Kind]
		if !ok {
			cat = "Outros"
		}
		if agrupar[r.Kind] {
			id := infraNodeID("grupo/" + r.Kind)
			if !grupoDesenhado[r.Kind] {
				grupoDesenhado[r.Kind] = true
				label := sanitizeMermaidLabel(fmt.Sprintf("%d recursos\\n(%s)", porKind[r.Kind], r.Kind))
				d.nos = append(d.nos, infraNo{id: id, label: label, categoria: cat})
			}
			noDe[r.ID()] = id
			d.agrupados++
			continue
		}
		id := infraNodeID(r.ID())
		noDe[r.ID()] = id
		// Com namespace no rótulo, a mesma base aplicada em dois overlays não
		// vira dois nós iguais
		nome := r.Name
		if r.Namespace != "" {
			nome = r.Namespace + "/" + r.Name
		}
		label := sanitizeMermaidLabel(fmt.Sprintf("%s\\n(%s)", nome, r.Kind))
		d.nos = append(d.nos, infraNo{id: id, label: label, categoria: cat})
	}
	d.recursos = len(visible)

	// Arestas: as que chegam a um grupo apontam para ele, sem repetir; a que
	// fica dentro do mesmo grupo some
	seenEdges := make(map[string]bool)
	for _, rel := range relacoes {
		from, okFrom := noDe[rel.From]
		to, okTo := noDe[rel.To]
		if !okFrom || !okTo || (from == to && rel.From != rel.To) {
			continue
		}
		edgeKey := fmt.Sprintf("%s->%s->%s", from, to, rel.Type)
		if seenEdges[edgeKey] {
			continue
		}
		seenEdges[edgeKey] = true
		d.arestas = append(d.arestas, infraAresta{from, to, rel.Type})
	}
	return d
}

// GenerateInfraMermaid gera um diagrama Mermaid da topologia de infraestrutura.
// O nível de detalhe controla quais recursos aparecem:
//   - overview: Helm charts, ArgoCD apps, workloads, networking, compose services,
//     Kustomize e Terraform, sem os isolados que não definem a topologia e com no
//     máximo MaxNosOverview nós (acima disso, agrupados por kind)
//   - full: todos os recursos (RBAC, ConfigMaps, CRDs, etc.)
func GenerateInfraMermaid(bp *discovery.InfraBlueprint, detail DetailLevel) string {
	d := montarInfra(bp, detail)
	if len(d.nos) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("flowchart TD\n")

	// Agrupar nós por categoria
	groups := make(map[string][]infraNo)
	for _, n := range d.nos {
		groups[n.categoria] = append(groups[n.categoria], n)
	}

	// Ordenar categorias para output determinístico
	categories := make([]string, 0, len(groups))
	for cat := range groups {
		categories = append(categories, cat)
	}
	sort.Strings(categories)

	// Gerar subgraphs por categoria
	for _, cat := range categories {
		subgraphID := infraNodeID(cat)
		b.WriteString(fmt.Sprintf("  subgraph %s[\"%s\"]\n", subgraphID, cat))
		for _, n := range groups[cat] {
			b.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", n.id, n.label))
		}
		b.WriteString("  end\n")
	}

	// Gerar edges
	for _, e := range d.arestas {
		b.WriteString(fmt.Sprintf("  %s -->|%s| %s\n", e.from, e.typ, e.to))
	}

	return b.String()
}

// InfraStats contém estatísticas do diagrama de infraestrutura.
type InfraStats struct {
	TotalResources   int
	VisibleResources int // recursos representados no diagrama, inclusive os agrupados
	Nodes            int // nós desenhados (um grupo conta como um)
	Agrupados        int // recursos que entraram num nó agrupado pelo teto do overview
	Relations        int // arestas desenhadas
	Analyzers        []string
	Detail           DetailLevel
}

// GetInfraStats retorna as estatísticas do diagrama que o GenerateInfraMermaid
// desenha para o mesmo nível de detalhe.
func GetInfraStats(bp *discovery.InfraBlueprint, detail DetailLevel) InfraStats {
	if bp == nil {
		return InfraStats{}
	}
	d := montarInfra(bp, detail)
	return InfraStats{
		TotalResources:   len(bp.Resources),
		VisibleResources: d.recursos,
		Nodes:            len(d.nos),
		Agrupados:        d.agrupados,
		Relations:        len(d.arestas),
		Analyzers:        bp.Analyzers,
		Detail:           detail,
	}
}

var (
	// setaMermaid casa as setas de um flowchart, com ou sem texto (-->|x|, -- x -->).
	setaMermaid = regexp.MustCompile(`\s*(?:--[^->|]*-->|--[ox>-]|==>|===|-\.->|-\.-)\s*(?:\|[^|]*\|)?\s*`)
	// idMermaid é o identificador no começo de um trecho do flowchart.
	idMermaid = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]*`)
)

// ContarMermaid conta os nós e as arestas de um flowchart Mermaid, para as estatísticas
// descreverem o que foi gravado quando o texto veio da IA. É uma leitura de linha a linha
// do que o Atlas e os prompts produzem, não um parser completo do Mermaid.
func ContarMermaid(texto string) (nos, arestas int) {
	vistos := map[string]bool{}
	for _, linha := range strings.Split(texto, "\n") {
		linha = strings.TrimSpace(linha)
		if linha == "" || strings.HasPrefix(linha, "```") || strings.HasPrefix(linha, "%%") {
			continue
		}
		primeira := strings.Fields(linha)[0]
		switch primeira {
		case "flowchart", "graph", "subgraph", "end", "style", "classDef", "class",
			"linkStyle", "direction", "click":
			continue
		}
		trechos := setaMermaid.Split(linha, -1)
		for _, t := range trechos {
			for _, parte := range strings.Split(t, "&") {
				if id := idMermaid.FindString(strings.TrimSpace(parte)); id != "" {
					vistos[id] = true
				}
			}
		}
		arestas += len(trechos) - 1
	}
	return len(vistos), arestas
}
