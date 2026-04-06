package secrets

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	ybyerrors "github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/pkg/services/shared"
)

// deriveHMACKey gera uma chave para HMAC baseada no hostname + path do arquivo.
func deriveHMACKey(filePath string) []byte {
	hostname, _ := os.Hostname()
	key := sha256.Sum256([]byte(hostname + ":" + filePath))
	return key[:]
}

// computeHMAC calcula HMAC-SHA256 do conteúdo usando a chave derivada.
func computeHMAC(data []byte, key []byte) string {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return hex.EncodeToString(mac.Sum(nil))
}

type secretsService struct {
	runner shared.Runner
	fs     shared.Filesystem
}

func NewService(runner shared.Runner, fs shared.Filesystem) Service {
	return &secretsService{runner: runner, fs: fs}
}

// validateSecretValue rejeita valores de secret que contêm caracteres de controle.
func validateSecretValue(value, fieldName string) error {
	for _, r := range value {
		if r == '\n' || r == '\r' || r == '\t' || r == '\x00' {
			return ybyerrors.New(ybyerrors.ErrCodeValidation,
				fmt.Sprintf("valor de %s contém caracteres de controle não permitidos", fieldName))
		}
	}
	return nil
}

func (s *secretsService) GenerateWebhook(ctx context.Context, opts Options) (string, error) {
	provider := opts.Provider
	secretVal := opts.SecretVal

	if secretVal != "" {
		if err := validateSecretValue(secretVal, "webhook secret"); err != nil {
			return "", err
		}
	}

	if secretVal == "" {
		out, err := s.runner.RunCombinedOutput(ctx, "openssl", "rand", "-hex", "20")
		if err != nil {
			return "", ybyerrors.Wrap(err, ybyerrors.ErrCodeExec, "falha ao gerar secret aleatório")
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
		return "", ybyerrors.Wrap(err, ybyerrors.ErrCodeExec, "falha ao gerar secret com kubectl")
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
		return "", ybyerrors.Wrap(err, ybyerrors.ErrCodeExec, "falha ao gerar senha MinIO")
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
		return "", ybyerrors.Wrap(err, ybyerrors.ErrCodeExec, "falha ao gerar secret com kubectl")
	}

	err = s.sealAndSave(ctx, secretYaml, opts.OutputPath)
	if err != nil {
		return "", err
	}

	return user, nil
}

func (s *secretsService) CreateGitHubToken(ctx context.Context, opts Options) error {
	if err := validateSecretValue(opts.Token, "github token"); err != nil {
		return err
	}

	secretYaml, err := s.runner.RunCombinedOutput(ctx, "kubectl", "create", "secret", "generic", "github-token",
		"--from-literal=token="+opts.Token,
		"--namespace", "argocd",
		"--dry-run=client", "-o", "yaml")
	if err != nil {
		return ybyerrors.Wrap(err, ybyerrors.ErrCodeExec, "erro ao gerar secret github-token")
	}

	err = s.runner.RunStdin(ctx, string(secretYaml), "kubectl", "apply", "-f", "-")
	if err != nil {
		return ybyerrors.Wrap(err, ybyerrors.ErrCodeExec, "erro ao aplicar secret github-token")
	}

	return nil
}

func (s *secretsService) BackupKeys(ctx context.Context, opts Options) (string, error) {
	out, err := s.runner.RunCombinedOutput(ctx, "kubectl", "get", "secret", "-n", "sealed-secrets", "-l", "sealedsecrets.bitnami.com/sealed-secrets-key=active", "-o", "name")
	keyName := strings.TrimSpace(string(out))

	if err != nil || keyName == "" {
		return "", ybyerrors.New(ybyerrors.ErrCodeExec, "chave não encontrada").
			WithHint("Verifique se o controller sealed-secrets está instalado e com chaves ativas")
	}

	keyName = strings.ReplaceAll(keyName, "secret/", "")

	err = s.fs.MkdirAll(filepath.Dir(opts.OutputPath), 0755)
	if err != nil {
		return "", ybyerrors.Wrap(err, ybyerrors.ErrCodeIO, "erro ao criar diretório")
	}

	bkpData, err := s.runner.RunCombinedOutput(ctx, "kubectl", "get", "secret", keyName, "-n", "sealed-secrets", "-o", "yaml")
	if err != nil {
		return "", ybyerrors.Wrap(err, ybyerrors.ErrCodeExec, "erro ao buscar backup do kubernetes")
	}

	err = s.fs.WriteFile(opts.OutputPath, bkpData, 0600)
	if err != nil {
		return "", ybyerrors.Wrap(err, ybyerrors.ErrCodeIO, "erro ao salvar backup")
	}

	// Calcular e salvar HMAC-SHA256 do backup
	hmacKey := deriveHMACKey(opts.OutputPath)
	hmacValue := computeHMAC(bkpData, hmacKey)
	hmacPath := opts.OutputPath + ".hmac"
	if err := s.fs.WriteFile(hmacPath, []byte(hmacValue), 0600); err != nil {
		return "", ybyerrors.Wrap(err, ybyerrors.ErrCodeIO, "erro ao salvar HMAC do backup")
	}

	return keyName, nil
}

func (s *secretsService) RestoreKeys(ctx context.Context, opts Options) error {
	_, err := s.fs.Stat(opts.OutputPath)
	if err != nil {
		return ybyerrors.Wrap(err, ybyerrors.ErrCodeIO, "arquivo de backup não encontrado")
	}

	// Verificar integridade HMAC do backup
	hmacPath := opts.OutputPath + ".hmac"
	if _, statErr := s.fs.Stat(hmacPath); statErr == nil {
		backupData, readErr := s.fs.ReadFile(opts.OutputPath)
		if readErr != nil {
			return ybyerrors.Wrap(readErr, ybyerrors.ErrCodeIO, "erro ao ler backup para verificação de integridade")
		}
		storedHMAC, readErr := s.fs.ReadFile(hmacPath)
		if readErr != nil {
			return ybyerrors.Wrap(readErr, ybyerrors.ErrCodeIO, "erro ao ler arquivo HMAC")
		}

		hmacKey := deriveHMACKey(opts.OutputPath)
		expectedHMAC := computeHMAC(backupData, hmacKey)
		if string(storedHMAC) != expectedHMAC {
			return ybyerrors.New(ybyerrors.ErrCodeValidation, "verificação de integridade falhou: HMAC do backup não confere. O arquivo pode ter sido adulterado")
		}
	} else {
		slog.Warn("arquivo HMAC não encontrado, pulando verificação de integridade", "path", hmacPath)
	}

	if err := s.runner.Run(ctx, "kubectl", "create", "ns", "sealed-secrets"); err != nil {
		slog.Warn("falha ao criar namespace sealed-secrets (pode já existir)", "erro", err)
	}

	err = s.runner.Run(ctx, "kubectl", "apply", "-f", opts.OutputPath)
	if err != nil {
		return ybyerrors.Wrap(err, ybyerrors.ErrCodeExec, "erro ao aplicar chave")
	}

	err = s.runner.Run(ctx, "kubectl", "delete", "pod", "-n", "sealed-secrets", "-l", "app.kubernetes.io/name=sealed-secrets")
	if err != nil {
		return ybyerrors.Wrap(err, ybyerrors.ErrCodeExec, "erro ao reiniciar controller")
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
		return ybyerrors.Wrap(err, ybyerrors.ErrCodeExec, "erro ao encriptar com sops")
	}

	if err := s.fs.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return ybyerrors.Wrap(err, ybyerrors.ErrCodeIO, "erro ao criar diretório")
	}

	if err := s.fs.WriteFile(outputPath, encrypted, 0600); err != nil {
		return ybyerrors.Wrap(err, ybyerrors.ErrCodeIO, "erro ao salvar arquivo")
	}

	return nil
}

// GenerateAgeKey gera um par de chaves age e salva em outputPath.
// Retorna a chave pública gerada.
func (s *secretsService) GenerateAgeKey(ctx context.Context, outputPath string) (string, error) {
	if err := s.fs.MkdirAll(filepath.Dir(outputPath), 0700); err != nil {
		return "", ybyerrors.Wrap(err, ybyerrors.ErrCodeIO, "erro ao criar diretório para chave age")
	}

	out, err := s.runner.RunCombinedOutput(ctx, "age-keygen", "-o", outputPath)
	if err != nil {
		return "", ybyerrors.Wrap(err, ybyerrors.ErrCodeExec, "erro ao gerar chave age")
	}

	// age-keygen imprime "Public key: age1xxx..." na saída combinada
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "Public key: ") {
			return strings.TrimPrefix(line, "Public key: "), nil
		}
	}

	return "", ybyerrors.New(ybyerrors.ErrCodeExec, "chave pública não encontrada na saída do age-keygen")
}

// GenerateSecretYAML gera um Secret Kubernetes em formato YAML via dry-run.
func (s *secretsService) GenerateSecretYAML(ctx context.Context, name, namespace, key, value string) ([]byte, error) {
	if err := validateSecretValue(value, "secret"); err != nil {
		return nil, err
	}

	out, err := s.runner.RunCombinedOutput(ctx, "kubectl", "create", "secret", "generic", name,
		"--namespace", namespace,
		fmt.Sprintf("--from-literal=%s=%s", key, value),
		"--dry-run=client", "-o", "yaml")
	if err != nil {
		return nil, ybyerrors.Wrap(err, ybyerrors.ErrCodeExec, "falha ao gerar secret YAML")
	}
	return out, nil
}

// SealWithKubeseal encripta secretYAML com kubeseal e salva em outputPath.
func (s *secretsService) SealWithKubeseal(ctx context.Context, secretYAML []byte, outputPath string) error {
	sealedYaml, err := s.runner.RunStdinOutput(ctx, string(secretYAML), "kubeseal", "--format", "yaml")
	if err != nil {
		return ybyerrors.Wrap(err, ybyerrors.ErrCodeExec, "erro ao executar kubeseal")
	}

	if err := s.fs.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return ybyerrors.Wrap(err, ybyerrors.ErrCodeIO, "erro ao criar diretório")
	}

	if err := s.fs.WriteFile(outputPath, sealedYaml, 0600); err != nil {
		return ybyerrors.Wrap(err, ybyerrors.ErrCodeIO, "erro ao salvar sealed secret")
	}

	return nil
}

// RotateKeys remove a chave ativa do Sealed Secrets e reinicia o controller,
// forçando a geração de uma nova chave de encriptação.
func (s *secretsService) RotateKeys(ctx context.Context) error {
	// Remove a chave ativa atual
	err := s.runner.Run(ctx, "kubectl", "delete", "secret", "-n", "sealed-secrets",
		"-l", "sealedsecrets.bitnami.com/sealed-secrets-key=active")
	if err != nil {
		return ybyerrors.Wrap(err, ybyerrors.ErrCodeExec, "falha ao remover chave ativa do Sealed Secrets")
	}

	// Reinicia o controller para gerar nova chave
	err = s.runner.Run(ctx, "kubectl", "rollout", "restart", "deployment/sealed-secrets-controller", "-n", "sealed-secrets")
	if err != nil {
		return ybyerrors.Wrap(err, ybyerrors.ErrCodeExec, "falha ao reiniciar controller do Sealed Secrets")
	}

	// Aguarda o controller ficar pronto
	err = s.runner.Run(ctx, "kubectl", "rollout", "status", "deployment/sealed-secrets-controller", "-n", "sealed-secrets", "--timeout=60s")
	if err != nil {
		return ybyerrors.Wrap(err, ybyerrors.ErrCodeExec, "controller do Sealed Secrets não ficou pronto após reinício")
	}

	return nil
}

func (s *secretsService) sealAndSave(ctx context.Context, input []byte, outputFile string) error {
	sealedYaml, err := s.runner.RunStdinOutput(ctx, string(input), "kubeseal", "--controller-name=sealed-secrets", "--controller-namespace=sealed-secrets", "--format=yaml")
	if err != nil {
		return ybyerrors.Wrap(err, ybyerrors.ErrCodeExec, "erro ao executar kubeseal")
	}

	if err := s.fs.MkdirAll(filepath.Dir(outputFile), 0755); err != nil {
		return ybyerrors.Wrap(err, ybyerrors.ErrCodeIO, "erro ao criar diretório")
	}

	if err := s.fs.WriteFile(outputFile, sealedYaml, 0600); err != nil {
		return ybyerrors.Wrap(err, ybyerrors.ErrCodeIO, "erro ao salvar arquivo")
	}

	return nil
}
