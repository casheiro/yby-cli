/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Verifica status do cluster e apps",
	Long: `Exibe informações sobre os nós do cluster, pods do Argo CD e Ingresses configurados.
Equivalente ao antigo 'make status'.`,
	Example: `  yby status`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(titleStyle.Render("📊 Status do Cluster"))
		fmt.Println("---------------------------------------")

		if _, err := lookPath("kubectl"); err != nil {
			fmt.Println(crossStyle.Render("kubectl não encontrado"))
			return
		}

		// Nodes
		fmt.Println(headerStyle.Render("🖥️  Nodes"))
		out, err := execCommand("kubectl", "get", "nodes").CombinedOutput()
		if err != nil {
			fmt.Println(crossStyle.Render("Erro ao obter nodes (Cluster rodando?)"))
			fmt.Println(grayStyle.Render(string(out)))
		} else {
			fmt.Println(strings.TrimSpace(string(out)))
		}

		// ArgoCD Pods
		fmt.Println("")
		fmt.Println(headerStyle.Render("🐙 Argo CD Pods"))
		out, err = execCommand("kubectl", "get", "pods", "-n", "argocd").CombinedOutput()
		if err == nil {
			fmt.Println(strings.TrimSpace(string(out)))
		} else {
			fmt.Println(warningStyle.Render("Namespace argocd não encontrado ou vazio."))
		}

		// Ingress
		fmt.Println("")
		fmt.Println(headerStyle.Render("🔗 Ingresses"))
		out, err = execCommand("kubectl", "get", "ingress", "-A").CombinedOutput()
		if err == nil {
			if len(out) > 0 {
				fmt.Println(strings.TrimSpace(string(out)))
			} else {
				fmt.Println("Nenhum ingress encontrado.")
			}
		}

		// KEDA ScaledObjects
		fmt.Println("")
		fmt.Println(headerStyle.Render("⚡ Autoscaling (KEDA)"))
		out, err = execCommand("kubectl", "get", "scaledobjects", "-A").CombinedOutput()
		if err == nil {
			if len(out) > 0 {
				fmt.Println(strings.TrimSpace(string(out)))
			} else {
				fmt.Println("Nenhum ScaledObject encontrado (KEDA ativo mas sem regras).")
			}
		} else {
			fmt.Println(warningStyle.Render("KEDA não detectado (CRDs ausentes?)"))
		}

		// Kepler Stats
		fmt.Println("")
		fmt.Println(headerStyle.Render("🍃 Eficiência Energética (Kepler)"))
		out, err = execCommand("kubectl", "get", "pods", "-n", "kepler", "-l", "app.kubernetes.io/name=kepler").CombinedOutput()
		if err == nil && len(out) > 0 {
			if strings.Contains(string(out), "Running") {
				fmt.Println(checkStyle.Render("Sensor Kepler ATIVO e monitorando o cluster."))
			} else {
				fmt.Println(warningStyle.Render("Sensor Kepler instalado mas não está 'Running'."))
			}
		} else {
			fmt.Println(crossStyle.Render("Sensor Kepler não encontrado no namespace 'kepler'."))
		}
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
