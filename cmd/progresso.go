package cmd

import (
	"bytes"
	"context"
	stdErr "errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/casheiro/yby-cli/v5/pkg/errors"
	"github.com/casheiro/yby-cli/v5/pkg/logger"
	"github.com/casheiro/yby-cli/v5/pkg/services/shared"
	"github.com/casheiro/yby-cli/v5/pkg/ui"
)

// novoProgresso cria as etapas de um comando longo (up, bootstrap, destroy, upgrade);
// substituível em testes para fixar o relógio e a detecção de terminal.
var novoProgresso = func(w io.Writer, rotulos ...string) *ui.Progresso {
	return ui.NovoProgresso(w, rotulos...)
}

// intervaloAnimacao é de quanto em quanto tempo a etapa atual é redesenhada no terminal,
// para o tempo correr enquanto um comando externo trabalha.
var intervaloAnimacao = time.Second

// saidaDoLog é para onde o log vai fora do desvio das etapas (os.Stderr; substituível em
// testes).
var saidaDoLog io.Writer = os.Stderr

// animar redesenha a etapa atual de prog no terminal a cada intervaloAnimacao e, enquanto
// isso, passa o log pelo Progresso: uma linha de log (um aviso, ou o --log-level=debug)
// sai acima do trecho redesenhado em vez de ser apagada pelo tique seguinte e deixar uma
// cópia velha da etapa na tela. A função devolvida para os dois e pode ser chamada mais
// de uma vez.
func animar(prog *ui.Progresso) (parar func()) {
	pararAnimacao := prog.Animar(intervaloAnimacao)
	restaurar := func() {}
	if prog.Terminal() {
		restaurar = logger.Desviar(prog.Escritor(saidaDoLog))
	}
	var uma sync.Once
	return func() {
		uma.Do(func() {
			pararAnimacao()
			restaurar()
		})
	}
}

// runnerQuieto guarda a saída dos comandos externos (k3d, helm, kubectl, git) em vez de
// mandá-la ao terminal, onde ela atropelaria as etapas redesenhadas no lugar. A saída vai
// para o log em debug e, na falha, as últimas linhas entram no erro.
type runnerQuieto struct{ shared.Runner }

func (r runnerQuieto) Run(ctx context.Context, name string, args ...string) error {
	saida, err := r.Runner.RunCombinedOutput(ctx, name, args...)
	return depoisDoComando(name, saida, err)
}

func (r runnerQuieto) RunStdin(ctx context.Context, stdin string, name string, args ...string) error {
	saida, err := r.Runner.RunStdinOutput(ctx, stdin, name, args...)
	// com Output, o stderr do comando vem no ExitError
	var ee *exec.ExitError
	if stdErr.As(err, &ee) {
		saida = append(saida, ee.Stderr...)
	}
	return depoisDoComando(name, saida, err)
}

// linhasNoErro é quantas linhas finais da saída de um comando que falhou entram no erro.
const linhasNoErro = 3

func depoisDoComando(nome string, saida []byte, err error) error {
	slog.Debug("saída de comando externo", "comando", nome, "saida", string(saida))
	if err == nil {
		return nil
	}
	var linhas []string
	for _, l := range strings.Split(strings.ReplaceAll(string(saida), "\r\n", "\n"), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			linhas = append(linhas, l)
		}
	}
	return comUltimasLinhas(err, linhas)
}

// comUltimasLinhas junta ao erro de um comando as últimas linhasNoErro linhas da saída.
func comUltimasLinhas(err error, linhas []string) error {
	if len(linhas) == 0 {
		return err
	}
	if len(linhas) > linhasNoErro {
		linhas = linhas[len(linhas)-linhasNoErro:]
	}
	return fmt.Errorf("%w: %s", err, strings.Join(linhas, " · "))
}

// atrasoAviso é quanto uma linha do stderr espera, com o comando ainda rodando, antes de
// runnerAoVivo mostrá-la.
const atrasoAviso = 3 * time.Second

// runnerAoVivo roda Run e RunStdin sem mandar a saída direto ao terminal e sem guardá-la
// inteira: cada linha vai para o log em debug na hora, e só as últimas entram no erro.
// Serve aos comandos que falam com clusters remotos (bootstrap cluster) e aos que rodam
// por muito tempo (o kubectl port-forward do access).
//
// Com aviso, uma linha do stderr que segue sem resposta por atraso com o comando ainda
// rodando vai para aviso: é o pedido de login de um plugin exec do kubeconfig (o código
// do kubelogin no AKS, por exemplo), que sem isso sumiria e deixaria a etapa girando sem
// instrução. As linhas logo antes do fim do comando (o "NotFound" de uma checagem, o
// motivo de uma falha) não são mostradas: vão para o erro.
type runnerAoVivo struct {
	shared.Runner // RunCombinedOutput, RunStdinOutput e LookPath
	aviso         func(linha string)
	atraso        time.Duration
	// executar roda o comando; nil usa os/exec (substituível em testes).
	executar func(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, name string, args ...string) error
}

func (r runnerAoVivo) Run(ctx context.Context, name string, args ...string) error {
	return r.rodar(ctx, nil, name, args)
}

func (r runnerAoVivo) RunStdin(ctx context.Context, stdin string, name string, args ...string) error {
	return r.rodar(ctx, strings.NewReader(stdin), name, args)
}

func (r runnerAoVivo) rodar(ctx context.Context, stdin io.Reader, name string, args []string) error {
	executar := r.executar
	if executar == nil {
		executar = executarComando
	}
	cauda := &caudaDeLinhas{}
	var avisos *avisosAtrasados
	if r.aviso != nil {
		avisos = novosAvisosAtrasados(r.aviso, r.atraso)
	}
	stdout := &porLinha{fn: func(l string) {
		slog.Debug("saída de comando externo", "comando", name, "linha", l)
		cauda.add(l)
	}}
	stderr := &porLinha{fn: func(l string) {
		slog.Debug("saída de comando externo", "comando", name, "stderr", l)
		cauda.add(l)
		if avisos != nil {
			avisos.add(l)
		}
	}}
	err := executar(ctx, stdin, stdout, stderr, name, args...)
	stdout.fechar()
	stderr.fechar()
	if avisos != nil {
		avisos.parar()
	}
	if err == nil {
		return nil
	}
	return comUltimasLinhas(err, cauda.linhas())
}

// executarComando roda name com os/exec, ligando a entrada e as saídas dadas.
func executarComando(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, name string, args ...string) error {
	c := exec.CommandContext(ctx, name, args...)
	c.Stdin, c.Stdout, c.Stderr = stdin, stdout, stderr
	// um filho que herdou as saídas não segura o fim do comando
	c.WaitDelay = 2 * time.Second
	return c.Run()
}

// porLinha chama fn a cada linha completa escrita, sem o "\n" e sem espaços nas pontas;
// linhas vazias são puladas. É seguro para escritas concorrentes.
type porLinha struct {
	mu    sync.Mutex
	resto []byte
	fn    func(string)
}

func (p *porLinha) Write(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.resto = append(p.resto, b...)
	for {
		i := bytes.IndexByte(p.resto, '\n')
		if i < 0 {
			return len(b), nil
		}
		p.emitir(string(p.resto[:i]))
		p.resto = p.resto[i+1:]
	}
}

// fechar entrega a última linha, sem "\n".
func (p *porLinha) fechar() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.emitir(string(p.resto))
	p.resto = nil
}

func (p *porLinha) emitir(l string) {
	if l = strings.TrimSpace(l); l != "" {
		p.fn(l)
	}
}

// caudaDeLinhas guarda só as últimas linhasNoErro linhas.
type caudaDeLinhas struct {
	mu sync.Mutex
	ls []string
}

func (c *caudaDeLinhas) add(l string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ls = append(c.ls, l)
	if len(c.ls) > linhasNoErro {
		c.ls = c.ls[len(c.ls)-linhasNoErro:]
	}
}

func (c *caudaDeLinhas) linhas() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.ls...)
}

// avisosAtrasados entrega a aviso, em ordem, cada linha que fica atraso sem o comando
// terminar; ao parar, as que ainda esperam são descartadas.
type avisosAtrasados struct {
	fila   chan linhaComHora
	fim    chan struct{}
	pronto chan struct{}
}

type linhaComHora struct {
	texto string
	hora  time.Time
}

// filaDeAvisos é quantas linhas podem esperar; além disso, as novas são descartadas para
// não travar a saída do comando.
const filaDeAvisos = 64

func novosAvisosAtrasados(aviso func(string), atraso time.Duration) *avisosAtrasados {
	a := &avisosAtrasados{
		fila:   make(chan linhaComHora, filaDeAvisos),
		fim:    make(chan struct{}),
		pronto: make(chan struct{}),
	}
	go func() {
		defer close(a.pronto)
		for {
			select {
			case <-a.fim:
				return
			case l := <-a.fila:
				espera := time.NewTimer(time.Until(l.hora.Add(atraso)))
				select {
				case <-a.fim:
					espera.Stop()
					return
				case <-espera.C:
					aviso(l.texto)
				}
			}
		}
	}()
	return a
}

func (a *avisosAtrasados) add(l string) {
	select {
	case a.fila <- linhaComHora{texto: l, hora: time.Now()}:
	default:
	}
}

func (a *avisosAtrasados) parar() {
	close(a.fim)
	<-a.pronto
}

// dicaDe devolve o hint do primeiro YbyError na cadeia de err, ou padrao quando não há.
func dicaDe(err error, padrao string) string {
	var ye *errors.YbyError
	for e := err; stdErr.As(e, &ye); e = ye.Cause {
		if ye.Hint != "" {
			return ye.Hint
		}
	}
	return padrao
}
