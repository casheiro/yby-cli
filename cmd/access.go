/*
Copyright © 2025 Yby Team

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/
package cmd

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

// accessCmd represents the access command
// accessCmd represents the access command
var accessCmd = &cobra.Command{
	Use:   "access",
	Short: "Abre túneis de acesso aos serviços do cluster",
	Long: `Estabelece conexões seguras (port-forward) para os serviços administrativos:
- Argo CD (http://localhost:8085)
- Grafana Local (http://localhost:3001)
- Gera token para Headlamp

Você pode especificar um contexto (local/prod) com --context.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("🚀 Iniciando Acesso Unificado ao Cluster...")

		targetContext, _ := cmd.Flags().GetString("context")
		if targetContext == "" {
			var err error
			targetContext, err = getKubectlContext()
			if err != nil {
				fmt.Printf("❌ Erro ao detectar contexto atual: %v\n", err)
				return
			}
			fmt.Printf("📍 Contexto: %s (detectado automaticamente)\n", targetContext)
		} else {
			fmt.Printf("📍 Contexto: %s (definido via flag)\n", targetContext)
		}

		// 2. Obter Senha Argo CD
		argoPwd, err := getArgoPassword(targetContext)
		if err != nil {
			fmt.Printf("⚠️  Não foi possível obter senha do Argo CD: %v\n", err)
			argoPwd = "Erro ao recuperar"
		}

		// 3. Iniciar Port-Forwards
		fmt.Println("🔌 Estabelecendo túneis...")

		// Argo CD
		killPortForward("8085")
		go runPortForward(targetContext, "argocd", "svc/argocd-server", "8085:80")
		fmt.Println("   - Argo CD: http://localhost:8085")

		// Grafana Local (Docker)
		startLocalGrafana()

		fmt.Println("✅ Serviços iniciados em background (goroutines)")
		fmt.Println("")
		fmt.Println("📊 Credenciais:")
		fmt.Printf("   - Argo CD: admin / %s\n", argoPwd)
		fmt.Println("   - Grafana: admin / admin")

		// 4. Token Headlamp
		token, err := getHeadlampToken(targetContext)
		if err == nil {
			fmt.Println("")
			fmt.Println("🔑 Token Headlamp (copie abaixo):")
			fmt.Println(token)
		} else {
			fmt.Printf("⚠️  Erro ao gerar token Headlamp: %v\n", err)
		}

		fmt.Println("")
		fmt.Println("ℹ️  Pressione Ctrl+C para encerrar os túneis...")

		// Manter rodando até Ctrl+C
		select {}
	},
}

func init() {
	rootCmd.AddCommand(accessCmd)
	accessCmd.Flags().StringP("context", "c", "", "Nome do contexto Kubernetes (ex: yby-prod, k3d-yby-local)")
}

func getKubectlContext() (string, error) {
	out, err := exec.Command("kubectl", "config", "current-context").Output()
	return strings.TrimSpace(string(out)), err
}

func getArgoPassword(context string) (string, error) {
	// kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}"
	cmd := exec.Command("kubectl", "--context", context, "--insecure-skip-tls-verify", "-n", "argocd", "get", "secret", "argocd-initial-admin-secret", "-o", "jsonpath={.data.password}")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	decoded, err := base64.StdEncoding.DecodeString(string(out))
	return string(decoded), err
}

func runPortForward(context, namespace, resource, ports string) {
	// kubectl -n argocd port-forward svc/argocd-server 8085:80
	cmd := exec.Command("kubectl", "--context", context, "--insecure-skip-tls-verify", "-n", namespace, "port-forward", resource, ports)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		fmt.Printf("❌ Falha no port-forward %s: %v\n", resource, err)
	}
}

func killPortForward(port string) {
	exec.Command("pkill", "-f", fmt.Sprintf("port-forward.*%s", port)).Run()
}

func getHeadlampToken(context string) (string, error) {
	// kubectl create token admin-user -n kube-system --duration=24h
	cmd := exec.Command("kubectl", "--context", context, "--insecure-skip-tls-verify", "create", "token", "admin-user", "-n", "kube-system", "--duration=24h")
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

func startLocalGrafana() {
	// Tenta rodar o script existente. Assumindo execução da raiz do projeto.
	cmd := exec.Command("./scripts/start-local-grafana.sh")
	if _, err := os.Stat("./scripts/start-local-grafana.sh"); os.IsNotExist(err) {
		cmd = exec.Command("../scripts/start-local-grafana.sh")
	}

	if err := cmd.Start(); err != nil {
		fmt.Println("   - Grafana: Falha ao iniciar (verifique se o Docker está rodando)")
	} else {
		fmt.Println("   - Grafana: http://localhost:3001 (Iniciando container...)")
	}
}
