package discovery

// Blueprint represents the discovered project structure.
type Blueprint struct {
	Components []Component `json:"components"`
	Roots      []string    `json:"roots"`
}

// Component represents a discovered unit of software.
type Component struct {
	Name string   `json:"name"`
	Type string   `json:"type"` // "app", "lib", "infra", "config"
	Path string   `json:"path"`
	Tags []string `json:"tags,omitempty"`
}

// Rule defines criteria for identifying a component.
type Rule struct {
	MatchFile string
	Type      string
}
