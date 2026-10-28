package cmd

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	stdErr "errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"regexp"

	"strings"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/casheiro/yby-cli/v5/pkg/errors"
	"github.com/casheiro/yby-cli/v5/pkg/executor"
	"github.com/casheiro/yby-cli/v5/pkg/services/bootstrap"
	"github.com/casheiro/yby-cli/v5/pkg/services/environment"
	"github.com/casheiro/yby-cli/v5/pkg/services/network"
	"github.com/casheiro/yby-cli/v5/pkg/services/shared"
	"github.com/casheiro/yby-cli/v5/pkg/testutil"
	"github.com/casheiro/yby-cli/v5/pkg/ui"
)

// Goldens das saídas de bootstrap vps/cluster, up, access, destroy, uninstall e upgrade
// (T027). Os serviços rodam com mocks; o relógio das etapas só anda quando um mock
// "trabalha", para as durações saírem iguais em toda máquina.

// relogioFalso é o relógio das etapas nos goldens.
type relogioFalso struct {
	mu sync.Mutex
	t  time.Time
}

func novoRelogioFalso() *relogioFalso { return &relogioFalso{t: quandoGolden} }

func (r *relogioFalso) agora() time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.t
}

func (r *relogioFalso) avancar(d time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.t = r.t.Add(d)
}

// progressosFalsos guarda os ui.Progresso criados pelos comandos durante o teste.
type progressosFalsos struct {
	mu    sync.Mutex
	lista []*ui.Progresso
}

func (p *progressosFalsos) ultimo() *ui.Progresso {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.lista) == 0 {
		return nil
	}
	return p.lista[len(p.lista)-1]
}

// usarProgressoFalso troca novoProgresso por um fora do terminal (as linhas saem uma vez,
// sem códigos de controle), com o relógio rel e largura 64.
func usarProgressoFalso(t *testing.T, rel *relogioFalso) *progressosFalsos {
	t.Helper()
	criados := &progressosFalsos{}
	anterior := novoProgresso
	t.Cleanup(func() { novoProgresso = anterior })
	novoProgresso = func(w io.Writer, rotulos ...string) *ui.Progresso {
		p := ui.NovoProgressoCom(w, ui.OpcoesProgresso{
			Terminal: func(io.Writer) bool { return false },
			Relogio:  rel.agora,
			Largura:  64,
		}, rotulos...)
		criados.mu.Lock()
		criados.lista = append(criados.lista, p)
		criados.mu.Unlock()
		return p
	}
	return criados
}

// usarSaidaEscura é usarSaidaTexto com o tema escuro em truecolor (as cores entram no
// golden como códigos ANSI).
func usarSaidaEscura(t *testing.T) {
	t.Helper()
	usarSaidaTexto(t, false)
	escuro := lipgloss.HasDarkBackground()
	t.Cleanup(func() { lipgloss.SetHasDarkBackground(escuro) })
	ui.Configurar(ui.Opcoes{Tema: ui.TemaEscuro})
	lipgloss.SetColorProfile(termenv.TrueColor)
}

// cenariosDeSaida são os modos visuais conferidos: tema escuro e ASCII sem cor.
var cenariosDeSaida = []struct {
	sufixo   string
	preparar func(t *testing.T)
}{
	{"escuro", usarSaidaEscura},
	{"ascii", func(t *testing.T) { usarSaidaTexto(t, true) }},
}

// comErro junta a saída do comando ao erro como root.go o escreve (linha em branco e
// ui.WriteError), para o golden mostrar o que a pessoa vê.
func comErro(t *testing.T, saida string, err error) string {
	t.Helper()
	var ye *errors.YbyError
	require.True(t, stdErr.As(err, &ye), "o comando deveria devolver *errors.YbyError: %v", err)
	assert.NotEmpty(t, ye.GetHint(), "o erro deveria trazer uma dica")
	return saida + "\n" + ui.RenderError(ye)
}

// conferirSemEstiloAntigo falha com restos da saída antiga além dos de conferirSemRestos.
func conferirSemEstiloAntigo(t *testing.T, saida string) {
	t.Helper()
	conferirSemRestos(t, saida)
	for _, resto := range []string{"🚀", "🎉", "👉", "📡", "🔄", "📦", "📄", "💣", "🗑", "👋", "💡", "🛑", "🌍", "ℹ️", "📝", "📂", "⚙️", "->", "---"} {
		if strings.Contains(saida, resto) {
			t.Errorf("saída ainda tem %q:\n%s", resto, saida)
		}
	}
}

// ========================================================
// yby up
// ========================================================

// k8sFalso implementa bootstrap.K8sClient; cada chamada pode avançar o relógio.
type k8sFalso struct {
	rel     *relogioFalso
	esperar func(ctx context.Context) error
}

func (k *k8sFalso) WaitPodReady(context.Context, string, string, int) error { return nil }
func (k *k8sFalso) WaitCRD(context.Context, string, int) error              { return nil }
func (k *k8sFalso) NamespaceExists(context.Context, string) (bool, error)   { return true, nil }
func (k *k8sFalso) CreateNamespace(context.Context, string) error           { return nil }
func (k *k8sFalso) ApplyManifest(context.Context, string, string) error {
	k.rel.avancar(1100 * time.Millisecond)
	return nil
}
func (k *k8sFalso) PatchApplication(context.Context, string, string, string) error {
	k.rel.avancar(300 * time.Millisecond)
	return nil
}
func (k *k8sFalso) WaitApplicationHealthy(ctx context.Context, _, _ string, _ int) error {
	if k.esperar != nil {
		return k.esperar(ctx)
	}
	k.rel.avancar(25 * time.Second)
	return nil
}

// espelhoFalso implementa environment.MirrorService com a porta do túnel.
type espelhoFalso struct{ rel *relogioFalso }

func (e *espelhoFalso) EnsureGitServer() error { e.rel.avancar(4 * time.Second); return nil }
func (e *espelhoFalso) SetupTunnel(context.Context) error {
	e.rel.avancar(1200 * time.Millisecond)
	return nil
}
func (e *espelhoFalso) Sync() error                   { e.rel.avancar(600 * time.Millisecond); return nil }
func (e *espelhoFalso) StartSyncLoop(context.Context) {}
func (e *espelhoFalso) Porta() int                    { return 51234 }

// blueprintFalso é o FS do bootstrap: blueprint com o repositório e sem checkpoint.
func blueprintFalso() *testutil.MockFilesystem {
	return &testutil.MockFilesystem{
		ReadFileFunc: func(name string) ([]byte, error) {
			if strings.Contains(name, "bootstrap-state") {
				return nil, fs.ErrNotExist
			}
			return []byte("prompts:\n  - id: git.repoURL\n    default: https://github.com/casheiro/infra.git\n"), nil
		},
	}
}

// prepararUp troca as dependências do yby up por mocks: cluster que precisa ser criado
// (criar devolve criarErr), espelho e bootstrap de verdade com runner, FS e K8s falsos.
// No helm install do Argo CD, quadro recebe as etapas com a cota em andamento.
func prepararUp(t *testing.T, rel *relogioFalso, progs *progressosFalsos, criarErr error, quadro *string) {
	t.Helper()
	t.Setenv("YBY_ENV", "local")
	t.Setenv("YBY_CLUSTER_NAME", "")
	t.Setenv("GITHUB_REPO", "")

	origFactory, origRaiz, origLook := newLocalEnvironmentService, acharRaizInfra, lookPath
	t.Cleanup(func() {
		newLocalEnvironmentService, acharRaizInfra, lookPath = origFactory, origRaiz, origLook
	})
	dir := t.TempDir()
	acharRaizInfra = func() (string, error) { return dir, nil }
	// sem kubectl, o up não escreve o relatório do status no fim
	lookPath = func(f string) (string, error) { return "", fmt.Errorf("%s ausente", f) }

	newLocalEnvironmentService = func(root string, _ io.Writer) *environment.EnvironmentService {
		runner := &testutil.MockRunner{
			LookPathFunc: func(f string) (string, error) { return "/usr/bin/" + f, nil },
			RunFunc: func(ctx context.Context, name string, args ...string) error {
				if name == "helm" && len(args) > 0 && args[0] == "upgrade" {
					// o helm install do Argo CD gasta 24% do --timeout de 300 s
					rel.avancar(72 * time.Second)
					p := progs.ultimo()
					p.Avancar(0.24)
					if quadro != nil {
						*quadro = p.Quadro()
					}
					rel.avancar(10 * time.Second)
				}
				if name == "kubectl" {
					rel.avancar(400 * time.Millisecond)
				}
				return nil
			},
		}
		cluster := &mockClusterManager{
			existsFunc: func(context.Context, string) (bool, error) { return false, nil },
			createFunc: func(context.Context, string, string) error {
				rel.avancar(38 * time.Second)
				return criarErr
			},
		}
		bs := bootstrap.NewService(runner, blueprintFalso(), &k8sFalso{rel: rel})
		return environment.NewEnvironmentService(runner, &testutil.MockFilesystem{}, cluster, &espelhoFalso{rel: rel}, bs)
	}
}

func TestUpGolden(t *testing.T) {
	for _, c := range cenariosDeSaida {
		t.Run(c.sufixo, func(t *testing.T) {
			c.preparar(t)
			rel := novoRelogioFalso()
			progs := usarProgressoFalso(t, rel)
			var quadro string
			prepararUp(t, rel, progs, nil, &quadro)

			b := saidaDoComando(t, upCmd)
			require.NoError(t, upCmd.RunE(upCmd, []string{}))
			saida := b.String()

			conferirGolden(t, "up_local_"+c.sufixo, saida)
			conferirSemEstiloAntigo(t, saida)
			// o quadro do meio do caminho: feitas, a atual com a cota e as pendentes
			var tela bytes.Buffer
			ui.Title(&tela, "up", "ambiente local")
			conferirGolden(t, "up_andamento_"+c.sufixo, tela.String()+quadro)
			assert.Contains(t, quadro, "24%")
			assert.NotContains(t, saida, "\x1b[1A", "fora do terminal não há redesenho")
		})
	}
}

func TestUpGolden_FalhaAoCriarCluster(t *testing.T) {
	usarSaidaTexto(t, false)
	rel := novoRelogioFalso()
	progs := usarProgressoFalso(t, rel)
	prepararUp(t, rel, progs, fmt.Errorf("porta 6443 já está em uso"), nil)

	b := saidaDoComando(t, upCmd)
	err := upCmd.RunE(upCmd, []string{})
	require.Error(t, err)
	saida := comErro(t, b.String(), err)
	conferirGolden(t, "up_falha_cluster", saida)
	conferirSemEstiloAntigo(t, saida)
}

func TestUpGolden_Remoto(t *testing.T) {
	usarSaidaTexto(t, false)
	t.Setenv("YBY_ENV", "staging")
	origRaiz, origLook := acharRaizInfra, lookPath
	t.Cleanup(func() { acharRaizInfra, lookPath = origRaiz, origLook })
	dir := t.TempDir()
	acharRaizInfra = func() (string, error) { return dir, nil }
	lookPath = func(f string) (string, error) { return "", fmt.Errorf("%s ausente", f) }

	b := saidaDoComando(t, upCmd)
	require.NoError(t, upCmd.RunE(upCmd, []string{}))
	conferirGolden(t, "up_remoto", b.String())
	conferirSemEstiloAntigo(t, b.String())
}

// ========================================================
// yby bootstrap cluster
// ========================================================

func prepararBootstrapCluster(t *testing.T, rel *relogioFalso, raizErr error, fsys shared.Filesystem, runner shared.Runner) {
	t.Helper()
	t.Setenv("YBY_ENV", "")
	t.Setenv("GITHUB_REPO", "")
	origFactory, origRaiz := newBootstrapClusterService, acharRaizInfra
	t.Cleanup(func() { newBootstrapClusterService, acharRaizInfra = origFactory, origRaiz })
	acharRaizInfra = func() (string, error) { return "/infra", raizErr }
	newBootstrapClusterService = func(shared.Runner, shared.Filesystem) *bootstrap.BootstrapService {
		return bootstrap.NewService(runner, fsys, &k8sFalso{rel: rel})
	}
	bootstrapClusterCmd.SetContext(context.Background())
}

func runnerDoBootstrap(rel *relogioFalso) *testutil.MockRunner {
	return &testutil.MockRunner{
		LookPathFunc: func(f string) (string, error) { return "/usr/bin/" + f, nil },
		RunFunc: func(ctx context.Context, name string, args ...string) error {
			if name == "helm" && len(args) > 0 && args[0] == "upgrade" {
				rel.avancar(84 * time.Second)
			}
			return nil
		},
	}
}

func TestBootstrapClusterGolden(t *testing.T) {
	for _, c := range cenariosDeSaida {
		t.Run(c.sufixo, func(t *testing.T) {
			c.preparar(t)
			rel := novoRelogioFalso()
			usarProgressoFalso(t, rel)
			prepararBootstrapCluster(t, rel, nil, blueprintFalso(), runnerDoBootstrap(rel))

			b := saidaDoComando(t, bootstrapClusterCmd)
			require.NoError(t, bootstrapClusterCmd.RunE(bootstrapClusterCmd, []string{}))
			conferirGolden(t, "bootstrap_cluster_"+c.sufixo, b.String())
			conferirSemEstiloAntigo(t, b.String())
		})
	}
}

func TestBootstrapClusterGolden_FalhaSecrets(t *testing.T) {
	usarSaidaTexto(t, false)
	rel := novoRelogioFalso()
	usarProgressoFalso(t, rel)
	// estratégia sops sem o sops no PATH, e sem a raiz .yby/
	fsys := &testutil.MockFilesystem{ReadFileFunc: func(name string) ([]byte, error) {
		if strings.Contains(name, "bootstrap-state") {
			return nil, fs.ErrNotExist
		}
		return []byte("prompts:\n  - id: git.repoURL\n    default: https://x/infra.git\n  - id: secrets.strategy\n    default: sops\n"), nil
	}}
	runner := runnerDoBootstrap(rel)
	runner.LookPathFunc = func(f string) (string, error) {
		if f == "sops" {
			return "", fmt.Errorf("sops ausente")
		}
		return "/usr/bin/" + f, nil
	}
	prepararBootstrapCluster(t, rel, fmt.Errorf("sem .yby"), fsys, runner)

	b := saidaDoComando(t, bootstrapClusterCmd)
	err := bootstrapClusterCmd.RunE(bootstrapClusterCmd, []string{})
	require.Error(t, err)
	saida := comErro(t, b.String(), err)
	conferirGolden(t, "bootstrap_cluster_falha", saida)
	conferirSemEstiloAntigo(t, saida)
}

// ========================================================
// yby bootstrap vps
// ========================================================

// prepararVps troca o executor por um LocalExecutor com script falso (falha no script
// cujo rótulo é falhaEm) e o kubeconfig por um que só avança o relógio.
func prepararVps(t *testing.T, rel *relogioFalso, local bool, falhaEm string, sshErr error) {
	t.Helper()
	t.Setenv("K3S_TOKEN", "token-de-teste")
	t.Setenv("CLUSTER_NAME", "")
	origLocal, origSSH, origKube := novoExecutorLocal, novoExecutorSSH, configurarKubeconfig
	origHost, origUser, origPort, origVersao := vpsHost, vpsUser, vpsPort, k3sVersion
	t.Cleanup(func() {
		novoExecutorLocal, novoExecutorSSH, configurarKubeconfig = origLocal, origSSH, origKube
		vpsHost, vpsUser, vpsPort, k3sVersion = origHost, origUser, origPort, origVersao
		_ = bootstrapVpsCmd.Flags().Set("local", "false")
	})
	// o diretório do pacote não tem config/cluster-values.yaml: vale a versão padrão
	_, err := os.Stat("config/cluster-values.yaml")
	require.True(t, os.IsNotExist(err), "o teste espera a versão padrão do k3s")

	duracoes := map[string]time.Duration{
		"requisitos mínimos":     300 * time.Millisecond,
		"sistema e dependências": 64 * time.Second,
		"firewall (ufw)":         2 * time.Second,
		"docker":                 41 * time.Second,
		"k3s v1.31.2+k3s1":       93 * time.Second,
	}
	falso := func(etapas shared.Etapas) executor.Executor {
		atual := ""
		return &executor.LocalExecutor{
			Etapas: rotuloVisto{Etapas: etapas, visto: &atual},
			// a etapa "requisitos mínimos" lê o /proc/meminfo: 8 GB
			LerArquivo: func(path string) ([]byte, error) {
				rel.avancar(duracoes["requisitos mínimos"])
				return []byte("MemTotal:        8000000 kB\n"), nil
			},
			Executar: func(script string) ([]byte, error) {
				rel.avancar(duracoes[atual])
				if atual == falhaEm {
					return []byte("Reading package lists...\nE: Unable to locate package iptables-persistent\n"), fmt.Errorf("exit status 100")
				}
				return []byte("ok"), nil
			},
		}
	}
	novoExecutorLocal = falso
	novoExecutorSSH = func(user, host, port string, etapas shared.Etapas) (executor.Executor, error) {
		rel.avancar(800 * time.Millisecond)
		if sshErr != nil {
			return nil, sshErr
		}
		return falso(etapas), nil
	}
	configurarKubeconfig = func(executor.Executor, string, shared.Runner) error {
		rel.avancar(1500 * time.Millisecond)
		return nil
	}
	if local {
		require.NoError(t, bootstrapVpsCmd.Flags().Set("local", "true"))
	} else {
		vpsHost, vpsUser, vpsPort = "203.0.113.10", "ubuntu", "22"
	}
}

// rotuloVisto repassa as etapas e guarda o rótulo da atual, para o script falso saber
// qual etapa está rodando.
type rotuloVisto struct {
	shared.Etapas
	visto *string
}

func (r rotuloVisto) Iniciar(rotulo string) {
	*r.visto = rotulo
	r.Etapas.Iniciar(rotulo)
}

func TestBootstrapVpsGolden(t *testing.T) {
	for _, c := range cenariosDeSaida {
		t.Run(c.sufixo, func(t *testing.T) {
			c.preparar(t)
			rel := novoRelogioFalso()
			usarProgressoFalso(t, rel)
			prepararVps(t, rel, false, "", nil)

			b := saidaDoComando(t, bootstrapVpsCmd)
			require.NoError(t, bootstrapVpsCmd.RunE(bootstrapVpsCmd, []string{}))
			conferirGolden(t, "bootstrap_vps_"+c.sufixo, b.String())
			conferirSemEstiloAntigo(t, b.String())
		})
	}
}

func TestBootstrapVpsGolden_LocalComFalhaNoScript(t *testing.T) {
	usarSaidaTexto(t, false)
	rel := novoRelogioFalso()
	usarProgressoFalso(t, rel)
	prepararVps(t, rel, true, "sistema e dependências", nil)

	b := saidaDoComando(t, bootstrapVpsCmd)
	err := bootstrapVpsCmd.RunE(bootstrapVpsCmd, []string{})
	require.Error(t, err)
	saida := comErro(t, b.String(), err)
	conferirGolden(t, "bootstrap_vps_falha_script", saida)
	conferirSemEstiloAntigo(t, saida)
}

func TestBootstrapVpsGolden_FalhaSSH(t *testing.T) {
	usarSaidaTexto(t, false)
	rel := novoRelogioFalso()
	usarProgressoFalso(t, rel)
	prepararVps(t, rel, false, "", fmt.Errorf("ssh: handshake failed: knownhosts: key is unknown"))

	b := saidaDoComando(t, bootstrapVpsCmd)
	err := bootstrapVpsCmd.RunE(bootstrapVpsCmd, []string{})
	require.Error(t, err)
	saida := comErro(t, b.String(), err)
	conferirGolden(t, "bootstrap_vps_falha_ssh", saida)
	conferirSemEstiloAntigo(t, saida)
}

// ========================================================
// yby access
// ========================================================

// redeFalsa implementa network.ClusterNetworkManager sem cluster.
type redeFalsa struct {
	contextoErr error
	semMinio    bool
}

func (r *redeFalsa) GetCurrentContext() (string, error) {
	return "k3d-yby-local", r.contextoErr
}
func (r *redeFalsa) GetSecretValue(_ context.Context, _, ns, secret, key string) (string, error) {
	switch {
	case secret == "argocd-initial-admin-secret":
		return "Xk2p9sQ", nil
	case secret == "minio-secret" && key == "rootUser":
		return "minio", nil
	case secret == "minio-secret":
		return "minio123", nil
	}
	return "", fmt.Errorf("sem secret %s/%s", ns, secret)
}
func (r *redeFalsa) HasService(_ context.Context, _, ns, svc string) bool {
	if svc == "minio" && ns == "storage" {
		return !r.semMinio
	}
	return svc == "system-kube-prometheus-sta-prometheus"
}

// PortForward termina na hora, como se o túnel fechasse: o access volta sem ctrl+c.
func (r *redeFalsa) PortForward(context.Context, string, string, string, string) error {
	return nil
}
func (r *redeFalsa) CreateToken(context.Context, string, string, string, string) (string, error) {
	return "eyJhbGciOiJSUzI1NiIsImtpZCI6InRlc3RlIn0.e30.assinatura", nil
}
func (r *redeFalsa) KillPortForward(string) {}

// containerFalso implementa network.LocalContainerManager.
type containerFalso struct{ disponivel bool }

func (c *containerFalso) IsAvailable() bool                  { return c.disponivel }
func (c *containerFalso) StartGrafana(context.Context) error { return nil }

func prepararAccess(t *testing.T, rede *redeFalsa, docker bool) {
	t.Helper()
	origRede, origContainer := newNetworkAdapter, newContainerAdapter
	t.Cleanup(func() {
		newNetworkAdapter, newContainerAdapter = origRede, origContainer
		_ = accessCmd.Flags().Set("context", "")
	})
	newNetworkAdapter = func() network.ClusterNetworkManager { return rede }
	newContainerAdapter = func() network.LocalContainerManager { return &containerFalso{disponivel: docker} }
}

func TestAccessGolden(t *testing.T) {
	for _, c := range cenariosDeSaida {
		t.Run(c.sufixo, func(t *testing.T) {
			c.preparar(t)
			prepararAccess(t, &redeFalsa{}, true)
			b := saidaDoComando(t, accessCmd)
			require.NoError(t, accessCmd.RunE(accessCmd, []string{}))
			conferirGolden(t, "access_"+c.sufixo, b.String())
			conferirSemEstiloAntigo(t, b.String())
		})
	}
}

func TestAccessGolden_SemMinioNemDocker(t *testing.T) {
	usarSaidaTexto(t, false)
	prepararAccess(t, &redeFalsa{semMinio: true}, false)
	b := saidaDoComando(t, accessCmd)
	require.NoError(t, accessCmd.RunE(accessCmd, []string{}))
	conferirGolden(t, "access_sem_minio_docker", b.String())
}

func TestAccessGolden_FalhaContexto(t *testing.T) {
	usarSaidaTexto(t, false)
	prepararAccess(t, &redeFalsa{contextoErr: fmt.Errorf("current-context is not set")}, true)
	b := saidaDoComando(t, accessCmd)
	err := accessCmd.RunE(accessCmd, []string{})
	require.Error(t, err)
	saida := comErro(t, b.String(), err)
	conferirGolden(t, "access_falha_contexto", saida)
	conferirSemEstiloAntigo(t, saida)
}

// ========================================================
// yby destroy
// ========================================================

func prepararDestroy(t *testing.T, rel *relogioFalso, env string, deleteErr error) {
	t.Helper()
	t.Setenv("YBY_ENV", env)
	t.Setenv("YBY_CLUSTER_NAME", "")
	origCluster, origPrompter := newDestroyClusterManager, prompter
	t.Cleanup(func() {
		newDestroyClusterManager, prompter = origCluster, origPrompter
		_ = destroyCmd.Flags().Set("yes-destroy-production", "false")
	})
	newDestroyClusterManager = func() environment.ClusterManager {
		return &mockClusterManager{deleteFunc: func(context.Context, string) error {
			rel.avancar(3200 * time.Millisecond)
			return deleteErr
		}}
	}
	destroyCmd.SetContext(context.Background())
}

func TestDestroyGolden(t *testing.T) {
	for _, c := range cenariosDeSaida {
		t.Run(c.sufixo, func(t *testing.T) {
			c.preparar(t)
			rel := novoRelogioFalso()
			usarProgressoFalso(t, rel)
			prepararDestroy(t, rel, "local", nil)
			b := saidaDoComando(t, destroyCmd)
			require.NoError(t, destroyCmd.RunE(destroyCmd, []string{}))
			conferirGolden(t, "destroy_"+c.sufixo, b.String())
			conferirSemEstiloAntigo(t, b.String())
		})
	}
}

func TestDestroyGolden_ProducaoConfirmadaEFalha(t *testing.T) {
	usarSaidaTexto(t, false)
	rel := novoRelogioFalso()
	usarProgressoFalso(t, rel)
	prepararDestroy(t, rel, "staging", fmt.Errorf("cluster 'yby-staging' não existe"))
	t.Setenv("YBY_CLUSTER_NAME", "yby-staging")
	require.NoError(t, destroyCmd.Flags().Set("yes-destroy-production", "true"))
	pediu := ""
	prompter = &mockPrompter{confirmarNomeFunc: func(acao, nome string) (bool, error) {
		pediu = nome
		return true, nil
	}}

	b := saidaDoComando(t, destroyCmd)
	err := destroyCmd.RunE(destroyCmd, []string{})
	require.Error(t, err)
	assert.Equal(t, "yby-staging", pediu, "ambiente não-local confirma digitando o nome do cluster dele")
	saida := comErro(t, b.String(), err)
	conferirGolden(t, "destroy_falha", saida)
	conferirSemEstiloAntigo(t, saida)
}

// ========================================================
// yby uninstall
// ========================================================

func prepararUninstall(t *testing.T, removerErr error) {
	t.Helper()
	saveUninstallMocks(t)
	origVersao := Version
	t.Cleanup(func() { Version = origVersao })
	Version = "v5.6.1"
	osExecutable = func() (string, error) { return "/usr/local/bin/yby", nil }
	osRemove = func(string) error { return removerErr }
	responderUninstall(true, nil)
}

func TestUninstallGolden(t *testing.T) {
	for _, c := range cenariosDeSaida {
		t.Run(c.sufixo, func(t *testing.T) {
			c.preparar(t)
			prepararUninstall(t, nil)
			b := saidaDoComando(t, uninstallCmd)
			require.NoError(t, uninstallCmd.RunE(uninstallCmd, []string{}))
			conferirGolden(t, "uninstall_"+c.sufixo, b.String())
			conferirSemEstiloAntigo(t, b.String())
		})
	}
}

func TestUninstallGolden_Falhas(t *testing.T) {
	casos := []struct {
		nome string
		err  error
	}{
		{"uninstall_falha_permissao", fmt.Errorf("remove /usr/local/bin/yby: permission denied")},
		{"uninstall_falha_outra", fmt.Errorf("remove /usr/local/bin/yby: text file busy")},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			usarSaidaTexto(t, false)
			prepararUninstall(t, c.err)
			b := saidaDoComando(t, uninstallCmd)
			err := uninstallCmd.RunE(uninstallCmd, []string{})
			require.Error(t, err)
			saida := comErro(t, b.String(), err)
			conferirGolden(t, c.nome, saida)
			conferirSemEstiloAntigo(t, saida)
		})
	}
}

func TestUninstallGolden_Cancelado(t *testing.T) {
	usarSaidaTexto(t, false)
	prepararUninstall(t, nil)
	responderUninstall(false, nil)
	removeu := false
	osRemove = func(string) error { removeu = true; return nil }
	b := saidaDoComando(t, uninstallCmd)
	require.NoError(t, uninstallCmd.RunE(uninstallCmd, []string{}))
	assert.False(t, removeu)
	conferirGolden(t, "uninstall_cancelado", b.String())
}

// ========================================================
// yby upgrade
// ========================================================

// releaseFalso monta um release com o tar.gz do binário e o checksums.txt; corromper
// troca o conteúdo baixado para o checksum não conferir.
func prepararUpgrade(t *testing.T, rel *relogioFalso, atual, disponivel string, corromper bool) {
	t.Helper()
	origGet, origInstalar, origVersao := httpGet, instalarBinario, Version
	origSistema, origArq := sistemaUpgrade, arquiteturaUpgrade
	t.Cleanup(func() {
		httpGet, instalarBinario, Version = origGet, origInstalar, origVersao
		sistemaUpgrade, arquiteturaUpgrade = origSistema, origArq
		for _, f := range []string{"check", "force"} {
			_ = upgradeCmd.Flags().Set(f, "false")
		}
		_ = upgradeCmd.Flags().Set("version", "")
	})
	Version = atual
	// a mesma plataforma em toda máquina, para o golden não variar
	sistemaUpgrade, arquiteturaUpgrade = "linux", "amd64"

	pacote := createTestTarGz(t, "yby", []byte("binário novo"))
	asset := fmt.Sprintf("yby_%s_linux_amd64.tar.gz", disponivel)
	somaBytes := sha256.Sum256(pacote)
	soma := hex.EncodeToString(somaBytes[:])
	if corromper {
		pacote = append(pacote, 0)
	}
	release := fmt.Sprintf(`{"tag_name":%q,"assets":[{"name":%q,"browser_download_url":"https://exemplo/asset"},{"name":"checksums.txt","browser_download_url":"https://exemplo/checksums"}]}`, disponivel, asset)
	httpGet = func(url string) (*http.Response, error) {
		corpo := []byte(release)
		switch url {
		case "https://exemplo/asset":
			rel.avancar(2400 * time.Millisecond)
			corpo = pacote
		case "https://exemplo/checksums":
			corpo = []byte(soma + "  " + asset + "\n")
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(corpo))}, nil
	}
	instalarBinario = func(binario []byte) (string, error) {
		rel.avancar(200 * time.Millisecond)
		if string(binario) != "binário novo" {
			return "", fmt.Errorf("binário inesperado")
		}
		return "/usr/local/bin/yby", nil
	}
}

func TestUpgradeGolden(t *testing.T) {
	for _, c := range cenariosDeSaida {
		t.Run(c.sufixo, func(t *testing.T) {
			c.preparar(t)
			rel := novoRelogioFalso()
			usarProgressoFalso(t, rel)
			prepararUpgrade(t, rel, "v5.6.0", "v5.6.1", false)
			b := saidaDoComando(t, upgradeCmd)
			require.NoError(t, upgradeCmd.RunE(upgradeCmd, []string{}))
			conferirGolden(t, "upgrade_"+c.sufixo, b.String())
			conferirSemEstiloAntigo(t, b.String())
		})
	}
}

func TestUpgradeGolden_CheckEEmDia(t *testing.T) {
	usarSaidaTexto(t, false)
	rel := novoRelogioFalso()
	usarProgressoFalso(t, rel)

	prepararUpgrade(t, rel, "v5.6.0", "v5.6.1", false)
	require.NoError(t, upgradeCmd.Flags().Set("check", "true"))
	b := saidaDoComando(t, upgradeCmd)
	require.NoError(t, upgradeCmd.RunE(upgradeCmd, []string{}))
	conferirGolden(t, "upgrade_check", b.String())

	// o yby upgrade comum, sem --check, numa versão já em dia
	prepararUpgrade(t, rel, "v5.6.1", "v5.6.1", false)
	require.NoError(t, upgradeCmd.Flags().Set("check", "false"))
	b = saidaDoComando(t, upgradeCmd)
	require.NoError(t, upgradeCmd.RunE(upgradeCmd, []string{}))
	conferirGolden(t, "upgrade_em_dia", b.String())
}

func TestUpgradeGolden_ChecksumInvalido(t *testing.T) {
	usarSaidaTexto(t, false)
	rel := novoRelogioFalso()
	usarProgressoFalso(t, rel)
	prepararUpgrade(t, rel, "v5.6.0", "v5.6.1", true)
	b := saidaDoComando(t, upgradeCmd)
	err := upgradeCmd.RunE(upgradeCmd, []string{})
	require.Error(t, err)
	saida := comErro(t, b.String(), err)
	// as somas dependem do gzip da versão do Go: o golden não as guarda
	saida = regexp.MustCompile(`[0-9a-f]{64}`).ReplaceAllString(saida, "<sha256>")
	conferirGolden(t, "upgrade_falha_checksum", saida)
	conferirSemEstiloAntigo(t, saida)
}
