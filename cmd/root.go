package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var contextFlag string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "yby",
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
	// If context flag is set, we override using the standard Env Var mechanism
	// capable of being read by pkg/context
	if contextFlag != "" {
		os.Setenv("YBY_ENV", contextFlag)
	}
}
