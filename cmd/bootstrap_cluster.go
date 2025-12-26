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

		// 0. Resolve Infra Root
		root, err := FindInfraRoot()
		if err != nil {
			fmt.Println(warningStyle.Render("⚠️  Raiz da infraestrutura não encontrada (.yby/). Assumindo diretório atual '.'"))
			root = "."
		} else {
			fmt.Printf("📂 Infraestrutura detectada em: %s\n", root)
		}

		// Load Version Config from Blueprint (Infra as Data)
		argoVersion := "5.51.6" // Default fallback
		argoChart := "argo/argo-cd"

		// 0. Resolve Config (Blueprint)
		blueprintRepo := getRepoURLFromBlueprint()

		// 1. Pre-checks
		checkEnvVars(blueprintRepo)
		ensureToolsInstalled()

		// 1. Ensure Template Assets (Self-Repair)
		// This must happen before reading blueprint or applying manifests
		// because the blueprint or manifests might be missing themselves.
		repoURL := os.Getenv("GITHUB_REPO")
		if repoURL == "" {
			repoURL = blueprintRepo
		}
		ensureTemplateAssets(repoURL, root)

		blueprintPath := JoinInfra(root, ".yby/blueprint.yaml")
		// Existing code used ".yby/blueprint.yaml". If we are in infra/, that works if .yby is in infra/.
		// Let's verify ensuring assets are relative to CWD.

		if blueprintPath != "" {
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
		}

		// 1. Bootstrap Argo CD & System
		fmt.Println(headerStyle.Render("🌱 Fase 1: Bootstrap do Sistema"))

		fmt.Println(stepStyle.Render("📦 Instalando Argo CD e Argo Workflows..."))
		executeHelmRepoAdd("argo", "https://argoproj.github.io/argo-helm")
		createNamespace("argocd")

		// Helm Upgrade ArgoCD
		// Helm Upgrade ArgoCD
		runCommand("helm", "upgrade", "--install", "argocd", argoChart,
			"--namespace", "argocd",
			"--version", argoVersion,
			"-f", JoinInfra(root, "config/cluster-values.yaml"),
			"--wait", "--timeout", "300s")

		// Argo Workflows & Events (Manifests)
		createNamespace("argo")
		runCommand("kubectl", "apply", "-n", "argo", "-f", JoinInfra(root, "manifests/upstream/argo-workflows.yaml"))

		createNamespace("argo-events")
		runCommand("kubectl", "apply", "-f", JoinInfra(root, "manifests/upstream/argo-events.yaml"))

		fmt.Println(stepStyle.Render("⏳ Aguardando controladores..."))
		waitPodReady("app=workflow-controller", "argo")
		waitPodReady("controller=sensor-controller", "argo-events")

		// 2. Install System Chart
		// 2. Install System Chart
		fmt.Println(stepStyle.Render("⚙️  Instalando Chart System (CRDs e Controllers)..."))
		runCommand("helm", "dependency", "build", JoinInfra(root, "charts/system"))

		// Workaround CRDs
		if _, err := os.Stat(JoinInfra(root, "charts/system/crds")); err == nil {
			runCommand("kubectl", "apply", "-f", JoinInfra(root, "charts/system/crds/"))
		}

		runCommand("helm", "upgrade", "--install", "system", JoinInfra(root, "charts/system"),
			"--namespace", "argocd",
			"--create-namespace",
			"-f", JoinInfra(root, "config/cluster-values.yaml"),
			"--wait", "--timeout", "600s")

		// 3. Secrets
		fmt.Println(headerStyle.Render("🔐 Fase 2: Configuração de Segredos"))
		// 3. Secrets
		fmt.Println(headerStyle.Render("🔐 Fase 2: Configuração de Segredos"))
		configureSecrets(repoURL)

		// 4. Wait for CRDs
		fmt.Println(stepStyle.Render("⏳ Aguardando CRDs críticos..."))
		waitCRD("servicemonitors.monitoring.coreos.com")
		waitCRD("certificates.cert-manager.io")

		// 5. Bootstrap Config (Root App)
		// 5. Bootstrap Config (Root App)
		fmt.Println(headerStyle.Render("🚀 Fase 3: Bootstrap de Configuração"))
		fmt.Println(stepStyle.Render("Applying Root App..."))
		runCommand("kubectl", "apply", "-f", JoinInfra(root, "manifests/argocd/root-app.yaml"))

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

func checkEnvVars(blueprintRepo string) {
	// GITHUB_REPO Strategy: Env > Blueprint > Fail
	if os.Getenv("GITHUB_REPO") == "" {
		if blueprintRepo != "" {
			fmt.Printf("ℹ️  Usando repo do Blueprint: %s\n", blueprintRepo)
			os.Setenv("GITHUB_REPO", blueprintRepo)
		} else {
			fmt.Println(crossStyle.Render("❌ Variável GITHUB_REPO faltando e não encontrada no Blueprint."))
			fmt.Println(warningStyle.Render("Defina no .env, exporte ou execute 'yby init' novamente."))
			os.Exit(1)
		}
	}

	// GITHUB_TOKEN Strategy: Env > Check Context > Warn/Fail
	if os.Getenv("GITHUB_TOKEN") == "" {
		// Looser check: if running locally, we tolerate missing token
		// contextFlag is global from root.go
		isLocal := (contextFlag == "local" || os.Getenv("YBY_ENV") == "local")

		if isLocal {
			fmt.Println(warningStyle.Render("⚠️  GITHUB_TOKEN não definido. Operando em modo Local Mirror (sem autenticação upstream)."))
			fmt.Println("   (Se seu repositório for privado, o ArgoCD pode falhar ao sincronizar)")
		} else {
			fmt.Println(crossStyle.Render("❌ Variável GITHUB_TOKEN faltando."))
			fmt.Println(warningStyle.Render("Necessário para bootstrap em ambientes remotos."))
			os.Exit(1)
		}
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

func configureSecrets(repoURL string) {
	// Argo CD Repo Secret
	// repo := os.Getenv("GITHUB_REPO") // Already passed as arg
	token := os.Getenv("GITHUB_TOKEN")

	if token == "" {
		fmt.Println(itemStyle.Render("Skipping Secrets Config (Token not provided)..."))
		return
	}
	fmt.Println(itemStyle.Render("Configurando Argo CD Repo Secret..."))
	cmd := exec.Command("kubectl", "create", "secret", "generic", "argocd-repo-creds", "-n", "argocd",
		fmt.Sprintf("--from-literal=url=%s", repoURL),
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

// ensureTemplateAssets checks and restores critical files and directories
// ensureTemplateAssets checks and restores critical files and directories
func ensureTemplateAssets(repoURL, root string) {
	fmt.Println(headerStyle.Render("🛠️  Auto-Repair: Verificando integridade do projeto..."))

	baseUrl := "https://raw.githubusercontent.com/casheiro/yby-template/main"
	// repoURL is passed as arg now

	// 1. Critical Files (Download & Template)
	type Manifest struct {
		Url          string
		Path         string
		Replacements map[string]string
	}

	// 0. Detect Git Prefix for Monorepo/Subdir Support (infra/ integration)
	gitPrefix := getGitPrefix()
	if gitPrefix != "" {
		fmt.Printf("📂 Detectado subdiretório Git: %s. Ajustando paths do ArgoCD...\n", gitPrefix)
	}

	manifests := []Manifest{
		{
			Url:  baseUrl + "/manifests/upstream/argo-workflows.yaml",
			Path: JoinInfra(root, "manifests/upstream/argo-workflows.yaml"),
		},
		{
			Url:  baseUrl + "/manifests/upstream/argo-events.yaml",
			Path: JoinInfra(root, "manifests/upstream/argo-events.yaml"),
		},
		{
			Url:  baseUrl + "/manifests/argocd/root-app.yaml",
			Path: JoinInfra(root, "manifests/argocd/root-app.yaml"),
			Replacements: map[string]string{
				"https://github.com/my-user/yby-template": repoURL,
				"path: charts/bootstrap":                  fmt.Sprintf("path: %scharts/bootstrap", gitPrefix),
			},
		},
		{
			Url:  baseUrl + "/manifests/projects/yby-project.yaml",
			Path: JoinInfra(root, "manifests/projects/yby-project.yaml"),
			Replacements: map[string]string{
				// Add the current repo to the whitelist by appending it after the generic one
				"  - 'https://github.com/*/yby'": fmt.Sprintf("  - 'https://github.com/*/yby'\n  - '%s'", repoURL),
			},
		},
	}

	for _, m := range manifests {
		if _, err := os.Stat(m.Path); os.IsNotExist(err) {
			downloadAndTemplate(m.Url, m.Path, m.Replacements)
		}
	}

	// 2. Critical Directories (Clone & Restore)
	dirs := []string{
		JoinInfra(root, "charts/system"),
		JoinInfra(root, "templates/workflows"),
	}

	missingDirs := []string{}
	for _, d := range dirs {
		if _, err := os.Stat(d); os.IsNotExist(err) {
			missingDirs = append(missingDirs, d)
		}
	}

	if len(missingDirs) > 0 {
		fmt.Printf("%s Diretórios críticos faltando: %s. Iniciando restauração via clone...\n", warningStyle.Render("⚠️"), strings.Join(missingDirs, ", "))
		restoreAssetsFromClone(missingDirs)
	} else {
		fmt.Println(checkStyle.Render("✅ Integridade verificada."))
	}
}

func downloadAndTemplate(url, destPath string, replacements map[string]string) {
	fmt.Printf("%s Baixando e Configurando %s...\n", stepStyle.Render("⬇️"), destPath)

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

	// Read Body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("%s Erro ao ler corpo do arquivo %s: %v\n", crossStyle.String(), destPath, err)
		os.Exit(1)
	}
	content := string(bodyBytes)

	// Apply Replacements
	for old, new := range replacements {
		content = strings.ReplaceAll(content, old, new)
	}

	// Write file
	if err := os.WriteFile(destPath, []byte(content), 0644); err != nil {
		fmt.Printf("%s Erro ao salvar arquivo %s: %v\n", crossStyle.String(), destPath, err)
		os.Exit(1)
	}
}

func restoreAssetsFromClone(targets []string) {
	tempDir, err := os.MkdirTemp("", "yby-restore")
	if err != nil {
		fmt.Printf("%s Erro ao criar temp dir: %v\n", crossStyle.String(), err)
		return
	}
	defer os.RemoveAll(tempDir)

	fmt.Printf("%s Clonando template para recuperação (pode levar alguns segundos)...\n", stepStyle.Render("⏳"))

	// Clone depth 1
	cmd := exec.Command("git", "clone", "--depth", "1", "https://github.com/casheiro/yby-template.git", tempDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Printf("%s Erro ao clonar template: %v\nOutput: %s\n", crossStyle.String(), err, string(out))
		return
	}

	for _, target := range targets {
		srcPath := filepath.Join(tempDir, target)
		// Check if it exists in repo
		if _, err := os.Stat(srcPath); os.IsNotExist(err) {
			fmt.Printf("%s Aviso: %s não encontrado no repo de template.\n", warningStyle.String(), target)
			continue
		}

		fmt.Printf("%s Restaurando %s...\n", stepStyle.Render("♻️"), target)
		// Copy dir
		if err := copyDir(srcPath, target); err != nil {
			fmt.Printf("%s Erro ao copiar %s: %v\n", crossStyle.String(), target, err)
		}
	}
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, _ := filepath.Rel(src, path)
		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		// Copy file
		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer srcFile.Close()

		dstFile, err := os.Create(dstPath)
		if err != nil {
			return err
		}
		defer dstFile.Close()

		_, err = io.Copy(dstFile, srcFile)
		return err
	})
}

func getGitPrefix() string {
	cmd := exec.Command("git", "rev-parse", "--show-prefix")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "" // Not a git repo or error, assume root
	}
	return strings.TrimSpace(string(out))
}
func getRepoURLFromBlueprint() string {
	root, err := FindInfraRoot()
	if err != nil {
		return ""
	}
	path := JoinInfra(root, ".yby/blueprint.yaml")

	if data, err := os.ReadFile(path); err == nil {
		var bp Blueprint
		if err := yaml.Unmarshal(data, &bp); err == nil {
			for _, prompt := range bp.Prompts {
				if prompt.ID == "git.repoURL" {
					if val, ok := prompt.Default.(string); ok && val != "" {
						return val
					}
				}
			}
		}
	}
	return ""
}
