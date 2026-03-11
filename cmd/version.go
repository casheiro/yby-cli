/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
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
	RunE: func(cmd *cobra.Command, args []string) error {
		logFormat, _ := cmd.Root().PersistentFlags().GetString("log-format")

		if logFormat == "json" {
			info := map[string]string{
				"version": Version,
				"commit":  commit,
				"date":    date,
				"os":      runtime.GOOS,
				"arch":    runtime.GOARCH,
			}
			return json.NewEncoder(os.Stdout).Encode(info)
		}

		info := fmt.Sprintf("yby version %s", Version)

		if commit != "none" {
			info += fmt.Sprintf(" (%s)", commit)
		}

		if date != "unknown" {
			info += fmt.Sprintf(" compilado em %s", date)
		}

		info += fmt.Sprintf(" [%s/%s]", runtime.GOOS, runtime.GOARCH)

		fmt.Println(info)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
