package bootstrap

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/casheiro/yby-cli/pkg/retry"
	"github.com/casheiro/yby-cli/pkg/services/shared"
	"gopkg.in/yaml.v3"
)

type BootstrapService struct {
	Runner shared.Runner
	FS     shared.Filesystem
	K8s    K8sClient
}

func NewService(r shared.Runner, f shared.Filesystem, k K8sClient) *BootstrapService {
	return &BootstrapService{
		Runner: r,
		FS:     f,
		K8s:    k,
	}
}

type BootstrapOptions struct {
	Root        string
	RepoURL     string
	Context     string
	Environment string
}

func (s *BootstrapService) Run(ctx context.Context, opts BootstrapOptions) error {
	// 0. Resolve Blueprint for versions
	argoVersion := "5.51.6"
	argoChart := "argo/argo-cd"
	blueprintRepo := s.getRepoURLFromBlueprint(opts.Root)

	// 1. Initial Checks
	if err := s.ensureToolsInstalled(); err != nil {
		return err
	}

	if err := s.checkEnvVars(opts.Context, opts.Environment, blueprintRepo); err != nil {
		return err
	}

	// 2. Resolve final repo URL
	finalRepo := os.Getenv("GITHUB_REPO")
	if finalRepo == "" {
		finalRepo = blueprintRepo
	}

	// 3. Fase 1: Bootstrap do Sistema
	if err := s.phaseSystemBootstrap(ctx, opts.Root, argoChart, argoVersion); err != nil {
		return err
	}

	// 4. Fase 2: Configuração de Segredos
	if err := s.phaseSecrets(ctx, opts.Root, finalRepo); err != nil {
		return err
	}

	// 5. Fase 3: Bootstrap de Configuração
	if err := s.phaseConfigBootstrap(ctx, opts.Root, finalRepo, opts.Context, opts.Environment); err != nil {
		return err
	}

	return nil
}

func (s *BootstrapService) ensureToolsInstalled() error {
	for _, tool := range []string{"kubectl", "helm"} {
		if _, err := s.Runner.LookPath(tool); err != nil {
			return fmt.Errorf("%s não encontrado", tool)
		}
	}
	return nil
}

func (s *BootstrapService) checkEnvVars(contextFlag, envEnv, blueprintRepo string) error {
	// Simplified logic from bootstrap_cluster.go
	repo := os.Getenv("GITHUB_REPO")
	if repo == "" && blueprintRepo == "" {
		isLocal := (contextFlag == "local" || envEnv == "local")
		if !isLocal {
			return fmt.Errorf("Variável GITHUB_REPO faltando")
		}
	}
	return nil
}

func (s *BootstrapService) phaseSystemBootstrap(ctx context.Context, root, chart, version string) error {
	// Helm Repo Add
	err := retry.DoWithDefault(ctx, func() error {
		return s.Runner.Run(ctx, "helm", "repo", "add", "argo", "https://argoproj.github.io/argo-helm")
	})
	if err != nil {
		return err
	}

	// Namespaces
	for _, ns := range []string{"argocd", "argo", "argo-events"} {
		err := retry.DoWithDefault(ctx, func() error {
			return s.K8s.CreateNamespace(ctx, ns)
		})
		if err != nil {
			return err
		}
	}

	// Helm Install CD
	return retry.DoWithDefault(ctx, func() error {
		return s.Runner.Run(ctx, "helm", "upgrade", "--install", "argocd", chart,
			"--namespace", "argocd",
			"--version", version,
			"-f", filepath.Join(root, "config/cluster-values.yaml"),
			"--wait", "--timeout", "300s")
	})
}

func (s *BootstrapService) phaseSecrets(ctx context.Context, root, repoURL string) error {
	strategy := s.detectSecretsStrategy(root)

	switch strategy {
	case "sops":
		// Verificar se sops e age estão disponíveis
		if _, err := s.Runner.LookPath("sops"); err != nil {
			return fmt.Errorf("estratégia SOPS configurada, mas 'sops' não encontrado no PATH. Instale em: https://github.com/getsops/sops")
		}
		if _, err := s.Runner.LookPath("age"); err != nil {
			return fmt.Errorf("estratégia SOPS configurada, mas 'age' não encontrado no PATH. Instale em: https://github.com/FiloSottile/age")
		}
	case "external-secrets":
		// Verificar se há CRD do ESO instalado
		_ = s.Runner.Run(ctx, "kubectl", "get", "crd", "externalsecrets.external-secrets.io")
	default:
		// sealed-secrets: verificar controller
		_ = s.Runner.Run(ctx, "kubectl", "get", "deployment", "-n", "sealed-secrets", "sealed-secrets")
	}

	return nil
}

// detectSecretsStrategy lê a estratégia de secrets do blueprint do projeto.
func (s *BootstrapService) detectSecretsStrategy(root string) string {
	path := filepath.Join(root, ".yby", "blueprint.yaml")
	data, err := s.FS.ReadFile(path)
	if err != nil {
		return "sealed-secrets"
	}

	var bp struct {
		Prompts []struct {
			ID      string      `yaml:"id"`
			Default interface{} `yaml:"default"`
		} `yaml:"prompts"`
	}

	if err := yaml.Unmarshal(data, &bp); err != nil {
		return "sealed-secrets"
	}

	for _, p := range bp.Prompts {
		if p.ID == "secrets.strategy" {
			if val, ok := p.Default.(string); ok {
				return val
			}
		}
	}

	return "sealed-secrets"
}

func (s *BootstrapService) phaseConfigBootstrap(ctx context.Context, root, repoURL, ctxFlag, envEnv string) error {
	// Applying Root App
	manifest := filepath.Join(root, "manifests/argocd/root-app.yaml")
	err := retry.DoWithDefault(ctx, func() error {
		return s.K8s.ApplyManifest(ctx, manifest, "argocd")
	})
	if err != nil {
		return err
	}

	// Handle local patching
	isLocal := (ctxFlag == "local" || envEnv == "local")
	if isLocal {
		patch := `{"spec": {"source": {"repoURL": "git://git-server.yby-system.svc:9418/repo.git"}}}`
		err := retry.DoWithDefault(ctx, func() error {
			return s.K8s.PatchApplication(ctx, "root-app", "argocd", patch)
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *BootstrapService) getRepoURLFromBlueprint(root string) string {
	path := filepath.Join(root, ".yby/blueprint.yaml")
	data, err := s.FS.ReadFile(path)
	if err != nil {
		return ""
	}

	var bp struct {
		Prompts []struct {
			ID      string      `yaml:"id"`
			Default interface{} `yaml:"default"`
		} `yaml:"prompts"`
	}

	if err := yaml.Unmarshal(data, &bp); err != nil {
		return ""
	}

	for _, p := range bp.Prompts {
		if p.ID == "git.repoURL" {
			if val, ok := p.Default.(string); ok {
				return val
			}
		}
	}

	return ""
}
