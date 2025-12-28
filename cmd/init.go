/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/casheiro/yby-cli/pkg/scaffold"
	"github.com/spf13/cobra"
)

// InitOptions holds the flags for headless mode
type InitOptions struct {
	Topology            string
	Workflow            string
	IncludeDevContainer bool
	IncludeCI           bool

	// Project Details
	GitRepo     string
	GitBranch   string
	Domain      string
	Email       string
	Environment string

	// Modules
	EnableKepler bool
	EnableMinio  bool
	EnableKEDA   bool
}

var opts InitOptions

func init() {
	rootCmd.AddCommand(initCmd)

	// Bind Flags
	initCmd.Flags().StringVar(&opts.Topology, "topology", "", "Topology strategy: single, standard, complete")
	initCmd.Flags().StringVar(&opts.Workflow, "workflow", "", "Workflow pattern: essential, gitflow, trunkbased")
	initCmd.Flags().BoolVar(&opts.IncludeDevContainer, "include-devcontainer", false, "Generate .devcontainer configuration")
	initCmd.Flags().BoolVar(&opts.IncludeCI, "include-ci", true, "Enable CI/CD generation")

	initCmd.Flags().StringVar(&opts.GitRepo, "git-repo", "", "Git Repository URL")
	initCmd.Flags().StringVar(&opts.GitBranch, "git-branch", "main", "Main git branch")
	initCmd.Flags().StringVar(&opts.Domain, "domain", "yby.local", "Cluster base domain")
	initCmd.Flags().StringVar(&opts.Email, "email", "admin@yby.local", "Admin email")
	initCmd.Flags().StringVar(&opts.Environment, "env", "dev", "Initial environment name")

	initCmd.Flags().BoolVar(&opts.EnableKepler, "enable-kepler", false, "Enable Kepler module")
	initCmd.Flags().BoolVar(&opts.EnableMinio, "enable-minio", false, "Enable MinIO module")
	initCmd.Flags().BoolVar(&opts.EnableKEDA, "enable-keda", false, "Enable KEDA module")
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Inicializa um novo projeto Yby (Scaffold)",
	Long: `Gera a estrutura inicial do projeto (Charts, Manifests, Workflows) baseada em padrões.
Suporta execução interativa (Wizard) ou Headless (Flags).`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("🌱 Yby Smart Init (Native Engine)")

		// 1. Build Context (Merge Flags + Prompts)
		ctx := buildContext(&opts)

		// 2. Execute Scaffold
		fmt.Println("🚀 Gerando arquivos...")

		targetDir := "."
		if err := scaffold.Apply(targetDir, ctx); err != nil {
			fmt.Printf("❌ Erro ao gerar scaffold: %v\n", err)
			os.Exit(1)
		}

		// 3. Post-Scaffold: Generate Values Files for Environments
		baseValues := "config/cluster-values.yaml"
		// Verify if scaffold created baseValues
		if _, err := os.Stat(baseValues); err == nil {
			for _, env := range ctx.Environments {
				target := fmt.Sprintf("config/values-%s.yaml", env)
				if _, err := os.Stat(target); os.IsNotExist(err) {
					// Copy content
					if content, err := os.ReadFile(baseValues); err == nil {
						// Simple replace if needed, or just clone
						_ = os.WriteFile(target, content, 0644)
						fmt.Printf("   📄 Generated Config: %s\n", target)
					}
				}
			}
		}

		fmt.Println("✅ Projeto inicializado com sucesso!")
		fmt.Println("   próximo passo: 'yby env list'")
	},
}

func buildContext(flags *InitOptions) *scaffold.BlueprintContext {
	ctx := &scaffold.BlueprintContext{
		GitRepoURL:         flags.GitRepo,
		GitBranch:          flags.GitBranch,
		Domain:             flags.Domain,
		Email:              flags.Email,
		Environment:        flags.Environment,
		EnableCI:           flags.IncludeCI,
		EnableDevContainer: flags.IncludeDevContainer,
		EnableKepler:       flags.EnableKepler,
		EnableMinio:        flags.EnableMinio,
		EnableKEDA:         flags.EnableKEDA,
		Topology:           flags.Topology,
		WorkflowPattern:    flags.Workflow,

		// Template Data
		GitRepo:     flags.GitRepo,
		ProjectName: deriveProjectName(flags.GitRepo),
	}

	// If flags are missing, ask via Survey (Interactive Mode)
	// We check strictly if Topology/Workflow are empty implies interaction needed.
	// Or we can ask specifically for what's missing.

	interactive := false
	if ctx.Topology == "" || ctx.WorkflowPattern == "" {
		interactive = true
	}

	if interactive {
		fmt.Println("------------------------------------")
		// Topology Prompt
		if ctx.Topology == "" {
			prompt := &survey.Select{
				Message: "Selecione a Topologia de Ambientes:",
				Options: []string{"single", "standard", "complete"},
				Help:    "single: Apenas 1 env. standard: Local+Prod. complete: Local+Dev+Staging+Prod",
				Default: "standard",
			}
			if err := survey.AskOne(prompt, &ctx.Topology); err != nil {
				fmt.Println("❌ Cancelado")
				os.Exit(1)
			}
		}

		// Workflow Prompt
		if ctx.WorkflowPattern == "" {
			prompt := &survey.Select{
				Message: "Selecione o Padrão de Workflow (CI/CD):",
				Options: []string{"essential", "gitflow", "trunkbased"},
				Help:    "essential: Apenas checks básica. gitflow: Release automatizado. trunkbased: CD rápido.",
				Default: "gitflow",
			}
			if err := survey.AskOne(prompt, &ctx.WorkflowPattern); err != nil {
				fmt.Println("❌ Cancelado")
				os.Exit(1)
			}
		}

		// Ask for Git Repo if missing
		if ctx.GitRepoURL == "" {
			prompt := &survey.Input{
				Message: "Qual a URL do repositório Git?",
			}
			if err := survey.AskOne(prompt, &ctx.GitRepoURL); err != nil {
				fmt.Println("❌ Cancelado")
				os.Exit(1)
			}
		}

		// Ask for DevContainer
		if !flags.IncludeDevContainer {
			prompt := &survey.Confirm{
				Message: "Deseja incluir configuração de DevContainer (.devcontainer)?",
				Default: true,
			}
			if err := survey.AskOne(prompt, &ctx.EnableDevContainer); err != nil {
				fmt.Println("❌ Cancelado")
				os.Exit(1)
			}
		}
	}

	// Post-Validation / Defaults

	// Calculate Environments list based on Topology
	switch ctx.Topology {
	case "single":
		ctx.Environments = []string{"prod"}
	case "standard":
		ctx.Environments = []string{"local", "prod"}
	case "complete":
		ctx.Environments = []string{"local", "dev", "staging", "prod"}
	default:
		ctx.Environments = []string{"local"}
	}

	return ctx
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
