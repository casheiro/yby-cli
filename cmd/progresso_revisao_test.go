package cmd

import (
	"bytes"
	"context"
	stdErr "errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/pkg/executor"
	"github.com/casheiro/yby-cli/pkg/logger"
	"github.com/casheiro/yby-cli/pkg/services/network"
	"github.com/casheiro/yby-cli/pkg/services/shared"
	"github.com/casheiro/yby-cli/pkg/ui"
)

// Testes dos ajustes da revisão da T027: runner ao vivo, log durante o redesenho,
// memória do bootstrap vps, dicas do upgrade e adaptadores do access.

// avisosGravados guarda as linhas recebidas pelo aviso do runnerAoVivo.
type avisosGravados struct {
	mu     sync.Mutex
	linhas []string
}

func (a *avisosGravados) add(l string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.linhas = append(a.linhas, l)
}

func (a *avisosGravados) todas() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]string(nil), a.linhas...)
}

func TestRunnerAoVivo_PedidoDeLoginApareceDuranteOComando(t *testing.T) {
	avisos := &avisosGravados{}
	r := runnerAoVivo{
		aviso:  avisos.add,
		atraso: 10 * time.Millisecond,
		executar: func(_ context.Context, _ io.Reader, stdout, stderr io.Writer, name string, _ ...string) error {
			// o kubelogin do AKS pede o login no stderr e espera a pessoa
			fmt.Fprintln(stderr, "To sign in, use a web browser to open https://microsoft.com/devicelogin and enter the code ABC123 to authenticate.")
			time.Sleep(80 * time.Millisecond)
			fmt.Fprint(stdout, "Release \"argocd\" has been upgraded.\n")
			return nil
		},
	}
	require.NoError(t, r.Run(context.Background(), "helm", "upgrade", "--install"))
	assert.Equal(t, []string{"To sign in, use a web browser to open https://microsoft.com/devicelogin and enter the code ABC123 to authenticate."},
		avisos.todas(), "o pedido de login aparece enquanto o comando espera; o stdout não")
}

func TestRunnerAoVivo_ErroNoFimNaoViraAviso(t *testing.T) {
	avisos := &avisosGravados{}
	r := runnerAoVivo{
		aviso:  avisos.add,
		atraso: time.Second,
		executar: func(_ context.Context, stdin io.Reader, _, stderr io.Writer, _ string, _ ...string) error {
			if stdin != nil {
				entrada, _ := io.ReadAll(stdin)
				fmt.Fprintf(stderr, "lido: %s\n", entrada)
			}
			fmt.Fprint(stderr, "linha 1\nlinha 2\nlinha 3\nError from server (NotFound): deployments.apps \"sealed-secrets\" not found")
			return stdErr.New("exit status 1")
		},
	}
	err := r.Run(context.Background(), "kubectl", "get", "deployment")
	require.Error(t, err)
	assert.Empty(t, avisos.todas(), "a linha logo antes do fim do comando não é mostrada")
	assert.Equal(t, `exit status 1: linha 2 · linha 3 · Error from server (NotFound): deployments.apps "sealed-secrets" not found`, err.Error())

	err = r.RunStdin(context.Background(), "manifesto", "kubectl", "apply", "-f", "-")
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "lido: manifesto", "só as 3 últimas linhas entram no erro")
}

func TestRunnerAoVivo_SemAvisoMandaTudoAoLog(t *testing.T) {
	var log bytes.Buffer
	anterior := slog.Default()
	t.Cleanup(func() { slog.SetDefault(anterior) })
	slog.SetDefault(slog.New(slog.NewTextHandler(&log, &slog.HandlerOptions{Level: slog.LevelDebug})))

	r := runnerAoVivo{executar: func(_ context.Context, _ io.Reader, stdout, stderr io.Writer, _ string, _ ...string) error {
		fmt.Fprint(stdout, "Forwarding from 127.0.0.1:8085 -> 8080\n\n")
		fmt.Fprint(stdout, "Handling connection for 8085")
		fmt.Fprint(stderr, "E1024 portforward.go:413] lost connection\n")
		return nil
	}}
	require.NoError(t, r.Run(context.Background(), "kubectl", "port-forward"))
	for _, l := range []string{"Forwarding from 127.0.0.1:8085", "Handling connection for 8085", "lost connection"} {
		assert.Contains(t, log.String(), l)
	}
}

func TestRunnerAoVivo_ComandoDeVerdade(t *testing.T) {
	// sem executar, roda pelo os/exec: um comando inexistente devolve o erro do exec
	err := runnerAoVivo{}.Run(context.Background(), "yby-comando-que-nao-existe")
	assert.Error(t, err)
}

func TestPorLinha_LinhaPartida(t *testing.T) {
	var linhas []string
	p := &porLinha{fn: func(l string) { linhas = append(linhas, l) }}
	_, _ = p.Write([]byte("meia "))
	_, _ = p.Write([]byte("linha\r\noutra\n  \nfim"))
	p.fechar()
	assert.Equal(t, []string{"meia linha", "outra", "fim"}, linhas)
}

func TestAvisosAtrasados_FilaCheiaDescarta(t *testing.T) {
	bloqueio := make(chan struct{})
	a := novosAvisosAtrasados(func(string) { <-bloqueio }, 0)
	for i := 0; i < filaDeAvisos+10; i++ {
		a.add("linha") // não trava mesmo com a fila cheia
	}
	close(bloqueio)
	a.parar()
}

func TestAnimar_LogPassaPelasEtapasNoTerminal(t *testing.T) {
	usarSaidaTexto(t, true)
	anteriorLog, anteriorSaida, anteriorIntervalo := slog.Default(), saidaDoLog, intervaloAnimacao
	t.Cleanup(func() {
		slog.SetDefault(anteriorLog)
		saidaDoLog, intervaloAnimacao = anteriorSaida, anteriorIntervalo
	})
	logger.InitGlobal(logger.Config{Level: "info", Format: "text"})
	var tela bytes.Buffer
	saidaDoLog = &tela
	intervaloAnimacao = time.Hour // o tique não entra no teste

	prog := ui.NovoProgressoCom(&tela, ui.OpcoesProgresso{Terminal: func(io.Writer) bool { return true }, Largura: 64}, "kubeconfig")
	prog.Iniciar("kubeconfig")
	parar := animar(prog)
	slog.Warn("TLS verify desabilitado")
	prog.Concluir("")
	parar()
	parar()

	saida := tela.String()
	// o aviso entra depois de o trecho vivo sair, e a etapa volta logo abaixo dele
	assert.Contains(t, saida, "\x1b[1A\r\x1b[Jtime=")
	i := strings.Index(saida, "TLS verify desabilitado")
	require.True(t, i > 0, "%q", saida)
	assert.Contains(t, saida[i:], "+ kubeconfig")
	assert.Equal(t, 1, strings.Count(saida, "TLS verify desabilitado"))
}

func TestAnimar_ForaDoTerminalNaoDesvia(t *testing.T) {
	usarSaidaTexto(t, true)
	anteriorSaida := saidaDoLog
	t.Cleanup(func() { saidaDoLog = anteriorSaida })
	var desviado bytes.Buffer
	saidaDoLog = &desviado
	prog := ui.NovoProgressoCom(io.Discard, ui.OpcoesProgresso{Terminal: func(io.Writer) bool { return false }}, "a")
	animar(prog)()
	assert.Empty(t, desviado.String())
}

// executorDeMemoria é um executor.Executor que só responde ao /proc/meminfo.
type executorDeMemoria struct {
	executor.Executor
	meminfo string
	err     error
}

func (e executorDeMemoria) FetchFile(string) ([]byte, error) { return []byte(e.meminfo), e.err }

func TestConferirMemoria(t *testing.T) {
	usarSaidaTexto(t, true)
	anterior := esperaMemoriaBaixa
	t.Cleanup(func() { esperaMemoriaBaixa = anterior })
	esperaMemoriaBaixa = 0

	casos := []struct {
		nome, meminfo, quer string
		err                 error
	}{
		{"ok", "MemTotal:        8000000 kB\nMemFree: 1 kB\n", "  v requisitos mínimos   7812 MB de memória", nil},
		{"baixa", "MemTotal:        2000000 kB\n", "  v requisitos mínimos   1953 MB de memória\n  ! abaixo dos 4 GB da stack · segue em 0,0 s, ctrl+c cancela", nil},
		{"ilegível", "sem o campo\n", "  v requisitos mínimos   memória não identificada", nil},
		{"falha", "", "  x requisitos mínimos", fmt.Errorf("permission denied")},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			var b bytes.Buffer
			prog := ui.NovoProgressoCom(&b, ui.OpcoesProgresso{Terminal: func(io.Writer) bool { return false }, Largura: 64}, "requisitos mínimos")
			err := conferirMemoria(executorDeMemoria{meminfo: c.meminfo, err: c.err}, prog)
			assert.Equal(t, c.err != nil, err != nil)
			linhas := strings.Split(strings.TrimRight(b.String(), "\n"), "\n")
			for i := range linhas {
				linhas[i] = strings.TrimRight(strings.TrimSuffix(linhas[i], "0,0 s"), " ")
			}
			assert.Equal(t, c.quer, strings.Join(linhas, "\n"))
		})
	}
}

func TestUpgrade_DicaDo404(t *testing.T) {
	usarSaidaTexto(t, true)
	rel := novoRelogioFalso()
	prepararUpgrade(t, rel, "v5.6.0", "v5.6.1", false)
	httpGet = func(string) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader("{}"))}, nil
	}

	dica := func() string {
		saidaDoComando(t, upgradeCmd)
		err := upgradeCmd.RunE(upgradeCmd, []string{})
		var ye *errors.YbyError
		require.True(t, stdErr.As(err, &ye), "%v", err)
		return ye.Hint
	}
	assert.Equal(t, "nenhum release publicado foi encontrado; veja github.com/casheiro/yby-cli/releases", dica(),
		"sem --version, a dica não fala da tag")
	require.NoError(t, upgradeCmd.Flags().Set("version", "v9.9.9"))
	assert.Equal(t, "o release v9.9.9 não foi encontrado; confira a tag em --version", dica())
}

func TestAccess_AdaptadoresNaoEscrevemNoTerminal(t *testing.T) {
	rede, ok := newNetworkAdapter().(*network.RealClusterNetworkManager)
	require.True(t, ok)
	_, ok = rede.Runner.(runnerAoVivo)
	assert.True(t, ok, "o kubectl do access roda pelo runnerAoVivo, sem tocar no terminal")

	docker, ok := newContainerAdapter().(*network.DockerContainerManager)
	require.True(t, ok)
	_, ok = docker.Runner.(runnerAoVivo)
	assert.True(t, ok, "o docker do access roda pelo runnerAoVivo")
}

// garante que runnerAoVivo continua um shared.Runner
var _ shared.Runner = runnerAoVivo{}
