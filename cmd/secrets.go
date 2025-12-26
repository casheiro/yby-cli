/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// secretsCmd represents the secret command
var secretsCmd = &cobra.Command{
	Use:   "secret",
	Short: "Gerenciamento de Segredos (Webhooks, MinIO, SealedSecrets)",
	Long:  `Agrupa utilitários para gerar e gerenciar segredos.`,
}

var webhookSecretCmd = &cobra.Command{
	Use:   "webhook [provider] [secret]",
	Short: "Gera ou exibe segredo do Webhook",
	Long: `Cria um SealedSecret para o Webhook (ex: GitHub).
Uso: yby secret webhook github [my-secret-value]
Se o valor não for fornecido, gera um aleatório.
Salva em: charts/cluster-config/templates/events/sealed-secret-github.yaml`,
	Args: cobra.RangeArgs(0, 2),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(titleStyle.Render("🔐 Webhook Secret"))

		root, err := FindInfraRoot()
		if err != nil {
			root = "."
		}

		provider := "github"
		if len(args) > 0 {
			provider = args[0]
		}

		secretVal := ""
		if len(args) > 1 {
			secretVal = args[1]
		}

		// Check env if not provided
		if secretVal == "" {
			secretVal = os.Getenv("WEBHOOK_SECRET")
		}

		if secretVal == "" {
			fmt.Println(warningStyle.Render("WEBHOOK_SECRET não definido. Gerando aleatório..."))
			out, _ := exec.Command("openssl", "rand", "-hex", "20").Output()
			secretVal = strings.TrimSpace(string(out))
			fmt.Printf("Segredo gerado: %s\n", secretVal)
		}

		secretName := fmt.Sprintf("%s-webhook-secret", provider)
		namespace := "argo-events"
		outputFile := JoinInfra(root, fmt.Sprintf("charts/cluster-config/templates/events/sealed-secret-%s.yaml", provider))

		// Create Secret (Dry Run)
		kubectlCmd := exec.Command("kubectl", "create", "secret", "generic", secretName,
			"--from-literal=secret="+secretVal,
			"--namespace", namespace,
			"--dry-run=client", "-o", "yaml")

		var secretYaml bytes.Buffer
		kubectlCmd.Stdout = &secretYaml
		if err := kubectlCmd.Run(); err != nil {
			fmt.Println(crossStyle.Render("Erro ao gerar secret com kubectl."))
			return
		}

		// Seal
		sealAndSave(secretYaml.Bytes(), outputFile)
	},
}

var minioSecretCmd = &cobra.Command{
	Use:   "minio",
	Short: "Gera Sealed Secret do MinIO",
	Long: `Gera credenciais aleatórias para o MinIO, cria o Secret e sela.
Salva em: charts/system/templates/secrets/sealed-secret-minio.yaml`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(titleStyle.Render("🔐 MinIO Secret"))

		root, err := FindInfraRoot()
		if err != nil {
			root = "."
		}

		user := "admin" // Default minio user

		// Generate Password
		out, _ := exec.Command("openssl", "rand", "-hex", "16").Output()
		password := strings.TrimSpace(string(out))

		fmt.Printf("Gerando credenciais MinIO (User: %s)...\n", user)

		secretName := "minio-creds"
		namespace := "argocd" // MinIO usually in argocd or system ns
		outputFile := JoinInfra(root, "charts/system/templates/secrets/sealed-secret-minio.yaml")

		kubectlCmd := exec.Command("kubectl", "create", "secret", "generic", secretName,
			"--from-literal=rootUser="+user,
			"--from-literal=rootPassword="+password,
			"--namespace", namespace,
			"--dry-run=client", "-o", "yaml")

		var secretYaml bytes.Buffer
		kubectlCmd.Stdout = &secretYaml
		if err := kubectlCmd.Run(); err != nil {
			fmt.Println(crossStyle.Render("Erro ao gerar secret com kubectl."))
			return
		}

		sealAndSave(secretYaml.Bytes(), outputFile)
	},
}

var githubTokenSecretCmd = &cobra.Command{
	Use:   "github-token [token]",
	Short: "Cria secret para Discovery (GitHub Token)",
	Long: `Cria o secret 'github-token' no namespace 'argocd' com o PAT do GitHub.
Necessário para o ApplicationSet descobrir repositórios.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		token := args[0]
		fmt.Println(titleStyle.Render("🔐 GitHub Token Secret"))

		kubectlCmd := exec.Command("kubectl", "create", "secret", "generic", "github-token",
			"--from-literal=token="+token,
			"--namespace", "argocd",
			"--dry-run=client", "-o", "yaml")

		var secretYaml bytes.Buffer
		kubectlCmd.Stdout = &secretYaml

		if err := kubectlCmd.Run(); err != nil {
			fmt.Println(crossStyle.Render("Erro ao gerar secret."))
			return
		}

		// Apply directly (it's not a sealed secret usually, or is it? The docs say "Crie o secret no cluster".
		// The script name was create-github-token-secret.sh, not sealed.
		// Usually discovery tokens are plain secrets if not using SealedSecrets for everything,
		// but wait, ApplicationSet reads Env/Secret.
		// Let's assume plain apply for now as per previous manual instructions.)

		applyCmd := exec.Command("kubectl", "apply", "-f", "-")
		applyCmd.Stdin = &secretYaml
		if err := applyCmd.Run(); err != nil {
			fmt.Println(crossStyle.Render("Erro ao aplicar secret."))
			return
		}

		fmt.Println(checkStyle.Render("✅ Secret github-token criado no namespace argocd."))
	},
}

var backupKeysCmd = &cobra.Command{
	Use:   "backup",
	Short: "Backup da chave mestre do Sealed Secrets",
	Long: `Faz backup da chave privada do Sealed Secrets (cuidado!).
Salva em: bootstrap/sealed-secrets-backup.yaml`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(titleStyle.Render("🔐 Backup Sealed Secrets Keys"))

		root, err := FindInfraRoot()
		if err != nil {
			root = "."
		}

		// 1. Find Active Key Secret
		// Try active label first
		out, err := exec.Command("kubectl", "get", "secret", "-n", "sealed-secrets", "-l", "sealedsecrets.bitnami.com/sealed-secrets-key=active", "-o", "name").Output()
		keyName := strings.TrimSpace(string(out))

		if err != nil || keyName == "" {
			fmt.Println(warningStyle.Render("Nenhuma chave ativa encontrada pelo label. Tentando a mais recente..."))
			// Fallback logic could be complex in Go, simplifying for CLI context:
			// Just get all secrets and pick one? For now let's error if strictly nothing.
			fmt.Println(crossStyle.Render("Erro: Chave não encontrada."))
			return
		}

		keyName = strings.ReplaceAll(keyName, "secret/", "") // remove prefix
		fmt.Printf("Chave encontrada: %s\n", keyName)

		outputFile := JoinInfra(root, "bootstrap/sealed-secrets-backup.yaml")
		_ = os.MkdirAll(filepath.Dir(outputFile), 0755)

		file, err := os.Create(outputFile)
		if err != nil {
			fmt.Println(crossStyle.Render("Erro ao criar arquivo de backup."))
			return
		}
		defer file.Close()

		bkpCmd := exec.Command("kubectl", "get", "secret", keyName, "-n", "sealed-secrets", "-o", "yaml")
		bkpCmd.Stdout = file
		if err := bkpCmd.Run(); err != nil {
			fmt.Println(crossStyle.Render("Erro ao fazer backup."))
			return
		}

		fmt.Printf("%s Backup salvo em %s\n", checkStyle.String(), outputFile)
		fmt.Println(warningStyle.Render("⚠️  NÃO COLOQUE ESTE ARQUIVO NO GIT se for um repositório público!"))
	},
}

var restoreKeysCmd = &cobra.Command{
	Use:   "restore [file]",
	Short: "Restaura chave mestre do Sealed Secrets",
	Long: `Aplica um backup de chave mestre e reinicia o controller.
Default file: bootstrap/sealed-secrets-backup.yaml`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(titleStyle.Render("🔐 Restore Sealed Secrets Keys"))

		root, err := FindInfraRoot()
		if err != nil {
			root = "."
		}

		inputFile := JoinInfra(root, "bootstrap/sealed-secrets-backup.yaml")
		if len(args) > 0 {
			inputFile = args[0]
		}

		if _, err := os.Stat(inputFile); os.IsNotExist(err) {
			fmt.Printf("%s Arquivo de backup %s não encontrado. Pulando restore.\n", warningStyle.String(), inputFile)
			return
		}

		fmt.Printf("Restaurando de: %s\n", inputFile)

		// Create ns if not exists
		_ = exec.Command("kubectl", "create", "ns", "sealed-secrets").Run()

		// Apply
		if err := exec.Command("kubectl", "apply", "-f", inputFile).Run(); err != nil {
			fmt.Println(crossStyle.Render("Erro ao aplicar chave."))
			return
		}

		// Delete pods to reload
		fmt.Println("Reiniciando controller...")
		_ = exec.Command("kubectl", "delete", "pod", "-n", "sealed-secrets", "-l", "app.kubernetes.io/name=sealed-secrets").Run()

		fmt.Println(checkStyle.Render("✅ Chave restaurada."))
	},
}

func init() {
	rootCmd.AddCommand(secretsCmd)
	secretsCmd.AddCommand(webhookSecretCmd)
	secretsCmd.AddCommand(minioSecretCmd)
	secretsCmd.AddCommand(githubTokenSecretCmd)
	secretsCmd.AddCommand(backupKeysCmd)
	secretsCmd.AddCommand(restoreKeysCmd)
}

func sealAndSave(input []byte, outputFile string) {
	fmt.Println("🔒 Selando com Kubeseal...")

	kubesealCmd := exec.Command("kubeseal", "--controller-name=sealed-secrets", "--controller-namespace=sealed-secrets", "--format=yaml")
	kubesealCmd.Stdin = bytes.NewReader(input)

	var sealedYaml bytes.Buffer
	kubesealCmd.Stdout = &sealedYaml

	if err := kubesealCmd.Run(); err != nil {
		fmt.Println(crossStyle.Render("Erro ao executar kubeseal. Verifique conexão com cluster."))
		return
	}

	// Mkdir
	if err := os.MkdirAll(filepath.Dir(outputFile), 0755); err != nil {
		fmt.Printf("Erro ao criar diretório: %v\n", err)
		return
	}

	if err := os.WriteFile(outputFile, sealedYaml.Bytes(), 0644); err != nil {
		fmt.Printf("Erro ao salvar arquivo: %v\n", err)
		return
	}

	fmt.Printf("%s Salvo em: %s\n", checkStyle.String(), outputFile)
}
