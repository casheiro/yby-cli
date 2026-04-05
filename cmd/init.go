/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/casheiro/yby-cli/pkg/ai"
	"github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/pkg/filesystem"
	"github.com/casheiro/yby-cli/pkg/plugin"
	"github.com/casheiro/yby-cli/pkg/scaffold"
	"github.com/casheiro/yby-cli/pkg/services/shared"
	"github.com/casheiro/yby-cli/pkg/services/validate"
	"github.com/casheiro/yby-cli/pkg/templates"
	"github.com/spf13/cobra"
)

// askInput é um atalho mockável para prompts de input nos testes.
var askInput = func(title, defaultVal string) (string, error) {
	return prompter.Input(title, defaultVal)
}

// askSelect é um atalho mockável para prompts de select nos testes.
var askSelect = func(title string, options []string, defaultVal string) (string, error) {
	return prompter.Select(title, options, defaultVal)
}

// askConfirm é um atalho mockável para prompts de confirm nos testes.
var askConfirm = func(title string, defaultVal bool) (bool, error) {
	return prompter.Confirm(title, defaultVal)
}

// askMultiSelect é um atalho mockável para prompts de multi-select nos testes.
var askMultiSelect = func(title string, options []string, defaults []string) ([]string, error) {
	return prompter.MultiSelect(title, options, defaults)
}

// InitOptions holds the flags for headless mode
type InitOptions struct {
	Topology            string
	Workflow            string
	IncludeDevContainer bool
	IncludeCI           bool

	// Project Details
	TargetDir   string // New Flag
	GitRepo     string
	GitBranch   string
	ProjectName string // New Flag
	Description string // AI Context Flag
	AIProvider  string // explicit provider
	Domain      string
	Email       string
	Environment string

	// Secrets
	SecretsStrategy string

	// Modules
	EnableKepler        bool
	EnableMinio         bool
	EnableKEDA          bool
	EnableMetricsServer bool

	// Modes
	Offline        bool
	NonInteractive bool
	Force          bool
	Update         bool
}

var opts InitOptions

func init() {
	rootCmd.AddCommand(initCmd)

	// Bind Flags
	initCmd.Flags().StringVar(&opts.Topology, "topology", "", "Estratégia de Topologia: single, standard, complete")
	initCmd.Flags().StringVar(&opts.Workflow, "workflow", "", "Padrão de Workflow: essential, gitflow, trunkbased")
	initCmd.Flags().BoolVar(&opts.IncludeDevContainer, "include-devcontainer", false, "Gerar configuração .devcontainer")
	initCmd.Flags().BoolVar(&opts.IncludeCI, "include-ci", true, "Habilitar geração de CI/CD")
	initCmd.Flags().BoolVar(&opts.Offline, "offline", false, "Modo Offline: Pula verificações de Git remoto e usa defaults locais")
	initCmd.Flags().BoolVar(&opts.NonInteractive, "non-interactive", false, "Modo Não-Interativo: Falha se argumentos obrigatórios estiverem faltando (Ideal para VPS/CI)")
	initCmd.Flags().BoolVar(&opts.Force, "force", false, "Sobrescrever projeto existente sem confirmação")
	initCmd.Flags().BoolVar(&opts.Update, "update", false, "Atualiza scaffold preservando alterações do usuário")

	initCmd.Flags().StringVarP(&opts.TargetDir, "target-dir", "t", "", "Diretório alvo para inicialização do projeto")
	initCmd.Flags().StringVar(&opts.GitRepo, "git-repo", "", "URL do Repositório Git")
	initCmd.Flags().StringVar(&opts.GitBranch, "git-branch", "main", "Branch principal do git")
	initCmd.Flags().StringVar(&opts.ProjectName, "project-name", "", "Nome do Projeto/Slug (Sobrescreve derivação padrão)")
	initCmd.Flags().StringVar(&opts.Description, "description", "", "Descrição em linguagem natural do projeto (Habilita geração por IA)")
	initCmd.Flags().StringVar(&opts.AIProvider, "ai-provider", "", "Forçar provedor de IA específico (ollama, gemini, openai)")
	initCmd.Flags().StringVar(&opts.Domain, "domain", "yby.local", "Domínio base do cluster")
	initCmd.Flags().StringVar(&opts.Email, "email", "admin@yby.local", "Email do admin")
	initCmd.Flags().StringVar(&opts.Environment, "env", "dev", "Nome do ambiente inicial")

	initCmd.Flags().StringVar(&opts.SecretsStrategy, "secrets-strategy", "external-secrets", "Estratégia de secrets: sealed-secrets, external-secrets, sops")

	initCmd.Flags().BoolVar(&opts.EnableKepler, "enable-kepler", false, "Habilitar módulo Kepler")
	initCmd.Flags().BoolVar(&opts.EnableMinio, "enable-minio", false, "Habilitar módulo MinIO")
	initCmd.Flags().BoolVar(&opts.EnableKEDA, "enable-keda", false, "Habilitar módulo KEDA")
	initCmd.Flags().BoolVar(&opts.EnableMetricsServer, "enable-metrics-server", false, "Habilitar Metrics Server (Observability)")
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Inicializa um novo projeto Yby (Scaffold)",
	Long: `Gera a estrutura inicial do projeto (Charts, Manifests, Workflows) baseada em padrões.
Suporta execução interativa (Wizard) ou Headless (Flags).`,
	Example: `  # Modo Interativo (Wizard)
  yby init

  # Modo Headless (CI/CD ou Scripts)
  yby init --project-name meu-app --git-repo https://github.com/org/repo.git --topology standard --workflow gitflow --target-dir infra

  # AI-Native Initialization
  yby init --description "A secure payment gateway for crypto assets"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Validação: --update e --force são mutuamente exclusivos
		if opts.Update && opts.Force {
			return errors.New(errors.ErrCodeValidation,
				"--update e --force são mutuamente exclusivos. Use --update para merge ou --force para sobrescrever")
		}

		fmt.Println("🌱 Yby Smart Init (Native Engine)")

		// 1. Build Context (Merge Flags + Prompts)
		// Initialize Plugin Manager
		pm := plugin.NewManager()
		if err := pm.Discover(); err != nil {
			fmt.Printf("⚠️  Erro na descoberta de plugins: %v\n", err)
		} else {
			plugins := pm.ListPlugins()
			if len(plugins) > 0 {
				fmt.Printf("🔌 %d Plugins carregados: ", len(plugins))
				names := []string{}
				for _, p := range plugins {
					names = append(names, p.Name)
				}
				fmt.Println(strings.Join(names, ", "))
			}
		}

		ctx, errCtx := buildContext(&opts)
		if errCtx != nil {
			return errCtx
		}

		// Hook: context (Enrich BlueprintContext)
		if err := pm.ExecuteContextHook(ctx); err != nil {
			fmt.Printf("⚠️  Erro no hook 'context' dos plugins: %v\n", err)
		}

		// 1.5 Detecção de init duplo (pular se --update, que trata isso depois)
		targetDir := resolveTargetDir(opts.TargetDir)
		blueprintPath := filepath.Join(targetDir, ".yby", "blueprint.yaml")
		if !opts.Update {
			if _, err := os.Stat(blueprintPath); err == nil {
				// Projeto já inicializado
				if opts.Force {
					fmt.Println("⚠️  Projeto Yby já inicializado. Sobrescrevendo (--force).")
				} else if opts.NonInteractive {
					return errors.New(errors.ErrCodeValidation,
						"projeto Yby já inicializado neste diretório. Usar --force para sobrescrever")
				} else {
					confirm, err := askConfirm("Projeto Yby já inicializado neste diretório. Deseja sobrescrever?", false)
					if err != nil || !confirm {
						return errors.New(errors.ErrCodeValidation, "operação cancelada pelo usuário")
					}
				}
			}
		}

		// 1.6 Carregar manifest anterior como defaults (se existir e não for --force puro)
		var existingManifest *scaffold.ProjectManifest
		if m, err := scaffold.LoadProjectManifest(targetDir); err == nil {
			existingManifest = m
			defaults := scaffold.ManifestToContext(existingManifest)
			scaffold.MergeContextDefaults(ctx, defaults)
			fmt.Println("📋 Configurações anteriores carregadas como base.")
		}

		// 1.7 Fluxo --update: merge inteligente preservando alterações do usuário
		if opts.Update {
			if existingManifest == nil {
				return errors.New(errors.ErrCodeValidation,
					"--update requer um projeto já inicializado com manifest (.yby/project.yaml)")
			}

			return runUpdateFlow(targetDir, ctx, existingManifest, &opts)
		}

		// 2. Execute Scaffold
		fmt.Println("🚀 Gerando arquivos...")

		// Prepare CompositeFS
		// Layers: [Plugins Assets...] + [Core Embed Assets]
		// LIFO: Plugin adds layer to be checked BEFORE core.
		// CompositeFS iterates backwards. So we want Core at index 0?
		// NewCompositeFS(layers...). Open iterates len-1 down to 0.
		// So if we want Plugin to override Core, Plugin must be AT HIGHER INDEX.
		// Layers: [Core, Plugin1, Plugin2]
		// Open -> check Plugin2, then Plugin1, then Core.

		layers := []fs.FS{templates.Assets} // Core at base

		// Hook: assets (Get local paths)
		assetPaths := pm.GetAssets()
		for _, path := range assetPaths {
			// Add as os.DirFS
			// Note: We need to ensure the structure inside path matches "assets/..." expectation of engine?
			// Engine walks "assets". So if plugin returns "/path/to/plugin/assets",
			// and inside that we have "assets/file.yaml", it works.
			// Or does plugin return root that CONTAINS assets folder?
			// Let's assume plugin returns path that HAS "assets" subdirectory or IS the root?
			// Standard: Plugin assets folder should probably replicate structure.
			// If engine walks "assets", then FS.Open("assets/...") is called.
			// So os.DirFS(path) works if path contains "assets".
			layers = append(layers, os.DirFS(path))
		}

		compositeFS := filesystem.NewCompositeFS(layers...)

		if err := scaffold.Apply(targetDir, ctx, compositeFS); err != nil {
			return errors.Wrap(err, errors.ErrCodeManifest, "Erro ao gerar scaffold")
		}

		// 2.5 Generative AI Layer
		// Try to initialize AI Provider (Ollama for now)
		// Factory: Detect Best Available Provider (Ollama -> Gemini -> OpenAI)
		bgCtx := context.Background()
		aiProvider := ai.GetProvider(bgCtx, opts.AIProvider)

		// Check availability
		if aiProvider != nil {
			fmt.Printf("🤖 Motor de IA Detectado: %s\n", aiProvider.Name())

			description := opts.Description

			if description != "" {
				fmt.Printf("🧠 Processando... (Analisando: '%s')\n", description)
				blueprint, err := generateGovernanceViaCompletion(bgCtx, aiProvider, description)
				if err != nil {
					fmt.Printf("⚠️ Falha na geração por IA: %v. Usando templates estáticos.\n", err)
				} else {
					fmt.Printf("✨ Domínio Inferido: %s (%s)\n", blueprint.Domain, blueprint.RiskLevel)

					// Write Generated Files
					for _, f := range blueprint.Files {
						fullPath := filepath.Join(targetDir, f.Path)

						// FIX: Check if path is root-level to respect engine logic location?
						// Actually, engine.go logic handles file copying from ASSETS.
						// Here we are writing AI Generated files directly.
						// We need to respect the same 'root' logic as engine.go for .synapstor!
						// If path starts with .synapstor, it should go to RepoRoot, not targetDir (if targetDir is infra).
						// Duplicating logic here is dangerous?
						// Ideally AI provider just returns files and we use scaffold to write them?
						// For now, let's keep it simple but maybe check path prefix.

						if strings.HasPrefix(f.Path, ".github") {
							// Try to resolve git root to avoid putting .github inside infra/
							if gitRoot, err := scaffold.GetGitRoot(); err == nil && gitRoot != "" {
								fullPath = filepath.Join(gitRoot, f.Path)
							} else if targetDir != "." && targetDir != "" {
								// Fallback to CWD
								wd, _ := os.Getwd()
								fullPath = filepath.Join(wd, f.Path)
							}
						}

						// Ensure dirs
						_ = os.MkdirAll(filepath.Dir(fullPath), 0755)
						if err := os.WriteFile(fullPath, []byte(f.Content), 0644); err == nil {
							fmt.Printf("   📝 Gerado por IA: %s\n", f.Path)
						}
					}
				}
			}
		} else if opts.Description != "" {
			fmt.Println("⚠️  AVISO: Funcionalidade de IA solicitada, mas nenhum provedor configurado ou disponível.")
			fmt.Println("    Verifique se o Ollama está rodando ou se as chaves de API (GEMINI_API_KEY, OPENAI_API_KEY) estão definidas.")
		}

		// 3. Post-Scaffold: Generate Values Files for Environments (diferenciados)
		for _, env := range ctx.Environments {
			target := filepath.Join(targetDir, fmt.Sprintf("config/values-%s.yaml", env))
			if _, err := os.Stat(target); os.IsNotExist(err) {
				content := scaffold.RenderEnvironmentValues(ctx, env)
				_ = os.MkdirAll(filepath.Dir(target), 0755)
				_ = os.WriteFile(target, []byte(content), 0644)
				fmt.Printf("   📄 Generated Config: %s\n", target)
			}
		}

		// 4. Validação pós-scaffold (apenas warnings, não bloqueia)
		fmt.Println("\n🔍 Validando charts gerados...")
		runner := &shared.RealRunner{}
		helmRunner := &validate.RealHelmRunner{Runner: runner}
		validateSvc := validate.NewService(helmRunner)

		chartCandidates := []string{
			filepath.Join(targetDir, "charts/system"),
			filepath.Join(targetDir, "charts/bootstrap"),
			filepath.Join(targetDir, "charts/cluster-config"),
		}

		// Filtrar apenas charts que existem
		var existingCharts []string
		for _, c := range chartCandidates {
			if _, err := os.Stat(filepath.Join(c, "Chart.yaml")); err == nil {
				existingCharts = append(existingCharts, c)
			}
		}

		if len(existingCharts) > 0 {
			valuesFile := filepath.Join(targetDir, "config/cluster-values.yaml")
			report, err := validateSvc.Run(bgCtx, existingCharts, valuesFile)
			if err != nil || !report.Success {
				fmt.Printf("⚠️  Validação detectou problemas (não bloqueante):\n")
				if err != nil {
					fmt.Printf("   %v\n", err)
				}
				for _, cr := range report.Charts {
					if cr.Error != "" {
						fmt.Printf("   %s: %s\n", cr.Chart, cr.Error)
					}
				}
				fmt.Println("   Execute 'yby validate' para detalhes completos.")
			} else {
				fmt.Println("✅ Charts validados com sucesso!")
			}
		}

		// 5. Persistir Project Manifest (.yby/project.yaml)
		if err := scaffold.SaveProjectManifest(targetDir, ctx); err != nil {
			fmt.Printf("⚠️  Falha ao salvar project manifest: %v\n", err)
		}

		fmt.Println("✅ Projeto inicializado com sucesso!")
		fmt.Println("   próximo passo: 'yby env list'")
		return nil
	},
}

func buildContext(flags *InitOptions) (*scaffold.BlueprintContext, error) {
	ctx := &scaffold.BlueprintContext{
		GitRepoURL:          flags.GitRepo,
		GitBranch:           flags.GitBranch,
		Domain:              flags.Domain,
		Email:               flags.Email,
		Environment:         flags.Environment,
		EnableCI:            flags.IncludeCI,
		EnableDevContainer:  flags.IncludeDevContainer,
		EnableKepler:        flags.EnableKepler,
		EnableMinio:         flags.EnableMinio,
		EnableKEDA:          flags.EnableKEDA,
		EnableMetricsServer: flags.EnableMetricsServer,
		SecretsStrategy:     flags.SecretsStrategy,
		Topology:            flags.Topology,
		WorkflowPattern:     flags.Workflow,

		// Template Data
		GitRepo:     flags.GitRepo,
		ProjectName: resolveProjectName(flags),
	}

	// Populate Github details
	if org := extractGithubOrg(flags.GitRepo); org != "" {
		ctx.GithubOrg = org
		ctx.GithubDiscovery = true
	} else {
		// Default to a placeholder if needed, or disable discovery
		// If Discovery is enabled by default in schema, we should provide something?
		// Schema says enabled: true default.
		// If we leave empty, helm might complain OR render empty string.
		// Let's set a default "yby-org" if strictly needed, or trust user fills it.
		// For Zero Config, we try our best.
		if flags.Offline {
			ctx.GithubOrg = "yby-local"
			ctx.GithubDiscovery = false // Disable in offline/local mirror mode usually?
		}
	}

	// Calculate RepoRootPath (for ArgoCD)
	if gitRoot, err := scaffold.GetGitRoot(); err == nil && gitRoot != "" {
		// Determine absolute target path
		targetDir := "."
		if flags.TargetDir != "" {
			targetDir = flags.TargetDir
		}
		absTarget, _ := filepath.Abs(targetDir)

		// Calculate relative path from GitRoot to TargetDir
		if rel, err := filepath.Rel(gitRoot, absTarget); err == nil {
			// If target is same as root, rel is "." -> empty for path joining
			if rel == "." {
				ctx.RepoRootPath = ""
			} else {
				// Ensure forward slashes for ArgoCD/K8s manifests
				ctx.RepoRootPath = filepath.ToSlash(rel)
			}
		}
	}

	// Enrich with AI Context
	inferContext(ctx)

	// If flags are missing, ask via Survey (Interactive Mode)
	// We check strictly if Topology/Workflow are empty implies interaction needed.
	// Or we can ask specifically for what's missing.

	interactive := false
	// Interaction logic:
	// If NonInteractive is TRUE -> User explicitly wants NO prompts. We must VALIDATE required flags.
	// If NonInteractive is FALSE -> We default to interactive if critical flags are missing.

	if flags.NonInteractive {
		// VALIDATION MODE
		missing := validateNonInteractiveFlags(ctx, flags)
		if len(missing) > 0 {
			return nil, errors.New(errors.ErrCodeValidation, fmt.Sprintf("Modo --non-interactive ativo, mas argumentos obrigatórios estão faltando: %s", strings.Join(missing, ", ")))
		}
		interactive = false
	} else {
		// Default behavior: specific flags missing trigger interaction
		if ctx.Topology == "" || ctx.WorkflowPattern == "" {
			interactive = true
		}
	}

	if interactive {
		fmt.Println("------------------------------------")
		// Directory Prompt
		if flags.TargetDir == "" {
			dir, err := askInput("Onde deseja inicializar o projeto? (caminho relativo ou absoluto)", ".")
			if err != nil {
				return nil, errors.Wrap(err, errors.ErrCodeValidation, "Prompt cancelado pelo usuário")
			}
			flags.TargetDir = dir
		}

		// Topology Prompt
		if ctx.Topology == "" {
			val, err := askSelect("Selecione a Topologia de Ambientes:", []string{"single", "standard", "complete"}, "standard")
			if err != nil {
				return nil, errors.Wrap(err, errors.ErrCodeValidation, "Prompt cancelado pelo usuário")
			}
			ctx.Topology = val
		}

		// Workflow Prompt
		if ctx.WorkflowPattern == "" {
			val, err := askSelect("Selecione o Padrão de Workflow (CI/CD):", []string{"essential", "gitflow", "trunkbased"}, "gitflow")
			if err != nil {
				return nil, errors.Wrap(err, errors.ErrCodeValidation, "Prompt cancelado pelo usuário")
			}
			ctx.WorkflowPattern = val
		}

		// Ask for Git Repo if missing
		if ctx.GitRepoURL == "" {
			if flags.Offline {
				ctx.GitRepoURL = "http://git-server.yby-system.svc/repo.git"
			} else {
				val, err := askInput("Qual a URL do repositório Git?", "")
				if err != nil {
					return nil, errors.Wrap(err, errors.ErrCodeValidation, "Prompt cancelado pelo usuário")
				}
				ctx.GitRepoURL = val
			}
		}

		// Ask for Project Name (after Git Repo is resolved)
		defaultName := ctx.ProjectName
		if ctx.GitRepoURL != "" && defaultName == "yby-project" {
			defaultName = deriveProjectName(ctx.GitRepoURL)
		}

		val, _ := askInput("Nome do Projeto (Slug para K8s):", defaultName)
		ctx.ProjectName = val

		// Ask for Project Details (Domain / Email)
		if ctx.Domain == "yby.local" {
			val, _ := askInput("Defina o Domínio Base do Cluster:", "yby.local")
			ctx.Domain = val
		}

		if ctx.Email == "admin@yby.local" {
			val, _ := askInput("Email para Admin/Certificados:", "admin@yby.local")
			ctx.Email = val
		}

		// Modules Selection (MultiSelect)
		defaults := []string{}
		if ctx.EnableKepler {
			defaults = append(defaults, "Kepler (Eficiência Energética)")
		}

		selectedModules, err := askMultiSelect("Selecione os Módulos Adicionais (Add-ons):",
			[]string{"Kepler (Eficiência Energética)", "MinIO (Object Storage Local)", "KEDA (Event-Driven Autoscaling)", "Observability Core (Metrics Server)"},
			defaults)
		if err != nil {
			return nil, errors.Wrap(err, errors.ErrCodeValidation, "Prompt cancelado pelo usuário")
		}

		// Mapeia a seleção de volta para o contexto
		ctx.EnableKepler, ctx.EnableMinio, ctx.EnableKEDA, ctx.EnableMetricsServer = mapModuleSelection(selectedModules)

		// Ask for DevContainer
		if !flags.IncludeDevContainer {
			devContainer, err := askConfirm("Deseja incluir configuração de DevContainer (.devcontainer)?", true)
			if err != nil {
				return nil, errors.Wrap(err, errors.ErrCodeValidation, "Prompt cancelado pelo usuário")
			}
			ctx.EnableDevContainer = devContainer
		}

		// ---------------------------------------------------------
		// AI & Governance Section
		// ---------------------------------------------------------
		if !flags.Offline {
			enableAI, _ := askConfirm("Deseja ativar o Assistente de IA (Synapstor & Governança)?", true)

			if enableAI {
				// Provider Selection
				if flags.AIProvider == "" {
					provider, _ := askSelect("Selecione o Provedor de IA:", []string{"auto", "ollama", "gemini", "openai"}, "auto")
					if provider == "auto" {
						provider = ""
					}
					flags.AIProvider = provider
				}

				// Description
				if flags.Description == "" {
					desc, _ := askInput("Descreva seu projeto (em linguagem natural):", "")
					flags.Description = desc
				}
			}
		}
	}

	// Post-Validation / Defaults

	// Sanitizar project name em modo interativo
	if interactive && ctx.ProjectName != "" {
		ctx.ProjectName = scaffold.SanitizeProjectName(ctx.ProjectName)
	}

	// Validar contexto (project-name, domain, email, git-repo, topology, workflow)
	if err := scaffold.ValidateContext(ctx); err != nil {
		return nil, err
	}

	// Calcula lista de ambientes baseada na topologia
	ctx.Environments = environmentsForTopology(ctx.Topology)

	// Modo offline: garante que "local" está presente
	if flags.Offline {
		ctx.Environments = ensureLocalEnvironment(ctx.Environments)
	}

	// Valida se o ambiente atual existe na topologia
	newEnv, valid := validateEnvironment(ctx.Environment, ctx.Environments)
	if !valid {
		fmt.Printf("⚠️  Ambiente inicial '%s' não existe na topologia '%s'. Ajustando para '%s'.\n", ctx.Environment, ctx.Topology, newEnv)
		ctx.Environment = newEnv
	}

	return ctx, nil
}

func resolveProjectName(flags *InitOptions) string {
	if flags.ProjectName != "" {
		return flags.ProjectName
	}
	return deriveProjectName(flags.GitRepo)
}

func deriveProjectName(repoURL string) string {
	if repoURL == "" {
		return "yby-project"
	}

	// 1. Clean URL
	repoURL = strings.TrimSpace(repoURL)
	repoURL = strings.TrimSuffix(repoURL, "/")
	repoURL = strings.TrimSuffix(repoURL, ".git")

	// 2. Extract last part
	parts := strings.Split(repoURL, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}

	return "yby-project"
}

func extractGithubOrg(repoURL string) string {
	if repoURL == "" {
		return ""
	}
	repoURL = strings.TrimSpace(repoURL)
	repoURL = strings.TrimSuffix(repoURL, "/")
	repoURL = strings.TrimSuffix(repoURL, ".git")

	// Handle git@github.com:org/repo
	if strings.Contains(repoURL, "git@") {
		parts := strings.Split(repoURL, ":")
		if len(parts) == 2 {
			repoURL = parts[1]
		}
	} else {
		// Handle https://github.com/org/repo
		parts := strings.Split(repoURL, "github.com/")
		if len(parts) == 2 {
			repoURL = parts[1]
		}
	}

	parts := strings.Split(repoURL, "/")
	if len(parts) >= 2 {
		return parts[0] // Org is the first part
	}
	return ""
}

// resolveTargetDir retorna o diretório alvo para o scaffold.
// Se o TargetDir das opções estiver vazio, retorna ".".
func resolveTargetDir(targetDir string) string {
	if targetDir != "" {
		return targetDir
	}
	return "."
}

// environmentsForTopology retorna a lista de ambientes baseada na topologia informada.
func environmentsForTopology(topology string) []string {
	switch topology {
	case "single":
		return []string{"local"}
	case "standard":
		return []string{"local", "prod"}
	case "complete":
		return []string{"local", "dev", "staging", "prod"}
	default:
		return []string{"local"}
	}
}

// ensureLocalEnvironment garante que o ambiente "local" está presente na lista,
// adicionando-o ao início caso não exista. Usado no modo offline.
func ensureLocalEnvironment(envs []string) []string {
	for _, e := range envs {
		if e == "local" {
			return envs
		}
	}
	return append([]string{"local"}, envs...)
}

// validateEnvironment verifica se o ambiente atual é válido para a topologia.
// Se não for, retorna o primeiro ambiente da lista e found=false.
func validateEnvironment(env string, envs []string) (string, bool) {
	for _, e := range envs {
		if e == env {
			return env, true
		}
	}
	if len(envs) > 0 {
		return envs[0], false
	}
	return env, false
}

// mapModuleSelection converte a lista de seleções do prompt MultiSelect
// em flags booleanas de módulos.
func mapModuleSelection(selectedModules []string) (kepler, minio, keda, metricsServer bool) {
	for _, m := range selectedModules {
		if strings.Contains(m, "Kepler") {
			kepler = true
		}
		if strings.Contains(m, "MinIO") {
			minio = true
		}
		if strings.Contains(m, "KEDA") {
			keda = true
		}
		if strings.Contains(m, "Observability") {
			metricsServer = true
		}
	}
	return
}

// validateNonInteractiveFlags valida que os campos obrigatórios estão presentes
// no modo não-interativo. Retorna a lista de flags faltantes.
func validateNonInteractiveFlags(ctx *scaffold.BlueprintContext, flags *InitOptions) []string {
	missing := []string{}
	if ctx.Topology == "" {
		missing = append(missing, "--topology")
	}
	if ctx.WorkflowPattern == "" {
		missing = append(missing, "--workflow")
	}
	if ctx.GitRepoURL == "" && !flags.Offline {
		if ctx.ProjectName == "yby-project" && flags.ProjectName == "" {
			missing = append(missing, "--project-name OR --git-repo")
		}
	}
	return missing
}

// resolveAIFilePath determina o caminho completo para um arquivo gerado por IA.
// Arquivos .github são colocados na raiz do repositório Git, não dentro do targetDir.
func resolveAIFilePath(filePath, targetDir, gitRoot string) string {
	fullPath := filepath.Join(targetDir, filePath)

	if strings.HasPrefix(filePath, ".github") {
		if gitRoot != "" {
			return filepath.Join(gitRoot, filePath)
		}
		if targetDir != "." && targetDir != "" {
			// Sem gitRoot, usa o diretório de trabalho atual
			// (nesta função pura, retornamos apenas baseado no targetDir)
			return filePath // retorna o caminho relativo ao CWD
		}
	}

	return fullPath
}

// inferContext populates AI-related context fields based on heuristics
func inferContext(ctx *scaffold.BlueprintContext) {
	name := strings.ToLower(ctx.ProjectName)

	// Defaults
	ctx.BusinessDomain = "General Purpose"
	ctx.ImpactLevel = "Medium"
	ctx.Archetype = "Cloud-Native Application"

	// Heuristics
	if strings.Contains(name, "bank") || strings.Contains(name, "pay") || strings.Contains(name, "wallet") || strings.Contains(name, "fin") {
		ctx.BusinessDomain = "Fintech / Financial Services"
		ctx.ImpactLevel = "Critical (High Security Requirement)"
	} else if strings.Contains(name, "shop") || strings.Contains(name, "store") || strings.Contains(name, "comm") || strings.Contains(name, "cart") {
		ctx.BusinessDomain = "E-Commerce / Retail"
		ctx.ImpactLevel = "High (Availability Requirement)"
	} else if strings.Contains(name, "data") || strings.Contains(name, "etl") || strings.Contains(name, "flow") || strings.Contains(name, "lake") {
		ctx.BusinessDomain = "Data Engineering"
		ctx.Archetype = "Data Pipeline / Batch Processing"
	} else if strings.Contains(name, "api") || strings.Contains(name, "svc") || strings.Contains(name, "gate") {
		ctx.Archetype = "Backend Microservice"
	}

	if ctx.Topology == "complete" {
		ctx.ImpactLevel += " (Enterprise Topology)"
	}
}

// runUpdateFlow executa o fluxo de --update: gera scaffold em tmpdir, computa merge plan e aplica.
func runUpdateFlow(targetDir string, ctx *scaffold.BlueprintContext, manifest *scaffold.ProjectManifest, flags *InitOptions) error {
	fmt.Println("🔄 Modo Update: analisando alterações...")

	// 1. Gerar scaffold em diretório temporário
	tmpDir, err := os.MkdirTemp("", "yby-update-*")
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeIO, "erro ao criar diretório temporário para update")
	}
	defer os.RemoveAll(tmpDir)

	layers := []fs.FS{templates.Assets}
	compositeFS := filesystem.NewCompositeFS(layers...)

	if err := scaffold.Apply(tmpDir, ctx, compositeFS); err != nil {
		return errors.Wrap(err, errors.ErrCodeManifest, "erro ao gerar scaffold para comparação")
	}

	// 2. Computar plano de merge
	manifestHashes := manifest.Spec.FileHashes
	if manifestHashes == nil {
		manifestHashes = make(map[string]string)
	}

	plan, err := scaffold.ComputeMergePlan(manifestHashes, targetDir, tmpDir)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeManifest, "erro ao computar plano de merge")
	}

	// 3. Exibir resumo do plano
	summary := plan.Summary()
	fmt.Println("\nPlano de Merge:")
	if n := summary[scaffold.ActionNone]; n > 0 {
		fmt.Printf("  %d arquivos sem alterações\n", n)
	}
	if n := summary[scaffold.ActionUpdate]; n > 0 {
		fmt.Printf("  %d arquivos a atualizar\n", n)
	}
	if n := summary[scaffold.ActionPreserve]; n > 0 {
		fmt.Printf("  %d arquivos preservados\n", n)
	}
	if n := summary[scaffold.ActionConflict]; n > 0 {
		fmt.Printf("  %d conflitos detectados\n", n)
	}
	if n := summary[scaffold.ActionNew]; n > 0 {
		fmt.Printf("  %d arquivos novos\n", n)
	}

	// Se não há nada a fazer, sair cedo
	totalChanges := summary[scaffold.ActionUpdate] + summary[scaffold.ActionConflict] + summary[scaffold.ActionNew]
	if totalChanges == 0 {
		fmt.Println("\n✅ Nenhuma alteração necessária. Projeto já está atualizado.")
		return nil
	}

	// 4. Confirmar com o usuário
	if !flags.NonInteractive {
		confirm, err := askConfirm("Deseja aplicar o plano de merge?", true)
		if err != nil || !confirm {
			return errors.New(errors.ErrCodeValidation, "operação cancelada pelo usuário")
		}
	}

	// 5. Aplicar plano com resolver de conflitos
	resolver := &scaffold.NonInteractiveResolver{Strategy: "conflict-markers"}
	if err := scaffold.ApplyMergePlan(plan, targetDir, tmpDir, resolver); err != nil {
		return errors.Wrap(err, errors.ErrCodeManifest, "erro ao aplicar plano de merge")
	}

	// 6. Salvar manifest com novos hashes
	newHashes, err := scaffold.ComputeDirHashes(targetDir)
	if err != nil {
		fmt.Printf("⚠️  Falha ao computar hashes pós-merge: %v\n", err)
	} else {
		if err := scaffold.SaveProjectManifest(targetDir, ctx, newHashes); err != nil {
			fmt.Printf("⚠️  Falha ao salvar project manifest: %v\n", err)
		}
	}

	fmt.Println("✅ Scaffold atualizado com sucesso!")
	return nil
}
