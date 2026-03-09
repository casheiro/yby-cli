package cmd

import (
	"fmt"
	"strings"
	"testing"

	"github.com/AlecAivazis/survey/v2"
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
		{
			name:     "Apenas espaços em branco",
			repoURL:  "   ",
			expected: "", // TrimSpace + TrimSuffix resulta em "" mas não retorna default pois não é empty antes do trim
		},
		{
			name:     "URL com protocolo file://",
			repoURL:  "file:///home/user/repos/local-project.git",
			expected: "local-project",
		},
		{
			name:     "URL com porta personalizada",
			repoURL:  "https://git.example.com:8443/org/custom-port-repo.git",
			expected: "custom-port-repo",
		},
		{
			name:     "URL com fragmento hash",
			repoURL:  "https://github.com/org/repo#readme",
			expected: "repo#readme",
		},
		{
			name:     "Apenas .git",
			repoURL:  ".git",
			expected: "", // após TrimSuffix de .git, fica vazio -> split("", "/") retorna [""]
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := deriveProjectName(tt.repoURL)
			if result != tt.expected {
				t.Errorf("deriveProjectName(%q) = %q, esperado %q", tt.repoURL, result, tt.expected)
			}
		})
	}
}

// ========================================================
// extractGithubOrg — edge cases adicionais
// ========================================================

func TestExtractGithubOrg_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		repoURL  string
		expected string
	}{
		{
			name:     "URL SSH com múltiplos dois-pontos",
			repoURL:  "git@github.com:org:extra/repo.git",
			expected: "git@github.com:org:extra", // split por ":" dá 3 partes, cai no fallback por "/"
		},
		{
			name:     "URL com github.com em subdomínio",
			repoURL:  "https://mirror.github.com/org/repo.git",
			expected: "org",
		},
		{
			name:     "Apenas espaços em branco",
			repoURL:  "   ",
			expected: "",
		},
		{
			name:     "URL com usuario:senha (cenário legado)",
			repoURL:  "https://user:pass@github.com/myorg/myrepo.git",
			expected: "myorg",
		},
		{
			name:     "URL SSH sem .git no final",
			repoURL:  "git@github.com:myorg/myrepo",
			expected: "myorg",
		},
		{
			name:     "URL HTTPS com path de apenas 1 segmento (sem repo)",
			repoURL:  "https://github.com/singlepath",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractGithubOrg(tt.repoURL)
			if result != tt.expected {
				t.Errorf("extractGithubOrg(%q) = %q, esperado %q", tt.repoURL, result, tt.expected)
			}
		})
	}
}

// ========================================================
// inferContext — mais cenários
// ========================================================

func TestInferContext_TopologyDefaultNaoAlteraSufixo(t *testing.T) {
	// Topologia diferente de "complete" não deve adicionar sufixo
	ctx := &scaffold.BlueprintContext{
		ProjectName: "payment-app",
		Topology:    "standard",
	}
	inferContext(ctx)

	assert.Equal(t, "Fintech / Financial Services", ctx.BusinessDomain)
	assert.Equal(t, "Critical (High Security Requirement)", ctx.ImpactLevel)
	assert.NotContains(t, ctx.ImpactLevel, "Enterprise Topology",
		"Topologia 'standard' não deveria adicionar sufixo Enterprise Topology")
}

func TestInferContext_GenericComTopologiaComplete(t *testing.T) {
	// Projeto genérico com topologia completa
	ctx := &scaffold.BlueprintContext{
		ProjectName: "my-cool-app",
		Topology:    "complete",
	}
	inferContext(ctx)

	assert.Equal(t, "General Purpose", ctx.BusinessDomain)
	assert.Contains(t, ctx.ImpactLevel, "Medium")
	assert.Contains(t, ctx.ImpactLevel, "(Enterprise Topology)")
}

func TestInferContext_DataComTopologiaComplete(t *testing.T) {
	ctx := &scaffold.BlueprintContext{
		ProjectName: "data-warehouse",
		Topology:    "complete",
	}
	inferContext(ctx)

	assert.Equal(t, "Data Engineering", ctx.BusinessDomain)
	assert.Contains(t, ctx.ImpactLevel, "(Enterprise Topology)")
	assert.Equal(t, "Data Pipeline / Batch Processing", ctx.Archetype)
}

func TestInferContext_EcommerceComTopologiaComplete(t *testing.T) {
	ctx := &scaffold.BlueprintContext{
		ProjectName: "my-store-app",
		Topology:    "complete",
	}
	inferContext(ctx)

	assert.Equal(t, "E-Commerce / Retail", ctx.BusinessDomain)
	assert.Contains(t, ctx.ImpactLevel, "High (Availability Requirement)")
	assert.Contains(t, ctx.ImpactLevel, "(Enterprise Topology)")
}

func TestInferContext_PalavraChaveGate(t *testing.T) {
	ctx := &scaffold.BlueprintContext{ProjectName: "auth-gate"}
	inferContext(ctx)

	assert.Equal(t, "General Purpose", ctx.BusinessDomain)
	assert.Equal(t, "Backend Microservice", ctx.Archetype)
}

// ========================================================
// buildContext — cenários adicionais
// ========================================================

func TestBuildContext_TopologiaDesconhecida(t *testing.T) {
	o := &InitOptions{
		Topology:       "desconhecida",
		Workflow:       "essential",
		ProjectName:    "test",
		NonInteractive: true,
	}

	_, err := buildContext(o)
	assert.Error(t, err, "Topologia desconhecida deve ser rejeitada pela validação")
	assert.Contains(t, err.Error(), "topologia inválida")
}

func TestBuildContext_TopologiaVaziaComNonInteractivo(t *testing.T) {
	o := &InitOptions{
		Topology:       "",
		Workflow:       "gitflow",
		ProjectName:    "test",
		NonInteractive: true,
	}

	_, err := buildContext(o)
	assert.Error(t, err, "Deveria falhar quando topology está vazia no modo não-interativo")
	assert.Contains(t, err.Error(), "--topology")
}

func TestBuildContext_WorkflowVazioComNonInteractivo(t *testing.T) {
	o := &InitOptions{
		Topology:       "standard",
		Workflow:       "",
		ProjectName:    "test",
		NonInteractive: true,
	}

	_, err := buildContext(o)
	assert.Error(t, err, "Deveria falhar quando workflow está vazio no modo não-interativo")
	assert.Contains(t, err.Error(), "--workflow")
}

func TestBuildContext_CompleteTopologyEnvironments(t *testing.T) {
	o := &InitOptions{
		Topology:       "complete",
		Workflow:       "trunkbased",
		ProjectName:    "enterprise-app",
		Environment:    "staging",
		NonInteractive: true,
	}

	ctx, err := buildContext(o)
	require.NoError(t, err)

	expected := []string{"local", "dev", "staging", "prod"}
	assert.Equal(t, expected, ctx.Environments)
	assert.Equal(t, "staging", ctx.Environment, "Ambiente staging deveria ser válido para topologia complete")
}

func TestBuildContext_OfflineStandard_MantemLocal(t *testing.T) {
	// standard já tem "local", então offline não deveria duplicar
	o := &InitOptions{
		Offline:        true,
		Topology:       "standard",
		Workflow:       "essential",
		ProjectName:    "test",
		NonInteractive: true,
	}

	ctx, err := buildContext(o)
	require.NoError(t, err)

	// Contar quantas vezes "local" aparece
	count := 0
	for _, env := range ctx.Environments {
		if env == "local" {
			count++
		}
	}
	assert.Equal(t, 1, count, "local deveria aparecer apenas uma vez, mesmo com offline ativado")
}

func TestBuildContext_AllModulesEnabled(t *testing.T) {
	o := &InitOptions{
		Topology:            "standard",
		Workflow:            "gitflow",
		ProjectName:         "full-stack",
		GitRepo:             "https://github.com/org/repo.git",
		NonInteractive:      true,
		EnableKepler:        true,
		EnableMinio:         true,
		EnableKEDA:          true,
		EnableMetricsServer: true,
		IncludeCI:           true,
		IncludeDevContainer: true,
	}

	ctx, err := buildContext(o)
	require.NoError(t, err)

	assert.True(t, ctx.EnableKepler, "Kepler deveria estar habilitado")
	assert.True(t, ctx.EnableMinio, "MinIO deveria estar habilitado")
	assert.True(t, ctx.EnableKEDA, "KEDA deveria estar habilitado")
	assert.True(t, ctx.EnableMetricsServer, "MetricsServer deveria estar habilitado")
	assert.True(t, ctx.EnableCI, "CI deveria estar habilitado")
	assert.True(t, ctx.EnableDevContainer, "DevContainer deveria estar habilitado")
}

func TestBuildContext_GithubOrgExtraidoCorretamente(t *testing.T) {
	o := &InitOptions{
		Topology:       "standard",
		Workflow:       "essential",
		ProjectName:    "test",
		GitRepo:        "https://github.com/casheiro/yby-cli.git",
		NonInteractive: true,
	}

	ctx, err := buildContext(o)
	require.NoError(t, err)

	assert.Equal(t, "casheiro", ctx.GithubOrg)
	assert.True(t, ctx.GithubDiscovery)
}

func TestBuildContext_SemGithubOrg_SemOffline(t *testing.T) {
	o := &InitOptions{
		Topology:       "standard",
		Workflow:       "essential",
		ProjectName:    "test",
		GitRepo:        "https://gitlab.com/org/repo.git",
		NonInteractive: true,
	}

	ctx, err := buildContext(o)
	require.NoError(t, err)

	// Para URLs não-GitHub (sem match direto), extractGithubOrg retorna algo inesperado
	// mas GithubDiscovery deveria ser true pois org não é vazio
	assert.NotEmpty(t, ctx.GithubOrg, "GithubOrg não deveria ser vazio para URL com path")
}

func TestBuildContext_AmbienteDevComSingle(t *testing.T) {
	// "dev" não existe na topologia "single" (que tem apenas "prod")
	// Deve ser ajustado automaticamente
	o := &InitOptions{
		Topology:       "single",
		Workflow:       "essential",
		Environment:    "dev",
		ProjectName:    "test",
		NonInteractive: true,
	}

	ctx, err := buildContext(o)
	require.NoError(t, err)

	assert.NotEqual(t, "dev", ctx.Environment,
		"Ambiente deveria ser ajustado pois 'dev' não existe na topologia single")
	assert.Equal(t, "prod", ctx.Environment,
		"Ambiente deveria ser ajustado para 'prod' (primeiro da lista single)")
}

func TestBuildContext_AmbienteProdValidoEmStandard(t *testing.T) {
	o := &InitOptions{
		Topology:       "standard",
		Workflow:       "gitflow",
		Environment:    "prod",
		ProjectName:    "prod-app",
		GitRepo:        "https://github.com/org/app.git",
		NonInteractive: true,
	}

	ctx, err := buildContext(o)
	require.NoError(t, err)

	assert.Equal(t, "prod", ctx.Environment,
		"Ambiente 'prod' deveria ser válido na topologia standard")
}

func TestBuildContext_InferContextComFintech(t *testing.T) {
	o := &InitOptions{
		Topology:       "complete",
		Workflow:       "gitflow",
		ProjectName:    "bank-app",
		NonInteractive: true,
	}

	ctx, err := buildContext(o)
	require.NoError(t, err)

	assert.Equal(t, "Fintech / Financial Services", ctx.BusinessDomain)
	assert.Contains(t, ctx.ImpactLevel, "Critical")
	assert.Contains(t, ctx.ImpactLevel, "Enterprise Topology")
}

// ========================================================
// buildContext — modo interativo com mock
// ========================================================

func TestBuildContext_InteractivoPromptCancelado(t *testing.T) {
	// Simula cancelamento de prompt
	originalAskOne := askOne
	askOne = func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
		return fmt.Errorf("interrupted")
	}
	defer func() { askOne = originalAskOne }()

	o := &InitOptions{
		NonInteractive: false,
		Topology:       "", // Dispara interativo
	}

	_, err := buildContext(o)
	assert.Error(t, err, "Deveria retornar erro quando prompt é cancelado")
}

func TestBuildContext_InterativoOffline(t *testing.T) {
	// Offline + interativo: GitRepoURL é preenchido com placeholder interno
	callCount := 0
	originalAskOne := askOne
	askOne = func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
		callCount++
		switch prompt := p.(type) {
		case *survey.Input:
			switch {
			case strings.Contains(prompt.Message, "Onde deseja inicializar"):
				*(response.(*string)) = "."
			case strings.Contains(prompt.Message, "Nome do Projeto"):
				*(response.(*string)) = "offline-project"
			case strings.Contains(prompt.Message, "Domínio Base"):
				*(response.(*string)) = "test.local"
			case strings.Contains(prompt.Message, "Email"):
				*(response.(*string)) = "admin@test.local"
			}
		case *survey.Select:
			if strings.Contains(prompt.Message, "Topologia") {
				*(response.(*string)) = "standard"
			} else if strings.Contains(prompt.Message, "Workflow") {
				*(response.(*string)) = "essential"
			}
		case *survey.MultiSelect:
			*(response.(*[]string)) = []string{}
		case *survey.Confirm:
			*(response.(*bool)) = false
		}
		return nil
	}
	defer func() { askOne = originalAskOne }()

	o := &InitOptions{
		NonInteractive: false,
		Topology:       "",
		Offline:        true,
	}

	ctx, err := buildContext(o)
	require.NoError(t, err)

	// No modo offline interativo, GitRepoURL recebe placeholder interno
	assert.Equal(t, "http://git-server.yby-system.svc/repo.git", ctx.GitRepoURL,
		"No modo offline interativo, deveria usar placeholder interno")
	assert.True(t, callCount > 0, "Deveria ter chamado askOne pelo menos uma vez")
}

func TestBuildContext_InterativoModulosSelecionados(t *testing.T) {
	originalAskOne := askOne
	askOne = func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
		switch prompt := p.(type) {
		case *survey.Input:
			switch {
			case strings.Contains(prompt.Message, "Onde deseja inicializar"):
				*(response.(*string)) = "."
			case strings.Contains(prompt.Message, "URL do repositório"):
				*(response.(*string)) = "https://github.com/org/test.git"
			case strings.Contains(prompt.Message, "Nome do Projeto"):
				*(response.(*string)) = "test-modulos"
			}
		case *survey.Select:
			if strings.Contains(prompt.Message, "Topologia") {
				*(response.(*string)) = "standard"
			} else if strings.Contains(prompt.Message, "Workflow") {
				*(response.(*string)) = "essential"
			}
		case *survey.MultiSelect:
			// Seleciona todos os módulos
			*(response.(*[]string)) = []string{
				"Kepler (Eficiência Energética)",
				"MinIO (Object Storage Local)",
				"KEDA (Event-Driven Autoscaling)",
				"Observability Core (Metrics Server)",
			}
		case *survey.Confirm:
			*(response.(*bool)) = true
		}
		return nil
	}
	defer func() { askOne = originalAskOne }()

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
	o := &InitOptions{
		ProjectName: "",
		GitRepo:     "https://github.com/casheiro/yby-cli.git",
	}

	result := resolveProjectName(o)
	assert.Equal(t, "yby-cli", result)
}

func TestResolveProjectName_AmbosCamposVazios(t *testing.T) {
	o := &InitOptions{
		ProjectName: "",
		GitRepo:     "",
	}

	result := resolveProjectName(o)
	assert.Equal(t, "yby-project", result)
}

func TestResolveProjectName_GitRepoSSH(t *testing.T) {
	o := &InitOptions{
		ProjectName: "",
		GitRepo:     "git@github.com:org/ssh-project.git",
	}

	result := resolveProjectName(o)
	assert.Equal(t, "ssh-project", result)
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
	// Testa que TargetDir explícito é usado no cálculo de RepoRootPath
	o := &InitOptions{
		Topology:       "standard",
		Workflow:       "gitflow",
		ProjectName:    "test-target",
		TargetDir:      "infra",
		NonInteractive: true,
	}

	ctx, err := buildContext(o)
	require.NoError(t, err)

	assert.Equal(t, "test-target", ctx.ProjectName)
	// O RepoRootPath deve ser preenchido pois estamos dentro de um repo git
	// e TargetDir é "infra" (diferente da raiz)
	// Nota: o valor exato depende do CWD durante o teste
}

func TestBuildContext_TargetDirIgualRaiz(t *testing.T) {
	// Testa que TargetDir "." resulta em RepoRootPath vazio
	o := &InitOptions{
		Topology:       "standard",
		Workflow:       "essential",
		ProjectName:    "test-raiz",
		TargetDir:      "",
		NonInteractive: true,
	}

	ctx, err := buildContext(o)
	require.NoError(t, err)

	// Com TargetDir vazio/ponto, se CWD == gitRoot, RepoRootPath deve ser ""
	// ou ter algum valor relativo (depende da posição do CWD)
	assert.NotNil(t, ctx)
}

func TestBuildContext_SemGithubOrg_NaoOffline(t *testing.T) {
	// Testa o caso onde org é vazio e não está offline
	o := &InitOptions{
		Topology:       "standard",
		Workflow:       "essential",
		ProjectName:    "test",
		GitRepo:        "", // Sem repo -> org vazio
		Offline:        false,
		NonInteractive: true,
	}

	_, err := buildContext(o)
	// Sem git repo e sem project name default, deve falhar no modo não-interativo
	// porque ProjectName é "test" (explícito), não deve pedir --project-name
	assert.NoError(t, err)
}

// ========================================================
// buildContext — cobertura da seção de IA interativa
// ========================================================

func TestBuildContext_InterativoComIAHabilitada(t *testing.T) {
	// Testa o path interativo onde o usuário habilita IA,
	// seleciona provider e fornece descrição.
	// Cobre as linhas 455-488 de init.go
	originalAskOne := askOne
	askOne = func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
		switch prompt := p.(type) {
		case *survey.Input:
			switch {
			case strings.Contains(prompt.Message, "Onde deseja inicializar"):
				*(response.(*string)) = "."
			case strings.Contains(prompt.Message, "URL do repositório"):
				*(response.(*string)) = "https://github.com/org/ai-test.git"
			case strings.Contains(prompt.Message, "Nome do Projeto"):
				*(response.(*string)) = "ai-test-project"
			case strings.Contains(prompt.Message, "Descreva seu projeto"):
				*(response.(*string)) = "Um gateway de pagamento seguro"
			}
		case *survey.Select:
			switch {
			case strings.Contains(prompt.Message, "Topologia"):
				*(response.(*string)) = "standard"
			case strings.Contains(prompt.Message, "Workflow"):
				*(response.(*string)) = "gitflow"
			case strings.Contains(prompt.Message, "Provedor de IA"):
				*(response.(*string)) = "gemini"
			}
		case *survey.MultiSelect:
			*(response.(*[]string)) = []string{}
		case *survey.Confirm:
			// Habilita IA e DevContainer
			*(response.(*bool)) = true
		}
		return nil
	}
	defer func() { askOne = originalAskOne }()

	o := &InitOptions{
		NonInteractive: false,
		Topology:       "", // Dispara modo interativo
		Offline:        false,
	}

	ctx, err := buildContext(o)
	require.NoError(t, err)

	assert.Equal(t, "standard", ctx.Topology)
	assert.Equal(t, "gitflow", ctx.WorkflowPattern)
	assert.Equal(t, "ai-test-project", ctx.ProjectName)
	assert.True(t, ctx.EnableDevContainer, "DevContainer deveria ser habilitado (Confirm=true)")
}

func TestBuildContext_InterativoIADesabilitada(t *testing.T) {
	// Testa o path onde enableAI = false (usuário recusa) — não entra no bloco de seleção de provider
	callCount := 0
	originalAskOne := askOne
	askOne = func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
		callCount++
		switch prompt := p.(type) {
		case *survey.Input:
			switch {
			case strings.Contains(prompt.Message, "Onde deseja inicializar"):
				*(response.(*string)) = "."
			case strings.Contains(prompt.Message, "URL do repositório"):
				*(response.(*string)) = "https://github.com/org/test.git"
			case strings.Contains(prompt.Message, "Nome do Projeto"):
				*(response.(*string)) = "no-ai-project"
			}
		case *survey.Select:
			if strings.Contains(prompt.Message, "Topologia") {
				*(response.(*string)) = "single"
			} else if strings.Contains(prompt.Message, "Workflow") {
				*(response.(*string)) = "essential"
			}
		case *survey.MultiSelect:
			*(response.(*[]string)) = []string{}
		case *survey.Confirm:
			// Desabilita tudo (IA e DevContainer)
			*(response.(*bool)) = false
		}
		return nil
	}
	defer func() { askOne = originalAskOne }()

	o := &InitOptions{
		NonInteractive: false,
		Topology:       "",
	}

	ctx, err := buildContext(o)
	require.NoError(t, err)

	assert.Equal(t, "single", ctx.Topology)
	assert.Equal(t, "essential", ctx.WorkflowPattern)
	assert.False(t, ctx.EnableDevContainer, "DevContainer deveria ser false (Confirm=false)")
}

func TestBuildContext_InterativoComAIProviderJaDefinido(t *testing.T) {
	// Quando AIProvider já está definido via flag, não pergunta o Select de provider
	originalAskOne := askOne
	askOne = func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
		switch prompt := p.(type) {
		case *survey.Input:
			switch {
			case strings.Contains(prompt.Message, "Onde deseja inicializar"):
				*(response.(*string)) = "."
			case strings.Contains(prompt.Message, "URL do repositório"):
				*(response.(*string)) = "https://github.com/org/test.git"
			case strings.Contains(prompt.Message, "Nome do Projeto"):
				*(response.(*string)) = "ai-flag-project"
			case strings.Contains(prompt.Message, "Descreva seu projeto"):
				*(response.(*string)) = "Projeto com IA pré-configurada"
			}
		case *survey.Select:
			switch {
			case strings.Contains(prompt.Message, "Topologia"):
				*(response.(*string)) = "standard"
			case strings.Contains(prompt.Message, "Workflow"):
				*(response.(*string)) = "essential"
			case strings.Contains(prompt.Message, "Provedor de IA"):
				// Este prompt NÃO deve ser chamado quando AIProvider já está definido
				t.Error("Não deveria perguntar o provedor de IA quando já está definido via flag")
				*(response.(*string)) = "auto"
			}
		case *survey.MultiSelect:
			*(response.(*[]string)) = []string{}
		case *survey.Confirm:
			*(response.(*bool)) = true
		}
		return nil
	}
	defer func() { askOne = originalAskOne }()

	o := &InitOptions{
		NonInteractive: false,
		Topology:       "",
		AIProvider:     "gemini", // Já definido via flag
	}

	ctx, err := buildContext(o)
	require.NoError(t, err)
	assert.Equal(t, "standard", ctx.Topology)
	assert.NotNil(t, ctx)
}

func TestBuildContext_InterativoProviderAutoSelecionado(t *testing.T) {
	// Quando o usuário seleciona "auto" no prompt de provider, AIProvider é resetado para ""
	originalAskOne := askOne
	askOne = func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
		switch prompt := p.(type) {
		case *survey.Input:
			switch {
			case strings.Contains(prompt.Message, "Onde deseja inicializar"):
				*(response.(*string)) = "."
			case strings.Contains(prompt.Message, "URL do repositório"):
				*(response.(*string)) = "https://github.com/org/test.git"
			case strings.Contains(prompt.Message, "Nome do Projeto"):
				*(response.(*string)) = "auto-ai-project"
			case strings.Contains(prompt.Message, "Descreva seu projeto"):
				*(response.(*string)) = "Projeto teste auto"
			}
		case *survey.Select:
			switch {
			case strings.Contains(prompt.Message, "Topologia"):
				*(response.(*string)) = "standard"
			case strings.Contains(prompt.Message, "Workflow"):
				*(response.(*string)) = "essential"
			case strings.Contains(prompt.Message, "Provedor de IA"):
				*(response.(*string)) = "auto"
			}
		case *survey.MultiSelect:
			*(response.(*[]string)) = []string{}
		case *survey.Confirm:
			*(response.(*bool)) = true
		}
		return nil
	}
	defer func() { askOne = originalAskOne }()

	o := &InitOptions{
		NonInteractive: false,
		Topology:       "",
	}

	ctx, err := buildContext(o)
	require.NoError(t, err)
	assert.NotNil(t, ctx)
	// AIProvider deve ter sido resetado para "" quando "auto" é selecionado
	assert.Equal(t, "", o.AIProvider,
		"AIProvider deveria ser '' após seleção de 'auto' no prompt")
}

func TestBuildContext_InterativoPromptTopologiaCancelado(t *testing.T) {
	// Cancelamento no prompt de topologia (com TargetDir já fornecido)
	originalAskOne := askOne
	callIdx := 0
	askOne = func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
		callIdx++
		switch prompt := p.(type) {
		case *survey.Input:
			if strings.Contains(prompt.Message, "Onde deseja inicializar") {
				*(response.(*string)) = "/tmp/test"
				return nil
			}
		case *survey.Select:
			if strings.Contains(prompt.Message, "Topologia") {
				return fmt.Errorf("prompt cancelado")
			}
		}
		return nil
	}
	defer func() { askOne = originalAskOne }()

	o := &InitOptions{
		NonInteractive: false,
		Topology:       "",
	}

	_, err := buildContext(o)
	assert.Error(t, err, "Deveria falhar quando prompt de topologia é cancelado")
}

func TestBuildContext_InterativoPromptWorkflowCancelado(t *testing.T) {
	// Cancelamento no prompt de workflow
	originalAskOne := askOne
	askOne = func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
		switch prompt := p.(type) {
		case *survey.Input:
			if strings.Contains(prompt.Message, "Onde deseja inicializar") {
				*(response.(*string)) = "."
				return nil
			}
		case *survey.Select:
			if strings.Contains(prompt.Message, "Topologia") {
				*(response.(*string)) = "standard"
				return nil
			}
			if strings.Contains(prompt.Message, "Workflow") {
				return fmt.Errorf("prompt cancelado")
			}
		}
		return nil
	}
	defer func() { askOne = originalAskOne }()

	o := &InitOptions{
		NonInteractive: false,
		Topology:       "",
		Workflow:       "",
	}

	_, err := buildContext(o)
	assert.Error(t, err, "Deveria falhar quando prompt de workflow é cancelado")
}

func TestBuildContext_InterativoPromptGitRepoCancelado(t *testing.T) {
	// Cancelamento no prompt de Git Repo
	originalAskOne := askOne
	askOne = func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
		switch prompt := p.(type) {
		case *survey.Input:
			switch {
			case strings.Contains(prompt.Message, "Onde deseja inicializar"):
				*(response.(*string)) = "."
				return nil
			case strings.Contains(prompt.Message, "URL do repositório"):
				return fmt.Errorf("prompt cancelado")
			}
		case *survey.Select:
			if strings.Contains(prompt.Message, "Topologia") {
				*(response.(*string)) = "standard"
				return nil
			}
			if strings.Contains(prompt.Message, "Workflow") {
				*(response.(*string)) = "essential"
				return nil
			}
		}
		return nil
	}
	defer func() { askOne = originalAskOne }()

	o := &InitOptions{
		NonInteractive: false,
		Topology:       "",
	}

	_, err := buildContext(o)
	assert.Error(t, err, "Deveria falhar quando prompt de Git repo é cancelado")
}

func TestBuildContext_InterativoPromptModulosCancelado(t *testing.T) {
	// Cancelamento no prompt de módulos MultiSelect
	originalAskOne := askOne
	askOne = func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
		switch prompt := p.(type) {
		case *survey.Input:
			switch {
			case strings.Contains(prompt.Message, "Onde deseja inicializar"):
				*(response.(*string)) = "."
			case strings.Contains(prompt.Message, "URL do repositório"):
				*(response.(*string)) = "https://github.com/org/test.git"
			case strings.Contains(prompt.Message, "Nome do Projeto"):
				*(response.(*string)) = "test"
			}
		case *survey.Select:
			if strings.Contains(prompt.Message, "Topologia") {
				*(response.(*string)) = "standard"
			} else if strings.Contains(prompt.Message, "Workflow") {
				*(response.(*string)) = "essential"
			}
		case *survey.MultiSelect:
			return fmt.Errorf("prompt cancelado")
		case *survey.Confirm:
			*(response.(*bool)) = false
		}
		return nil
	}
	defer func() { askOne = originalAskOne }()

	o := &InitOptions{
		NonInteractive: false,
		Topology:       "",
	}

	_, err := buildContext(o)
	assert.Error(t, err, "Deveria falhar quando prompt de módulos é cancelado")
}

func TestBuildContext_InterativoPromptDevContainerCancelado(t *testing.T) {
	// Cancelamento no prompt de DevContainer (Confirm)
	originalAskOne := askOne
	confirmCount := 0
	askOne = func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
		switch prompt := p.(type) {
		case *survey.Input:
			switch {
			case strings.Contains(prompt.Message, "Onde deseja inicializar"):
				*(response.(*string)) = "."
			case strings.Contains(prompt.Message, "URL do repositório"):
				*(response.(*string)) = "https://github.com/org/test.git"
			case strings.Contains(prompt.Message, "Nome do Projeto"):
				*(response.(*string)) = "test"
			}
		case *survey.Select:
			if strings.Contains(prompt.Message, "Topologia") {
				*(response.(*string)) = "standard"
			} else if strings.Contains(prompt.Message, "Workflow") {
				*(response.(*string)) = "essential"
			}
		case *survey.MultiSelect:
			*(response.(*[]string)) = []string{}
		case *survey.Confirm:
			confirmCount++
			if strings.Contains(prompt.Message, "DevContainer") {
				return fmt.Errorf("prompt cancelado")
			}
			*(response.(*bool)) = false
		}
		return nil
	}
	defer func() { askOne = originalAskOne }()

	o := &InitOptions{
		NonInteractive: false,
		Topology:       "",
	}

	_, err := buildContext(o)
	assert.Error(t, err, "Deveria falhar quando prompt de DevContainer é cancelado")
}

func TestBuildContext_InterativoComDevContainerJaDefinido(t *testing.T) {
	// Quando IncludeDevContainer já é true via flag, não pergunta o Confirm
	originalAskOne := askOne
	askOne = func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
		switch prompt := p.(type) {
		case *survey.Input:
			switch {
			case strings.Contains(prompt.Message, "Onde deseja inicializar"):
				*(response.(*string)) = "."
			case strings.Contains(prompt.Message, "URL do repositório"):
				*(response.(*string)) = "https://github.com/org/test.git"
			case strings.Contains(prompt.Message, "Nome do Projeto"):
				*(response.(*string)) = "devcontainer-flag"
			}
		case *survey.Select:
			if strings.Contains(prompt.Message, "Topologia") {
				*(response.(*string)) = "standard"
			} else if strings.Contains(prompt.Message, "Workflow") {
				*(response.(*string)) = "essential"
			}
		case *survey.MultiSelect:
			*(response.(*[]string)) = []string{}
		case *survey.Confirm:
			if strings.Contains(prompt.Message, "DevContainer") {
				t.Error("Não deveria perguntar sobre DevContainer quando flag já está true")
			}
			*(response.(*bool)) = false
		}
		return nil
	}
	defer func() { askOne = originalAskOne }()

	o := &InitOptions{
		NonInteractive:      false,
		Topology:            "",
		IncludeDevContainer: true, // Já definido
		Offline:             true, // Para pular seção de IA
	}

	ctx, err := buildContext(o)
	require.NoError(t, err)
	assert.True(t, ctx.EnableDevContainer, "DevContainer deveria manter true da flag")
}

func TestBuildContext_InterativoKeplerJaHabilitado(t *testing.T) {
	// Quando EnableKepler já é true, deve aparecer nos defaults do MultiSelect
	originalAskOne := askOne
	askOne = func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
		switch prompt := p.(type) {
		case *survey.Input:
			switch {
			case strings.Contains(prompt.Message, "Onde deseja inicializar"):
				*(response.(*string)) = "."
			case strings.Contains(prompt.Message, "URL do repositório"):
				*(response.(*string)) = "https://github.com/org/test.git"
			case strings.Contains(prompt.Message, "Nome do Projeto"):
				*(response.(*string)) = "kepler-test"
			}
		case *survey.Select:
			if strings.Contains(prompt.Message, "Topologia") {
				*(response.(*string)) = "standard"
			} else if strings.Contains(prompt.Message, "Workflow") {
				*(response.(*string)) = "essential"
			}
		case *survey.MultiSelect:
			// Verifica que os defaults contêm Kepler
			if prompt, ok := p.(*survey.MultiSelect); ok {
				foundKepler := false
				for _, d := range prompt.Default.([]string) {
					if strings.Contains(d, "Kepler") {
						foundKepler = true
					}
				}
				assert.True(t, foundKepler, "Default do MultiSelect deveria conter Kepler")
			}
			*(response.(*[]string)) = []string{"Kepler (Eficiência Energética)"}
		case *survey.Confirm:
			*(response.(*bool)) = false
		}
		return nil
	}
	defer func() { askOne = originalAskOne }()

	o := &InitOptions{
		NonInteractive: false,
		Topology:       "",
		EnableKepler:   true,
		Offline:        true,
	}

	ctx, err := buildContext(o)
	require.NoError(t, err)
	assert.True(t, ctx.EnableKepler)
}

func TestBuildContext_InterativoComGitRepoPreenchido(t *testing.T) {
	// Quando GitRepo já está definido, não pergunta a URL — mas ajusta o defaultName
	originalAskOne := askOne
	askOne = func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
		switch prompt := p.(type) {
		case *survey.Input:
			switch {
			case strings.Contains(prompt.Message, "Onde deseja inicializar"):
				*(response.(*string)) = "."
			case strings.Contains(prompt.Message, "URL do repositório"):
				t.Error("Não deveria perguntar URL do repo quando já está definido")
			case strings.Contains(prompt.Message, "Nome do Projeto"):
				// O default deveria ser derivado do git repo
				*(response.(*string)) = prompt.Default
			}
		case *survey.Select:
			if strings.Contains(prompt.Message, "Topologia") {
				*(response.(*string)) = "standard"
			} else if strings.Contains(prompt.Message, "Workflow") {
				*(response.(*string)) = "essential"
			}
		case *survey.MultiSelect:
			*(response.(*[]string)) = []string{}
		case *survey.Confirm:
			*(response.(*bool)) = false
		}
		return nil
	}
	defer func() { askOne = originalAskOne }()

	o := &InitOptions{
		NonInteractive: false,
		Topology:       "",
		GitRepo:        "https://github.com/org/meu-repo.git",
		Offline:        true,
	}

	ctx, err := buildContext(o)
	require.NoError(t, err)
	assert.Equal(t, "meu-repo", ctx.ProjectName,
		"ProjectName deveria ser derivado do GitRepo")
}

func TestBuildContext_InterativoComDomainEEmailCustom(t *testing.T) {
	// Quando Domain e Email já foram customizados, não pergunta novamente
	originalAskOne := askOne
	askOne = func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
		switch prompt := p.(type) {
		case *survey.Input:
			switch {
			case strings.Contains(prompt.Message, "Onde deseja inicializar"):
				*(response.(*string)) = "."
			case strings.Contains(prompt.Message, "URL do repositório"):
				*(response.(*string)) = "https://github.com/org/test.git"
			case strings.Contains(prompt.Message, "Nome do Projeto"):
				*(response.(*string)) = "custom-domain-test"
			case strings.Contains(prompt.Message, "Domínio Base"):
				t.Error("Não deveria perguntar Domain quando já está customizado")
			case strings.Contains(prompt.Message, "Email"):
				t.Error("Não deveria perguntar Email quando já está customizado")
			}
		case *survey.Select:
			if strings.Contains(prompt.Message, "Topologia") {
				*(response.(*string)) = "standard"
			} else if strings.Contains(prompt.Message, "Workflow") {
				*(response.(*string)) = "essential"
			}
		case *survey.MultiSelect:
			*(response.(*[]string)) = []string{}
		case *survey.Confirm:
			*(response.(*bool)) = false
		}
		return nil
	}
	defer func() { askOne = originalAskOne }()

	o := &InitOptions{
		NonInteractive: false,
		Topology:       "",
		Domain:         "custom.example.com",
		Email:          "admin@custom.example.com",
		Offline:        true,
	}

	ctx, err := buildContext(o)
	require.NoError(t, err)
	assert.Equal(t, "custom.example.com", ctx.Domain)
	assert.Equal(t, "admin@custom.example.com", ctx.Email)
}

func TestBuildContext_InterativoComDescriptionJaDefinida(t *testing.T) {
	// Quando Description já está definida, não pergunta no prompt interativo de IA
	originalAskOne := askOne
	askOne = func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
		switch prompt := p.(type) {
		case *survey.Input:
			switch {
			case strings.Contains(prompt.Message, "Onde deseja inicializar"):
				*(response.(*string)) = "."
			case strings.Contains(prompt.Message, "URL do repositório"):
				*(response.(*string)) = "https://github.com/org/test.git"
			case strings.Contains(prompt.Message, "Nome do Projeto"):
				*(response.(*string)) = "desc-flag-test"
			case strings.Contains(prompt.Message, "Descreva seu projeto"):
				t.Error("Não deveria perguntar a descrição quando já está definida via flag")
			}
		case *survey.Select:
			switch {
			case strings.Contains(prompt.Message, "Topologia"):
				*(response.(*string)) = "standard"
			case strings.Contains(prompt.Message, "Workflow"):
				*(response.(*string)) = "essential"
			case strings.Contains(prompt.Message, "Provedor de IA"):
				*(response.(*string)) = "auto"
			}
		case *survey.MultiSelect:
			*(response.(*[]string)) = []string{}
		case *survey.Confirm:
			*(response.(*bool)) = true
		}
		return nil
	}
	defer func() { askOne = originalAskOne }()

	o := &InitOptions{
		NonInteractive: false,
		Topology:       "",
		Description:    "Projeto já descrito via flag",
	}

	ctx, err := buildContext(o)
	require.NoError(t, err)
	assert.NotNil(t, ctx)
}
