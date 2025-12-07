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
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Exibe informações de versão do Yby CLI",
	Long:  `Mostra a versão atual, hash do commit, data de build e informações do sistema.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(titleStyle.Render("🚀 Yby CLI - Version Info"))
		fmt.Println("---------------------------------------")

		fmt.Printf("%s: %s\n", headerStyle.Render("Versão"), grayStyle.Render(version))
		fmt.Printf("%s: %s\n", headerStyle.Render("Commit"), grayStyle.Render(commit))
		fmt.Printf("%s: %s\n", headerStyle.Render("Data"), grayStyle.Render(date))
		fmt.Printf("%s: %s/%s\n", headerStyle.Render("OS/Arch"), grayStyle.Render(runtime.GOOS), grayStyle.Render(runtime.GOARCH))
		fmt.Println("---------------------------------------")
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
