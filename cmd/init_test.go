package cmd

import (
	"testing"

	"github.com/casheiro/yby-cli/pkg/scaffold"
)

func TestResolveProjectName(t *testing.T) {
	tests := []struct {
		name     string
		opts     *InitOptions
		expected string
	}{
		{
			name: "Use explicit project name",
			opts: &InitOptions{
				ProjectName: "my-explicit-project",
				GitRepo:     "https://github.com/org/repo.git",
			},
			expected: "my-explicit-project",
		},
		{
			name: "Derive from git repo when no project name",
			opts: &InitOptions{
				ProjectName: "",
				GitRepo:     "https://github.com/org/awesome-app.git",
			},
			expected: "awesome-app",
		},
		{
			name: "Use default when no project name and no git repo",
			opts: &InitOptions{
				ProjectName: "",
				GitRepo:     "",
			},
			expected: "yby-project",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolveProjectName(tt.opts)
			if result != tt.expected {
				t.Errorf("resolveProjectName() = %s, want %s", result, tt.expected)
			}
		})
	}
}

func TestDeriveProjectName(t *testing.T) {
	tests := []struct {
		name     string
		repoURL  string
		expected string
	}{
		{
			name:     "HTTPS URL with .git",
			repoURL:  "https://github.com/casheiro/yby-cli.git",
			expected: "yby-cli",
		},
		{
			name:     "HTTPS URL without .git",
			repoURL:  "https://github.com/casheiro/yby-cli",
			expected: "yby-cli",
		},
		{
			name:     "SSH URL",
			repoURL:  "git@github.com:casheiro/yby-cli.git",
			expected: "yby-cli",
		},
		{
			name:     "GitLab URL",
			repoURL:  "https://gitlab.com/myorg/my-project.git",
			expected: "my-project",
		},
		{
			name:     "Bitbucket URL",
			repoURL:  "https://bitbucket.org/team/repo-name.git",
			expected: "repo-name",
		},
		{
			name:     "URL with trailing slash",
			repoURL:  "https://github.com/org/project/",
			expected: "project",
		},
		{
			name:     "Empty URL",
			repoURL:  "",
			expected: "yby-project",
		},
		{
			name:     "Invalid URL",
			repoURL:  "not-a-valid-url",
			expected: "not-a-valid-url", // Returns the string itself when no slashes
		},
		{
			name:     "URL with multiple slashes",
			repoURL:  "https://github.com/org/sub/project.git",
			expected: "project",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := deriveProjectName(tt.repoURL)
			if result != tt.expected {
				t.Errorf("deriveProjectName(%s) = %s, want %s", tt.repoURL, result, tt.expected)
			}
		})
	}
}

func TestExtractGithubOrg(t *testing.T) {
	tests := []struct {
		name     string
		repoURL  string
		expected string
	}{
		{
			name:     "HTTPS GitHub URL",
			repoURL:  "https://github.com/casheiro/yby-cli.git",
			expected: "casheiro",
		},
		{
			name:     "SSH GitHub URL",
			repoURL:  "git@github.com:casheiro/yby-cli.git",
			expected: "casheiro",
		},
		{
			name:     "GitHub URL without .git",
			repoURL:  "https://github.com/myorg/myrepo",
			expected: "myorg",
		},
		{
			name:     "GitLab URL (not GitHub)",
			repoURL:  "https://gitlab.com/myorg/myrepo.git",
			expected: "https:", // extractGithubOrg splits by github.com/, doesn't find it, returns first part
		},
		{
			name:     "Bitbucket URL (not GitHub)",
			repoURL:  "https://bitbucket.org/team/repo.git",
			expected: "https:", // Same behavior
		},
		{
			name:     "Empty URL",
			repoURL:  "",
			expected: "",
		},
		{
			name:     "Invalid URL",
			repoURL:  "not-a-url",
			expected: "",
		},
		{
			name:     "GitHub URL with www",
			repoURL:  "https://www.github.com/org/repo.git",
			expected: "org",
		},
		{
			name:     "GitHub URL with trailing slash",
			repoURL:  "https://github.com/org/repo/",
			expected: "org",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractGithubOrg(tt.repoURL)
			if result != tt.expected {
				t.Errorf("extractGithubOrg(%s) = %s, want %s", tt.repoURL, result, tt.expected)
			}
		})
	}
}

func TestInferContext(t *testing.T) {
	tests := []struct {
		name           string
		ctx            *scaffold.BlueprintContext
		expectedDomain string
		expectedImpact string
		expectedArch   string
	}{
		{
			name: "Fintech keywords in project name",
			ctx: &scaffold.BlueprintContext{
				ProjectName: "payment-gateway",
			},
			expectedDomain: "Fintech / Financial Services",
			expectedImpact: "Critical (High Security Requirement)",
			expectedArch:   "Cloud-Native Application", // Default archetype
		},
		{
			name: "E-commerce keywords",
			ctx: &scaffold.BlueprintContext{
				ProjectName: "online-store",
			},
			expectedDomain: "E-Commerce / Retail",
			expectedImpact: "High (Availability Requirement)",
			expectedArch:   "Cloud-Native Application",
		},
		{
			name: "Data engineering keywords",
			ctx: &scaffold.BlueprintContext{
				ProjectName: "data-pipeline",
			},
			expectedDomain: "Data Engineering",
			expectedImpact: "Medium",
			expectedArch:   "Data Pipeline / Batch Processing",
		},
		{
			name: "API/Gateway keywords",
			ctx: &scaffold.BlueprintContext{
				ProjectName: "api-gateway",
			},
			expectedDomain: "Fintech / Financial Services", // 'gate' matches fintech, 'api' matches archetype
			expectedImpact: "Critical (High Security Requirement)",
			expectedArch:   "Backend Microservice",
		},
		{
			name: "Generic project (no keywords)",
			ctx: &scaffold.BlueprintContext{
				ProjectName: "my-app",
			},
			expectedDomain: "General Purpose",
			expectedImpact: "Medium",
			expectedArch:   "Cloud-Native Application",
		},
		{
			name: "Empty project name",
			ctx: &scaffold.BlueprintContext{
				ProjectName: "",
			},
			expectedDomain: "General Purpose",
			expectedImpact: "Medium",
			expectedArch:   "Cloud-Native Application",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inferContext(tt.ctx)

			if tt.ctx.BusinessDomain != tt.expectedDomain {
				t.Errorf("BusinessDomain = %s, want %s", tt.ctx.BusinessDomain, tt.expectedDomain)
			}

			if tt.ctx.ImpactLevel != tt.expectedImpact {
				t.Errorf("ImpactLevel = %s, want %s", tt.ctx.ImpactLevel, tt.expectedImpact)
			}

			if tt.ctx.Archetype != tt.expectedArch {
				t.Errorf("Archetype = %s, want %s", tt.ctx.Archetype, tt.expectedArch)
			}
		})
	}
}

func TestBuildContext_BasicFields(t *testing.T) {
	opts := &InitOptions{
		ProjectName:         "test-project",
		GitRepo:             "https://github.com/org/repo.git",
		GitBranch:           "main",
		Domain:              "test.local",
		Email:               "admin@test.local",
		Environment:         "dev",
		Topology:            "standard",
		Workflow:            "gitflow",
		IncludeCI:           true,
		IncludeDevContainer: true,
		EnableKepler:        true,
		EnableMinio:         false,
		EnableKEDA:          false,
		EnableMetricsServer: true,
	}

	ctx := buildContext(opts)

	// Verify basic fields
	if ctx.ProjectName != "test-project" {
		t.Errorf("ProjectName = %s, want test-project", ctx.ProjectName)
	}

	if ctx.GitRepoURL != "https://github.com/org/repo.git" {
		t.Errorf("GitRepoURL = %s, want https://github.com/org/repo.git", ctx.GitRepoURL)
	}

	if ctx.GitBranch != "main" {
		t.Errorf("GitBranch = %s, want main", ctx.GitBranch)
	}

	if ctx.Domain != "test.local" {
		t.Errorf("Domain = %s, want test.local", ctx.Domain)
	}

	if ctx.Email != "admin@test.local" {
		t.Errorf("Email = %s, want admin@test.local", ctx.Email)
	}

	if ctx.Environment != "dev" {
		t.Errorf("Environment = %s, want dev", ctx.Environment)
	}

	if ctx.Topology != "standard" {
		t.Errorf("Topology = %s, want standard", ctx.Topology)
	}

	if ctx.WorkflowPattern != "gitflow" {
		t.Errorf("WorkflowPattern = %s, want gitflow", ctx.WorkflowPattern)
	}

	// Verify feature flags
	if !ctx.EnableCI {
		t.Error("EnableCI should be true")
	}

	if !ctx.EnableDevContainer {
		t.Error("EnableDevContainer should be true")
	}

	if !ctx.EnableKepler {
		t.Error("EnableKepler should be true")
	}

	if ctx.EnableMinio {
		t.Error("EnableMinio should be false")
	}

	if ctx.EnableKEDA {
		t.Error("EnableKEDA should be false")
	}

	if !ctx.EnableMetricsServer {
		t.Error("EnableMetricsServer should be true")
	}

	// Verify GitHub org extraction
	if ctx.GithubOrg != "org" {
		t.Errorf("GithubOrg = %s, want org", ctx.GithubOrg)
	}

	if !ctx.GithubDiscovery {
		t.Error("GithubDiscovery should be true when org is extracted")
	}
}

func TestBuildContext_Defaults(t *testing.T) {
	opts := &InitOptions{}

	ctx := buildContext(opts)

	// Verify defaults
	if ctx.ProjectName != "yby-project" {
		t.Errorf("ProjectName = %s, want yby-project (default)", ctx.ProjectName)
	}

	if ctx.GitBranch != "main" {
		t.Errorf("GitBranch = %s, want main (default)", ctx.GitBranch)
	}

	if ctx.Domain != "local" {
		t.Errorf("Domain = %s, want local (default)", ctx.Domain)
	}

	if ctx.Environment != "local" {
		t.Errorf("Environment = %s, want local (default)", ctx.Environment)
	}

	if ctx.Topology != "single" {
		t.Errorf("Topology = %s, want single (default)", ctx.Topology)
	}

	if ctx.WorkflowPattern != "essential" {
		t.Errorf("WorkflowPattern = %s, want essential (default)", ctx.WorkflowPattern)
	}

	// Verify default environments based on topology
	if len(ctx.Environments) == 0 {
		t.Error("Environments should not be empty")
	}
}

func TestBuildContext_TopologyEnvironments(t *testing.T) {
	tests := []struct {
		name         string
		topology     string
		expectedEnvs []string
	}{
		{
			name:         "Single topology",
			topology:     "single",
			expectedEnvs: []string{"local"},
		},
		{
			name:         "Standard topology",
			topology:     "standard",
			expectedEnvs: []string{"local", "prod"},
		},
		{
			name:         "Complete topology",
			topology:     "complete",
			expectedEnvs: []string{"local", "dev", "staging", "prod"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := &InitOptions{
				Topology: tt.topology,
			}

			ctx := buildContext(opts)

			if len(ctx.Environments) != len(tt.expectedEnvs) {
				t.Errorf("Environments length = %d, want %d", len(ctx.Environments), len(tt.expectedEnvs))
			}

			for i, env := range tt.expectedEnvs {
				if i >= len(ctx.Environments) || ctx.Environments[i] != env {
					t.Errorf("Environment[%d] = %v, want %s", i, ctx.Environments, env)
				}
			}
		})
	}
}
