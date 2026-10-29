/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	"encoding/json"
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
	RunE: func(cmd *cobra.Command, args []string) error {
		logFormat, _ := cmd.Root().PersistentFlags().GetString("log-format")
		v := versaoInstalada()

		if logFormat == "json" {
			info := map[string]string{
				"version": v.Versao,
				"commit":  v.Commit,
				"date":    v.Data,
				"os":      runtime.GOOS,
				"arch":    runtime.GOARCH,
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(info)
		}

		fmt.Fprintln(cmd.OutOrStdout(), linhaVersao(v))
		return nil
	},
}

// linhaVersao monta a linha do yby version. No release: "yby version 5.8.3 (abc1234)
// compilado em <data>"; no go install a data é a do commit, não a do build.
func linhaVersao(v infoVersao) string {
	info := fmt.Sprintf("yby version %s", v.Versao)
	temCommit, temData := v.Commit != "none", v.Data != "unknown"
	switch {
	case v.DoBuildInfo && temCommit && temData:
		info += fmt.Sprintf(" (%s, commit de %s)", v.Commit, v.Data)
	case v.DoBuildInfo && temData:
		info += fmt.Sprintf(" (commit de %s)", v.Data)
	default:
		if temCommit {
			info += fmt.Sprintf(" (%s)", v.Commit)
		}
		if temData {
			info += fmt.Sprintf(" compilado em %s", v.Data)
		}
	}
	return info + fmt.Sprintf(" [%s/%s]", runtime.GOOS, runtime.GOARCH)
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
