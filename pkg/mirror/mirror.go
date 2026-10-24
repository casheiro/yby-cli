package mirror

import (
	"context"
	"embed"
	stdErrors "errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"time"

	ybyerrors "github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/pkg/retry"
	"github.com/casheiro/yby-cli/pkg/services/shared"
	"github.com/casheiro/yby-cli/pkg/ui"
)

//go:embed manifests/*.yaml
var manifestsFS embed.FS

// Forwarder define a interface para port-forwarding de pods K8s
type Forwarder interface {
	Start(ctx context.Context) (int, error)
	Stop()
	HealthCheck(ctx context.Context) error
}

// ForwarderFactory cria instâncias de Forwarder
type ForwarderFactory func(namespace, service string, targetPort int) (Forwarder, error)

// defaultForwarderFactory usa NewPortForwarder real
func defaultForwarderFactory(namespace, service string, targetPort int) (Forwarder, error) {
	return NewPortForwarder(namespace, service, targetPort)
}

// MirrorManager handles the in-cluster Git mirror for Hybrid GitOps
type MirrorManager struct {
	LocalPath string // Path to the local git repo (e.g. ".")
	Namespace string
	Runner    shared.Runner

	// ForwarderFactory permite injetar factory customizada para testes
	ForwarderFactory ForwarderFactory

	// mu protege acesso concorrente a forwarder e localPort
	mu sync.Mutex

	// Forwarder instance for local access
	forwarder Forwarder
	localPort int

	// healthInterval define o intervalo entre verificações de saúde (padrão 10s)
	healthInterval time.Duration
	// syncInterval define o intervalo entre syncs sem erro (padrão 5s)
	syncInterval time.Duration

	// Saida recebe as linhas do auto-sync para as pessoas (formato de ui.Steps); nil usa
	// os.Stdout.
	Saida io.Writer
	// smu serializa as linhas de Saida, escritas pelo laço de sync e pela reconexão.
	smu sync.Mutex
}

// linha escreve um evento do auto-sync, como uma linha de ui.Steps.
func (m *MirrorManager) linha(estado ui.EstadoEtapa, rotulo, nota string) {
	m.smu.Lock()
	defer m.smu.Unlock()
	w := m.Saida
	if w == nil {
		w = os.Stdout
	}
	ui.Steps(w, []ui.Etapa{{Rotulo: rotulo, Estado: estado, Nota: nota}}, 0)
}

// comCausa põe o motivo curto de err (ex.: "src refspec HEAD does not match any") antes da
// nota de uma linha de falha, para a pessoa saber o que corrigir sem religar com
// --log-level=debug. Vem primeiro porque Steps corta a nota na largura da linha.
func comCausa(err error, nota string) string {
	causa := shared.CausaCurta(err)
	// o ctrl+c que interrompe a espera não é motivo de falha
	if causa == "" || stdErrors.Is(err, context.Canceled) {
		return nota
	}
	return causa + " · " + nota
}

// Porta devolve a porta local do túnel até o git-server, ou 0 sem túnel.
func (m *MirrorManager) Porta() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.localPort
}

func NewManager(localPath string, runner shared.Runner) *MirrorManager {
	return &MirrorManager{
		LocalPath:        localPath,
		Namespace:        "yby-system",
		Runner:           runner,
		ForwarderFactory: defaultForwarderFactory,
	}
}

// EnsureGitServer deploys the git-server to the cluster if not present
func (m *MirrorManager) EnsureGitServer() error {
	// 1. Create Namespace
	nsTpl, err := manifestsFS.ReadFile("manifests/namespace.yaml")
	if err != nil {
		return ybyerrors.Wrap(err, ybyerrors.ErrCodeIO, "falha ao ler o manifesto do namespace do git server")
	}
	nsManifest := fmt.Sprintf(string(nsTpl), m.Namespace)

	if err := m.Runner.RunStdin(context.Background(), nsManifest, "kubectl", "apply", "-f", "-"); err != nil {
		return ybyerrors.Wrap(err, ybyerrors.ErrCodeExec, "falha ao criar o namespace do git server")
	}

	// 2. Apply Manifests (Deployment + Service)
	serverTpl, err := manifestsFS.ReadFile("manifests/git-server.yaml")
	if err != nil {
		return ybyerrors.Wrap(err, ybyerrors.ErrCodeIO, "falha ao ler o manifesto do git server")
	}
	manifest := fmt.Sprintf(string(serverTpl), m.Namespace, m.Namespace)

	if err := m.Runner.RunStdin(context.Background(), manifest, "kubectl", "apply", "-f", "-"); err != nil {
		return ybyerrors.Wrap(err, ybyerrors.ErrCodeExec, "falha ao aplicar os manifestos do git server")
	}

	// Wait for rollout
	if err := m.Runner.Run(context.Background(), "kubectl", "rollout", "status", "deployment/git-server", "-n", m.Namespace, "--timeout=60s"); err != nil {
		return ybyerrors.Wrap(err, ybyerrors.ErrCodeExec, "o git server não subiu")
	}

	return nil
}

// SetupTunnel establishes the port-forward tunnel
func (m *MirrorManager) SetupTunnel(ctx context.Context) error {
	factory := m.ForwarderFactory
	if factory == nil {
		factory = defaultForwarderFactory
	}

	pf, err := factory(m.Namespace, "git-server", 9418)
	if err != nil {
		return ybyerrors.Wrap(err, ybyerrors.ErrCodePortForward, "falha ao preparar o túnel até o git server")
	}

	port, err := pf.Start(ctx)
	if err != nil {
		return ybyerrors.Wrap(err, ybyerrors.ErrCodePortForward, "falha ao abrir o túnel até o git server")
	}

	m.mu.Lock()
	m.forwarder = pf
	m.localPort = port
	m.mu.Unlock()

	slog.Debug("túnel estabelecido", "local", port, "destino", "git-server:9418")
	return nil
}

// Sync pushes local changes to the in-cluster git server via the tunnel
func (m *MirrorManager) Sync() error {
	m.mu.Lock()
	port := m.localPort
	m.mu.Unlock()

	if port == 0 {
		return ybyerrors.New(ybyerrors.ErrCodePortForward, "o túnel até o git server não foi aberto")
	}

	remoteURL := fmt.Sprintf("git://localhost:%d/repo.git", port)

	if err := m.Runner.Run(context.Background(), "git", "push", remoteURL, "HEAD:main", "--force"); err != nil {
		return ybyerrors.Wrap(err, ybyerrors.ErrCodeExec, "o git push para o git server falhou")
	}

	return nil
}

// reconnect reconecta o port-forward com retry exponencial
func (m *MirrorManager) reconnect(ctx context.Context) error {
	m.linha(ui.EtapaAtual, "túnel", "conexão perdida · reconectando")
	slog.Debug("iniciando reconexão do port-forward", "namespace", m.Namespace)

	m.mu.Lock()
	if m.forwarder != nil {
		m.forwarder.Stop()
		m.forwarder = nil
		m.localPort = 0
	}
	m.mu.Unlock()

	tentativa := 0
	return retry.Do(ctx, retry.Options{
		InitialInterval:     2 * time.Second,
		MaxInterval:         30 * time.Second,
		MaxElapsedTime:      5 * time.Minute,
		RandomizationFactor: 0.3,
		Multiplier:          2.0,
	}, func() error {
		tentativa++
		err := m.SetupTunnel(ctx)
		if err != nil {
			slog.Debug("tentativa de reconexão falhou", "error", err)
			m.linha(ui.EtapaFalha, "túnel", comCausa(err, fmt.Sprintf("tentativa %d de reconexão falhou", tentativa)))
		}
		return err
	})
}

// backoffDelay calcula o delay com backoff exponencial para erros consecutivos de sync
func (m *MirrorManager) backoffDelay(errs int) time.Duration {
	base := 5 * time.Second
	shift := errs
	if shift > 5 {
		shift = 5
	}
	delay := base * time.Duration(1<<shift)
	if delay > 3*time.Minute {
		delay = 3 * time.Minute
	}
	return delay
}

// StartSyncLoop monitora mudanças e sincroniza automaticamente com health check e backoff
func (m *MirrorManager) StartSyncLoop(ctx context.Context) {
	syncInterval := m.syncInterval
	if syncInterval == 0 {
		syncInterval = 5 * time.Second
	}
	healthInterval := m.healthInterval
	if healthInterval == 0 {
		healthInterval = 10 * time.Second
	}

	syncTicker := time.NewTicker(syncInterval)
	healthTicker := time.NewTicker(healthInterval)
	defer syncTicker.Stop()
	defer healthTicker.Stop()

	m.linha(ui.EtapaAtual, "auto-sync", "envia o código a cada "+ui.FormatarDuracao(syncInterval)+" · ctrl+c encerra")

	// Sync inicial; o sucesso não ganha linha (o yby up já mostra a etapa do sync)
	if err := m.Sync(); err != nil {
		slog.Debug("sync inicial falhou", "error", err)
		m.linha(ui.EtapaFalha, "sync", comCausa(err, "tenta de novo em "+ui.FormatarDuracao(syncInterval)))
	}

	var consecutiveErrs int
	for {
		select {
		case <-ctx.Done():
			m.mu.Lock()
			if m.forwarder != nil {
				m.forwarder.Stop()
			}
			m.mu.Unlock()
			return

		case <-healthTicker.C:
			m.mu.Lock()
			fwd := m.forwarder
			m.mu.Unlock()
			if fwd != nil {
				if err := fwd.HealthCheck(ctx); err != nil {
					slog.Debug("health check falhou, iniciando reconexão", "error", err)
					if err := m.reconnect(ctx); err != nil {
						slog.Debug("reconexão falhou definitivamente", "error", err)
						m.linha(ui.EtapaFalha, "túnel", comCausa(err, "não reconectou · rode yby up de novo"))
					} else {
						m.linha(ui.EtapaFeita, "túnel", fmt.Sprintf("reconectado · localhost:%d", m.Porta()))
						consecutiveErrs = 0
					}
				}
			}

		case <-syncTicker.C:
			if err := m.Sync(); err != nil {
				consecutiveErrs++
				delay := m.backoffDelay(consecutiveErrs)
				slog.Debug("sync falhou", "erros_consecutivos", consecutiveErrs, "proximo_em", delay, "error", err)
				m.linha(ui.EtapaFalha, "sync", comCausa(err, fmt.Sprintf("tentativa %d · próxima em %s", consecutiveErrs, ui.FormatarDuracao(delay))))
				syncTicker.Reset(delay)
			} else {
				if consecutiveErrs > 0 {
					m.linha(ui.EtapaFeita, "sync", "de volta em dia")
					syncTicker.Reset(syncInterval)
				}
				consecutiveErrs = 0
			}
		}
	}
}
