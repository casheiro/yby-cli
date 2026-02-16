package scaffold

import (
	"testing"
)

func TestShouldSkip_CIWorkflows(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		ctx        *BlueprintContext
		shouldSkip bool
	}{
		{
			name: "Skip workflows when CI disabled",
			path: "assets/.github/workflows/ci.yaml",
			ctx: &BlueprintContext{
				EnableCI: false,
			},
			shouldSkip: true,
		},
		{
			name: "Include workflows when CI enabled and pattern matches",
			path: "assets/.github/workflows/gitflow/release.yaml",
			ctx: &BlueprintContext{
				EnableCI:        true,
				WorkflowPattern: "gitflow",
			},
			shouldSkip: false,
		},
		{
			name: "Skip workflows when pattern doesn't match",
			path: "assets/.github/workflows/trunkbased/deploy.yaml",
			ctx: &BlueprintContext{
				EnableCI:        true,
				WorkflowPattern: "gitflow",
			},
			shouldSkip: true,
		},
		{
			name: "Don't skip workflows directory itself",
			path: "assets/.github/workflows",
			ctx: &BlueprintContext{
				EnableCI:        true,
				WorkflowPattern: "gitflow",
			},
			shouldSkip: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldSkip(tt.path, tt.ctx)
			if result != tt.shouldSkip {
				t.Errorf("shouldSkip() = %v, want %v", result, tt.shouldSkip)
			}
		})
	}
}

func TestShouldSkip_DevContainer(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		enabled    bool
		shouldSkip bool
	}{
		{
			name:       "Skip devcontainer when disabled",
			path:       "assets/.devcontainer/devcontainer.json",
			enabled:    false,
			shouldSkip: true,
		},
		{
			name:       "Include devcontainer when enabled",
			path:       "assets/.devcontainer/devcontainer.json",
			enabled:    true,
			shouldSkip: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &BlueprintContext{
				EnableDevContainer: tt.enabled,
			}
			result := shouldSkip(tt.path, ctx)
			if result != tt.shouldSkip {
				t.Errorf("shouldSkip() = %v, want %v", result, tt.shouldSkip)
			}
		})
	}
}

func TestShouldSkip_Modules(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		ctx        *BlueprintContext
		shouldSkip bool
	}{
		{
			name: "Skip Kepler when disabled",
			path: "assets/charts/kepler/values.yaml",
			ctx: &BlueprintContext{
				EnableKepler: false,
			},
			shouldSkip: true,
		},
		{
			name: "Include Kepler when enabled",
			path: "assets/charts/kepler/values.yaml",
			ctx: &BlueprintContext{
				EnableKepler: true,
			},
			shouldSkip: false,
		},
		{
			name: "Skip MinIO when disabled",
			path: "assets/charts/minio/Chart.yaml",
			ctx: &BlueprintContext{
				EnableMinio: false,
			},
			shouldSkip: true,
		},
		{
			name: "Skip KEDA when disabled",
			path: "assets/charts/keda/templates/deployment.yaml",
			ctx: &BlueprintContext{
				EnableKEDA: false,
			},
			shouldSkip: true,
		},
		{
			name: "Skip Metrics Server when disabled",
			path: "assets/manifests/observability/metrics-server.yaml",
			ctx: &BlueprintContext{
				EnableMetricsServer: false,
			},
			shouldSkip: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldSkip(tt.path, tt.ctx)
			if result != tt.shouldSkip {
				t.Errorf("shouldSkip() = %v, want %v", result, tt.shouldSkip)
			}
		})
	}
}

func TestShouldSkip_RepoFiles(t *testing.T) {
	ctx := &BlueprintContext{}

	repoFiles := []string{
		"assets/LICENSE",
		"assets/CONTRIBUTING.md",
		"assets/README.md",
	}

	for _, path := range repoFiles {
		t.Run(path, func(t *testing.T) {
			if !shouldSkip(path, ctx) {
				t.Errorf("shouldSkip(%s) should be true (repo file)", path)
			}
		})
	}
}

func TestShouldSkip_Discovery(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		enabled    bool
		shouldSkip bool
	}{
		{
			name:       "Skip discovery when disabled",
			path:       "assets/charts/discovery/values.yaml",
			enabled:    false,
			shouldSkip: true,
		},
		{
			name:       "Include discovery when enabled",
			path:       "assets/charts/discovery/values.yaml",
			enabled:    true,
			shouldSkip: false,
		},
		{
			name:       "Skip crossplane when discovery disabled",
			path:       "assets/manifests/crossplane/provider.yaml",
			enabled:    false,
			shouldSkip: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &BlueprintContext{
				EnableDiscovery: tt.enabled,
			}
			result := shouldSkip(tt.path, ctx)
			if result != tt.shouldSkip {
				t.Errorf("shouldSkip() = %v, want %v", result, tt.shouldSkip)
			}
		})
	}
}

func TestShouldSkip_RegularFiles(t *testing.T) {
	ctx := &BlueprintContext{
		EnableCI:            true,
		EnableDevContainer:  true,
		EnableKepler:        true,
		EnableMinio:         true,
		EnableKEDA:          true,
		EnableMetricsServer: true,
		EnableDiscovery:     true,
		WorkflowPattern:     "gitflow",
	}

	regularFiles := []string{
		"assets/config/cluster-values.yaml",
		"assets/charts/system/Chart.yaml",
		"assets/manifests/argocd/root-app.yaml",
	}

	for _, path := range regularFiles {
		t.Run(path, func(t *testing.T) {
			if shouldSkip(path, ctx) {
				t.Errorf("shouldSkip(%s) should be false (regular file)", path)
			}
		})
	}
}
