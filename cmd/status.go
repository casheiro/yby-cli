/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	"bytes"
	"fmt"
	"log/slog"
	"strings"

	"github.com/casheiro/yby-cli/pkg/errors"
	"github.com/spf13/cobra"
)

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Verifica status do cluster e apps",
	Long: `Exibe informações sobre os nós do cluster, pods do Argo CD e Ingresses configurados.
Equivalente ao antigo 'make status'.`,
	Example: `  yby status`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println(titleStyle.Render("📊 Status do Cluster"))
		fmt.Println("---------------------------------------")

		if _, err := lookPath("kubectl"); err != nil {
			return errors.New(errors.ErrCodeCmdNotFound, "kubectl não encontrado")
		}

		// Nodes
		fmt.Println(headerStyle.Render("🖥️  Nodes"))
		c := execCommand("kubectl", "get", "nodes")
		var stdout, stderr bytes.Buffer
		c.Stdout = &stdout
		c.Stderr = &stderr
		err := c.Run()
		out := stdout.Bytes()
		if err != nil {
			fmt.Println(crossStyle.Render("Erro ao obter nodes (Cluster rodando?)"))
			fmt.Println(grayStyle.Render(string(out)))
			if stderr.Len() > 0 {
				slog.Debug("kubectl stderr", "output", stderr.String())
			}
		} else {
			fmt.Println(strings.TrimSpace(string(out)))
		}

		// ArgoCD Pods
		fmt.Println("")
		fmt.Println(headerStyle.Render("🐙 Argo CD Pods"))
		c = execCommand("kubectl", "get", "pods", "-n", "argocd")
		stdout.Reset()
		stderr.Reset()
		c.Stdout = &stdout
		c.Stderr = &stderr
		err = c.Run()
		out = stdout.Bytes()
		if err == nil {
			fmt.Println(strings.TrimSpace(string(out)))
		} else {
			fmt.Println(warningStyle.Render("Namespace argocd não encontrado ou vazio."))
			if stderr.Len() > 0 {
				slog.Debug("kubectl stderr", "output", stderr.String())
			}
		}

		// Ingress
		fmt.Println("")
		fmt.Println(headerStyle.Render("🔗 Ingresses"))
		c = execCommand("kubectl", "get", "ingress", "-A")
		stdout.Reset()
		stderr.Reset()
		c.Stdout = &stdout
		c.Stderr = &stderr
		err = c.Run()
		out = stdout.Bytes()
		if err == nil {
			if len(out) > 0 {
				fmt.Println(strings.TrimSpace(string(out)))
			} else {
				fmt.Println("Nenhum ingress encontrado.")
			}
		} else if stderr.Len() > 0 {
			slog.Debug("kubectl stderr", "output", stderr.String())
		}

		// KEDA ScaledObjects
		fmt.Println("")
		fmt.Println(headerStyle.Render("⚡ Autoscaling (KEDA)"))
		c = execCommand("kubectl", "get", "scaledobjects", "-A")
		stdout.Reset()
		stderr.Reset()
		c.Stdout = &stdout
		c.Stderr = &stderr
		err = c.Run()
		out = stdout.Bytes()
		if err == nil {
			if len(out) > 0 {
				fmt.Println(strings.TrimSpace(string(out)))
			} else {
				fmt.Println("Nenhum ScaledObject encontrado (KEDA ativo mas sem regras).")
			}
		} else {
			fmt.Println(warningStyle.Render("KEDA não detectado (CRDs ausentes?)"))
			if stderr.Len() > 0 {
				slog.Debug("kubectl stderr", "output", stderr.String())
			}
		}

		// Kepler Stats
		fmt.Println("")
		fmt.Println(headerStyle.Render("🍃 Eficiência Energética (Kepler)"))
		c = execCommand("kubectl", "get", "pods", "-n", "kepler", "-l", "app.kubernetes.io/name=kepler")
		stdout.Reset()
		stderr.Reset()
		c.Stdout = &stdout
		c.Stderr = &stderr
		err = c.Run()
		out = stdout.Bytes()
		if err == nil && len(out) > 0 {
			if strings.Contains(string(out), "Running") {
				fmt.Println(checkStyle.Render("Sensor Kepler ATIVO e monitorando o cluster."))
			} else {
				fmt.Println(warningStyle.Render("Sensor Kepler instalado mas não está 'Running'."))
			}
		} else {
			fmt.Println(crossStyle.Render("Sensor Kepler não encontrado no namespace 'kepler'."))
			if err != nil && stderr.Len() > 0 {
				slog.Debug("kubectl stderr", "output", stderr.String())
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
