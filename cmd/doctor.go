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

// doctorCmd represents the doctor command
var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Verifica dependências e saúde do ambiente",
	Long: `Verifica se as ferramentas necessárias (kubectl, helm, kubeseal) estão instaladas
e se há conexão com o cluster Kubernetes configurado.`,
	Example: `  yby doctor`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(titleStyle.Render("🩺  Yby Doctor - Verificação de Saúde"))
		fmt.Println("----------------------------------------")

		fmt.Println(headerStyle.Render("💻 Recursos do Sistema (Local)"))
		checkSystemResources()

		fmt.Println(headerStyle.Render("🛠️  Ferramentas Essenciais"))
		checkTool("kubectl")
		checkTool("helm")
		checkTool("kubeseal")
		checkTool("argocd")
		checkTool("git")
		checkTool("direnv")
		checkDockerPermissions()

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

func checkSystemResources() {
	// Simple check for Linux/Mac using common commands
	// Memory
	cmd := exec.Command("grep", "MemTotal", "/proc/meminfo")
	out, err := cmd.Output()
	if err == nil {
		// Linux
		fmt.Printf("%s Memória (Linux): %s", checkStyle.String(), strings.TrimSpace(strings.Replace(string(out), "MemTotal:", "", 1)))
	} else {
		// Mac/Other fallback
		fmt.Printf("%s Verificação de memória detalhada ignorada (OS não Linux)\n", stepStyle.String())
	}
}

func checkDockerPermissions() {
	err := exec.Command("docker", "info").Run()
	if err != nil {
		fmt.Printf("%s %-10s: %s\n", crossStyle.String(), "docker", warningStyle.Render("Erro de permissão ou não rodando (tente 'sudo' ou adicione user ao grupo docker)"))
	} else {
		fmt.Printf("%s %-10s: %s\n", checkStyle.String(), "docker", grayStyle.Render("Daemon acessível"))
	}
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
