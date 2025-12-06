package cmd

import (
	"fmt"
	"os"

	"github.com/casheiro/yby-cli/pkg/config"
	"github.com/casheiro/yby-cli/pkg/context"
	"github.com/spf13/cobra"
)

var contextFlag string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "cli",
	Short: "Yby - Zero-Touch Kubernetes Automation",
	Long: `Yby CLI: Ferramenta oficial para automação e gerenciamento de clusters
seguindo os princípios GitOps.`,
	PersistentPreRun: initConfig,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&contextFlag, "context", "c", "", "Define o contexto de execução (ex: local, staging, prod)")
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// initConfig reads in config file and ENV variables if set.
func initConfig(cmd *cobra.Command, args []string) {
	// Initialize managers
	// We assume we are running from project root for now
	wd, _ := os.Getwd()
	ctxManager := context.NewManager(wd)

	// Load config (.ybyrc)
	cfg, err := config.Load()
	if err != nil {
		// Silent error on load, effectively uses defaults
		cfg = &config.Config{}
	}

	// Resolve Active Context
	activeContext, err := ctxManager.ResolveActive(contextFlag, cfg)
	if err != nil {
		fmt.Printf("Erro resolvendo contexto: %v\n", err)
		os.Exit(1)
	}

	// Load Environment (Strict Isolation)
	if err := ctxManager.LoadContext(activeContext); err != nil {
		fmt.Printf("❌ Erro carregando contexto '%s': %v\n", activeContext, err)
		os.Exit(1)
	}

	// Optional: Feedback to user if verbose?
	// fmt.Printf("(Context: %s)\n", activeContext)
}
