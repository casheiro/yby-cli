/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	"fmt"
	"os"

	"github.com/casheiro/yby-cli/pkg/context"
	"github.com/spf13/cobra"
)

var envCmd = &cobra.Command{
	Use:   "env",
	Short: "Gerencia ambientes e contextos (dev, staging, prod)",
}

// env list
var envListCmd = &cobra.Command{
	Use:   "list",
	Short: "Lista os ambientes disponíveis",
	Run: func(cmd *cobra.Command, args []string) {
		mgr := context.NewManager(".")
		manifest, err := mgr.LoadManifest()
		if err != nil {
			fmt.Println("❌", err)
			return
		}

		fmt.Println("Ambientes disponíveis:")
		for name, env := range manifest.Environments {
			prefix := "  "
			if name == manifest.Current {
				prefix = "* "
			}
			fmt.Printf("%s%s (%s): %s\n", prefix, name, env.Type, env.Description)
		}
	},
}

// env use <name>
var envUseCmd = &cobra.Command{
	Use:   "use [name]",
	Short: "Define o ambiente ativo",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		mgr := context.NewManager(".")

		if err := mgr.SetCurrent(name); err != nil {
			fmt.Println("❌", err)
			os.Exit(1)
		}

		fmt.Printf("✅ Contexto alterado para '%s'\n", name)
	},
}

// env show
var envShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Mostra detalhes do ambiente atual",
	Run: func(cmd *cobra.Command, args []string) {
		mgr := context.NewManager(".")
		name, env, err := mgr.GetCurrent()
		if err != nil {
			fmt.Println("❌", err)
			return
		}

		fmt.Printf("Ambiente Ativo: %s\n", name)
		fmt.Printf("Tipo: %s\n", env.Type)
		fmt.Printf("Values: %s\n", env.Values)
		if env.URL != "" {
			fmt.Printf("URL: %s\n", env.URL)
		}
	},
}

// env create
var envCreateCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Cria um novo ambiente e gera values correspondente",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
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
				return
			}
		}
		if envType == "" {
			envType = "remote" // default
			// Could add prompt here
		}
		if description == "" {
			description = fmt.Sprintf("Environment %s", name)
		}

		mgr := context.NewManager(".")
		if err := mgr.AddEnvironment(name, envType, description); err != nil {
			fmt.Println("❌", err)
			os.Exit(1)
		}

		fmt.Printf("✅ Ambiente '%s' criado com sucesso!\n", name)
		fmt.Printf("   Arquivo de configuração: config/values-%s.yaml\n", name)
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
