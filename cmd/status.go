/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Verifica status do cluster e apps",
	Long: `Exibe informações sobre os nós do cluster, pods do Argo CD e Ingresses configurados.
Equivalente ao antigo 'make status'.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(titleStyle.Render("📊 Status do Cluster"))
		fmt.Println("---------------------------------------")

		if _, err := exec.LookPath("kubectl"); err != nil {
			fmt.Println(crossStyle.Render("kubectl não encontrado"))
			return
		}

		// Nodes
		fmt.Println(headerStyle.Render("🖥️  Nodes"))
		out, err := exec.Command("kubectl", "get", "nodes").CombinedOutput()
		if err != nil {
			fmt.Println(crossStyle.Render("Erro ao obter nodes (Cluster rodando?)"))
			fmt.Println(grayStyle.Render(string(out)))
		} else {
			fmt.Println(strings.TrimSpace(string(out)))
		}

		// ArgoCD Pods
		fmt.Println("")
		fmt.Println(headerStyle.Render("🐙 Argo CD Pods"))
		out, err = exec.Command("kubectl", "get", "pods", "-n", "argocd").CombinedOutput()
		if err == nil {
			fmt.Println(strings.TrimSpace(string(out)))
		} else {
			fmt.Println(warningStyle.Render("Namespace argocd não encontrado ou vazio."))
		}

		// Ingress
		fmt.Println("")
		fmt.Println(headerStyle.Render("🔗 Ingresses"))
		out, err = exec.Command("kubectl", "get", "ingress", "-A").CombinedOutput()
		if err == nil {
			if len(out) > 0 {
				fmt.Println(strings.TrimSpace(string(out)))
			} else {
				fmt.Println("Nenhum ingress encontrado.")
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
