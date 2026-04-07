package analyzers

import "fmt"

// InfraResource representa um recurso de infraestrutura descoberto.
type InfraResource struct {
	Kind      string            `json:"kind"`                // "Deployment", "Service", "HelmChart", "ComposeService", "TerraformResource"
	APIGroup  string            `json:"api_group,omitempty"` // "apps/v1", "helm", "compose", "terraform"
	Name      string            `json:"name"`                // nome do recurso
	Namespace string            `json:"namespace,omitempty"` // namespace K8s (quando aplicável)
	Path      string            `json:"path"`                // arquivo de origem (relativo ao root)
	Labels    map[string]string `json:"labels,omitempty"`    // labels/tags do recurso
	Metadata  map[string]string `json:"metadata,omitempty"`  // dados extras (image, version, etc.)
}

// ID retorna um identificador único para o recurso (Kind/Name).
func (r InfraResource) ID() string {
	if r.Namespace != "" {
		return fmt.Sprintf("%s/%s/%s", r.Kind, r.Namespace, r.Name)
	}
	return fmt.Sprintf("%s/%s", r.Kind, r.Name)
}

// InfraRelation representa uma dependência entre recursos de infraestrutura.
type InfraRelation struct {
	From string `json:"from"` // ID do recurso origem
	To   string `json:"to"`   // ID do recurso destino
	Type string `json:"type"` // "selects", "routes", "depends_on", "deploys", "references", "includes"
}

// AnalyzerResult é o retorno de cada analyzer.
type AnalyzerResult struct {
	Resources []InfraResource `json:"resources"`
	Relations []InfraRelation `json:"relations,omitempty"`
	Type      string          `json:"type"` // "helm", "k8s", "compose", "kustomize", "terraform"
}

// Analyzer é a interface que cada analyzer de infraestrutura implementa.
type Analyzer interface {
	// Name retorna o identificador do analyzer ("helm", "k8s", etc.)
	Name() string
	// Analyze recebe o path raiz do projeto e os arquivos relevantes,
	// retornando os recursos e relações encontrados.
	Analyze(rootPath string, files []string) (*AnalyzerResult, error)
}
