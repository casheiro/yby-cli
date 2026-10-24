/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/pkg/services/secrets"
	"github.com/casheiro/yby-cli/pkg/services/shared"
	"github.com/casheiro/yby-cli/pkg/ui"
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
		w := cmd.OutOrStdout()
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
			return falhaComHint(err, errors.ErrCodeExec, "falha ao gerar webhook secret", hintSealedSecrets)
		}

		tituloComando(w, "secret", "webhook · "+provider)
		linhas := []linhaCheck{{ui.CheckOk, "SealedSecret", "salvo", outputFile}}
		if opts.SecretVal == "" {
			linhas = append([]linhaCheck{{ui.CheckAviso, "WEBHOOK_SECRET", "ausente", "gerado um segredo aleatório"}}, linhas...)
		}
		checksAlinhados(w, linhas...)
		if opts.SecretVal == "" {
			ui.Section(w, "segredo gerado")
			saidaRecuada(w, finalSecret, false)
		}
		return nil
	},
}

var minioSecretCmd = &cobra.Command{
	Use:   "minio",
	Short: "Gera Sealed Secret do MinIO",
	Long: `Gera credenciais aleatórias para o MinIO, cria o Secret e sela.
Salva em: charts/system/templates/secrets/sealed-secret-minio.yaml`,
	RunE: func(cmd *cobra.Command, args []string) error {
		w := cmd.OutOrStdout()
		tituloComando(w, "secret", "minio")

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

		linhaEmAndamento(w, "gerando as credenciais do MinIO (usuário admin)")
		_, err = svc.GenerateMinIO(cmd.Context(), opts)
		if err != nil {
			return falhaComHint(err, errors.ErrCodeExec, "falha ao gerar secret MinIO", hintSealedSecrets)
		}

		checksAlinhados(w, linhaCheck{ui.CheckOk, "SealedSecret", "salvo", outputFile})
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
		runner := &shared.RealRunner{}
		fsys := &shared.RealFilesystem{}
		svc := newSecretsService(runner, fsys)

		opts := secrets.Options{Token: token}

		if err := svc.CreateGitHubToken(cmd.Context(), opts); err != nil {
			return falhaComHint(err, errors.ErrCodeExec, "falha ao criar secret github-token",
				"confira o token e o acesso ao cluster: kubectl get namespace argocd")
		}

		w := cmd.OutOrStdout()
		tituloComando(w, "secret", "github-token")
		checksAlinhados(w, linhaCheck{ui.CheckOk, "github-token", "criado", "Secret no namespace argocd"})
		return nil
	},
}

var backupKeysCmd = &cobra.Command{
	Use:   "backup [file]",
	Short: "Backup da chave mestre do Sealed Secrets",
	Long: `Faz backup da chave privada do Sealed Secrets (cuidado!).
Salva em: bootstrap/sealed-secrets-backup.yaml (default) ou no caminho especificado.`,
	RunE: func(cmd *cobra.Command, args []string) error {
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
			return falhaComHint(err, errors.ErrCodeExec, "falha no backup de chaves Sealed Secrets", hintSealedSecrets)
		}

		w := cmd.OutOrStdout()
		tituloComando(w, "secret", "backup")
		linhas := []linhaCheck{
			{ui.CheckOk, "chave", "encontrada", keyName},
			{ui.CheckOk, "backup", "salvo", outputFile},
		}
		if len(args) == 0 {
			linhas = append(linhas, linhaCheck{ui.CheckAviso, "git", "não versione", "nunca num repositório público"})
		}
		checksAlinhados(w, linhas...)
		return nil
	},
}

var restoreKeysCmd = &cobra.Command{
	Use:   "restore [file]",
	Short: "Restaura chave mestre do Sealed Secrets",
	Long: `Aplica um backup de chave mestre e reinicia o controller.
Default file: bootstrap/sealed-secrets-backup.yaml`,
	RunE: func(cmd *cobra.Command, args []string) error {
		w := cmd.OutOrStdout()
		tituloComando(w, "secret", "restore")

		root, err := FindInfraRoot()
		if err != nil {
			root = "."
		}

		inputFile := JoinInfra(root, "bootstrap/sealed-secrets-backup.yaml")
		if len(args) > 0 {
			inputFile = args[0]
		}

		linhaEmAndamento(w, "restaurando de "+inputFile)

		runner := &shared.RealRunner{}
		fsys := &shared.RealFilesystem{}
		svc := newSecretsService(runner, fsys)

		opts := secrets.Options{OutputPath: inputFile}

		if err := svc.RestoreKeys(cmd.Context(), opts); err != nil {
			return falhaComHint(err, errors.ErrCodeExec, "falha na restauração de chaves Sealed Secrets",
				"confira o arquivo de backup e se o Sealed Secrets está no cluster: kubectl get pods -n sealed-secrets")
		}

		checksAlinhados(w, linhaCheck{ui.CheckOk, "chave", "restaurada", "controller reiniciado"})
		return nil
	},
}

var initSOPSCmd = &cobra.Command{
	Use:   "init-sops",
	Short: "Inicializa SOPS + age para o projeto",
	Long: `Gera um par de chaves age e exibe a chave pública para configurar .sops.yaml.
A chave privada é salva localmente e NUNCA deve ir para o Git.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			homeDir = "."
		}
		keyPath := filepath.Join(homeDir, ".sops", "age-key.txt")

		runner := &shared.RealRunner{}
		fsys := &shared.RealFilesystem{}
		svc := newSecretsService(runner, fsys)

		publicKey, err := svc.GenerateAgeKey(cmd.Context(), keyPath)
		if err != nil {
			return falhaComHint(err, errors.ErrCodeExec, "falha ao gerar chave age",
				"instale o age (age-keygen): https://github.com/FiloSottile/age")
		}

		w := cmd.OutOrStdout()
		tituloComando(w, "secret", "init-sops")
		checksAlinhados(w,
			linhaCheck{ui.CheckOk, "chave privada", "salva", keyPath},
			linhaCheck{ui.CheckAviso, "git", "não versione", "a chave privada nunca vai para o Git"},
		)
		ui.Section(w, "chave pública age")
		saidaRecuada(w, publicKey, false)
		ui.Section(w, "configuração do sops")
		fmt.Fprintln(w, "  "+ui.S().TintaSuave.Render("adicione ao .sops.yaml do projeto:"))
		saidaRecuada(w, fmt.Sprintf("creation_rules:\n  - path_regex: \\.yaml$\n    age: %s\n", publicKey), false)
		return nil
	},
}

var initESOBackend string

var initESOCmd = &cobra.Command{
	Use:   "init-eso",
	Short: "Gera scaffold do SecretStore para External Secrets Operator",
	Long: `Gera o YAML de ClusterSecretStore para o backend especificado.
Use com --backend: vault, aws, gcp, azure`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if initESOBackend == "" {
			return errors.New(errors.ErrCodeValidation, "backend obrigatório").
				WithHint("informe o backend: yby secret init-eso --backend vault|aws|gcp|azure")
		}

		backendOpts := secrets.BackendOpts{
			Name:      initESOBackend,
			Namespace: "external-secrets",
		}

		yamlContent, err := secrets.GenerateSecretStore(initESOBackend, backendOpts)
		if err != nil {
			return errors.Wrap(err, errors.ErrCodeValidation, "falha ao gerar SecretStore").
				WithHint("use --backend vault, aws, gcp ou azure")
		}

		root, err := FindInfraRoot()
		if err != nil {
			root = "."
		}

		outputFile := JoinInfra(root, fmt.Sprintf("bootstrap/secret-store-%s.yaml", initESOBackend))
		hintEscrita := "confira a permissão de escrita em " + filepath.Dir(outputFile)
		if err := os.MkdirAll(filepath.Dir(outputFile), 0755); err != nil {
			return errors.Wrap(err, errors.ErrCodeIO, "falha ao criar diretório").WithHint(hintEscrita)
		}
		if err := os.WriteFile(outputFile, []byte(yamlContent), 0644); err != nil {
			return errors.Wrap(err, errors.ErrCodeIO, "falha ao salvar arquivo").WithHint(hintEscrita)
		}

		// o título sai só depois de gravar: uma falha antes disso aparece só com o erro
		w := cmd.OutOrStdout()
		tituloComando(w, "secret", "init-eso · "+initESOBackend)
		checksAlinhados(w, linhaCheck{ui.CheckOk, "SecretStore", "salvo", outputFile})
		ui.Section(w, "próximos passos")
		ui.Passos(w,
			"edite o arquivo gerado com os valores do seu ambiente",
			"instale o External Secrets Operator no cluster",
			"aplique com kubectl apply -f "+outputFile,
		)
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
	secretsCmd.AddCommand(initSOPSCmd)
	secretsCmd.AddCommand(initESOCmd)

	initESOCmd.Flags().StringVar(&initESOBackend, "backend", "", "Backend ESO: vault, aws, gcp, azure")
}
