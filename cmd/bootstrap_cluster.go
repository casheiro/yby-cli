/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var bootstrapClusterCmd = &cobra.Command{
	Use:   "cluster",
	Short: "Instala a stack GitOps (ArgoCD, Workflows) no cluster conectado",
	Long: `Executa o bootstrap completo do cluster, instalando:
1. Argo CD, Argo Workflows e Argo Events (Infraestrutura)
2. System Charts (CRDs, Cert-Manager, Monitoring)
3. Configuração de Secrets (Git Credentials, Tokens)
4. Aplicação Root (App of Apps) para início do GitOps
5. Versions são lidas de .yby/blueprint.yaml se disponível.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(titleStyle.Render("🚀 Yby Bootstrap - Cluster GitOps"))
		fmt.Println("---------------------------------------")

		// 0. Pre-checks
		checkEnvVars()
		ensureToolsInstalled()

		// Load Version Config from Blueprint (Infra as Data)
		argoVersion := "5.51.6" // Default fallback
		argoChart := "argo/argo-cd"

		blueprintPath := ".yby/blueprint.yaml"
		if data, err := os.ReadFile(blueprintPath); err == nil {
			var bp struct {
				Infrastructure struct {
					Argocd struct {
						Version string `yaml:"version"`
						Chart   string `yaml:"chart"`
					} `yaml:"argocd"`
				} `yaml:"infrastructure"`
			}
			if err := yaml.Unmarshal(data, &bp); err == nil {
				if bp.Infrastructure.Argocd.Version != "" {
					argoVersion = bp.Infrastructure.Argocd.Version
					fmt.Printf("📋 Versão ArgoCD definida no Blueprint: %s\n", argoVersion)
				}
				if bp.Infrastructure.Argocd.Chart != "" {
					argoChart = bp.Infrastructure.Argocd.Chart
				}
			}
		}

		// 1. Bootstrap Argo CD & System
		fmt.Println(headerStyle.Render("🌱 Fase 1: Bootstrap do Sistema"))

		fmt.Println(stepStyle.Render("📦 Instalando Argo CD e Argo Workflows..."))
		executeHelmRepoAdd("argo", "https://argoproj.github.io/argo-helm")
		createNamespace("argocd")

		// Helm Upgrade ArgoCD
		runCommand("helm", "upgrade", "--install", "argocd", argoChart,
			"--namespace", "argocd",
			"--version", argoVersion,
			"-f", "config/cluster-values.yaml",
			"--wait", "--timeout", "300s")

		// Argo Workflows & Events (Manifests)
		downloadManifest("https://raw.githubusercontent.com/casheiro/yby-template/main/manifests/upstream/argo-workflows.yaml", "manifests/upstream/argo-workflows.yaml")
		createNamespace("argo")
		runCommand("kubectl", "apply", "-n", "argo", "-f", "manifests/upstream/argo-workflows.yaml")

		downloadManifest("https://raw.githubusercontent.com/casheiro/yby-template/main/manifests/upstream/argo-events.yaml", "manifests/upstream/argo-events.yaml")
		createNamespace("argo-events")
		runCommand("kubectl", "apply", "-f", "manifests/upstream/argo-events.yaml")

		fmt.Println(stepStyle.Render("⏳ Aguardando controladores..."))
		waitPodReady("app=workflow-controller", "argo")
		waitPodReady("controller=sensor-controller", "argo-events")

		// 2. Install System Chart
		fmt.Println(stepStyle.Render("⚙️  Instalando Chart System (CRDs e Controllers)..."))
		runCommand("helm", "dependency", "build", "charts/system")

		// Workaround CRDs
		if _, err := os.Stat("charts/system/crds"); err == nil {
			runCommand("kubectl", "apply", "-f", "charts/system/crds/")
		}

		runCommand("helm", "upgrade", "--install", "system", "charts/system",
			"--namespace", "argocd",
			"--create-namespace",
			"-f", "config/cluster-values.yaml",
			"--wait", "--timeout", "600s")

		// 3. Secrets
		fmt.Println(headerStyle.Render("🔐 Fase 2: Configuração de Segredos"))
		configureSeconds()

		// 4. Wait for CRDs
		fmt.Println(stepStyle.Render("⏳ Aguardando CRDs críticos..."))
		waitCRD("servicemonitors.monitoring.coreos.com")
		waitCRD("certificates.cert-manager.io")

		// 5. Bootstrap Config (Root App)
		fmt.Println(headerStyle.Render("🚀 Fase 3: Bootstrap de Configuração"))
		fmt.Println(stepStyle.Render("Applying Root App..."))
		runCommand("kubectl", "apply", "-f", "manifests/argocd/root-app.yaml")

		fmt.Println(stepStyle.Render("🔄 Forçando Sync inicial..."))
		time.Sleep(5 * time.Second)
		_ = exec.Command("kubectl", "patch", "application", "root-app", "-n", "argocd", "--type", "merge", "-p", "{\"operation\": {\"sync\": {\"prune\": true}}}").Run()

		fmt.Println("\n" + checkStyle.Render("🎉 Bootstrap do Cluster concluído!"))
		fmt.Println("👉 Execute 'yby access' para acessar os dashboards.")
	},
}

func init() {
	bootstrapCmd.AddCommand(bootstrapClusterCmd)
}

func checkEnvVars() {
	required := []string{"GITHUB_REPO", "GITHUB_TOKEN"}
	missing := []string{}
	for _, v := range required {
		if os.Getenv(v) == "" {
			missing = append(missing, v)
		}
	}
	if len(missing) > 0 {
		fmt.Printf("%s Variáveis de ambiente faltando: %s\n", crossStyle.String(), strings.Join(missing, ", "))
		fmt.Println(warningStyle.Render("Defina-as no arquivo .env ou exporte-as."))
		os.Exit(1)
	}
}

func ensureToolsInstalled() {
	if _, err := exec.LookPath("kubectl"); err != nil {
		fmt.Println(crossStyle.Render("kubectl não encontrado."))
		os.Exit(1)
	}
	if _, err := exec.LookPath("helm"); err != nil {
		fmt.Println(crossStyle.Render("helm não encontrado."))
		os.Exit(1)
	}
}

func executeHelmRepoAdd(name, url string) {
	_ = exec.Command("helm", "repo", "add", name, url).Run()
	_ = exec.Command("helm", "repo", "update", name).Run()
}

func createNamespace(ns string) {
	_ = exec.Command("kubectl", "create", "namespace", ns).Run()
}

func runCommand(name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	fmt.Printf("%s Executando: %s %s\n", grayStyle.Render("Exec >"), name, strings.Join(args, " "))
	if err := cmd.Run(); err != nil {
		fmt.Printf("%s Erro ao executar %s\n", crossStyle.String(), name)
		os.Exit(1)
	}
}

func waitPodReady(label, ns string) {
	cmd := exec.Command("kubectl", "wait", "--for=condition=Ready", "pod", "-l", label, "-n", ns, "--timeout=300s")
	if err := cmd.Run(); err != nil {
		fmt.Printf("%s Timeout aguardando pod %s no namespace %s\n", warningStyle.String(), label, ns)
	}
}

func waitCRD(crdName string) {
	cmd := exec.Command("kubectl", "wait", "--for", "condition=established", "--timeout=60s", "crd/"+crdName)
	if err := cmd.Run(); err != nil {
		fmt.Printf("%s Timeout aguardando CRD %s\n", warningStyle.String(), crdName)
	}
}

func configureSeconds() {
	// Argo CD Repo Secret
	repo := os.Getenv("GITHUB_REPO")
	token := os.Getenv("GITHUB_TOKEN")

	fmt.Println(itemStyle.Render("Configurando Argo CD Repo Secret..."))
	cmd := exec.Command("kubectl", "create", "secret", "generic", "argocd-repo-creds", "-n", "argocd",
		fmt.Sprintf("--from-literal=url=%s", repo),
		fmt.Sprintf("--from-literal=password=%s", token),
		"--from-literal=username=git",
		"--from-literal=type=git",
		"--dry-run=client", "-o", "yaml")

	applyCmd := exec.Command("kubectl", "apply", "-f", "-")
	applyCmd.Stdin, _ = cmd.StdoutPipe()
	_ = cmd.Start()
	_ = applyCmd.Run()
	_ = cmd.Wait()

	_ = exec.Command("kubectl", "label", "secret", "argocd-repo-creds", "-n", "argocd", "argocd.argoproj.io/secret-type=repository", "--overwrite").Run()

	// Github Token for AppSet
	fmt.Println(itemStyle.Render("Configurando Github Token Secret..."))
	cmdToken := exec.Command("kubectl", "create", "secret", "generic", "github-token", "-n", "argocd",
		fmt.Sprintf("--from-literal=token=%s", token),
		"--dry-run=client", "-o", "yaml")

	applyCmdToken := exec.Command("kubectl", "apply", "-f", "-")
	applyCmdToken.Stdin, _ = cmdToken.StdoutPipe()
	_ = cmdToken.Start()
	_ = applyCmdToken.Run()
	_ = cmdToken.Wait()

	// Restore Sealed Secrets Keys
	fmt.Println(itemStyle.Render("Verificando backup de chaves Sealed Secrets..."))
	// We call the restore command internally. It checks for the file itself.
	restoreKeysCmd.Run(restoreKeysCmd, []string{})

	// Webhook Secret
	fmt.Println(itemStyle.Render("Verificando Webhook Secret..."))
	webhookSecret := os.Getenv("WEBHOOK_SECRET")

	// If env var is set, or if we want to auto-generate (the command handles auto-generation if empty arg provided?
	// The command expects [provider] [secret]. If secret is empty/missing, it generates from env?
	// Let's check secrets.go again.
	// secrets.go: "if secretVal == "" { secretVal = os.Getenv("WEBHOOK_SECRET") } ... if "" -> generate random"
	// So we can just call it with "github".

	webhookSecretCmd.Run(webhookSecretCmd, []string{"github", webhookSecret})
}

func downloadManifest(url, destPath string) {
	if _, err := os.Stat(destPath); err == nil {
		return // File exists
	}

	fmt.Printf("%s Baixando %s...\n", stepStyle.Render("⬇️"), destPath)

	// Ensure directory exists
	dir := filepath.Dir(destPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Printf("%s Erro ao criar diretório %s: %v\n", crossStyle.String(), dir, err)
		os.Exit(1)
	}

	// Download
	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("%s Erro ao baixar %s: %v\n", crossStyle.String(), url, err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("%s Erro ao baixar %s: Status %s\n", crossStyle.String(), url, resp.Status)
		os.Exit(1)
	}

	// Create file
	out, err := os.Create(destPath)
	if err != nil {
		fmt.Printf("%s Erro ao criar arquivo %s: %v\n", crossStyle.String(), destPath, err)
		os.Exit(1)
	}
	defer out.Close()

	// Write content
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		fmt.Printf("%s Erro ao salvar arquivo %s: %v\n", crossStyle.String(), destPath, err)
		os.Exit(1)
	}
}
