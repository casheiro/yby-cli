package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	ybyerrors "github.com/casheiro/yby-cli/v5/pkg/errors"
	"github.com/casheiro/yby-cli/v5/pkg/retry"
	"github.com/casheiro/yby-cli/v5/pkg/scaffold"
	"github.com/casheiro/yby-cli/v5/pkg/services/shared"
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

	slog.Debug("checkpoint salvo", "fase", phase)
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
	// Etapas recebe o andamento de cada fase (ui.Progresso no comando); nil descarta.
	Etapas shared.Etapas
}

// Rótulos das etapas do bootstrap, uma por fase.
const (
	RotuloArgoCD  = "argo cd"
	RotuloSecrets = "secrets"
	RotuloRootApp = "root-app"
)

// Rotulos lista, na ordem, as etapas do bootstrap.
var Rotulos = []string{RotuloArgoCD, RotuloSecrets, RotuloRootApp}

// limiteHelm é o --timeout do helm install do Argo CD; a etapa mostra a cota dele.
const limiteHelm = 300 * time.Second

// intervaloCota é de quanto em quanto tempo a cota das esperas longas avança.
var intervaloCota = time.Second

// notaJaFeita marca a fase pulada porque o checkpoint diz que já terminou.
const notaJaFeita = "já feito · retomado do checkpoint"

func (s *BootstrapService) Run(ctx context.Context, opts BootstrapOptions) error {
	etapas := shared.OuMudas(opts.Etapas)
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
		slog.Debug("checkpoint detectado, retomando bootstrap", "fase_completa", cp.Phase)
	} else {
		cp = nil // checkpoint de outro contexto, ignorar
	}

	// fase roda uma fase como uma etapa: pulada quando o checkpoint diz que já terminou,
	// ✕ quando falha e ✓ com a nota devolvida quando passa.
	fase := func(phase BootstrapPhase, rotulo string, rodar func() (string, error)) error {
		etapas.Iniciar(rotulo)
		if phaseCompleted(cp, phase) {
			slog.Debug("pulando fase já completada", "fase", phase)
			etapas.Concluir(notaJaFeita)
			return nil
		}
		nota, err := rodar()
		if err != nil {
			etapas.Falhar("")
			return err
		}
		if err := s.saveCheckpoint(phase, opts); err != nil {
			slog.Warn("não foi possível salvar checkpoint", "fase", phase, "erro", err)
		}
		etapas.Concluir(nota)
		return nil
	}

	// 3. Fase 1: Bootstrap do Sistema (helm install do Argo CD, com a cota do --timeout)
	err = fase(PhaseSystem, RotuloArgoCD, func() (string, error) {
		parar := shared.AcompanharLimite(etapas, limiteHelm, intervaloCota)
		defer parar()
		return "argo-cd " + argoVersion, s.phaseSystemBootstrap(ctx, opts.Root, argoChart, argoVersion, ov)
	})
	if err != nil {
		return err
	}

	// 4. Fase 2: Configuração de Segredos; o que falta (controller, CRD, encriptação) não
	// interrompe o bootstrap e sai como alerta logo abaixo da etapa
	var avisoSecrets string
	err = fase(PhaseSecrets, RotuloSecrets, func() (string, error) {
		nota, aviso, err := s.phaseSecretsNota(ctx, opts.Root, opts.PlainSecrets)
		avisoSecrets = aviso
		return nota, err
	})
	if err != nil {
		return err
	}
	if avisoSecrets != "" {
		etapas.Avisar(avisoSecrets)
	}

	// 5. Fase 3: Bootstrap de Configuração
	err = fase(PhaseConfig, RotuloRootApp, func() (string, error) {
		return "aplicada", s.phaseConfigBootstrap(ctx, opts.Root, finalRepo, opts.Context, opts.Environment, ov)
	})
	if err != nil {
		return err
	}

	// Bootstrap completo — limpar checkpoint
	s.clearCheckpoint()

	return nil
}

func (s *BootstrapService) ensureToolsInstalled() error {
	for _, tool := range []string{"kubectl", "helm"} {
		if _, err := s.Runner.LookPath(tool); err != nil {
			return ybyerrors.New(ybyerrors.ErrCodeCmdNotFound, fmt.Sprintf("%s não encontrado", tool)).
				WithHint("rode yby setup para instalar as dependências")
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
			return ybyerrors.New(ybyerrors.ErrCodeValidation, "Variável GITHUB_REPO faltando").
				WithHint("defina GITHUB_REPO ou git.repoURL em .yby/blueprint.yaml")
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
	_, _, err := s.phaseSecretsNota(ctx, root, plainSecrets)
	return err
}

// phaseSecretsNota faz a fase de secrets e devolve a nota da etapa (a estratégia) e, se
// for o caso, o aviso para as pessoas (sem encriptação, controller ou CRD ausente).
func (s *BootstrapService) phaseSecretsNota(ctx context.Context, root string, plainSecrets bool) (nota, aviso string, err error) {
	if plainSecrets {
		slog.Debug("Modo plain-secrets ativo — secrets NÃO serão encriptados (apenas para dev local)")
		return "plain-secrets", "secrets sem encriptação · só para dev local", nil
	}

	strategy := s.detectSecretsStrategy(root)

	switch strategy {
	case "sops":
		// Verificar se sops e age estão disponíveis
		if _, err := s.Runner.LookPath("sops"); err != nil {
			return "", "", ybyerrors.New(ybyerrors.ErrCodeCmdNotFound, "estratégia SOPS configurada, mas 'sops' não encontrado no PATH").
				WithHint("instale o sops (github.com/getsops/sops) e rode o bootstrap de novo")
		}
		if _, err := s.Runner.LookPath("age"); err != nil {
			return "", "", ybyerrors.New(ybyerrors.ErrCodeCmdNotFound, "estratégia SOPS configurada, mas 'age' não encontrado no PATH").
				WithHint("instale o age (github.com/FiloSottile/age) e rode o bootstrap de novo")
		}
	case "external-secrets":
		// Verificar se há CRD do ESO instalado
		if err := s.Runner.Run(ctx, "kubectl", "get", "crd", "externalsecrets.external-secrets.io"); err != nil {
			slog.Debug("CRD do External Secrets Operator não encontrado", "erro", err)
			return strategy, "CRD do External Secrets Operator ainda ausente no cluster", nil
		}
	default:
		// sealed-secrets: verificar controller
		if err := s.Runner.Run(ctx, "kubectl", "get", "deployment", "-n", "sealed-secrets", "sealed-secrets"); err != nil {
			slog.Debug("controller sealed-secrets não encontrado", "erro", err)
			return strategy, "controller do sealed-secrets ainda não instalado no cluster", nil
		}
	}

	return strategy, "", nil
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
