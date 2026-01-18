/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// These variables are populated by GoReleaser during build
var (
	Version = "dev"
	commit  = "none"
	date    = "unknown"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Exibe informações de versão do Yby CLI",
	Long:  `Mostra a versão atual, hash do commit, data de build e informações do sistema.`,
	Run: func(cmd *cobra.Command, args []string) {
		info := fmt.Sprintf("yby version %s", Version)

		if commit != "none" {
			info += fmt.Sprintf(" (%s)", commit)
		}

		if date != "unknown" {
			info += fmt.Sprintf(" built at %s", date)
		}

		info += fmt.Sprintf(" [%s/%s]", runtime.GOOS, runtime.GOARCH)

		fmt.Println(info)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
