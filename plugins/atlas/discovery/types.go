package discovery

// Blueprint representa a estrutura descoberta do projeto.
type Blueprint struct {
	Components []Component `json:"components"`
	Relations  []Relation  `json:"relations,omitempty"`
	Roots      []string    `json:"roots"`
}

// Component representa uma unidade de software descoberta.
type Component struct {
	Name      string            `json:"name"`
	Type      string            `json:"type"` // "app", "lib", "infra", "config", "helm", "kustomize"
	Path      string            `json:"path"`
	Language  string            `json:"language,omitempty"`  // "go", "nodejs", "python", "java", "rust", "csharp"
	Framework string            `json:"framework,omitempty"` // ex: "gin", "express", "django", "spring-boot"
	Tags      []string          `json:"tags,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// Relation representa um relacionamento entre componentes.
type Relation struct {
	From string `json:"from"` // caminho do componente de origem
	To   string `json:"to"`   // caminho do componente de destino
	Type string `json:"type"` // "imports", "builds", "deploys"
}

// Rule define critérios para identificar um componente.
type Rule struct {
	MatchFile string
	MatchGlob string
	Type      string
}

// AtlasConfig representa a configuração externa do Atlas.
type AtlasConfig struct {
	Ignores []string     `yaml:"ignores"`
	Rules   []RuleConfig `yaml:"rules"`
}

// RuleConfig define uma regra customizada de detecção.
type RuleConfig struct {
	MatchFile string `yaml:"match_file"`
	MatchGlob string `yaml:"match_glob"`
	Type      string `yaml:"type"`
}
