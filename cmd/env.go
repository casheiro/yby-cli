/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	"fmt"

	"github.com/casheiro/yby-cli/pkg/context"
	"github.com/casheiro/yby-cli/pkg/errors"
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
		mgr := context.NewManager(infraRoot)
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
		mgr := context.NewManager(infraRoot)

		if err := mgr.SetCurrent(name); err != nil {
			return errors.Wrap(err, errors.ErrCodeConfig, "Falha ao definir ambiente ativo")
		}

		fmt.Printf("✅ Contexto alterado para '%s'\n", name)
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
		mgr := context.NewManager(infraRoot)
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
			// Fallback or error? For create, maybe fallback to "." is okay?
			// But consistency suggests we should know where we are.
			// Let's print warning and use "." or just fail if strict.
			// The original code used ".", so let's default to "." if not found,
			// BUT if we want P5 support, we ideally want to find it.
			infraRoot = "."
		}
		mgr := context.NewManager(infraRoot)
		if err := mgr.AddEnvironment(name, envType, description); err != nil {
			return errors.Wrap(err, errors.ErrCodeConfig, "Falha ao criar ambiente")
		}

		fmt.Printf("✅ Ambiente '%s' criado com sucesso!\n", name)
		fmt.Printf("   Arquivo de configuração: config/values-%s.yaml\n", name)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(envCmd)
	envCmd.AddCommand(envListCmd)
	envCmd.AddCommand(envUseCmd)
	envCmd.AddCommand(envShowCmd)
	envCmd.AddCommand(envCreateCmd)

	envCreateCmd.Flags().String("type", "", "Tipo do ambiente (local/remote)")
	envCreateCmd.Flags().String("description", "", "Descrição do ambiente")
}
