/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	gocontext "context"
	"fmt"
	"time"

	"github.com/casheiro/yby-cli/pkg/cloud"
	ybycontext "github.com/casheiro/yby-cli/pkg/context"
	"github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/pkg/scaffold"
	"github.com/casheiro/yby-cli/pkg/services/shared"
	"github.com/spf13/cobra"
)

var envCmd = &cobra.Command{
	Use:     "env",
	Aliases: []string{"context"},
	Short:   "Gerencia ambientes e contextos (dev, staging, prod)",
}

// env list
var envListCmd = &cobra.Command{
	Use:   "list",
	Short: "Lista os ambientes disponíveis",
	Example: `  yby env list
  # Saída:
  # * local (local)
  #   prod (remote)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		infraRoot, err := FindInfraRoot()
		if err != nil {
			infraRoot = "."
		}
		mgr := ybycontext.NewManager(infraRoot)
		manifest, err := mgr.LoadManifest()
		if err != nil {
			return errors.Wrap(err, errors.ErrCodeConfig, "Falha ao carregar manifesto de ambientes")
		}

		fmt.Println("Ambientes disponíveis:")
		for name, env := range manifest.Environments {
			prefix := "  "
			if name == manifest.Current {
				prefix = "* "
			}
			fmt.Printf("%s%s (%s): %s\n", prefix, name, env.Type, env.Description)
		}
		return nil
	},
}

// env use <name>
var envUseCmd = &cobra.Command{
	Use:   "use [name]",
	Short: "Define o ambiente ativo",
	Example: `  yby env use prod
  # Atualiza automaticamente o contexto do kubectl e helm`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		infraRoot, err := FindInfraRoot()
		if err != nil {
			infraRoot = "."
		}
		mgr := ybycontext.NewManager(infraRoot)

		if err := mgr.SetCurrent(name); err != nil {
			return errors.Wrap(err, errors.ErrCodeConfig, "Falha ao definir ambiente ativo")
		}

		fmt.Printf("✅ Contexto alterado para '%s'\n", name)

		// Integração com kubectl context
		manifest, err := mgr.LoadManifest()
		if err == nil {
			if env, ok := manifest.Environments[name]; ok {
				if env.KubeContext != "" {
					runner := &shared.RealRunner{}
					if err := runner.Run(gocontext.Background(), "kubectl", "config", "use-context", env.KubeContext); err != nil {
						fmt.Printf("⚠️  Falha ao trocar kubectl context: %v\n", err)
					} else {
						fmt.Printf("   kubectl context: %s\n", env.KubeContext)
					}
				}
			}
		}

		return nil
	},
}

// env show
var envShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Mostra detalhes do ambiente atual",
	RunE: func(cmd *cobra.Command, args []string) error {
		infraRoot, err := FindInfraRoot()
		if err != nil {
			infraRoot = "."
		}
		mgr := ybycontext.NewManager(infraRoot)
		name, env, err := mgr.GetCurrent()
		if err != nil {
			return errors.Wrap(err, errors.ErrCodeConfig, "Falha ao obter detalhes do ambiente ativo")
		}

		fmt.Printf("Ambiente Ativo: %s\n", name)
		fmt.Printf("Tipo: %s\n", env.Type)
		fmt.Printf("Values: %s\n", env.Values)
		if env.URL != "" {
			fmt.Printf("URL: %s\n", env.URL)
		}

		// Exibe metadata cloud se presente
		if env.Cloud != nil {
			fmt.Println("\nCloud:")
			fmt.Printf("  Provider: %s\n", env.Cloud.Provider)
			if env.Cloud.Region != "" {
				fmt.Printf("  Region: %s\n", env.Cloud.Region)
			}
			if env.Cloud.Cluster != "" {
				fmt.Printf("  Cluster: %s\n", env.Cloud.Cluster)
			}
			if env.Cloud.Profile != "" {
				fmt.Printf("  Profile: %s\n", env.Cloud.Profile)
			}
			if env.Cloud.RoleARN != "" {
				fmt.Printf("  Role ARN: %s\n", env.Cloud.RoleARN)
			}

			// Tenta validar credenciais com timeout curto
			runner := &shared.RealRunner{}
			provider := cloud.GetProvider(runner, env.Cloud.Provider)
			if provider != nil {
				ctx, cancel := gocontext.WithTimeout(gocontext.Background(), 5*time.Second)
				defer cancel()
				if status, err := provider.ValidateCredentials(ctx); err == nil {
					if status.Authenticated {
						fmt.Printf("  Credenciais: ✅ válidas (%s)\n", status.Identity)
						if status.ExpiresAt != nil {
							fmt.Printf("  Expira em: %s\n", status.ExpiresAt.Format(time.RFC3339))
						}
					} else {
						fmt.Printf("  Credenciais: ❌ inválidas\n")
					}
				}
			}
		}

		return nil
	},
}

// env create
var envCreateCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Cria um novo ambiente e gera values correspondente",
	Example: `  yby env create qa --type remote --description "Quality Assurance"
  # Cria config/values-qa.yaml e adiciona entry em .yby/environments.yaml`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := ""
		if len(args) > 0 {
			name = args[0]
		}

		envType, _ := cmd.Flags().GetString("type")
		description, _ := cmd.Flags().GetString("description")

		// Interactive prompts if needed
		if name == "" {
			fmt.Print("Nome do ambiente (ex: qa, uat): ")
			if _, err := fmt.Scanln(&name); err != nil {
				return errors.Wrap(err, errors.ErrCodeValidation, "Prompt cancelado")
			}
		}
		if envType == "" {
			envType = "remote" // default
			// Could add prompt here
		}
		if description == "" {
			description = fmt.Sprintf("Environment %s", name)
		}

		infraRoot, err := FindInfraRoot()
		if err != nil {
			infraRoot = "."
		}

		kubeContext, _ := cmd.Flags().GetString("kube-context")
		namespace, _ := cmd.Flags().GetString("namespace")

		// Flags cloud
		cloudProvider, _ := cmd.Flags().GetString("cloud-provider")
		cloudRegion, _ := cmd.Flags().GetString("cloud-region")
		cloudCluster, _ := cmd.Flags().GetString("cloud-cluster")

		// Validação do cloud provider
		var cloudCfg *ybycontext.CloudConfig
		if cloudProvider != "" {
			validProviders := map[string]bool{"aws": true, "azure": true, "gcp": true}
			if !validProviders[cloudProvider] {
				return errors.New(errors.ErrCodeValidation,
					fmt.Sprintf("Cloud provider inválido: '%s'. Valores aceitos: aws, azure, gcp", cloudProvider))
			}

			runner := &shared.RealRunner{}
			provider := cloud.GetProvider(runner, cloudProvider)
			if provider == nil {
				return errors.New(errors.ErrCodeValidation,
					fmt.Sprintf("Provider '%s' não está registrado", cloudProvider))
			}

			// Configurar kubeconfig se cluster especificado
			if cloudCluster != "" {
				clusterInfo := cloud.ClusterInfo{
					Name:     cloudCluster,
					Region:   cloudRegion,
					Provider: cloudProvider,
				}
				ctx := gocontext.Background()
				if err := provider.ConfigureKubeconfig(ctx, clusterInfo); err != nil {
					fmt.Printf("⚠️  Falha ao configurar kubeconfig para cluster '%s': %v\n", cloudCluster, err)
				} else {
					fmt.Printf("   Kubeconfig configurado para cluster '%s'\n", cloudCluster)
				}
			}

			cloudCfg = &ybycontext.CloudConfig{
				Provider: cloudProvider,
				Region:   cloudRegion,
				Cluster:  cloudCluster,
			}
		}

		// Gera values estruturados a partir do project manifest, se disponível
		var valuesContent string
		if manifest, err := scaffold.LoadProjectManifest(infraRoot); err == nil {
			bpCtx := scaffold.ManifestToContext(manifest)
			valuesContent = scaffold.RenderEnvironmentValues(bpCtx, name)
		}

		env := ybycontext.Environment{
			Type:        envType,
			Description: description,
			KubeContext: kubeContext,
			Namespace:   namespace,
			Cloud:       cloudCfg,
		}

		mgr := ybycontext.NewManager(infraRoot)
		if err := mgr.AddEnvironment(name, env, valuesContent); err != nil {
			return errors.Wrap(err, errors.ErrCodeConfig, "Falha ao criar ambiente")
		}

		fmt.Printf("✅ Ambiente '%s' criado com sucesso!\n", name)
		fmt.Printf("   Arquivo de configuração: config/values-%s.yaml\n", name)
		if cloudCfg != nil {
			fmt.Printf("   Cloud provider: %s\n", cloudCfg.Provider)
			if cloudCfg.Region != "" {
				fmt.Printf("   Região: %s\n", cloudCfg.Region)
			}
			if cloudCfg.Cluster != "" {
				fmt.Printf("   Cluster: %s\n", cloudCfg.Cluster)
			}
		}

		// Validação de integridade após criação
		warnings, err := mgr.ValidateIntegrity()
		if err == nil && len(warnings) > 0 {
			fmt.Println("⚠️  Problemas encontrados:")
			for _, w := range warnings {
				fmt.Printf("   - %s\n", w)
			}
		}

		return nil
	},
}

// env check
var envCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Valida integridade dos ambientes configurados",
	RunE: func(cmd *cobra.Command, args []string) error {
		infraRoot, err := FindInfraRoot()
		if err != nil {
			infraRoot = "."
		}
		mgr := ybycontext.NewManager(infraRoot)

		warnings, err := mgr.ValidateIntegrity()
		if err != nil {
			return errors.Wrap(err, errors.ErrCodeConfig, "Falha ao validar integridade dos ambientes")
		}

		if len(warnings) == 0 {
			fmt.Println("✅ Todos os ambientes estão íntegros.")
		} else {
			fmt.Println("⚠️  Problemas encontrados:")
			for _, w := range warnings {
				fmt.Printf("   - %s\n", w)
			}
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(envCmd)
	envCmd.AddCommand(envListCmd)
	envCmd.AddCommand(envUseCmd)
	envCmd.AddCommand(envShowCmd)
	envCmd.AddCommand(envCreateCmd)
	envCmd.AddCommand(envCheckCmd)

	envCreateCmd.Flags().String("type", "", "Tipo do ambiente (local/remote)")
	envCreateCmd.Flags().String("description", "", "Descrição do ambiente")
	envCreateCmd.Flags().String("kube-context", "", "Contexto kubectl associado ao ambiente")
	envCreateCmd.Flags().String("namespace", "", "Namespace padrão do ambiente")
	envCreateCmd.Flags().String("cloud-provider", "", "Cloud provider (aws, azure, gcp)")
	envCreateCmd.Flags().String("cloud-region", "", "Região do cloud provider")
	envCreateCmd.Flags().String("cloud-cluster", "", "Nome do cluster cloud")
}
