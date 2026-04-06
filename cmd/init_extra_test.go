package cmd

import (
	"fmt"
	"strings"
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
// buildContext — modo interativo com mock
// ========================================================

// saveAndRestorePromptMocks salva e restaura todos os mocks de prompt.
func saveAndRestorePromptMocks(t *testing.T) {
	t.Helper()
	origInput := askInput
	origSelect := askSelect
	origConfirm := askConfirm
	origMultiSelect := askMultiSelect
	t.Cleanup(func() {
		askInput = origInput
		askSelect = origSelect
		askConfirm = origConfirm
		askMultiSelect = origMultiSelect
	})
}

// setupDefaultMocks configura mocks padrão para os prompts interativos.
func setupDefaultMocks() {
	askInput = func(title, defaultVal string) (string, error) {
		return defaultVal, nil
	}
	askSelect = func(title string, options []string, defaultVal string) (string, error) {
		if defaultVal != "" {
			return defaultVal, nil
		}
		if len(options) > 0 {
			return options[0], nil
		}
		return "", nil
	}
	askConfirm = func(title string, defaultVal bool) (bool, error) {
		return defaultVal, nil
	}
	askMultiSelect = func(title string, options []string, defaults []string) ([]string, error) {
		return defaults, nil
	}
}

func TestBuildContext_InteractivoPromptCancelado(t *testing.T) {
	saveAndRestorePromptMocks(t)

	askInput = func(title, defaultVal string) (string, error) {
		return "", fmt.Errorf("interrupted")
	}

	o := &InitOptions{
		NonInteractive: false,
		Topology:       "",
	}

	_, err := buildContext(o)
	assert.Error(t, err, "Deveria retornar erro quando prompt é cancelado")
}

func TestBuildContext_InterativoOffline(t *testing.T) {
	saveAndRestorePromptMocks(t)
	setupDefaultMocks()

	askSelect = func(title string, options []string, defaultVal string) (string, error) {
		if strings.Contains(title, "Topologia") {
			return "standard", nil
		}
		if strings.Contains(title, "Workflow") {
			return "essential", nil
		}
		return defaultVal, nil
	}
	askInput = func(title, defaultVal string) (string, error) {
		if strings.Contains(title, "Nome do Projeto") {
			return "offline-project", nil
		}
		if strings.Contains(title, "Domínio Base") {
			return "test.local", nil
		}
		if strings.Contains(title, "Email") {
			return "admin@test.local", nil
		}
		return defaultVal, nil
	}
	askConfirm = func(title string, defaultVal bool) (bool, error) {
		return false, nil
	}
	askMultiSelect = func(title string, options []string, defaults []string) ([]string, error) {
		return []string{}, nil
	}

	o := &InitOptions{
		NonInteractive: false,
		Topology:       "",
		Offline:        true,
	}

	ctx, err := buildContext(o)
	require.NoError(t, err)
	assert.Equal(t, "http://git-server.yby-system.svc/repo.git", ctx.GitRepoURL)
}

func TestBuildContext_InterativoModulosSelecionados(t *testing.T) {
	saveAndRestorePromptMocks(t)
	setupDefaultMocks()

	askSelect = func(title string, options []string, defaultVal string) (string, error) {
		if strings.Contains(title, "Topologia") {
			return "standard", nil
		}
		if strings.Contains(title, "Workflow") {
			return "essential", nil
		}
		return defaultVal, nil
	}
	askInput = func(title, defaultVal string) (string, error) {
		if strings.Contains(title, "URL do repositório") || strings.Contains(title, "Git") {
			return "https://github.com/org/test.git", nil
		}
		if strings.Contains(title, "Nome do Projeto") {
			return "test-modulos", nil
		}
		return defaultVal, nil
	}
	askMultiSelect = func(title string, options []string, defaults []string) ([]string, error) {
		return []string{
			"Kepler (Eficiência Energética)",
			"MinIO (Object Storage Local)",
			"KEDA (Event-Driven Autoscaling)",
			"Observability Core (Metrics Server)",
		}, nil
	}
	askConfirm = func(title string, defaultVal bool) (bool, error) {
		return true, nil
	}

	o := &InitOptions{
		NonInteractive: false,
		Topology:       "",
	}

	ctx, err := buildContext(o)
	require.NoError(t, err)

	assert.True(t, ctx.EnableKepler, "Kepler deveria estar habilitado via multiselect")
	assert.True(t, ctx.EnableMinio, "MinIO deveria estar habilitado via multiselect")
	assert.True(t, ctx.EnableKEDA, "KEDA deveria estar habilitado via multiselect")
	assert.True(t, ctx.EnableMetricsServer, "MetricsServer deveria estar habilitado via multiselect")
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

// ========================================================
// buildContext — cobertura da seção de IA interativa
// ========================================================

func TestBuildContext_InterativoComIAHabilitada(t *testing.T) {
	saveAndRestorePromptMocks(t)
	setupDefaultMocks()

	askSelect = func(title string, options []string, defaultVal string) (string, error) {
		if strings.Contains(title, "Topologia") {
			return "standard", nil
		}
		if strings.Contains(title, "Workflow") {
			return "gitflow", nil
		}
		if strings.Contains(title, "Provedor de IA") {
			return "gemini", nil
		}
		return defaultVal, nil
	}
	askInput = func(title, defaultVal string) (string, error) {
		if strings.Contains(title, "URL do repositório") || strings.Contains(title, "Git") {
			return "https://github.com/org/ai-test.git", nil
		}
		if strings.Contains(title, "Nome do Projeto") {
			return "ai-test-project", nil
		}
		if strings.Contains(title, "Descreva seu projeto") {
			return "Um gateway de pagamento seguro", nil
		}
		return defaultVal, nil
	}
	askConfirm = func(title string, defaultVal bool) (bool, error) {
		return true, nil
	}
	askMultiSelect = func(title string, options []string, defaults []string) ([]string, error) {
		return []string{}, nil
	}

	o := &InitOptions{NonInteractive: false, Topology: "", Offline: false}
	ctx, err := buildContext(o)
	require.NoError(t, err)
	assert.Equal(t, "standard", ctx.Topology)
	assert.Equal(t, "gitflow", ctx.WorkflowPattern)
	assert.Equal(t, "ai-test-project", ctx.ProjectName)
	assert.True(t, ctx.EnableDevContainer)
}

func TestBuildContext_InterativoIADesabilitada(t *testing.T) {
	saveAndRestorePromptMocks(t)
	setupDefaultMocks()

	askSelect = func(title string, options []string, defaultVal string) (string, error) {
		if strings.Contains(title, "Topologia") {
			return "single", nil
		}
		if strings.Contains(title, "Workflow") {
			return "essential", nil
		}
		return defaultVal, nil
	}
	askInput = func(title, defaultVal string) (string, error) {
		if strings.Contains(title, "URL do repositório") || strings.Contains(title, "Git") {
			return "https://github.com/org/test.git", nil
		}
		if strings.Contains(title, "Nome do Projeto") {
			return "no-ai-project", nil
		}
		return defaultVal, nil
	}
	askConfirm = func(title string, defaultVal bool) (bool, error) {
		return false, nil
	}
	askMultiSelect = func(title string, options []string, defaults []string) ([]string, error) {
		return []string{}, nil
	}

	o := &InitOptions{NonInteractive: false, Topology: ""}
	ctx, err := buildContext(o)
	require.NoError(t, err)
	assert.Equal(t, "single", ctx.Topology)
	assert.False(t, ctx.EnableDevContainer)
}

func TestBuildContext_InterativoComAIProviderJaDefinido(t *testing.T) {
	saveAndRestorePromptMocks(t)
	setupDefaultMocks()

	providerAsked := false
	askSelect = func(title string, options []string, defaultVal string) (string, error) {
		if strings.Contains(title, "Topologia") {
			return "standard", nil
		}
		if strings.Contains(title, "Workflow") {
			return "essential", nil
		}
		if strings.Contains(title, "Provedor de IA") {
			providerAsked = true
			return "auto", nil
		}
		return defaultVal, nil
	}
	askInput = func(title, defaultVal string) (string, error) {
		if strings.Contains(title, "URL do repositório") || strings.Contains(title, "Git") {
			return "https://github.com/org/test.git", nil
		}
		if strings.Contains(title, "Nome do Projeto") {
			return "ai-flag-project", nil
		}
		if strings.Contains(title, "Descreva seu projeto") {
			return "Projeto com IA pré-configurada", nil
		}
		return defaultVal, nil
	}
	askConfirm = func(title string, defaultVal bool) (bool, error) {
		return true, nil
	}
	askMultiSelect = func(title string, options []string, defaults []string) ([]string, error) {
		return []string{}, nil
	}

	o := &InitOptions{NonInteractive: false, Topology: "", AIProvider: "gemini"}
	ctx, err := buildContext(o)
	require.NoError(t, err)
	assert.Equal(t, "standard", ctx.Topology)
	assert.NotNil(t, ctx)
	assert.False(t, providerAsked, "Não deveria perguntar o provedor de IA quando já está definido via flag")
}

func TestBuildContext_InterativoProviderAutoSelecionado(t *testing.T) {
	saveAndRestorePromptMocks(t)
	setupDefaultMocks()

	askSelect = func(title string, options []string, defaultVal string) (string, error) {
		if strings.Contains(title, "Topologia") {
			return "standard", nil
		}
		if strings.Contains(title, "Workflow") {
			return "essential", nil
		}
		if strings.Contains(title, "Provedor de IA") {
			return "auto", nil
		}
		return defaultVal, nil
	}
	askInput = func(title, defaultVal string) (string, error) {
		if strings.Contains(title, "URL do repositório") || strings.Contains(title, "Git") {
			return "https://github.com/org/test.git", nil
		}
		if strings.Contains(title, "Nome do Projeto") {
			return "auto-ai-project", nil
		}
		if strings.Contains(title, "Descreva seu projeto") {
			return "Projeto teste auto", nil
		}
		return defaultVal, nil
	}
	askConfirm = func(title string, defaultVal bool) (bool, error) {
		return true, nil
	}
	askMultiSelect = func(title string, options []string, defaults []string) ([]string, error) {
		return []string{}, nil
	}

	o := &InitOptions{NonInteractive: false, Topology: ""}
	ctx, err := buildContext(o)
	require.NoError(t, err)
	assert.NotNil(t, ctx)
	assert.Equal(t, "", o.AIProvider)
}

func TestBuildContext_InterativoPromptTopologiaCancelado(t *testing.T) {
	saveAndRestorePromptMocks(t)
	setupDefaultMocks()

	askInput = func(title, defaultVal string) (string, error) {
		return defaultVal, nil
	}
	askSelect = func(title string, options []string, defaultVal string) (string, error) {
		if strings.Contains(title, "Topologia") {
			return "", fmt.Errorf("prompt cancelado")
		}
		return defaultVal, nil
	}

	o := &InitOptions{NonInteractive: false, Topology: ""}
	_, err := buildContext(o)
	assert.Error(t, err)
}

func TestBuildContext_InterativoPromptWorkflowCancelado(t *testing.T) {
	saveAndRestorePromptMocks(t)
	setupDefaultMocks()

	askSelect = func(title string, options []string, defaultVal string) (string, error) {
		if strings.Contains(title, "Topologia") {
			return "standard", nil
		}
		if strings.Contains(title, "Workflow") {
			return "", fmt.Errorf("prompt cancelado")
		}
		return defaultVal, nil
	}

	o := &InitOptions{NonInteractive: false, Topology: "", Workflow: ""}
	_, err := buildContext(o)
	assert.Error(t, err)
}

func TestBuildContext_InterativoPromptGitRepoCancelado(t *testing.T) {
	saveAndRestorePromptMocks(t)
	setupDefaultMocks()

	askSelect = func(title string, options []string, defaultVal string) (string, error) {
		if strings.Contains(title, "Topologia") {
			return "standard", nil
		}
		if strings.Contains(title, "Workflow") {
			return "essential", nil
		}
		return defaultVal, nil
	}
	askInput = func(title, defaultVal string) (string, error) {
		if strings.Contains(title, "Onde deseja inicializar") {
			return ".", nil
		}
		if strings.Contains(title, "URL do repositório") || strings.Contains(title, "Git") {
			return "", fmt.Errorf("prompt cancelado")
		}
		return defaultVal, nil
	}

	o := &InitOptions{NonInteractive: false, Topology: ""}
	_, err := buildContext(o)
	assert.Error(t, err)
}

func TestBuildContext_InterativoPromptModulosCancelado(t *testing.T) {
	saveAndRestorePromptMocks(t)
	setupDefaultMocks()

	askSelect = func(title string, options []string, defaultVal string) (string, error) {
		if strings.Contains(title, "Topologia") {
			return "standard", nil
		}
		if strings.Contains(title, "Workflow") {
			return "essential", nil
		}
		return defaultVal, nil
	}
	askInput = func(title, defaultVal string) (string, error) {
		if strings.Contains(title, "URL do repositório") || strings.Contains(title, "Git") {
			return "https://github.com/org/test.git", nil
		}
		if strings.Contains(title, "Nome do Projeto") {
			return "test", nil
		}
		return defaultVal, nil
	}
	askMultiSelect = func(title string, options []string, defaults []string) ([]string, error) {
		return nil, fmt.Errorf("prompt cancelado")
	}

	o := &InitOptions{NonInteractive: false, Topology: ""}
	_, err := buildContext(o)
	assert.Error(t, err)
}

func TestBuildContext_InterativoPromptDevContainerCancelado(t *testing.T) {
	saveAndRestorePromptMocks(t)
	setupDefaultMocks()

	askSelect = func(title string, options []string, defaultVal string) (string, error) {
		if strings.Contains(title, "Topologia") {
			return "standard", nil
		}
		if strings.Contains(title, "Workflow") {
			return "essential", nil
		}
		return defaultVal, nil
	}
	askInput = func(title, defaultVal string) (string, error) {
		if strings.Contains(title, "URL do repositório") || strings.Contains(title, "Git") {
			return "https://github.com/org/test.git", nil
		}
		if strings.Contains(title, "Nome do Projeto") {
			return "test", nil
		}
		return defaultVal, nil
	}
	askMultiSelect = func(title string, options []string, defaults []string) ([]string, error) {
		return []string{}, nil
	}
	askConfirm = func(title string, defaultVal bool) (bool, error) {
		if strings.Contains(title, "DevContainer") {
			return false, fmt.Errorf("prompt cancelado")
		}
		return false, nil
	}

	o := &InitOptions{NonInteractive: false, Topology: ""}
	_, err := buildContext(o)
	assert.Error(t, err)
}

func TestBuildContext_InterativoComDevContainerJaDefinido(t *testing.T) {
	saveAndRestorePromptMocks(t)
	setupDefaultMocks()

	devContainerAsked := false
	askSelect = func(title string, options []string, defaultVal string) (string, error) {
		if strings.Contains(title, "Topologia") {
			return "standard", nil
		}
		if strings.Contains(title, "Workflow") {
			return "essential", nil
		}
		return defaultVal, nil
	}
	askInput = func(title, defaultVal string) (string, error) {
		if strings.Contains(title, "URL do repositório") || strings.Contains(title, "Git") {
			return "https://github.com/org/test.git", nil
		}
		if strings.Contains(title, "Nome do Projeto") {
			return "devcontainer-flag", nil
		}
		return defaultVal, nil
	}
	askConfirm = func(title string, defaultVal bool) (bool, error) {
		if strings.Contains(title, "DevContainer") {
			devContainerAsked = true
		}
		return false, nil
	}
	askMultiSelect = func(title string, options []string, defaults []string) ([]string, error) {
		return []string{}, nil
	}

	o := &InitOptions{
		NonInteractive: false, Topology: "",
		IncludeDevContainer: true, Offline: true,
	}

	ctx, err := buildContext(o)
	require.NoError(t, err)
	assert.True(t, ctx.EnableDevContainer)
	assert.False(t, devContainerAsked, "Não deveria perguntar sobre DevContainer quando flag já está true")
}

func TestBuildContext_InterativoComGitRepoPreenchido(t *testing.T) {
	saveAndRestorePromptMocks(t)
	setupDefaultMocks()

	gitRepoAsked := false
	askSelect = func(title string, options []string, defaultVal string) (string, error) {
		if strings.Contains(title, "Topologia") {
			return "standard", nil
		}
		if strings.Contains(title, "Workflow") {
			return "essential", nil
		}
		return defaultVal, nil
	}
	askInput = func(title, defaultVal string) (string, error) {
		if strings.Contains(title, "URL do repositório") {
			gitRepoAsked = true
		}
		return defaultVal, nil
	}
	askConfirm = func(title string, defaultVal bool) (bool, error) {
		return false, nil
	}
	askMultiSelect = func(title string, options []string, defaults []string) ([]string, error) {
		return []string{}, nil
	}

	o := &InitOptions{
		NonInteractive: false, Topology: "",
		GitRepo: "https://github.com/org/meu-repo.git", Offline: true,
	}

	ctx, err := buildContext(o)
	require.NoError(t, err)
	assert.Equal(t, "meu-repo", ctx.ProjectName)
	assert.False(t, gitRepoAsked, "Não deveria perguntar URL do repo quando já está definido")
}

func TestBuildContext_InterativoComDomainEEmailCustom(t *testing.T) {
	saveAndRestorePromptMocks(t)
	setupDefaultMocks()

	domainAsked := false
	emailAsked := false
	askSelect = func(title string, options []string, defaultVal string) (string, error) {
		if strings.Contains(title, "Topologia") {
			return "standard", nil
		}
		if strings.Contains(title, "Workflow") {
			return "essential", nil
		}
		return defaultVal, nil
	}
	askInput = func(title, defaultVal string) (string, error) {
		if strings.Contains(title, "Domínio Base") {
			domainAsked = true
		}
		if strings.Contains(title, "Email") {
			emailAsked = true
		}
		if strings.Contains(title, "URL do repositório") || strings.Contains(title, "Git") {
			return "https://github.com/org/test.git", nil
		}
		if strings.Contains(title, "Nome do Projeto") {
			return "custom-domain-test", nil
		}
		return defaultVal, nil
	}
	askConfirm = func(title string, defaultVal bool) (bool, error) {
		return false, nil
	}
	askMultiSelect = func(title string, options []string, defaults []string) ([]string, error) {
		return []string{}, nil
	}

	o := &InitOptions{
		NonInteractive: false, Topology: "",
		Domain: "custom.example.com", Email: "admin@custom.example.com", Offline: true,
	}

	ctx, err := buildContext(o)
	require.NoError(t, err)
	assert.Equal(t, "custom.example.com", ctx.Domain)
	assert.Equal(t, "admin@custom.example.com", ctx.Email)
	assert.False(t, domainAsked, "Não deveria perguntar Domain quando já está customizado")
	assert.False(t, emailAsked, "Não deveria perguntar Email quando já está customizado")
}

func TestBuildContext_InterativoComDescriptionJaDefinida(t *testing.T) {
	saveAndRestorePromptMocks(t)
	setupDefaultMocks()

	descAsked := false
	askSelect = func(title string, options []string, defaultVal string) (string, error) {
		if strings.Contains(title, "Topologia") {
			return "standard", nil
		}
		if strings.Contains(title, "Workflow") {
			return "essential", nil
		}
		if strings.Contains(title, "Provedor de IA") {
			return "auto", nil
		}
		return defaultVal, nil
	}
	askInput = func(title, defaultVal string) (string, error) {
		if strings.Contains(title, "URL do repositório") || strings.Contains(title, "Git") {
			return "https://github.com/org/test.git", nil
		}
		if strings.Contains(title, "Nome do Projeto") {
			return "desc-flag-test", nil
		}
		if strings.Contains(title, "Descreva seu projeto") {
			descAsked = true
		}
		return defaultVal, nil
	}
	askConfirm = func(title string, defaultVal bool) (bool, error) {
		return true, nil
	}
	askMultiSelect = func(title string, options []string, defaults []string) ([]string, error) {
		return []string{}, nil
	}

	o := &InitOptions{
		NonInteractive: false, Topology: "",
		Description: "Projeto já descrito via flag",
	}

	ctx, err := buildContext(o)
	require.NoError(t, err)
	assert.NotNil(t, ctx)
	assert.False(t, descAsked, "Não deveria perguntar a descrição quando já está definida via flag")
}
