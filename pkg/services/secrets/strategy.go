package secrets

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/casheiro/yby-cli/pkg/services/shared"
)

// SecretStrategy define a interface para diferentes estratégias de gestão de secrets.
type SecretStrategy interface {
	// Name retorna o nome da estratégia.
	Name() string
	// GenerateSecret gera um secret encriptado conforme a estratégia.
	GenerateSecret(ctx context.Context, opts SecretOpts) error
	// ScaffoldTemplates retorna os nomes de templates que devem ser incluídos no scaffold.
	ScaffoldTemplates() []string
}

// SecretOpts contém as opções para geração de um secret.
type SecretOpts struct {
	Name         string
	Namespace    string
	Data         map[string]string
	OutputPath   string
	AgeRecipient string // para estratégia SOPS: chave pública age (opcional se .sops.yaml existir)
	StoreRef     string // para estratégia ESO: nome do ClusterSecretStore (padrão: cluster-secret-store)
}

// SealedSecretsStrategy implementa a estratégia SealedSecrets (Bitnami).
type SealedSecretsStrategy struct {
	runner shared.Runner
	fs     shared.Filesystem
}

// NewSealedSecretsStrategy cria uma nova instância da estratégia SealedSecrets.
func NewSealedSecretsStrategy(runner shared.Runner, fs shared.Filesystem) *SealedSecretsStrategy {
	return &SealedSecretsStrategy{runner: runner, fs: fs}
}

func (s *SealedSecretsStrategy) Name() string { return "sealed-secrets" }

func (s *SealedSecretsStrategy) ScaffoldTemplates() []string {
	return []string{"sealed-secret"}
}

func (s *SealedSecretsStrategy) GenerateSecret(ctx context.Context, opts SecretOpts) error {
	svc := &secretsService{runner: s.runner, fs: s.fs}
	args := []string{"create", "secret", "generic", opts.Name, "--namespace", opts.Namespace, "--dry-run=client", "-o", "yaml"}
	for k, v := range opts.Data {
		args = append(args, "--from-literal="+k+"="+v)
	}
	secretYaml, err := s.runner.RunCombinedOutput(ctx, "kubectl", args...)
	if err != nil {
		return err
	}
	return svc.sealAndSave(ctx, secretYaml, opts.OutputPath)
}

// ExternalSecretsStrategy implementa a estratégia External Secrets Operator (ESO).
type ExternalSecretsStrategy struct {
	runner shared.Runner
	fs     shared.Filesystem
}

// NewExternalSecretsStrategy cria uma nova instância da estratégia ESO.
func NewExternalSecretsStrategy(runner shared.Runner, fs shared.Filesystem) *ExternalSecretsStrategy {
	return &ExternalSecretsStrategy{runner: runner, fs: fs}
}

func (s *ExternalSecretsStrategy) Name() string { return "external-secrets" }

func (s *ExternalSecretsStrategy) ScaffoldTemplates() []string {
	return []string{"external-secret"}
}

func (s *ExternalSecretsStrategy) GenerateSecret(_ context.Context, opts SecretOpts) error {
	if opts.OutputPath == "" {
		// ESO sem caminho de saída: apenas referência, nada a gerar localmente.
		return nil
	}

	storeRef := opts.StoreRef
	if storeRef == "" {
		storeRef = "cluster-secret-store"
	}

	var dataSection string
	for k, v := range opts.Data {
		dataSection += fmt.Sprintf("  - secretKey: %s\n    remoteRef:\n      key: %s\n", k, v)
	}

	yaml := fmt.Sprintf(`apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: %s
  namespace: %s
spec:
  refreshInterval: 1h
  secretStoreRef:
    name: %s
    kind: ClusterSecretStore
  target:
    name: %s
    creationPolicy: Owner
  data:
%s`, opts.Name, opts.Namespace, storeRef, opts.Name, dataSection)

	if err := s.fs.MkdirAll(filepath.Dir(opts.OutputPath), 0755); err != nil {
		return fmt.Errorf("erro ao criar diretório: %w", err)
	}

	return s.fs.WriteFile(opts.OutputPath, []byte(yaml), 0644)
}

// SOPSStrategy implementa a estratégia SOPS + age.
type SOPSStrategy struct {
	runner shared.Runner
	fs     shared.Filesystem
}

// NewSOPSStrategy cria uma nova instância da estratégia SOPS.
func NewSOPSStrategy(runner shared.Runner, fs shared.Filesystem) *SOPSStrategy {
	return &SOPSStrategy{runner: runner, fs: fs}
}

func (s *SOPSStrategy) Name() string { return "sops" }

func (s *SOPSStrategy) ScaffoldTemplates() []string {
	return []string{"sops-secret"}
}

func (s *SOPSStrategy) GenerateSecret(ctx context.Context, opts SecretOpts) error {
	args := []string{"create", "secret", "generic", opts.Name, "--namespace", opts.Namespace, "--dry-run=client", "-o", "yaml"}
	for k, v := range opts.Data {
		args = append(args, "--from-literal="+k+"="+v)
	}

	secretYaml, err := s.runner.RunCombinedOutput(ctx, "kubectl", args...)
	if err != nil {
		return fmt.Errorf("erro ao gerar secret com kubectl: %w", err)
	}

	svc := &secretsService{runner: s.runner, fs: s.fs}
	return svc.EncryptWithSOPS(ctx, opts.AgeRecipient, secretYaml, opts.OutputPath)
}

// NewStrategy cria a estratégia de secrets apropriada com base no nome.
func NewStrategy(name string, runner shared.Runner, fs shared.Filesystem) SecretStrategy {
	switch name {
	case "sealed-secrets":
		return NewSealedSecretsStrategy(runner, fs)
	case "sops":
		return NewSOPSStrategy(runner, fs)
	default:
		return NewExternalSecretsStrategy(runner, fs)
	}
}
