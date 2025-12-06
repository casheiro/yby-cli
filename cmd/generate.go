/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	"github.com/spf13/cobra"
)

// generateCmd represents the generate command
var generateCmd = &cobra.Command{
	Use:     "generate",
	Short:   "Gerar recursos e manifestos (alias: gen)",
	Aliases: []string{"gen"},
	Long:    `Utilitário para gerar manifestos Kubernetes e outros recursos.`,
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}

func init() {
	rootCmd.AddCommand(generateCmd)
}
