/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/casheiro/yby-cli/pkg/templates"
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
	Example: `  # Bootstrap padrão (lê variáveis GITHUB_REPO e TOKEN do ambiente)
  yby bootstrap cluster

  # Forçar uso do blueprint para versões
  yby bootstrap cluster --context prod`,
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
		blueprintRepo := getRepoURLFromBlueprint(root)

		// 1. Pre-checks
		ensureToolsInstalled()
		checkEnvVars(blueprintRepo)

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

		// Workaround CRDs (ServerSideApply to fix BUG-018/SUG-009)
		if _, err := os.Stat(JoinInfra(root, "charts/system/crds")); err == nil {
			fmt.Println(stepStyle.Render("Applying CRDs (ServerSide)..."))
			runCommand("kubectl", "apply", "--server-side", "--force-conflicts", "-f", JoinInfra(root, "charts/system/crds/"))
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
		configureSecrets(root, repoURL)

		// 4. Wait for CRDs
		fmt.Println(stepStyle.Render("⏳ Aguardando CRDs críticos..."))
		waitCRD("servicemonitors.monitoring.coreos.com")
		waitCRD("certificates.cert-manager.io")

		// 5. Bootstrap Config (Root App)
		// 5. Bootstrap Config (Root App)
		fmt.Println(headerStyle.Render("🚀 Fase 3: Bootstrap de Configuração"))

		// 5.1 Ensure AppProject exists (Fix BUG-006)
		// We explicitly apply the project manifest first so root-app (which belongs to it) doesn't fail
		projectManifest := JoinInfra(root, "manifests/projects/yby-project.yaml")
		if _, err := os.Stat(projectManifest); err == nil {
			fmt.Println(stepStyle.Render("Applying AppProject..."))
			runCommand("kubectl", "apply", "-f", projectManifest)
		}

		fmt.Println(stepStyle.Render("Applying Root App..."))
		runCommand("kubectl", "apply", "-f", JoinInfra(root, "manifests/argocd/root-app.yaml"))

		// 5.2 Patch RepoURL for Local Mode (Fix BUG-009)
		// If we are using internal mirror, we must ensure root-app points to it.
		// The manifest might have the github URL.
		if os.Getenv("YBY_ENV") == "local" || contextFlag == "local" {
			internalRepo := "git://git-server.yby-system.svc:9418/repo.git"
			fmt.Printf("🔄 Patching Root App RepoURL to %s...\n", internalRepo)
			// We use merge patch. source.repoURL
			patch := fmt.Sprintf(`{"spec": {"source": {"repoURL": "%s"}}}`, internalRepo)
			_ = exec.Command("kubectl", "patch", "application", "root-app", "-n", "argocd", "--type", "merge", "-p", patch).Run()
		}

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
	// GITHUB_REPO Strategy: Env > Blueprint > Internal Mirror (Local) > Fail
	repo := os.Getenv("GITHUB_REPO")
	if repo == "" {
		if blueprintRepo != "" {
			fmt.Printf("ℹ️  Usando repo do Blueprint: %s\n", blueprintRepo)
			os.Setenv("GITHUB_REPO", blueprintRepo)
		} else {
			// Check if local
			isLocal := (contextFlag == "local" || os.Getenv("YBY_ENV") == "local")
			if isLocal {
				// Use internal mirror URL
				internalRepo := "git://git-server.yby-system.svc:9418/repo.git"
				fmt.Printf("ℹ️  Ambiente Local detectado sem GITHUB_REPO. Usando Mirror Interno: %s\n", internalRepo)
				os.Setenv("GITHUB_REPO", internalRepo)
			} else {
				fmt.Println(crossStyle.Render("❌ Variável GITHUB_REPO faltando e não encontrada no Blueprint."))
				fmt.Println(warningStyle.Render("Defina no .env, exporte ou execute 'yby init' novamente."))
				os.Exit(1)
			}
		}
	}

	// GITHUB_TOKEN Strategy: Env > Check Context > Warn/Fail
	if os.Getenv("GITHUB_TOKEN") == "" {
		// 1. Looser check: if running locally, we tolerate missing token
		isLocal := (contextFlag == "local" || os.Getenv("YBY_ENV") == "local")

		// 2. Secret Check: If secret already exists in cluster, we can proceed
		// This requires kubectl to be working (ensureToolsInstalled called before)
		secretExists := false
		if err := exec.Command("kubectl", "get", "secret", "argocd-repo-creds", "-n", "argocd").Run(); err == nil {
			secretExists = true
			fmt.Println(checkStyle.Render("✅ Credenciais GitHub encontradas no cluster (Secret: argocd-repo-creds)."))
		}

		if isLocal || secretExists {
			if !secretExists {
				fmt.Println(warningStyle.Render("⚠️  GITHUB_TOKEN não definido. Operando em modo Local Mirror (sem autenticação upstream)."))
				fmt.Println("   (Se seu repositório for privado, o ArgoCD pode falhar ao sincronizar)")
			} else {
				fmt.Println("   Ignorando verificação de token de ambiente.")
			}
		} else {
			fmt.Println(crossStyle.Render("❌ Variável GITHUB_TOKEN faltando."))
			fmt.Println(warningStyle.Render("Necessário para bootstrap em ambientes remotos ou sem credenciais prévias."))
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
	if err := execCommand("helm", "repo", "add", name, url).Run(); err != nil {
		fmt.Printf("%s Falha ao adicionar repo Helm '%s': %v\n", crossStyle.String(), name, err)
		osExit(1)
	}
	if err := execCommand("helm", "repo", "update", name).Run(); err != nil {
		fmt.Printf("%s Falha ao atualizar repo Helm '%s': %v\n", crossStyle.String(), name, err)
		osExit(1)
	}
}

func createNamespace(ns string) {
	out, err := execCommand("kubectl", "create", "namespace", ns).CombinedOutput()
	if err != nil {
		// Ignore if already exists
		if strings.Contains(string(out), "already exists") {
			return
		}
		fmt.Printf("%s Falha ao criar namespace '%s': %s\n", crossStyle.String(), ns, string(out))
		osExit(1)
	}
}

func runCommand(name string, args ...string) {
	cmd := execCommand(name, args...)
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

func configureSecrets(root, repoURL string) {
	// 1. Sealed Secrets (Critical - Must run even without GITHUB_TOKEN)
	// Restore Sealed Secrets Keys (Check Global First)
	fmt.Println(itemStyle.Render("Verificando backup de chaves Sealed Secrets..."))

	homeDir, _ := os.UserHomeDir()
	globalKeyPath := filepath.Join(homeDir, ".yby", "keys", "local-cluster.key")

	isLocal := (os.Getenv("YBY_ENV") == "local" || contextFlag == "local")

	if isLocal {
		if _, err := os.Stat(globalKeyPath); err == nil {
			fmt.Printf("🔑 Chave Global encontrada em: %s. Restaurando...\n", globalKeyPath)
			restoreKeysCmd.Run(restoreKeysCmd, []string{globalKeyPath})
		} else {
			fmt.Println("ℹ️  Chave Global não encontrada. Verificando backup local do projeto...")
			restoreKeysCmd.Run(restoreKeysCmd, []string{}) // Fallback to bootstrap/
		}
	} else {
		// Remote/Prod: Only check local project backup (or explicitly provided)
		restoreKeysCmd.Run(restoreKeysCmd, []string{})
	}

	// Auto-Backup Logic (Fix BUG-016 - Secure Global Backup)
	// Only for Local Environment
	if isLocal {
		if _, err := os.Stat(globalKeyPath); os.IsNotExist(err) {
			fmt.Println(warningStyle.Render("⚠️  Chave Global não encontrada. Iniciando Auto-Backup Seguro..."))

			fmt.Println("⏳ Aguardando criação da chave mestra (Sealed Secrets)...")
			gotKey := false
			// Increase timeout to 5 minutes (60 * 5s) to allow image pull on slow connections
			for i := 1; i <= 60; i++ {
				cmd := exec.Command("kubectl", "get", "secret", "-n", "sealed-secrets", "-l", "sealedsecrets.bitnami.com/sealed-secrets-key=active")
				if err := cmd.Run(); err == nil {
					gotKey = true
					break
				}
				if i%2 == 0 { // Log every 10s
					fmt.Printf("   ... aguardando chave (tentativa %d/60)\r", i)
				}
				time.Sleep(5 * time.Second)
			}
			fmt.Println() // New line after progress

			if gotKey {
				backupKeysCmd.Run(backupKeysCmd, []string{globalKeyPath})
				fmt.Printf("✅ Chave Mestra salva com segurança em: %s\n", globalKeyPath)
			} else {
				fmt.Println(crossStyle.Render("❌ Timeout aguardando chave mestra. Backup ignorado."))
			}
		}
	}

	// 2. Output Secrets (Requires Token)
	// Argo CD Repo Secret
	// repo := os.Getenv("GITHUB_REPO") // Already passed as arg
	token := os.Getenv("GITHUB_TOKEN")

	if token == "" {
		fmt.Println(itemStyle.Render("Pulando Configuração de Secrets (Token não fornecido)..."))
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
// ensureTemplateAssets checks and restores critical files and directories using Embedded Assets
func ensureTemplateAssets(repoURL, root string) {
	fmt.Println(headerStyle.Render("🛠️  Auto-Repair: Verificando integridade do projeto (Embedded)..."))

	// 0. Detect Git Prefix for Monorepo/Subdir Support (infra/ integration)
	gitPrefix := getGitPrefix()
	if gitPrefix != "" {
		fmt.Printf("📂 Detectado subdiretório Git: %s. Ajustando paths do ArgoCD...\n", gitPrefix)
	}

	// 1. Restore/Update Manifests with Replacements
	manifests := []struct {
		EmbedPath    string
		DestPath     string
		Replacements map[string]string
	}{
		{
			EmbedPath: "assets/manifests/upstream/argo-workflows.yaml",
			DestPath:  JoinInfra(root, "manifests/upstream/argo-workflows.yaml"),
		},
		{
			EmbedPath: "assets/manifests/upstream/argo-events.yaml",
			DestPath:  JoinInfra(root, "manifests/upstream/argo-events.yaml"),
		},
		{
			EmbedPath: "assets/manifests/argocd/root-app.yaml.tmpl",
			DestPath:  JoinInfra(root, "manifests/argocd/root-app.yaml"),
			Replacements: map[string]string{
				"{{ .GitRepo }}":         repoURL,
				"{{ .ProjectName }}":     "default",
				"path: charts/bootstrap": fmt.Sprintf("path: %scharts/bootstrap", gitPrefix),
			},
		},
		{
			EmbedPath: "assets/manifests/projects/yby-project.yaml.tmpl",
			DestPath:  JoinInfra(root, "manifests/projects/yby-project.yaml"),
			Replacements: map[string]string{
				// Add the current repo to the whitelist by appending it after the generic one
				"  - 'https://github.com/*/yby'": fmt.Sprintf("  - 'https://github.com/*/yby'\n  - '%s'", repoURL),
			},
		},
	}

	for _, m := range manifests {
		if _, err := os.Stat(m.DestPath); os.IsNotExist(err) {
			restoreEmbedFile(m.EmbedPath, m.DestPath, m.Replacements)
		}
	}

	// 2. Restore Critical Directories (Recursive)
	dirs := []struct {
		EmbedRoot string
		DestRoot  string
	}{
		{"assets/charts/system", JoinInfra(root, "charts/system")},
		{"assets/argo-workflows", JoinInfra(root, "templates/workflows")},
	}

	missingDirs := []string{}
	for _, d := range dirs {
		if _, err := os.Stat(d.DestRoot); os.IsNotExist(err) {
			missingDirs = append(missingDirs, d.DestRoot)
			fmt.Printf("%s Restaurando %s...\n", stepStyle.Render("♻️"), d.DestRoot)
			restoreEmbedDir(d.EmbedRoot, d.DestRoot)
		}
	}

	if len(missingDirs) == 0 {
		fmt.Println(checkStyle.Render("✅ Integridade verificada."))
	}
}

func restoreEmbedFile(embedPath, destPath string, replacements map[string]string) {
	fmt.Printf("%s Restaurando %s...\n", stepStyle.Render("📄"), destPath)

	data, err := templates.Assets.ReadFile(embedPath)
	if err != nil {
		fmt.Printf("%s Erro ao ler asset embedado %s: %v\n", crossStyle.String(), embedPath, err)
		os.Exit(1)
	}
	content := string(data)

	// Apply Replacements
	for old, new := range replacements {
		content = strings.ReplaceAll(content, old, new)
	}

	// Ensure dir
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		fmt.Printf("%s Erro ao criar diretório %s: %v\n", crossStyle.String(), filepath.Dir(destPath), err)
		os.Exit(1)
	}

	// Write file
	if err := os.WriteFile(destPath, []byte(content), 0644); err != nil {
		fmt.Printf("%s Erro ao salvar arquivo %s: %v\n", crossStyle.String(), destPath, err)
		os.Exit(1)
	}
}

func restoreEmbedDir(embedRoot, destRoot string) {
	err := fs.WalkDir(templates.Assets, embedRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		// Rel path from embedRoot -> e.g. "ClusterRole.yaml"
		rel, _ := filepath.Rel(embedRoot, path)
		destPath := filepath.Join(destRoot, rel)

		// Read and Write (Direct copy, no replacements)
		data, err := templates.Assets.ReadFile(path)
		if err != nil {
			return err
		}

		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return err
		}

		return os.WriteFile(destPath, data, 0644)
	})

	if err != nil {
		fmt.Printf("%s Erro ao restaurar diretório %s: %v\n", crossStyle.String(), destRoot, err)
	}
}

func getGitPrefix() string {
	cmd := exec.Command("git", "rev-parse", "--show-prefix")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "" // Not a git repo or error, assume root
	}
	return strings.TrimSpace(string(out))
}
func getRepoURLFromBlueprint(root string) string {
	// Check the blueprint in the discovered infra root
	p := JoinInfra(root, ".yby/blueprint.yaml")

	if data, err := os.ReadFile(p); err == nil {
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

// Blueprint structure for reading legacy blueprint.yaml purely for git repo url
type Blueprint struct {
	Prompts []struct {
		ID      string      `yaml:"id"`
		Default interface{} `yaml:"default"`
	} `yaml:"prompts"`
}
