/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	"fmt"
	"os"

	"github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/pkg/services/secrets"
	"github.com/casheiro/yby-cli/pkg/services/shared"
	"github.com/spf13/cobra"
)

// newSecretsService é a factory para criação do serviço de secrets (mockável em testes)
var newSecretsService = func(r shared.Runner, fs shared.Filesystem) secrets.Service {
	return secrets.NewService(r, fs)
}

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
	RunE: func(cmd *cobra.Command, args []string) error {
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

		if secretVal == "" {
			secretVal = os.Getenv("WEBHOOK_SECRET")
		}

		outputFile := JoinInfra(root, fmt.Sprintf("charts/cluster-config/templates/events/sealed-secret-%s.yaml", provider))

		runner := &shared.RealRunner{}
		fsys := &shared.RealFilesystem{}
		svc := newSecretsService(runner, fsys)

		opts := secrets.Options{
			Provider:   provider,
			SecretVal:  secretVal,
			OutputPath: outputFile,
		}

		finalSecret, err := svc.GenerateWebhook(cmd.Context(), opts)
		if err != nil {
			return errors.Wrap(err, errors.ErrCodeExec, "falha ao gerar webhook secret")
		}

		if opts.SecretVal == "" {
			fmt.Println(warningStyle.Render("WEBHOOK_SECRET não definido. Gerado aleatório."))
			fmt.Printf("Segredo gerado: %s\n", finalSecret)
		}
		fmt.Printf("%s Salvo em: %s\n", checkStyle.String(), outputFile)
		return nil
	},
}

var minioSecretCmd = &cobra.Command{
	Use:   "minio",
	Short: "Gera Sealed Secret do MinIO",
	Long: `Gera credenciais aleatórias para o MinIO, cria o Secret e sela.
Salva em: charts/system/templates/secrets/sealed-secret-minio.yaml`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println(titleStyle.Render("🔐 MinIO Secret"))

		root, err := FindInfraRoot()
		if err != nil {
			root = "."
		}

		outputFile := JoinInfra(root, "charts/system/templates/secrets/sealed-secret-minio.yaml")

		runner := &shared.RealRunner{}
		fsys := &shared.RealFilesystem{}
		svc := newSecretsService(runner, fsys)

		opts := secrets.Options{
			OutputPath: outputFile,
		}

		fmt.Println("Gerando credenciais MinIO (User: admin)...")
		_, err = svc.GenerateMinIO(cmd.Context(), opts)
		if err != nil {
			return errors.Wrap(err, errors.ErrCodeExec, "falha ao gerar secret MinIO")
		}

		fmt.Printf("%s Salvo em: %s\n", checkStyle.String(), outputFile)
		return nil
	},
}

var githubTokenSecretCmd = &cobra.Command{
	Use:   "github-token [token]",
	Short: "Cria secret para Discovery (GitHub Token)",
	Long: `Cria o secret 'github-token' no namespace 'argocd' com o PAT do GitHub.
Necessário para o ApplicationSet descobrir repositórios.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		token := args[0]
		fmt.Println(titleStyle.Render("🔐 GitHub Token Secret"))

		runner := &shared.RealRunner{}
		fsys := &shared.RealFilesystem{}
		svc := newSecretsService(runner, fsys)

		opts := secrets.Options{Token: token}

		if err := svc.CreateGitHubToken(cmd.Context(), opts); err != nil {
			return errors.Wrap(err, errors.ErrCodeExec, "falha ao criar secret github-token")
		}

		fmt.Println(checkStyle.Render("✅ Secret github-token criado no namespace argocd."))
		return nil
	},
}

var backupKeysCmd = &cobra.Command{
	Use:   "backup [file]",
	Short: "Backup da chave mestre do Sealed Secrets",
	Long: `Faz backup da chave privada do Sealed Secrets (cuidado!).
Salva em: bootstrap/sealed-secrets-backup.yaml (default) ou no caminho especificado.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println(titleStyle.Render("🔐 Backup Sealed Secrets Keys"))

		root, err := FindInfraRoot()
		if err != nil {
			root = "."
		}

		outputFile := JoinInfra(root, "bootstrap/sealed-secrets-backup.yaml")
		if len(args) > 0 {
			outputFile = args[0]
		}

		runner := &shared.RealRunner{}
		fsys := &shared.RealFilesystem{}
		svc := newSecretsService(runner, fsys)

		opts := secrets.Options{OutputPath: outputFile}

		keyName, err := svc.BackupKeys(cmd.Context(), opts)
		if err != nil {
			return errors.Wrap(err, errors.ErrCodeExec, "falha no backup de chaves Sealed Secrets")
		}

		fmt.Printf("Chave encontrada: %s\n", keyName)
		fmt.Printf("%s Backup salvo em %s\n", checkStyle.String(), outputFile)
		if len(args) == 0 {
			fmt.Println(warningStyle.Render("⚠️  NÃO COLOQUE ESTE ARQUIVO NO GIT se for um repositório público!"))
		}
		return nil
	},
}

var restoreKeysCmd = &cobra.Command{
	Use:   "restore [file]",
	Short: "Restaura chave mestre do Sealed Secrets",
	Long: `Aplica um backup de chave mestre e reinicia o controller.
Default file: bootstrap/sealed-secrets-backup.yaml`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println(titleStyle.Render("🔐 Restore Sealed Secrets Keys"))

		root, err := FindInfraRoot()
		if err != nil {
			root = "."
		}

		inputFile := JoinInfra(root, "bootstrap/sealed-secrets-backup.yaml")
		if len(args) > 0 {
			inputFile = args[0]
		}

		fmt.Printf("Restaurando de: %s\n", inputFile)

		runner := &shared.RealRunner{}
		fsys := &shared.RealFilesystem{}
		svc := newSecretsService(runner, fsys)

		opts := secrets.Options{OutputPath: inputFile}

		if err := svc.RestoreKeys(cmd.Context(), opts); err != nil {
			return errors.Wrap(err, errors.ErrCodeExec, "falha na restauração de chaves Sealed Secrets")
		}

		fmt.Println(checkStyle.Render("✅ Chave restaurada e controller reiniciado."))
		return nil
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
