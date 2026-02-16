package scaffold

import (
	"testing"
)

func TestBlueprintContext_DefaultValues(t *testing.T) {
	ctx := &BlueprintContext{}

	// Test that zero values are acceptable
	if ctx.ProjectName != "" {
		t.Errorf("Expected empty ProjectName, got %s", ctx.ProjectName)
	}

	if ctx.EnableCI != false {
		t.Error("Expected EnableCI to default to false")
	}

	if ctx.Data != nil {
		t.Error("Expected Data map to be nil by default")
	}
}

func TestBlueprintContext_WithData(t *testing.T) {
	ctx := &BlueprintContext{
		ProjectName: "test-project",
		Domain:      "test.local",
		Email:       "admin@test.local",
		GitRepoURL:  "https://github.com/org/repo",
		GitBranch:   "main",
		Environment: "dev",

		EnableCI:           true,
		EnableDevContainer: true,
		EnableKepler:       true,

		WorkflowPattern: "gitflow",
		Topology:        "standard",
		Environments:    []string{"local", "prod"},

		RepoRootPath:    "infra",
		BusinessDomain:  "Fintech",
		ImpactLevel:     "High",
		Archetype:       "Microservices",
		GithubDiscovery: true,
		GithubOrg:       "test-org",

		Data: map[string]interface{}{
			"custom_key": "custom_value",
		},
	}

	// Verify all fields are set correctly
	if ctx.ProjectName != "test-project" {
		t.Errorf("ProjectName = %s, want test-project", ctx.ProjectName)
	}

	if ctx.Domain != "test.local" {
		t.Errorf("Domain = %s, want test.local", ctx.Domain)
	}

	if !ctx.EnableCI {
		t.Error("EnableCI should be true")
	}

	if ctx.WorkflowPattern != "gitflow" {
		t.Errorf("WorkflowPattern = %s, want gitflow", ctx.WorkflowPattern)
	}

	if len(ctx.Environments) != 2 {
		t.Errorf("Environments length = %d, want 2", len(ctx.Environments))
	}

	if ctx.Data["custom_key"] != "custom_value" {
		t.Error("Data map not set correctly")
	}
}

func TestBlueprintContext_FeatureFlags(t *testing.T) {
	tests := []struct {
		name  string
		field string
		value bool
	}{
		{"EnableCI", "EnableCI", true},
		{"EnableDiscovery", "EnableDiscovery", true},
		{"EnableWhitelist", "EnableWhitelist", true},
		{"EnableKepler", "EnableKepler", true},
		{"EnableMinio", "EnableMinio", true},
		{"EnableKEDA", "EnableKEDA", true},
		{"EnableDevContainer", "EnableDevContainer", true},
		{"EnableMetricsServer", "EnableMetricsServer", true},
		{"GithubDiscovery", "GithubDiscovery", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &BlueprintContext{}

			// Use reflection to set field
			switch tt.field {
			case "EnableCI":
				ctx.EnableCI = tt.value
				if ctx.EnableCI != tt.value {
					t.Errorf("%s not set correctly", tt.field)
				}
			case "EnableDiscovery":
				ctx.EnableDiscovery = tt.value
				if ctx.EnableDiscovery != tt.value {
					t.Errorf("%s not set correctly", tt.field)
				}
			case "EnableWhitelist":
				ctx.EnableWhitelist = tt.value
				if ctx.EnableWhitelist != tt.value {
					t.Errorf("%s not set correctly", tt.field)
				}
			case "EnableKepler":
				ctx.EnableKepler = tt.value
				if ctx.EnableKepler != tt.value {
					t.Errorf("%s not set correctly", tt.field)
				}
			case "EnableMinio":
				ctx.EnableMinio = tt.value
				if ctx.EnableMinio != tt.value {
					t.Errorf("%s not set correctly", tt.field)
				}
			case "EnableKEDA":
				ctx.EnableKEDA = tt.value
				if ctx.EnableKEDA != tt.value {
					t.Errorf("%s not set correctly", tt.field)
				}
			case "EnableDevContainer":
				ctx.EnableDevContainer = tt.value
				if ctx.EnableDevContainer != tt.value {
					t.Errorf("%s not set correctly", tt.field)
				}
			case "EnableMetricsServer":
				ctx.EnableMetricsServer = tt.value
				if ctx.EnableMetricsServer != tt.value {
					t.Errorf("%s not set correctly", tt.field)
				}
			case "GithubDiscovery":
				ctx.GithubDiscovery = tt.value
				if ctx.GithubDiscovery != tt.value {
					t.Errorf("%s not set correctly", tt.field)
				}
			}
		})
	}
}

func TestBlueprintContext_TopologyPatterns(t *testing.T) {
	topologies := []string{"single", "standard", "complete"}

	for _, topology := range topologies {
		t.Run(topology, func(t *testing.T) {
			ctx := &BlueprintContext{
				Topology: topology,
			}

			if ctx.Topology != topology {
				t.Errorf("Topology = %s, want %s", ctx.Topology, topology)
			}
		})
	}
}

func TestBlueprintContext_WorkflowPatterns(t *testing.T) {
	patterns := []string{"essential", "gitflow", "trunkbased"}

	for _, pattern := range patterns {
		t.Run(pattern, func(t *testing.T) {
			ctx := &BlueprintContext{
				WorkflowPattern: pattern,
			}

			if ctx.WorkflowPattern != pattern {
				t.Errorf("WorkflowPattern = %s, want %s", ctx.WorkflowPattern, pattern)
			}
		})
	}
}

func TestBlueprintContext_DynamicData(t *testing.T) {
	ctx := &BlueprintContext{
		Data: make(map[string]interface{}),
	}

	// Test adding various types
	ctx.Data["string_val"] = "test"
	ctx.Data["int_val"] = 42
	ctx.Data["bool_val"] = true
	ctx.Data["slice_val"] = []string{"a", "b", "c"}
	ctx.Data["map_val"] = map[string]string{"key": "value"}

	if ctx.Data["string_val"] != "test" {
		t.Error("String value not stored correctly")
	}

	if ctx.Data["int_val"] != 42 {
		t.Error("Int value not stored correctly")
	}

	if ctx.Data["bool_val"] != true {
		t.Error("Bool value not stored correctly")
	}

	if len(ctx.Data) != 5 {
		t.Errorf("Data map length = %d, want 5", len(ctx.Data))
	}
}
