package environment

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/casheiro/yby-cli/v5/pkg/errors"
	"github.com/casheiro/yby-cli/v5/pkg/services/bootstrap"
	"github.com/casheiro/yby-cli/v5/pkg/services/shared"
)

// validEnvironmentTypes lista todos os tipos de ambiente reconhecidos.
var validEnvironmentTypes = []string{"local", "remote", "eks", "aks", "gke"}

// IsValidEnvironmentType informa se t é um tipo de ambiente reconhecido.
func IsValidEnvironmentType(t string) bool {
	for _, v := range validEnvironmentTypes {
		if v == t {
			return true
		}
	}
	return false
}

// EnvironmentService orchestrates the 'up' command logic
type EnvironmentService struct {
	Runner    shared.Runner
	FS        shared.Filesystem
	Cluster   ClusterManager
	Mirror    MirrorService
	Bootstrap BootstrapService
	// Etapas recebe o andamento do up local (ui.Progresso no comando); nil descarta.
	Etapas shared.Etapas
}

// Rótulos das etapas do up local; o bootstrap acrescenta as suas (bootstrap.Rotulos).
const (
	RotuloGitServer   = "git server"
	RotuloTunel       = "túnel"
	RotuloSync        = "sync inicial"
	RotuloConvergente = "convergência"
)

// limiteConvergencia é quanto o up espera a root-app ficar Healthy.
const limiteConvergencia = 180 * time.Second

// intervaloCota é de quanto em quanto tempo a cota das esperas longas avança.
var intervaloCota = time.Second

// PlanoLocal devolve, na ordem, os rótulos das etapas do up local do cluster.
func PlanoLocal(cluster string) []string {
	plano := []string{RotuloCluster(cluster), RotuloGitServer, RotuloTunel, RotuloSync}
	plano = append(plano, bootstrap.Rotulos...)
	return append(plano, RotuloConvergente)
}

// RotuloCluster é o rótulo da etapa que cria ou inicia o cluster.
func RotuloCluster(nome string) string { return "cluster " + nome }

// avisoComCausa monta "o que houve · motivo curto de err · o que acontece agora".
func avisoComCausa(oque string, err error, depois string) string {
	if causa := shared.CausaCurta(err); causa != "" {
		oque += " · " + causa
	}
	return oque + " · " + depois
}

// porta é implementada por espelhos que expõem a porta local do túnel.
type porta interface{ Porta() int }

func NewEnvironmentService(
	runner shared.Runner,
	fs shared.Filesystem,
	cluster ClusterManager,
	mirror MirrorService,
	bs BootstrapService,
) *EnvironmentService {
	return &EnvironmentService{
		Runner:    runner,
		FS:        fs,
		Cluster:   cluster,
		Mirror:    mirror,
		Bootstrap: bs,
	}
}

func (s *EnvironmentService) Up(ctx context.Context, opts UpOptions) error {
	if opts.Environment == "local" {
		return s.runLocalUp(ctx, opts)
	}
	return s.runRemoteUp(ctx, opts)
}

func (s *EnvironmentService) runLocalUp(ctx context.Context, opts UpOptions) error {
	etapas := shared.OuMudas(s.Etapas)

	// 1. Check dependencies
	if _, err := s.Runner.LookPath("k3d"); err != nil {
		return errors.New(errors.ErrCodeCmdNotFound, "k3d não encontrado").
			WithHint("rode yby setup para instalar as dependências")
	}

	// 2. Cluster Lifecycle
	etapas.Iniciar(RotuloCluster(opts.ClusterName))
	exists, err := s.Cluster.Exists(ctx, opts.ClusterName)
	if err != nil || !exists {
		slog.Debug("Cluster não encontrado, criando", "cluster", opts.ClusterName)
		// Try to find config file
		configFile := "" // Logic for config path could be injected or resolved
		if err := s.Cluster.Create(ctx, opts.ClusterName, configFile); err != nil {
			etapas.Falhar("")
			return fmt.Errorf("falha ao criar cluster: %w", err)
		}
		etapas.Concluir("criado")
	} else {
		slog.Debug("Cluster já existe, garantindo start", "cluster", opts.ClusterName)
		if err := s.Cluster.Start(ctx, opts.ClusterName); err != nil {
			etapas.Falhar("")
			return fmt.Errorf("falha ao iniciar cluster: %w", err)
		}
		etapas.Concluir("iniciado")
	}

	// 3. Mirror & Sync
	etapas.Iniciar(RotuloGitServer)
	if err := s.Mirror.EnsureGitServer(); err != nil {
		etapas.Falhar("")
		return err
	}
	etapas.Concluir("")

	etapas.Iniciar(RotuloTunel)
	if err := s.Mirror.SetupTunnel(ctx); err != nil {
		etapas.Falhar("")
		return err
	}
	nota := ""
	if p, ok := s.Mirror.(porta); ok && p.Porta() > 0 {
		nota = fmt.Sprintf("localhost:%d", p.Porta())
	}
	etapas.Concluir(nota)

	etapas.Iniciar(RotuloSync)
	if err := s.Mirror.Sync(); err != nil {
		// o auto-sync tenta de novo: não interrompe o up, então é alerta, com o motivo
		slog.Debug("Falha no sync inicial", "error", err)
		etapas.Concluir("adiado")
		etapas.Avisar(avisoComCausa("o sync inicial falhou", err, "o auto-sync tenta de novo"))
	} else {
		etapas.Concluir("")
	}

	// 4. Bootstrap (marca as próprias etapas: argo cd, secrets, root-app)
	bsOpts := bootstrap.BootstrapOptions{
		Root:         opts.Root,
		Context:      "local",
		Environment:  "local",
		PlainSecrets: opts.PlainSecrets,
		Etapas:       s.Etapas,
	}
	if err := s.Bootstrap.Run(ctx, bsOpts); err != nil {
		return fmt.Errorf("falha no bootstrap: %w", err)
	}

	// 4.5 Aguardar convergência do ArgoCD, com a cota do tempo limite
	etapas.Iniciar(RotuloConvergente)
	parar := shared.AcompanharLimite(etapas, limiteConvergencia, intervaloCota)
	err = s.Bootstrap.WaitHealthy(ctx, "root-app", "argocd", int(limiteConvergencia/time.Second))
	parar()
	if err != nil {
		// NÃO retorna erro — o cluster pode convergir depois; por isso é alerta, não falha
		slog.Debug("Timeout aguardando convergência do ArgoCD", "error", err)
		etapas.Concluir("tempo esgotado")
		etapas.Avisar("a root-app ainda não convergiu · confira com yby status")
	} else {
		etapas.Concluir("root-app healthy")
	}

	// 5. Start Sync Loop (Async), salvo quando o chamador o inicia
	if !opts.SyncPeloChamador {
		go s.Mirror.StartSyncLoop(ctx)
	}

	slog.Debug("Ambiente pronto e sincronizado")
	return nil
}

func (s *EnvironmentService) runRemoteUp(ctx context.Context, opts UpOptions) error {
	slog.Info("Ambiente remoto detectado", "ambiente", opts.Environment)

	// 1. Verificar conectividade com o cluster
	slog.Info("Verificando conexão com o cluster remoto")
	output, err := s.Runner.RunCombinedOutput(ctx, "kubectl", "cluster-info")
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeClusterOffline,
			"falha ao conectar com cluster remoto").
			WithHint("Verifique se o kubeconfig está configurado corretamente com 'yby doctor'")
	}
	slog.Debug("Cluster info obtido", "output", strings.TrimSpace(string(output)))

	// 2. Verificar namespace existe (criar se necessário)
	ns := opts.Namespace
	if ns == "" {
		ns = "default"
	}
	slog.Info("Verificando namespace", "namespace", ns)
	_, err = s.Runner.RunCombinedOutput(ctx, "kubectl", "get", "namespace", ns)
	if err != nil {
		slog.Info("Namespace não encontrado, criando", "namespace", ns)
		if createErr := s.Runner.Run(ctx, "kubectl", "create", "namespace", ns); createErr != nil {
			return errors.Wrap(createErr, errors.ErrCodeExec,
				fmt.Sprintf("falha ao criar namespace '%s'", ns)).
				WithHint("Verifique se você tem permissão para criar namespaces no cluster")
		}
	}

	// 3. Verificar Argo CD está rodando
	slog.Info("Verificando Argo CD")
	argoOutput, err := s.Runner.RunCombinedOutput(ctx,
		"kubectl", "get", "pods", "-n", "argocd", "-l", "app.kubernetes.io/name=argocd-server", "--no-headers")
	if err != nil || strings.TrimSpace(string(argoOutput)) == "" {
		slog.Warn("Argo CD não detectado no cluster",
			"dica", "Execute 'yby bootstrap cluster' para instalar a stack GitOps")
	} else {
		slog.Info("Argo CD detectado no cluster")
	}

	slog.Info("Ambiente remoto verificado com sucesso", "ambiente", opts.Environment)
	return nil
}
