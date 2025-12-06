package cmd

import (
	"fmt"
	"os"

	"github.com/casheiro/yby/cli/pkg/config"
	"github.com/casheiro/yby/cli/pkg/context"
	"github.com/spf13/cobra"
)

var contextCmd = &cobra.Command{
	Use:   "context",
	Short: "Gerencia os contextos de execução (ambientes)",
	Long:  `Permite listar, selecionar e visualizar contextos (ex: local, staging, prod).`,
}

var listContextCmd = &cobra.Command{
	Use:   "list",
	Short: "Lista todos os contextos disponíveis",
	Run: func(cmd *cobra.Command, args []string) {
		manager := context.NewManager(".")
		contexts, err := manager.DetectContexts()
		if err != nil {
			fmt.Printf("Erro detectando contextos: %v\n", err)
			os.Exit(1)
		}

		cfg, _ := config.Load()
		current, _ := manager.ResolveActive("", cfg)

		fmt.Println("Contextos disponíveis:")
		for _, ctx := range contexts {
			prefix := "  "
			if ctx.Name == current {
				prefix = "* "
			}
			fmt.Printf("%s%s (%s)\n", prefix, ctx.Name, ctx.Type)
		}
	},
}

var useContextCmd = &cobra.Command{
	Use:   "use [name]",
	Short: "Define o contexto atual (salva localmente)",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		manager := context.NewManager(".")
		contexts, _ := manager.DetectContexts()

		valid := false
		for _, ctx := range contexts {
			if ctx.Name == name {
				valid = true
				break
			}
		}

		if !valid {
			fmt.Printf("❌ Contexto '%s' não encontrado.\n", name)
			// check if file exists despite detection to guide user
			if name != "local" && name != "default" {
				fmt.Printf("Dica: Crie um arquivo .env.%s para habilitar este contexto.\n", name)
			}
			os.Exit(1)
		}

		cfg, err := config.Load()
		if err != nil {
			// If error is just parsing, maybe init new
			cfg = &config.Config{}
		}

		cfg.CurrentContext = name
		if err := cfg.Save(); err != nil {
			fmt.Printf("❌ Erro ao salvar estado local: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✅ Contexto ativo alterado para: %s\n", name)
	},
}

var showContextCmd = &cobra.Command{
	Use:   "show",
	Short: "Mostra o contexto ativo atual",
	Run: func(cmd *cobra.Command, args []string) {
		manager := context.NewManager(".")
		cfg, _ := config.Load()
		// We pass empty flag string as we want to see what is configured,
		// but arguably users might want to see what WOULD be used if they passed a flag?
		// usually 'show' shows persistent state or resolved state.
		// Let's show resolved state assuming no flags for now, but contextCmd doesn't know about root flags yet easily without global vars.
		// We can access global `contextFlag` if we export it or put it in a shared place, but simpler:

		current, _ := manager.ResolveActive("", cfg)
		fmt.Println(current)
	},
}

func init() {
	rootCmd.AddCommand(contextCmd)
	contextCmd.AddCommand(listContextCmd)
	contextCmd.AddCommand(useContextCmd)
	contextCmd.AddCommand(showContextCmd)
}
