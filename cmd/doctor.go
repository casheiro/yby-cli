/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	"fmt"
	"os/exec"

	"github.com/spf13/cobra"
)

// doctorCmd represents the doctor command
var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Verifica dependências e saúde do ambiente",
	Long: `Verifica se as ferramentas necessárias (kubectl, helm, kubeseal) estão instaladas
e se há conexão com o cluster Kubernetes configurado.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(titleStyle.Render("🩺  Yby Doctor - Verificação de Saúde"))
		fmt.Println("----------------------------------------")

		fmt.Println(headerStyle.Render("🛠️  Ferramentas Essenciais"))
		checkTool("kubectl")
		checkTool("helm")
		checkTool("kubeseal")
		checkTool("argocd")
		checkTool("git")
		checkTool("direnv")

		fmt.Println(headerStyle.Render("🌐 Conectividade"))
		checkClusterConnection()

		fmt.Println(headerStyle.Render("🏥 Integridade da Plataforma (CRDs)"))
		checkCRD("servicemonitors.monitoring.coreos.com", "Prometheus Operator")
		checkCRD("clusterissuers.cert-manager.io", "Cert-Manager")
		checkCRD("scaledobjects.keda.sh", "KEDA")
	},
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}

func checkTool(name string) {
	path, err := exec.LookPath(name)
	if err != nil {
		fmt.Printf("%s %-10s: %s\n", crossStyle.String(), name, grayStyle.Render("Não encontrado"))
	} else {
		fmt.Printf("%s %-10s: %s\n", checkStyle.String(), name, grayStyle.Render(path))
	}
}

func checkClusterConnection() {
	fmt.Print(stepStyle.Render("🔄 Testando conexão com cluster... "))
	cmd := exec.Command("kubectl", "--insecure-skip-tls-verify", "get", "nodes")
	if err := cmd.Run(); err != nil {
		fmt.Printf("\n%s Falha ao conectar\n", crossStyle.String())
		fmt.Println(warningStyle.Render("   Dica: Verifique seu KUBECONFIG ou se o cluster está rodando."))
	} else {
		fmt.Printf("%s\n", checkStyle.String())
	}
}

func checkCRD(crdName, readableName string) {
	err := exec.Command("kubectl", "get", "crd", crdName).Run()
	if err != nil {
		fmt.Printf("%s %-25s: %s\n", crossStyle.String(), readableName, warningStyle.Render("Ausente (CRD não instalado)"))
	} else {
		fmt.Printf("%s %-25s: %s\n", checkStyle.String(), readableName, grayStyle.Render("Instalado"))
	}
}
