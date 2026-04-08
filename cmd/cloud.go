/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	"github.com/spf13/cobra"
)

var cloudCmd = &cobra.Command{
	Use:   "cloud",
	Short: "Gerencia integrações com provedores cloud (AWS, Azure, GCP)",
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}

func init() {
	rootCmd.AddCommand(cloudCmd)
}
