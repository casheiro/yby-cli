package cmd

import (
	"testing"

	"github.com/casheiro/yby-cli/pkg/scaffold"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========================================================
// Estrutura do initCmd (flags registradas)
// ========================================================

func TestInitCmd_Estrutura(t *testing.T) {
	assert.Equal(t, "init", initCmd.Use, "initCmd.Use deveria ser 'init'")
	assert.NotEmpty(t, initCmd.Short, "initCmd.Short não deveria ser vazio")
	assert.NotEmpty(t, initCmd.Long, "initCmd.Long não deveria ser vazio")
	assert.NotEmpty(t, initCmd.Example, "initCmd.Example não deveria ser vazio")
	assert.NotNil(t, initCmd.RunE, "initCmd.RunE não deveria ser nil")
}

func TestInitCmd_EhSubcomandoDeRoot(t *testing.T) {
	found := false
	for _, sub := range rootCmd.Commands() {
		if sub.Name() == "init" {
			found = true
			break
		}
	}
	assert.True(t, found, "init deveria ser subcomando de rootCmd")
}

func TestInitCmd_FlagsRegistradas(t *testing.T) {
	flags := []struct {
		name     string
		defValue string
	}{
		{"topology", ""},
		{"workflow", ""},
		{"include-devcontainer", "false"},
		{"include-ci", "true"},
		{"offline", "false"},
		{"non-interactive", "false"},
		{"target-dir", ""},
		{"git-repo", ""},
		{"git-branch", "main"},
		{"project-name", ""},
		{"description", ""},
		{"ai-provider", ""},
		{"domain", "yby.local"},
		{"email", "admin@yby.local"},
		{"env", "dev"},
		{"enable-kepler", "false"},
		{"enable-minio", "false"},
		{"enable-keda", "false"},
		{"enable-metrics-server", "false"},
	}

	for _, f := range flags {
		t.Run("flag_"+f.name, func(t *testing.T) {
			flag := initCmd.Flags().Lookup(f.name)
			assert.NotNil(t, flag, "Flag %q deveria estar registrada", f.name)
			if flag != nil {
				assert.Equal(t, f.defValue, flag.DefValue, "Valor padrão da flag %q", f.name)
			}
		})
	}
}

func TestInitCmd_FlagTargetDirTemShorthand(t *testing.T) {
	flag := initCmd.Flags().ShorthandLookup("t")
	assert.NotNil(t, flag, "Flag -t (shorthand de --target-dir) deveria existir")
	if flag != nil {
		assert.Equal(t, "target-dir", flag.Name)
	}
}

// ========================================================
// deriveProjectName — edge cases adicionais
// ========================================================

func TestDeriveProjectName_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		repoURL  string
		expected string
	}{
		{
			name:     "URL com caracteres especiais no nome do repo",
			repoURL:  "https://github.com/org/my-app_v2.0.git",
			expected: "my-app_v2.0",
		},
		{
			name:     "URL com query string (cenário incomum)",
			repoURL:  "https://github.com/org/repo?ref=main",
			expected: "repo?ref=main",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := deriveProjectName(tt.repoURL)
			if result != tt.expected {
				t.Errorf("deriveProjectName(%q) = %q, want %q", tt.repoURL, result, tt.expected)
			}
		})
	}
}

// ========================================================
// extractGithubOrg — testes
// ========================================================

func TestExtractGithubOrg_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		repoURL  string
		expected string
	}{
		{"URL sem github.com", "https://gitlab.com/org/repo.git", "https:"},
		{"URL SSH com org", "git@github.com:my-org/repo.git", "my-org"},
		{"URL vazia", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractGithubOrg(tt.repoURL)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ========================================================
// inferContext — testes
// ========================================================

func TestInferContext_PalavrasChave(t *testing.T) {
	tests := []struct {
		name           string
		projectName    string
		expectedDomain string
	}{
		{"fintech", "payment-gateway", "Fintech / Financial Services"},
		{"ecommerce", "online-store", "E-Commerce / Retail"},
		{"data", "data-pipeline", "Data Engineering"},
		{"api", "user-api", "General Purpose"},
		{"generico", "my-app", "General Purpose"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &scaffold.BlueprintContext{ProjectName: tt.projectName}
			inferContext(ctx)
			assert.Equal(t, tt.expectedDomain, ctx.BusinessDomain)
		})
	}
}

func TestInferContext_TopologiaCompleta(t *testing.T) {
	ctx := &scaffold.BlueprintContext{
		ProjectName: "payment-system",
		Topology:    "complete",
	}
	inferContext(ctx)
	assert.Contains(t, ctx.ImpactLevel, "Enterprise Topology")
}

func TestInferContext_CaseInsensitive(t *testing.T) {
	ctx := &scaffold.BlueprintContext{ProjectName: "MyPaymentGateway"}
	inferContext(ctx)
	assert.Equal(t, "Fintech / Financial Services", ctx.BusinessDomain)
}

// ========================================================
// resolveProjectName — mais edge cases
// ========================================================

func TestResolveProjectName_ApenasGitRepo(t *testing.T) {
	o := &InitOptions{ProjectName: "", GitRepo: "https://github.com/casheiro/yby-cli.git"}
	assert.Equal(t, "yby-cli", resolveProjectName(o))
}

func TestResolveProjectName_AmbosCamposVazios(t *testing.T) {
	o := &InitOptions{ProjectName: "", GitRepo: ""}
	assert.Equal(t, "yby-project", resolveProjectName(o))
}

func TestResolveProjectName_GitRepoSSH(t *testing.T) {
	o := &InitOptions{ProjectName: "", GitRepo: "git@github.com:org/ssh-project.git"}
	assert.Equal(t, "ssh-project", resolveProjectName(o))
}

// ========================================================
// InitOptions — verificação de campos da struct
// ========================================================

func TestInitOptions_CamposPadrao(t *testing.T) {
	o := InitOptions{}
	assert.Equal(t, "", o.Topology)
	assert.Equal(t, "", o.Workflow)
	assert.False(t, o.IncludeDevContainer)
	assert.False(t, o.IncludeCI)
	assert.Equal(t, "", o.TargetDir)
	assert.Equal(t, "", o.GitRepo)
	assert.Equal(t, "", o.GitBranch)
	assert.Equal(t, "", o.ProjectName)
	assert.Equal(t, "", o.Description)
	assert.Equal(t, "", o.AIProvider)
	assert.Equal(t, "", o.Domain)
	assert.Equal(t, "", o.Email)
	assert.Equal(t, "", o.Environment)
	assert.False(t, o.EnableKepler)
	assert.False(t, o.EnableMinio)
	assert.False(t, o.EnableKEDA)
	assert.False(t, o.EnableMetricsServer)
	assert.False(t, o.Offline)
	assert.False(t, o.NonInteractive)
}

// ========================================================
// buildContext — cobertura do cálculo de RepoRootPath
// ========================================================

func TestBuildContext_ComTargetDir(t *testing.T) {
	o := &InitOptions{
		Topology: "standard", Workflow: "gitflow", ProjectName: "test-target",
		TargetDir: "infra", NonInteractive: true,
	}
	ctx, err := buildContext(o)
	require.NoError(t, err)
	assert.Equal(t, "test-target", ctx.ProjectName)
}

func TestBuildContext_TargetDirIgualRaiz(t *testing.T) {
	o := &InitOptions{
		Topology: "standard", Workflow: "essential", ProjectName: "test-raiz",
		TargetDir: "", NonInteractive: true,
	}
	ctx, err := buildContext(o)
	require.NoError(t, err)
	assert.NotNil(t, ctx)
}

func TestBuildContext_SemGithubOrg_NaoOffline(t *testing.T) {
	o := &InitOptions{
		Topology: "standard", Workflow: "essential", ProjectName: "test",
		GitRepo: "", Offline: false, NonInteractive: true,
	}
	_, err := buildContext(o)
	assert.NoError(t, err)
}
