package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	ybyerrors "github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/pkg/retry"
	"github.com/casheiro/yby-cli/pkg/scaffold"
	"github.com/casheiro/yby-cli/pkg/services/shared"
	"gopkg.in/yaml.v3"
)

// BootstrapPhase representa uma fase do bootstrap.
type BootstrapPhase string

const (
	PhaseSystem  BootstrapPhase = "system"
	PhaseSecrets BootstrapPhase = "secrets"
	PhaseConfig  BootstrapPhase = "config"
)

// BootstrapCheckpoint armazena o estado de progresso do bootstrap para retomada.
type BootstrapCheckpoint struct {
	Phase       BootstrapPhase `json:"phase"`
	CompletedAt string         `json:"completed_at"`
	Root        string         `json:"root"`
	Environment string         `json:"environment"`
}

// checkpointPath retorna o caminho do arquivo de checkpoint.
const checkpointFile = ".yby/bootstrap-state.json"

func (s *BootstrapService) checkpointPath() (string, error) {
	home, err := s.FS.UserHomeDir()
	if err != nil {
		return "", ybyerrors.Wrap(err, ybyerrors.ErrCodeIO, "não foi possível obter diretório home")
	}
	return filepath.Join(home, checkpointFile), nil
}

// loadCheckpoint carrega o checkpoint salvo, se existir.
func (s *BootstrapService) loadCheckpoint() (*BootstrapCheckpoint, error) {
	path, err := s.checkpointPath()
	if err != nil {
		return nil, err
	}

	data, err := s.FS.ReadFile(path)
	if err != nil {
		return nil, nil // arquivo não existe → sem checkpoint
	}

	var cp BootstrapCheckpoint
	if err := json.Unmarshal(data, &cp); err != nil {
		slog.Warn("checkpoint corrompido, ignorando", "erro", err)
		return nil, nil
	}

	return &cp, nil
}

// saveCheckpoint persiste o checkpoint após uma fase completada.
func (s *BootstrapService) saveCheckpoint(phase BootstrapPhase, opts BootstrapOptions) error {
	path, err := s.checkpointPath()
	if err != nil {
		return err
	}

	if err := s.FS.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return ybyerrors.Wrap(err, ybyerrors.ErrCodeIO, "não foi possível criar diretório de checkpoint")
	}

	cp := BootstrapCheckpoint{
		Phase:       phase,
		CompletedAt: time.Now().UTC().Format(time.RFC3339),
		Root:        opts.Root,
		Environment: opts.Environment,
	}

	data, err := json.MarshalIndent(cp, "", "  ")
	if err != nil {
		return ybyerrors.Wrap(err, ybyerrors.ErrCodeIO, "não foi possível serializar checkpoint")
	}

	if err := s.FS.WriteFile(path, data, 0o600); err != nil {
		return ybyerrors.Wrap(err, ybyerrors.ErrCodeIO, "não foi possível salvar checkpoint")
	}

	slog.Info("checkpoint salvo", "fase", phase)
	return nil
}

// clearCheckpoint remove o arquivo de checkpoint após bootstrap completo.
func (s *BootstrapService) clearCheckpoint() {
	path, err := s.checkpointPath()
	if err != nil {
		return
	}
	// WriteFile com conteúdo vazio não serve; usamos remoção via FS.
	// Como Filesystem não tem Remove, sobrescrevemos com JSON vazio.
	if err := s.FS.WriteFile(path, []byte("{}"), 0o600); err != nil {
		slog.Warn("falha ao limpar checkpoint", "caminho", path, "erro", err)
	}
}

// phaseCompleted verifica se uma fase já foi completada no checkpoint.
func phaseCompleted(cp *BootstrapCheckpoint, phase BootstrapPhase) bool {
	if cp == nil {
		return false
	}
	order := map[BootstrapPhase]int{
		PhaseSystem:  1,
		PhaseSecrets: 2,
		PhaseConfig:  3,
	}
	return order[cp.Phase] >= order[phase]
}

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
	Root         string
	RepoURL      string
	Context      string
	Environment  string
	PlainSecrets bool
	Overrides    *scaffold.EnterpriseOverrides
}

func (s *BootstrapService) Run(ctx context.Context, opts BootstrapOptions) error {
	// 0. Resolve Blueprint for versions (com suporte a enterprise overrides)
	ov := opts.Overrides
	argoVersion := "5.51.6"
	if ov != nil {
		argoVersion = ov.ResolveChartVersion("argocd", argoVersion)
	}
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

	// Carregar checkpoint para retomada
	cp, err := s.loadCheckpoint()
	if err != nil {
		slog.Warn("não foi possível carregar checkpoint", "erro", err)
	}
	if cp != nil && cp.Root == opts.Root && cp.Environment == opts.Environment {
		slog.Info("checkpoint detectado, retomando bootstrap", "fase_completa", cp.Phase)
	} else {
		cp = nil // checkpoint de outro contexto, ignorar
	}

	// 3. Fase 1: Bootstrap do Sistema
	if !phaseCompleted(cp, PhaseSystem) {
		if err := s.phaseSystemBootstrap(ctx, opts.Root, argoChart, argoVersion, ov); err != nil {
			return err
		}
		if err := s.saveCheckpoint(PhaseSystem, opts); err != nil {
			slog.Warn("não foi possível salvar checkpoint", "fase", PhaseSystem, "erro", err)
		}
	} else {
		slog.Info("pulando fase já completada", "fase", PhaseSystem)
	}

	// 4. Fase 2: Configuração de Segredos
	if !phaseCompleted(cp, PhaseSecrets) {
		if err := s.phaseSecrets(ctx, opts.Root, finalRepo, opts.PlainSecrets); err != nil {
			return err
		}
		if err := s.saveCheckpoint(PhaseSecrets, opts); err != nil {
			slog.Warn("não foi possível salvar checkpoint", "fase", PhaseSecrets, "erro", err)
		}
	} else {
		slog.Info("pulando fase já completada", "fase", PhaseSecrets)
	}

	// 5. Fase 3: Bootstrap de Configuração
	if !phaseCompleted(cp, PhaseConfig) {
		if err := s.phaseConfigBootstrap(ctx, opts.Root, finalRepo, opts.Context, opts.Environment, ov); err != nil {
			return err
		}
		if err := s.saveCheckpoint(PhaseConfig, opts); err != nil {
			slog.Warn("não foi possível salvar checkpoint", "fase", PhaseConfig, "erro", err)
		}
	} else {
		slog.Info("pulando fase já completada", "fase", PhaseConfig)
	}

	// Bootstrap completo — limpar checkpoint
	s.clearCheckpoint()

	return nil
}

func (s *BootstrapService) ensureToolsInstalled() error {
	for _, tool := range []string{"kubectl", "helm"} {
		if _, err := s.Runner.LookPath(tool); err != nil {
			return ybyerrors.New(ybyerrors.ErrCodeCmdNotFound, fmt.Sprintf("%s não encontrado", tool))
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
			return ybyerrors.New(ybyerrors.ErrCodeValidation, "Variável GITHUB_REPO faltando")
		}
	}
	return nil
}

func (s *BootstrapService) phaseSystemBootstrap(ctx context.Context, root, chart, version string, ov *scaffold.EnterpriseOverrides) error {
	// Helm Repo Add (com suporte a mirror enterprise)
	repoURL := "https://argoproj.github.io/argo-helm"
	if ov != nil {
		repoURL = ov.ResolveHelmRepo(repoURL)
	}
	err := retry.DoWithDefault(ctx, func() error {
		return s.Runner.Run(ctx, "helm", "repo", "add", "argo", repoURL)
	})
	if err != nil {
		return err
	}

	// Resolver namespaces via enterprise overrides (backward-compat: sem overrides retorna original)
	resolveNS := func(ns string) string {
		if ov != nil {
			return ov.ResolveNamespace(ns)
		}
		return ns
	}

	// Namespaces
	for _, ns := range []string{resolveNS("argocd"), resolveNS("argo"), resolveNS("argo-events")} {
		nsToCreate := ns
		err := retry.DoWithDefault(ctx, func() error {
			return s.K8s.CreateNamespace(ctx, nsToCreate)
		})
		if err != nil {
			return err
		}
	}

	argocdNS := resolveNS("argocd")
	// Helm Install CD
	return retry.DoWithDefault(ctx, func() error {
		return s.Runner.Run(ctx, "helm", "upgrade", "--install", "argocd", chart,
			"--namespace", argocdNS,
			"--version", version,
			"-f", filepath.Join(root, "config/cluster-values.yaml"),
			"--wait", "--atomic", "--timeout", "300s")
	})
}

func (s *BootstrapService) phaseSecrets(ctx context.Context, root, repoURL string, plainSecrets bool) error {
	if plainSecrets {
		slog.Warn("Modo plain-secrets ativo — secrets NÃO serão encriptados (apenas para dev local)")
		return nil
	}

	strategy := s.detectSecretsStrategy(root)

	switch strategy {
	case "sops":
		// Verificar se sops e age estão disponíveis
		if _, err := s.Runner.LookPath("sops"); err != nil {
			return ybyerrors.New(ybyerrors.ErrCodeCmdNotFound, "estratégia SOPS configurada, mas 'sops' não encontrado no PATH").
				WithHint("Instale em: https://github.com/getsops/sops")
		}
		if _, err := s.Runner.LookPath("age"); err != nil {
			return ybyerrors.New(ybyerrors.ErrCodeCmdNotFound, "estratégia SOPS configurada, mas 'age' não encontrado no PATH").
				WithHint("Instale em: https://github.com/FiloSottile/age")
		}
	case "external-secrets":
		// Verificar se há CRD do ESO instalado
		if err := s.Runner.Run(ctx, "kubectl", "get", "crd", "externalsecrets.external-secrets.io"); err != nil {
			slog.Warn("CRD do External Secrets Operator não encontrado", "erro", err)
		}
	default:
		// sealed-secrets: verificar controller
		if err := s.Runner.Run(ctx, "kubectl", "get", "deployment", "-n", "sealed-secrets", "sealed-secrets"); err != nil {
			slog.Warn("controller sealed-secrets não encontrado", "erro", err)
		}
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

func (s *BootstrapService) phaseConfigBootstrap(ctx context.Context, root, repoURL, ctxFlag, envEnv string, ov *scaffold.EnterpriseOverrides) error {
	argocdNS := "argocd"
	if ov != nil {
		argocdNS = ov.ResolveNamespace(argocdNS)
	}

	// Applying Root App
	manifest := filepath.Join(root, "manifests/argocd/root-app.yaml")
	err := retry.DoWithDefault(ctx, func() error {
		return s.K8s.ApplyManifest(ctx, manifest, argocdNS)
	})
	if err != nil {
		return err
	}

	// Handle local patching
	isLocal := (ctxFlag == "local" || envEnv == "local")
	if isLocal {
		patch := `{"spec": {"source": {"repoURL": "git://git-server.yby-system.svc:9418/repo.git"}}}`
		err := retry.DoWithDefault(ctx, func() error {
			return s.K8s.PatchApplication(ctx, "root-app", argocdNS, patch)
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// WaitHealthy aguarda uma Application do ArgoCD ficar com status Healthy.
func (s *BootstrapService) WaitHealthy(ctx context.Context, name, namespace string, timeoutSeconds int) error {
	return s.K8s.WaitApplicationHealthy(ctx, name, namespace, timeoutSeconds)
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
