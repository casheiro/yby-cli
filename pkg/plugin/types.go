package plugin

// PluginManifest defines the metadata for a Yby CLI plugin.
type PluginManifest struct {
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Description string   `json:"description,omitempty"`
	Hooks       []string `json:"hooks"`
	// ConsomeContexto pede, no command, o contexto caro: os hooks context dos outros
	// plugins e o overview e o backlog do projeto. Declarar o hook context é fornecer
	// contexto, não consumir. O contexto barato (ambiente pedido) vai a todo plugin.
	ConsomeContexto bool `json:"consome_contexto,omitempty"`
}

// PluginRequest defines the structure sent to the plugin via STDIN or Env Var.
type PluginRequest struct {
	Hook string `json:"hook"`
	// Args são os argumentos do hook command e, no hook context, os pedidos do chamador ao
	// plugin (o Atlas aceita "infra", que acrescenta a topologia da infraestrutura; só a
	// ferramenta atlas_blueprint do Bard o pede).
	Args    []string               `json:"args,omitempty"`
	Context map[string]interface{} `json:"context,omitempty"`
}

// PluginResponse defines the structure received from the plugin via STDOUT.
type PluginResponse struct {
	Data  interface{} `json:"data,omitempty"`
	Error string      `json:"error,omitempty"`
}

// ContextPatch is the expected data structure for the "context" hook response.
type ContextPatch map[string]interface{}

// AssetsDefinition is the expected data structure for the "assets" hook response.
type AssetsDefinition struct {
	Path string `json:"path"` // Local absolute path or relative to plugin binary
}

// PluginInfrastructure carries infrastructure details.
type PluginInfrastructure struct {
	KubeConfig  string `json:"kube_config"`
	KubeContext string `json:"kube_context"`
	Namespace   string `json:"namespace"`
}

// PluginFullContext represents the enriched context payload.
type PluginFullContext struct {
	// Core Project Info
	ProjectName string `json:"project_name"`
	Environment string `json:"environment"`

	// Infrastructure
	Infra PluginInfrastructure `json:"infra"`

	// Configuration (parsed from values-*.yaml)
	Values map[string]interface{} `json:"values"`

	// Legacy/Generic Data bucket
	Data map[string]interface{} `json:"data,omitempty"`
}
