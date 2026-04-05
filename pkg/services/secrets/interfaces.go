package secrets

import "context"

// Options define os parâmetros para operações de secrets.
type Options struct {
	Provider   string
	SecretVal  string
	OutputPath string
	Token      string
}

// Service define o contrato do serviço de gestão de secrets.
type Service interface {
	GenerateWebhook(ctx context.Context, opts Options) (string, error)
	GenerateMinIO(ctx context.Context, opts Options) (string, error)
	CreateGitHubToken(ctx context.Context, opts Options) error
	BackupKeys(ctx context.Context, opts Options) (string, error)
	RestoreKeys(ctx context.Context, opts Options) error
	EncryptWithSOPS(ctx context.Context, ageRecipient string, secretYaml []byte, outputPath string) error
	GenerateAgeKey(ctx context.Context, outputPath string) (string, error)
	GenerateSecretYAML(ctx context.Context, name, namespace, key, value string) ([]byte, error)
	SealWithKubeseal(ctx context.Context, secretYAML []byte, outputPath string) error
	RotateKeys(ctx context.Context) error
}
