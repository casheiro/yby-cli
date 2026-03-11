package secrets

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/casheiro/yby-cli/pkg/services/shared"
)

type Options struct {
	Provider   string
	SecretVal  string
	OutputPath string
	Token      string
}

type Service interface {
	GenerateWebhook(ctx context.Context, opts Options) (string, error)
	GenerateMinIO(ctx context.Context, opts Options) (string, error)
	CreateGitHubToken(ctx context.Context, opts Options) error
	BackupKeys(ctx context.Context, opts Options) (string, error)
	RestoreKeys(ctx context.Context, opts Options) error
	EncryptWithSOPS(ctx context.Context, ageRecipient string, secretYaml []byte, outputPath string) error
	GenerateAgeKey(ctx context.Context, outputPath string) (string, error)
}

type secretsService struct {
	runner shared.Runner
	fs     shared.Filesystem
}

func NewService(runner shared.Runner, fs shared.Filesystem) Service {
	return &secretsService{runner: runner, fs: fs}
}

func (s *secretsService) GenerateWebhook(ctx context.Context, opts Options) (string, error) {
	provider := opts.Provider
	secretVal := opts.SecretVal

	if secretVal == "" {
		out, err := s.runner.RunCombinedOutput(ctx, "openssl", "rand", "-hex", "20")
		if err != nil {
			return "", fmt.Errorf("falha ao gerar secret aleatório: %w", err)
		}
		secretVal = strings.TrimSpace(string(out))
	}

	secretName := fmt.Sprintf("%s-webhook-secret", provider)
	namespace := "argo-events"

	// Dry-run create secret
	secretYaml, err := s.runner.RunCombinedOutput(ctx, "kubectl", "create", "secret", "generic", secretName,
		"--from-literal=secret="+secretVal,
		"--namespace", namespace,
		"--dry-run=client", "-o", "yaml")
	if err != nil {
		return "", fmt.Errorf("falha ao gerar secret com kubectl: %w", err)
	}

	err = s.sealAndSave(ctx, secretYaml, opts.OutputPath)
	if err != nil {
		return "", err
	}

	return secretVal, nil
}

func (s *secretsService) GenerateMinIO(ctx context.Context, opts Options) (string, error) {
	user := "admin"

	out, err := s.runner.RunCombinedOutput(ctx, "openssl", "rand", "-hex", "16")
	if err != nil {
		return "", fmt.Errorf("falha ao gerar senha MinIO: %w", err)
	}
	password := strings.TrimSpace(string(out))

	secretName := "minio-secret"
	namespace := "storage"

	secretYaml, err := s.runner.RunCombinedOutput(ctx, "kubectl", "create", "secret", "generic", secretName,
		"--from-literal=rootUser="+user,
		"--from-literal=rootPassword="+password,
		"--namespace", namespace,
		"--dry-run=client", "-o", "yaml")
	if err != nil {
		return "", fmt.Errorf("falha ao gerar secret com kubectl: %w", err)
	}

	err = s.sealAndSave(ctx, secretYaml, opts.OutputPath)
	if err != nil {
		return "", err
	}

	return user, nil
}

func (s *secretsService) CreateGitHubToken(ctx context.Context, opts Options) error {
	secretYaml, err := s.runner.RunCombinedOutput(ctx, "kubectl", "create", "secret", "generic", "github-token",
		"--from-literal=token="+opts.Token,
		"--namespace", "argocd",
		"--dry-run=client", "-o", "yaml")
	if err != nil {
		return fmt.Errorf("erro ao gerar secret github-token: %w", err)
	}

	err = s.runner.RunStdin(ctx, string(secretYaml), "kubectl", "apply", "-f", "-")
	if err != nil {
		return fmt.Errorf("erro ao aplicar secret github-token: %w", err)
	}

	return nil
}

func (s *secretsService) BackupKeys(ctx context.Context, opts Options) (string, error) {
	out, err := s.runner.RunCombinedOutput(ctx, "kubectl", "get", "secret", "-n", "sealed-secrets", "-l", "sealedsecrets.bitnami.com/sealed-secrets-key=active", "-o", "name")
	keyName := strings.TrimSpace(string(out))

	if err != nil || keyName == "" {
		return "", fmt.Errorf("chave não encontrada")
	}

	keyName = strings.ReplaceAll(keyName, "secret/", "")

	err = s.fs.MkdirAll(filepath.Dir(opts.OutputPath), 0755)
	if err != nil {
		return "", fmt.Errorf("erro ao criar diretório: %w", err)
	}

	bkpData, err := s.runner.RunCombinedOutput(ctx, "kubectl", "get", "secret", keyName, "-n", "sealed-secrets", "-o", "yaml")
	if err != nil {
		return "", fmt.Errorf("erro ao buscar backup do kubernetes: %w", err)
	}

	err = s.fs.WriteFile(opts.OutputPath, bkpData, 0644)
	if err != nil {
		return "", fmt.Errorf("erro ao salvar backup: %w", err)
	}

	return keyName, nil
}

func (s *secretsService) RestoreKeys(ctx context.Context, opts Options) error {
	_, err := s.fs.Stat(opts.OutputPath)
	if err != nil {
		return fmt.Errorf("arquivo de backup não encontrado: %w", err)
	}

	_ = s.runner.Run(ctx, "kubectl", "create", "ns", "sealed-secrets")

	err = s.runner.Run(ctx, "kubectl", "apply", "-f", opts.OutputPath)
	if err != nil {
		return fmt.Errorf("erro ao aplicar chave: %w", err)
	}

	err = s.runner.Run(ctx, "kubectl", "delete", "pod", "-n", "sealed-secrets", "-l", "app.kubernetes.io/name=sealed-secrets")
	if err != nil {
		return fmt.Errorf("erro ao reiniciar controller: %w", err)
	}

	return nil
}

// EncryptWithSOPS encripta secretYaml usando SOPS + age e salva em outputPath.
// Se ageRecipient for vazio, o SOPS usa .sops.yaml para determinar o destinatário.
func (s *secretsService) EncryptWithSOPS(ctx context.Context, ageRecipient string, secretYaml []byte, outputPath string) error {
	args := []string{"--encrypt", "--input-type", "yaml", "--output-type", "yaml"}
	if ageRecipient != "" {
		args = append(args, "--age", ageRecipient)
	}

	encrypted, err := s.runner.RunStdinOutput(ctx, string(secretYaml), "sops", args...)
	if err != nil {
		return fmt.Errorf("erro ao encriptar com sops: %w", err)
	}

	if err := s.fs.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("erro ao criar diretório: %w", err)
	}

	if err := s.fs.WriteFile(outputPath, encrypted, 0600); err != nil {
		return fmt.Errorf("erro ao salvar arquivo: %w", err)
	}

	return nil
}

// GenerateAgeKey gera um par de chaves age e salva em outputPath.
// Retorna a chave pública gerada.
func (s *secretsService) GenerateAgeKey(ctx context.Context, outputPath string) (string, error) {
	if err := s.fs.MkdirAll(filepath.Dir(outputPath), 0700); err != nil {
		return "", fmt.Errorf("erro ao criar diretório para chave age: %w", err)
	}

	out, err := s.runner.RunCombinedOutput(ctx, "age-keygen", "-o", outputPath)
	if err != nil {
		return "", fmt.Errorf("erro ao gerar chave age: %w", err)
	}

	// age-keygen imprime "Public key: age1xxx..." na saída combinada
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "Public key: ") {
			return strings.TrimPrefix(line, "Public key: "), nil
		}
	}

	return "", fmt.Errorf("chave pública não encontrada na saída do age-keygen")
}

func (s *secretsService) sealAndSave(ctx context.Context, input []byte, outputFile string) error {
	sealedYaml, err := s.runner.RunStdinOutput(ctx, string(input), "kubeseal", "--controller-name=sealed-secrets", "--controller-namespace=sealed-secrets", "--format=yaml")
	if err != nil {
		return fmt.Errorf("erro ao executar kubeseal: %w", err)
	}

	if err := s.fs.MkdirAll(filepath.Dir(outputFile), 0755); err != nil {
		return fmt.Errorf("erro ao criar diretório: %w", err)
	}

	if err := s.fs.WriteFile(outputFile, sealedYaml, 0644); err != nil {
		return fmt.Errorf("erro ao salvar arquivo: %w", err)
	}

	return nil
}
