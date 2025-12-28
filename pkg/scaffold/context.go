package scaffold

// BlueprintContext holds all data required to render templates and control the scaffold process.
type BlueprintContext struct {
	// Project configuration
	GitRepoURL  string
	GitBranch   string
	ProjectName string
	Domain      string
	Email       string
	Environment string // dev, staging, prod

	// Feature Flags for filtering
	EnableCI           bool
	EnableDiscovery    bool
	EnableWhitelist    bool
	EnableKepler       bool
	EnableMinio        bool
	EnableKEDA         bool
	EnableDevContainer bool // New Quick Win

	// Workflow Pattern Strategy
	WorkflowPattern string // "essential", "gitflow", "trunkbased"

	// Environments Strategy
	Topology     string   // "single", "standard", "complete"
	Environments []string // list of env names to generate values for

	// Template Data
	GitRepo         string
	GithubDiscovery bool
	GithubOrg       string
}
