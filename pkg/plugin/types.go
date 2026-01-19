package plugin

// PluginManifest defines the metadata for a Yby CLI plugin.
type PluginManifest struct {
	Name    string   `json:"name"`
	Version string   `json:"version"`
	Hooks   []string `json:"hooks"`
}

// PluginRequest defines the structure sent to the plugin via STDIN or Env Var.
type PluginRequest struct {
	Hook    string                 `json:"hook"`
	Args    []string               `json:"args,omitempty"` // For "command" hook
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
