package ui

import (
	"bytes"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/muesli/termenv"
)

// relogioDePasso devolve um relógio que avança passo a cada leitura, a partir de um
// instante fixo, para as durações saírem iguais em todo teste.
func relogioDePasso(passo time.Duration) func() time.Time {
	var mu sync.Mutex
	t := time.Date(2026, 10, 24, 9, 30, 0, 0, time.UTC)
	return func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		t = t.Add(passo)
		return t
	}
}

func progressoDeTeste(w io.Writer, tty bool, rotulos ...string) *Progresso {
	return NovoProgressoCom(w, OpcoesProgresso{
		Terminal: func(io.Writer) bool { return tty },
		Relogio:  relogioDePasso(2 * time.Second),
		Largura:  64,
	}, rotulos...)
}

func TestProgressoForaDoTerminal(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	var b bytes.Buffer
	p := progressoDeTeste(&b, false, "cluster yby-local", "argo cd", "root-app")
	if p.Terminal() {
		t.Fatal("progresso de teste deveria estar fora do terminal")
	}

	p.Iniciar("cluster yby-local")
	if b.Len() != 0 {
		t.Fatalf("fora do terminal, a etapa só sai quando termina; saiu %q", b.String())
	}
	p.Concluir("criado")
	p.Iniciar("argo cd")
	p.Avancar(0.5)
	p.Tique()
	p.Falhar("helm falhou")
	p.Detalhe("Error: timed out\n")

	saida := b.String()
	if strings.Contains(saida, "\x1b") {
		t.Errorf("fora do terminal não pode haver códigos de controle: %q", saida)
	}
	conferirGolden(t, "progresso_fora_do_terminal", saida)
	if strings.Count(saida, "argo cd") != 1 || strings.Contains(saida, "root-app") {
		t.Errorf("cada etapa sai uma vez e as pendentes não saem:\n%s", saida)
	}
}

func TestProgressoNoTerminalRedesenha(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	var b bytes.Buffer
	p := progressoDeTeste(&b, true, "cluster yby-local", "argo cd", "root-app")

	p.Iniciar("cluster yby-local")
	primeiro := b.String()
	if strings.Contains(primeiro, "\x1b[") {
		t.Errorf("o primeiro desenho não sobe linhas: %q", primeiro)
	}
	if !strings.Contains(primeiro, "root-app") {
		t.Errorf("no terminal, as pendentes aparecem: %q", primeiro)
	}
	p.Concluir("criado")
	p.Iniciar("argo cd")
	p.Avancar(0.64)

	// a cada mudança, sobe as linhas vivas (3, depois 2) e limpa antes de redesenhar
	saida := b.String()
	for _, c := range []string{"\x1b[3A\r\x1b[J", "\x1b[2A\r\x1b[J"} {
		if !strings.Contains(saida, c) {
			t.Errorf("faltou o código %q em %q", c, saida)
		}
	}
	conferirGolden(t, "progresso_quadro", p.Quadro())

	p.Concluir("")
	p.Iniciar("root-app")
	p.Detalhe("aplicado")
	p.Concluir("aplicado")
	etapas := p.Etapas()
	if len(etapas) != 3 {
		t.Fatalf("esperava 3 etapas, veio %d", len(etapas))
	}
	for _, e := range etapas {
		if e.Estado != EtapaFeita {
			t.Errorf("etapa %q deveria estar feita: %v", e.Rotulo, e.Estado)
		}
	}
}

func TestProgressoIniciarForaDoPlano(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	var b bytes.Buffer
	p := progressoDeTeste(&b, false, "a", "b")

	// "b" começa antes de "a": vai para o topo das não terminadas
	p.Iniciar("b")
	// iniciar outra conclui a atual sem nota
	p.Iniciar("extra")
	p.Concluir("")

	etapas := p.Etapas()
	rotulos := make([]string, len(etapas))
	for i, e := range etapas {
		rotulos[i] = e.Rotulo
	}
	if got := strings.Join(rotulos, ","); got != "b,extra,a" {
		t.Errorf("ordem das etapas = %s; esperava b,extra,a", got)
	}
	if etapas[0].Estado != EtapaFeita || etapas[0].Duracao != 2*time.Second {
		t.Errorf("b deveria estar feita em 2 s: %+v", etapas[0])
	}
	if etapas[2].Estado != EtapaPendente {
		t.Errorf("a deveria seguir pendente: %+v", etapas[2])
	}
}

func TestProgressoSemEtapaAtual(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	var b bytes.Buffer
	p := progressoDeTeste(&b, true)
	// sem etapa em andamento, nada acontece
	p.Avancar(0.3)
	p.Tique()
	p.Concluir("x")
	p.Falhar("x")
	p.Detalhe("")
	if b.Len() != 0 {
		t.Errorf("sem etapa, nada deveria sair: %q", b.String())
	}
	// fração fora do intervalo é limitada
	p.Iniciar("x")
	p.Avancar(3)
	if f := p.Etapas()[0].Fracao; f != 1 {
		t.Errorf("fração limitada a 1, veio %v", f)
	}
}

func TestProgressoDuracaoDaAtual(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	p := progressoDeTeste(io.Discard, false, "espera")
	p.Iniciar("espera")
	// cada leitura do relógio avança 2 s: a cópia vê 2 s de espera
	if d := p.Etapas()[0].Duracao; d != 2*time.Second {
		t.Errorf("duração da atual = %v; esperava 2 s", d)
	}
}

func TestNovoProgressoPadrao(t *testing.T) {
	var b bytes.Buffer
	p := NovoProgresso(&b, "a")
	if p.Terminal() {
		t.Error("um buffer não é terminal")
	}
	if p.largura != msgLargura {
		t.Errorf("largura padrão = %d; esperava %d", p.largura, msgLargura)
	}
}

// escritorSeguro protege um buffer para o teste do Animar, que escreve de outra goroutine.
type escritorSeguro struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (e *escritorSeguro) Write(p []byte) (int, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.b.Write(p)
}

func (e *escritorSeguro) String() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.b.String()
}

func TestProgressoAnimar(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)

	// fora do terminal, Animar não faz nada
	foraW := &escritorSeguro{}
	fora := progressoDeTeste(foraW, false, "a")
	fora.Iniciar("a")
	fora.Animar(time.Millisecond)()

	w := &escritorSeguro{}
	p := progressoDeTeste(w, true, "a")
	p.Iniciar("a")
	parar := p.Animar(time.Millisecond)
	limite := time.Now().Add(2 * time.Second)
	for strings.Count(w.String(), "\x1b[1A") < 2 && time.Now().Before(limite) {
		time.Sleep(time.Millisecond)
	}
	parar()
	parar() // parar duas vezes é seguro
	if strings.Count(w.String(), "\x1b[1A") < 2 {
		t.Errorf("Animar deveria redesenhar a etapa atual: %q", w.String())
	}
	if foraW.String() != "" {
		t.Errorf("fora do terminal, Animar não escreve: %q", foraW.String())
	}
}

func TestProgressoEscritorNoTerminal(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	var b bytes.Buffer
	p := progressoDeTeste(&b, true, "kubeconfig")
	p.Iniciar("kubeconfig")
	b.Reset()

	// uma linha de log no meio da etapa: o trecho vivo sai, a linha entra e ele volta
	log := p.Escritor(&b)
	if n, err := log.Write([]byte("level=WARN msg=\"TLS verify ")); err != nil || n != 27 {
		t.Fatalf("Write parcial = %d, %v", n, err)
	}
	if b.Len() != 0 {
		t.Fatalf("o resto sem \\n espera a linha completa; saiu %q", b.String())
	}
	_, _ = log.Write([]byte("desabilitado\"\n"))
	p.Tique()

	saida := b.String()
	esperado := "\x1b[1A\r\x1b[J" + "level=WARN msg=\"TLS verify desabilitado\"\n"
	if !strings.HasPrefix(saida, esperado) {
		t.Errorf("a linha de log deveria apagar o trecho vivo antes de sair:\n%q", saida)
	}
	// depois do log, o trecho foi redesenhado abaixo: o tique sobe só a linha da etapa,
	// e a linha de log continua na tela
	resto := strings.TrimPrefix(saida, esperado)
	if strings.Count(resto, "\x1b[1A") != 1 || strings.Count(saida, "TLS verify") != 1 {
		t.Errorf("o redesenho não pode apagar a linha de log:\n%q", saida)
	}
	if strings.Count(resto, "kubeconfig") != 2 {
		t.Errorf("a etapa deveria voltar abaixo do log e no tique:\n%q", resto)
	}
}

func TestProgressoEscritorForaDoTerminal(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	var etapas, log bytes.Buffer
	p := progressoDeTeste(&etapas, false, "a")
	p.Iniciar("a")
	_, _ = p.Escritor(&log).Write([]byte("parcial"))
	if log.String() != "parcial" || etapas.Len() != 0 {
		t.Errorf("fora do terminal o texto vai direto ao destino: log=%q etapas=%q", log.String(), etapas.String())
	}
}

func TestProgressoAvisar(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	var b bytes.Buffer
	p := progressoDeTeste(&b, false, "convergência")
	p.Iniciar("convergência")
	p.Concluir("tempo esgotado")
	p.Avisar("o Argo CD ainda não convergiu · confira com yby status")
	p.Avisar("  ")
	p.Avisar(strings.Repeat("x", 80))

	linhas := strings.Split(strings.TrimRight(b.String(), "\n"), "\n")
	if len(linhas) != 3 {
		t.Fatalf("esperava a etapa e dois avisos:\n%s", b.String())
	}
	if !strings.HasPrefix(linhas[0], "  ✓ convergência") {
		t.Errorf("a etapa sai antes do aviso: %q", linhas[0])
	}
	if linhas[1] != "  ▲ o Argo CD ainda não convergiu · confira com yby status" {
		t.Errorf("aviso = %q", linhas[1])
	}
	if w := len([]rune(linhas[2])); w != 64 || !strings.HasSuffix(linhas[2], "…") {
		t.Errorf("o aviso longo é cortado na largura (64): %q (%d)", linhas[2], w)
	}
}

func TestProgressoPausar(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	var b bytes.Buffer
	p := progressoDeTeste(&b, true, "kubeconfig", "fim")
	p.Iniciar("kubeconfig")
	retomar := p.Pausar()
	b.Reset()

	// em pausa, nem o tique nem o detalhe trazem o trecho vivo de volta
	p.Tique()
	p.Detalhe("lendo com sudo")
	if strings.Contains(b.String(), "kubeconfig") || strings.Contains(b.String(), "\x1b[") {
		t.Errorf("em pausa o trecho vivo fica fora da tela: %q", b.String())
	}
	retomar()
	retomar() // retomar duas vezes é seguro
	if strings.Count(b.String(), "kubeconfig") != 1 || !strings.Contains(b.String(), "fim") {
		t.Errorf("ao retomar, o trecho vivo volta uma vez: %q", b.String())
	}
}

func TestProgressoFalhaNoTerminalTiraAsPendentes(t *testing.T) {
	usarTema(t, true, termenv.Ascii, true)
	var b bytes.Buffer
	p := progressoDeTeste(&b, true, "cluster", "git server", "argo cd")
	p.Iniciar("cluster")
	p.Concluir("criado")
	p.Iniciar("git server")
	p.Falhar("")

	// depois da falha, o trecho redesenhado só tem o que já terminou
	depois := b.String()[strings.LastIndex(b.String(), "\x1b[J")+len("\x1b[J"):]
	if strings.Contains(depois, "argo cd") {
		t.Errorf("a pendente continuou na tela depois da falha: %q", depois)
	}
	if !strings.Contains(depois, "x git server") {
		t.Errorf("a etapa que falhou deveria ficar: %q", depois)
	}

	// se o comando seguir, as pendentes voltam
	p.Iniciar("argo cd")
	if !strings.HasSuffix(strings.TrimRight(b.String(), "\n"), "argo cd") {
		t.Errorf("a próxima etapa deveria aparecer: %q", b.String())
	}
}
